package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openchirp/framework"
	"github.com/openchirp/framework/pubsub"

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

type ByteTranslator struct {
	mqtt               pubsub.PubSub
	log                *logrus.Logger
	devices            map[string]*Device
	defaultType        FieldType
	pubOutgoingQueue   bool
	outgoingQueueTopic string
	sched              *Scheduler
}

func NewByteTranslator(mqtt pubsub.PubSub, log *logrus.Logger, defaultType string, outgoingQueueTopic string) (*ByteTranslator, error) {
	if ftype := ParseFieldType(defaultType); ftype != FieldTypeUnknown {
		bt := &ByteTranslator{
			mqtt:               mqtt,
			log:                log,
			devices:            make(map[string]*Device),
			defaultType:        ftype,
			outgoingQueueTopic: outgoingQueueTopic,
			pubOutgoingQueue:   true,
		}
		if len(outgoingQueueTopic) == 0 {
			bt.pubOutgoingQueue = false
		}
		bt.sched = NewScheduler(log)
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

	d := NewDevice(t, deviceid)

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
	if len(txnames) > 0 {
		d.txfields = strings.Split(txnames, ",")
	}
	// remove possible space
	for i, name := range d.txfields {
		d.txfields[i] = strings.TrimSpace(name)
	}
	// get tx field types
	txfieldtypes := make([]FieldType, 0)
	if len(txtypes) > 0 {
		for _, txf := range strings.Split(txtypes, ",") {
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

	/* Add to table of devices */
	t.devices[deviceid] = d

	/* Setup subscriptions */
	err = d.Subscribe()

	return nil, err
}

func (t *ByteTranslator) RemoveDevice(deviceid string) {
	// remove device from devices table
	if d, ok := t.devices[deviceid]; ok {
		delete(t.devices, deviceid)
		d.Unsubscribe()
	}
}

func (t *ByteTranslator) Stats() string {
	return fmt.Sprintf("SchedulerQueueLen=%d", t.sched.QueueLen())
}
