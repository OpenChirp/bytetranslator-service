package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openchirp/framework"

	"github.com/sirupsen/logrus"
)

const (
	defaultLittleEndian          = true
	defaultIncomingPrefix        = "UnnamedValueIn"
	defaultOutgoingPrefix        = "UnnamedValueOut"
	defaultTxDelay               = time.Duration(0)
	devicePrefix                 = "openchirp/devices/"
	deviceSuffix                 = "/" + framework.TransducerPrefix + "/"
	unnamedValueOutIndexBitWidth = 16
)

var ErrUnknownFieldType = errors.New("Could not parse FieldType")

// TODO: setup Device receiver functions to support min-heaps for scheduling
var unnamedOutgoingValPattern = regexp.MustCompile("^(?i:" + defaultOutgoingPrefix + ")(\\d+)$")

type MQTT interface {
	Subscribe(topic string, callback func(topic string, payload []byte)) error
	Unsubscribe(topics ...string) error
	Publish(topic string, payload interface{}) error
}

type Device struct {
	SchedulerData
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

func (d *Device) txBufferReset() {
	d.txbuffer = make(map[int][]byte)
}

func (d *Device) txBufferMarshal() []byte {
	if len(d.txbuffer) == 0 {
		return nil
	}
	return d.txm.MarshalValues(d.txbuffer)
}

func (d *Device) TxBufferReset() {
	d.txBufferReset()
}

func (d *Device) TxBufferAdd(value interface{}, findex int) (bool, error) {
	bytes, err := d.txm.MarshalValue(value, findex)
	fmt.Printf("Bytes: %v\n", bytes)
	d.txbuffer[findex] = bytes
	if len(d.txbuffer) == 1 {
		return true, err
	}
	return false, err
}

func (d *Device) TxBufferConsume() []byte {
	bytes := d.txBufferMarshal()
	d.txBufferReset()
	return bytes
}

type ByteTranslator struct {
	mqtt        MQTT
	log         *logrus.Logger
	devices     map[string]*Device
	defaultType FieldType
	sched       *Scheduler
}

func NewByteTranslator(mqtt MQTT, log *logrus.Logger, defaultType string) (*ByteTranslator, error) {
	if ftype := ParseFieldType(defaultType); ftype != FieldTypeUnknown {
		bt := &ByteTranslator{
			mqtt:        mqtt,
			log:         log,
			devices:     make(map[string]*Device),
			defaultType: ftype,
		}
		bt.sched = NewScheduler()
		bt.sched.Start()
		return bt, nil
	}
	return nil, ErrUnknownFieldType
}

// AddDevice adds a device to the bytetranslator manager.
// The first error is intended to be sent as status to the device, the second
// error is a runtime error
func (t *ByteTranslator) AddDevice(deviceid, rxnames, rxtypes, txnames, txtypes, endianness, delay string) (error, error) {
	var err error
	var isLittleEndian = defaultLittleEndian

	d := &Device{}

	d.deviceid = deviceid

	/* Parse all config parameters */
	// get delay
	d.txdelay = defaultTxDelay
	if len(delay) > 0 {
		d.txdelay, err = time.ParseDuration(delay)
		if err != nil {
			return fmt.Errorf("Failed to parse delay"), nil
		}
	}

	// get endianness
	if len(endianness) > 0 {
		switch strings.ToLower(endianness) {
		case "little":
			isLittleEndian = true
		case "big":
			isLittleEndian = false
		default:
			return fmt.Errorf("Failed to parse endianness"), nil
		}
	}

	// get rx field names
	if len(rxnames) > 0 {
		d.rxfields = strings.Split(rxnames, ",")
	}
	// remove possible space
	for i, name := range d.rxfields {
		d.rxfields[i] = strings.TrimSpace(name)
	}
	// get rx field types
	rxfieldtypes := make([]FieldType, 0)
	if len(rxtypes) > 0 {
		for _, rxf := range strings.Split(rxtypes, ",") {
			// remove possible space
			rxf = strings.TrimSpace(rxf)
			ft := ParseFieldType(rxf)
			if ft == FieldTypeUnknown {
				return fmt.Errorf("Failed to parse incoming field type \"%s\"", rxf), nil
			}
			rxfieldtypes = append(rxfieldtypes, ft)
		}
	}

	// get tx field names
	if len(rxnames) > 0 {
		d.txfields = strings.Split(txnames, ",")
	}
	// remove possible space
	for i, name := range d.txfields {
		d.txfields[i] = strings.TrimSpace(name)
	}
	// get tx field types
	txfieldtypes := make([]FieldType, 0)
	if len(txtypes) > 0 {
		for _, txf := range strings.Split(rxtypes, ",") {
			// remove possible space
			txf = strings.TrimSpace(txf)
			ft := ParseFieldType(txf)
			if ft == FieldTypeUnknown {
				return fmt.Errorf("Failed to parse outgoing field type \"%s\"", txf), nil
			}
			txfieldtypes = append(txfieldtypes, ft)
		}
	}

	/* Make byte marshalers based on configs */
	t.log.WithField("deviceid", deviceid).WithField("little_endian", isLittleEndian).Debug("Adding ByteMarshaller")
	d.rxm = NewByteMarshaller(rxfieldtypes, t.defaultType, isLittleEndian)
	d.txm = NewByteMarshaller(txfieldtypes, t.defaultType, isLittleEndian)

	/* Initialize TX Values Buffer */
	d.TxBufferReset()

	/* Add to table of devices */
	t.devices[deviceid] = d

	/* Setup subscriptions */
	d.lock.Lock()
	defer d.lock.Unlock()
	t.log.WithField("deviceid", deviceid).Debug("Subscribing to \"", devicePrefix+deviceid+deviceSuffix+"rawrx", "\"")
	err = t.mqtt.Subscribe(
		devicePrefix+deviceid+deviceSuffix+"+",
		func(topic string, payload []byte) {
			subtopic := topic[strings.LastIndex(topic, "/")+1:]
			logitem := t.log.WithField("deviceid", d.deviceid).WithField("subtopic", subtopic)
			logitem.Debug("Got something on ", topic)

			if subtopic == "rawrx" {
				logitem.Debug("Received rawrx")
				t.onRawRx(d, d.deviceid, payload)
			} else if groups := unnamedOutgoingValPattern.FindStringSubmatch(subtopic); len(groups) == 2 {
				findex, err := strconv.ParseUint(groups[1], 10, unnamedValueOutIndexBitWidth)
				logitem.Debugf("Received unnamed tx value on %s. Interpreted as field index %d", subtopic, findex)
				if err != nil {
					logitem.Errorf("Failed to parse field index of %s", subtopic)
					return
				}
				err = t.txQueue(d, string(payload), int(findex))
				if err != nil {
					logitem.Error(err)
					t.mqtt.Publish(devicePrefix+deviceid+deviceSuffix+"bytetranslator", fmt.Sprintf("%s: %v", subtopic, err))
				}
			} else {
				for findex, name := range d.txfields {
					if subtopic == name {
						logitem.Debug("Received tx value %s", subtopic)
						err := t.txQueue(d, string(payload), int(findex))
						if err != nil {
							logitem.Error(err)
							t.mqtt.Publish(devicePrefix+deviceid+deviceSuffix+"bytetranslator", fmt.Sprintf("%s: %v", subtopic, err))
						}
						// Not having it return right here would actually
						// allow for encoding redundant fields. We will let it
						// happen.
					}
				}
			}
		})

	return nil, err
}

func (t *ByteTranslator) RemoveDevice(deviceid string) {
	// remove device from devices table
	if d, ok := t.devices[deviceid]; ok {
		d.lock.Lock()
		defer d.lock.Unlock()
		delete(t.devices, deviceid)

		if len(d.txbuffer) > 0 {
			t.sched.Cancel(d)
		}

		// remove mqtt subscriptions
		t.mqtt.Unsubscribe(devicePrefix + deviceid + deviceSuffix + "rawrx")
		for _, txf := range d.txfields {
			t.mqtt.Unsubscribe(devicePrefix + deviceid + deviceSuffix + txf)
		}
	}
}

func (t *ByteTranslator) txQueue(d *Device, payload string, findex int) error {
	logitem := t.log.WithField("deviceid", d.deviceid)
	d.lock.Lock()
	defer d.lock.Unlock()

	logitem.Debugf("Scheduling tx for %v", d.txdelay)
	needSched, err := d.TxBufferAdd(string(payload), int(findex))
	if needSched {
		t.sched.Add(d, d.txdelay, func(dev *Device) {
			t.sendTx(dev)
		})
	}
	return err
}

func (t *ByteTranslator) sendTx(d *Device) {
	logitem := t.log.WithField("deviceid", d.deviceid)
	logitem.Debug("Sending TX")
	d.lock.Lock()
	bytes := d.TxBufferConsume()
	if bytes == nil {
		logitem.Debug("Buffer was reset")
		d.lock.Unlock()
		return
	}
	d.lock.Unlock()
	b64 := base64.StdEncoding.EncodeToString(bytes)
	err := t.mqtt.Publish(devicePrefix+d.deviceid+deviceSuffix+"rawtx", b64)
	if err != nil {
		logitem.Error("Failed to publish to device's rawtx topic: ", err)
		return
	}
	logitem.Debug("Published to rawtx")
}

func (t *ByteTranslator) onRawRx(d *Device, deviceid string, payload []byte) {
	logitem := t.log.WithField("deviceid", deviceid)
	logitem.Debug("Received rawrx for deviceid ", deviceid)
	binary, err := base64.StdEncoding.DecodeString(string(payload))
	if err != nil {
		t.log.Error("Failed decode base64 from rawrx", err)
		return
	}
	values, err := d.rxm.Unmarshal(binary)
	if err != nil {
		// had an error/warning while unmarshalling - publish error to /bytetranslator topic
		logitem.Warnf("Deviceid %s has trailing bytes", deviceid)
		err = t.mqtt.Publish(devicePrefix+deviceid+deviceSuffix+"bytetranslator", fmt.Sprint(err))
		if err != nil {
			logitem.Error("Failed to publish to device's bytetranslator topic: ", err)
			return
		}
		// more of warnings -- so let's continue
	}
	for i, value := range values {
		if i < len(d.rxfields) {
			// publish to the named topic
			topic := devicePrefix + deviceid + deviceSuffix + d.rxfields[i]
			logitem.Debugf("Publishing %s to %s", fmt.Sprint(value), topic)
			err = t.mqtt.Publish(topic, fmt.Sprint(value))
			if err != nil {
				logitem.Error("Failed to publish to device's \""+d.rxfields[i]+"\" topic: ", err)
				return
			}
		} else {
			// publish simply to the array index topic
			topic := devicePrefix + deviceid + deviceSuffix + fmt.Sprintf("%s%d", defaultIncomingPrefix, i)
			logitem.WithField("deviceid", deviceid).Debugf("Publishing %s to %s", fmt.Sprint(value), topic)
			err = t.mqtt.Publish(topic, fmt.Sprint(value))
			if err != nil {
				logitem.Error("Failed to publish to device's \""+fmt.Sprint(i)+"\" topic: ", err)
				return
			}
		}
	}
}
