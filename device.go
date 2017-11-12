package main

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Device struct {
	bt       *ByteTranslator // point to parent
	lock     sync.Mutex
	deviceid string
	rxm      *ByteMarshaller
	txm      *ByteMarshaller
	rxfields []string
	txfields []string

	// delay period
	txdelay time.Duration

	// time scheduled to send
	txbuffer map[int][]byte
}

func NewDevice(bt *ByteTranslator, deviceid string) *Device {
	d := new(Device)
	d.bt = bt
	d.deviceid = deviceid
	/* Initialize TX Values Buffer */
	d.txBufferReset()
	return d
}

func (d *Device) txBufferReset() {
	d.txbuffer = make(map[int][]byte)
}

func (d *Device) txBufferLen() int {
	return len(d.txbuffer)
}

func (d *Device) txBufferMarshal() []byte {
	if len(d.txbuffer) == 0 {
		return nil
	}
	return d.txm.MarshalValues(d.txbuffer)
}

func (d *Device) txBufferAdd(value interface{}, findex int) error {
	logitem := d.bt.log.WithField("deviceid", d.deviceid)
	bytes, err := d.txm.MarshalValue(value, findex)
	d.txbuffer[findex] = bytes
	logitem.Debugf("Queued field index %d with %v", findex, bytes)
	return err
}

func (d *Device) TxBufferConsume() []byte {
	d.lock.Lock()
	defer d.lock.Unlock()
	bytes := d.txBufferMarshal()
	d.txBufferReset()
	return bytes
}

func (d *Device) TxBufferReset() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.txBufferReset()
}

func (d *Device) PublishOutgoingQueueLength(length int) {
	logitem := d.bt.log.WithField("deviceid", d.deviceid)
	logitem.Debugf("Outgoing queue length is %d", length)
	if d.bt.pubOutgoingQueue {
		err := d.bt.mqtt.Publish(devicePrefix+d.deviceid+deviceSuffix+d.bt.outgoingQueueTopic, fmt.Sprint(length))
		if err != nil {
			logitem.Errorf("Failed to publish to device's %s topic: %v", d.bt.outgoingQueueTopic, err)
			return
		}
		logitem.Debugf("Published to %s", d.bt.outgoingQueueTopic)
	}
}

func (d *Device) TXQueueSend() {
	logitem := d.bt.log.WithField("deviceid", d.deviceid)
	logitem.Debug("Sending TX")
	bytes := d.TxBufferConsume()
	d.PublishOutgoingQueueLength(0)
	if bytes == nil {
		logitem.Debug("Sending TX: Buffer was already reset")
		return
	}
	b64 := base64.StdEncoding.EncodeToString(bytes)
	err := d.bt.mqtt.Publish(devicePrefix+d.deviceid+deviceSuffix+"rawtx", b64)
	if err != nil {
		logitem.Error("Failed to publish to device's rawtx topic: ", err)
		return
	}
	logitem.Debug("Published to rawtx")
}

func (d *Device) TXQueuePost(payload string, findex int) error {
	logitem := d.bt.log.WithField("deviceid", d.deviceid)
	d.lock.Lock()
	defer d.lock.Unlock()

	logitem.Debugf("Adding payload to tx queue")
	err := d.txBufferAdd(string(payload), int(findex))
	if d.txBufferLen() == 1 {
		logitem.Debugf("Scheduling tx for %v", d.txdelay)
		d.bt.sched.Add(d, d.txdelay)
	}
	d.PublishOutgoingQueueLength(d.txBufferLen())
	return err
}

func (d *Device) SchedulerAction() {
	logitem := d.bt.log.WithField("deviceid", d.deviceid)
	logitem.Debug("Received SchedulerAction")
	// Launch in independent go routine to avoid deadlocks with Unsubscribe
	go d.TXQueueSend()
}

func (d *Device) RawRx(payload string) {
	logitem := d.bt.log.WithField("deviceid", d.deviceid)
	binary, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		d.bt.log.Error("Failed decode base64 from rawrx", err)
		return
	}
	values, err := d.rxm.Unmarshal(binary)
	if err != nil {
		// had an error/warning while unmarshalling - publish error to /bytetranslator topic
		logitem.Warnf("Deviceid %s has trailing bytes", d.deviceid)
		err = d.bt.mqtt.Publish(devicePrefix+d.deviceid+deviceSuffix+"bytetranslator", fmt.Sprint(err))
		if err != nil {
			logitem.Error("Failed to publish to device's bytetranslator topic: ", err)
			return
		}
		// more of warnings -- so let's continue
	}
	for i, value := range values {
		if i < len(d.rxfields) {
			// publish to the named topic
			topic := devicePrefix + d.deviceid + deviceSuffix + d.rxfields[i]
			logitem.Debugf("Publishing %s to %s", fmt.Sprint(value), topic)
			err = d.bt.mqtt.Publish(topic, fmt.Sprint(value))
			if err != nil {
				logitem.Error("Failed to publish to device's \""+d.rxfields[i]+"\" topic: ", err)
				return
			}
		} else {
			// publish simply to the array index topic
			topic := devicePrefix + d.deviceid + deviceSuffix + fmt.Sprintf("%s%d", defaultIncomingPrefix, i)
			logitem.WithField("deviceid", d.deviceid).Debugf("Publishing %s to %s", fmt.Sprint(value), topic)
			err = d.bt.mqtt.Publish(topic, fmt.Sprint(value))
			if err != nil {
				logitem.Error("Failed to publish to device's \""+fmt.Sprint(i)+"\" topic: ", err)
				return
			}
		}
	}
}

func (d *Device) Subscribe() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.bt.log.WithField("deviceid", d.deviceid).Info("Subscribing to " + devicePrefix + d.deviceid + deviceSuffix + "+")

	err := d.bt.mqtt.Subscribe(
		devicePrefix+d.deviceid+deviceSuffix+"+",
		func(topic string, payload []byte) {
			subtopic := topic[strings.LastIndex(topic, "/")+1:]
			logitem := d.bt.log.WithField("deviceid", d.deviceid).WithField("subtopic", subtopic)

			if subtopic == "rawrx" {
				logitem.Debug("Received rawrx")
				d.RawRx(string(payload))
			} else if groups := unnamedOutgoingValPattern.FindStringSubmatch(subtopic); len(groups) == 2 {
				findex, err := strconv.ParseUint(groups[1], 10, unnamedValueOutIndexBitWidth)
				logitem.Debugf("Received unnamed tx value for index %d", findex)
				if err != nil {
					logitem.Errorf("Failed to parse field index of %s", subtopic)
					return
				}
				err = d.TXQueuePost(string(payload), int(findex))
				if err != nil {
					logitem.Error(err)
					d.bt.mqtt.Publish(devicePrefix+d.deviceid+deviceSuffix+"bytetranslator", fmt.Sprintf("%s: %v", subtopic, err))
				}
			} else {
				for findex, name := range d.txfields {
					if subtopic == name {
						logitem.Debugf("Received tx value %s", subtopic)
						err := d.TXQueuePost(string(payload), int(findex))
						if err != nil {
							logitem.Error(err)
							d.bt.mqtt.Publish(devicePrefix+d.deviceid+deviceSuffix+"bytetranslator", fmt.Sprintf("%s: %v", subtopic, err))
						}
						// Not having it return right here would actually
						// allow for encoding redundant fields. We will let it
						// happen.
					}
				}
			}
		})
	return err
}

func (d *Device) Unsubscribe() {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.bt.log.WithField("deviceid", d.deviceid).Info("Unsubscribing from " + devicePrefix + d.deviceid + deviceSuffix + "+")
	// remove mqtt subscriptions
	d.bt.mqtt.Unsubscribe(devicePrefix + d.deviceid + deviceSuffix + "+")

	if len(d.txbuffer) > 0 {
		// This could deadlock with the scheduler go routine calling
		// SchedulerAction after we have locked here, so we made the
		// SchedulerAction happen in it's own go routine
		d.bt.sched.Cancel(d)
	}

	d.txBufferReset()
}
