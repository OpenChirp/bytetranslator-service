package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultLittleEndian = true
	devicePrefix        = "openchirp/devices/"
	deviceSuffix        = "/transducer/"
)

var ErrUnknownFieldType = errors.New("Could not parse FieldType")

type MQTT interface {
	Subscribe(topic string, callback func(topic string, payload []byte)) error
	Unsubscribe(topics ...string) error
	Publish(topic string, payload interface{}) error
}

type Device struct {
	lock     sync.Mutex
	rxm      *ByteMarshaller
	txm      *ByteMarshaller
	rxfields []string
	txfields []string

	txdelay time.Duration
	// delay period
	txscheduled bool
	// time scheduled to send
}

// TODO: setup Device receiver functions to support min-heaps for scheduling

type ByteTranslator struct {
	mqtt        MQTT
	log         *logrus.Logger
	devices     map[string]*Device
	defaultType FieldType
}

func NewByteTranslator(mqtt MQTT, log *logrus.Logger, defaultType string) (*ByteTranslator, error) {
	if ftype := ParseFieldType(defaultType); ftype != FieldTypeUnknown {
		return &ByteTranslator{
			mqtt:        mqtt,
			log:         log,
			devices:     make(map[string]*Device),
			defaultType: ftype,
		}, nil
	}
	return nil, ErrUnknownFieldType
}

// AddDevice adds a device to the bytetranslator manager.
// The first error is intended to be sent as status to the device, the second
// error is a runtime error
func (t *ByteTranslator) AddDevice(deviceid, rxnames, rxtypes, txnames, txtypes, endianness, delay string) (error, error) {
	var err error
	var littleEndian = defaultLittleEndian

	d := &Device{}

	/* Parse all config parameters */
	// get delay
	d.txdelay = time.Duration(0)
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
			littleEndian = true
		case "big":
			littleEndian = false
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
	// get tx field types
	// TODO: Implement TX side

	/* Make byte marshaler based on config */
	d.rxm = NewByteMarshaller(rxfieldtypes, t.defaultType, littleEndian)

	/* Add to table of devices */
	t.devices[deviceid] = d

	/* Setup subscriptions */
	d.lock.Lock()
	defer d.lock.Unlock()
	t.log.Debug("Subscribing to \"", devicePrefix+deviceid+deviceSuffix+"rawrx", "\"")
	err = t.mqtt.Subscribe(
		devicePrefix+deviceid+deviceSuffix+"rawrx",
		func(topic string, payload []byte) {
			t.log.Debug("Received rawrx for deviceid ", deviceid)
			binary, err := base64.StdEncoding.DecodeString(string(payload))
			if err != nil {
				t.log.Error("Failed decode base64 from rawrx", err)
				return
			}
			values, err := d.rxm.Unmarshal(binary)
			if err != nil {
				// had an error/warning while unmarshalling - publish error to /bytetranslator topic
				err = t.mqtt.Publish(devicePrefix+deviceid+deviceSuffix+"bytetranslator", fmt.Sprint(err))
				if err != nil {
					t.log.Error("Failed to publish to device's bytetranslator topic: ", err)
					return
				}
				// more of warnings -- so let's continue
			}
			for i, value := range values {
				if i < len(d.rxfields) {
					// publish to the named topic
					err = t.mqtt.Publish(devicePrefix+deviceid+deviceSuffix+d.rxfields[i], fmt.Sprint(value))
					if err != nil {
						t.log.Error("Failed to publish to device's \""+d.rxfields[i]+"\" topic: ", err)
						return
					}
				} else {
					// publish simply to the array index topic
					err = t.mqtt.Publish(devicePrefix+deviceid+deviceSuffix+fmt.Sprint(i), fmt.Sprint(value))
					if err != nil {
						t.log.Error("Failed to publish to device's \""+fmt.Sprint(i)+"\" topic: ", err)
						return
					}
				}
			}
		})

	return nil, err
}

func (t *ByteTranslator) RemoveDevice(deviceid string) {
	// remove device from devices table
	d := t.devices[deviceid]
	d.lock.Lock()
	defer d.lock.Unlock()
	delete(t.devices, deviceid)

	// remove mqtt subscriptions
	t.mqtt.Unsubscribe(devicePrefix + deviceid + deviceSuffix + "rawrx")
	for _, txf := range d.txfields {
		t.mqtt.Unsubscribe(devicePrefix + deviceid + deviceSuffix + txf)
	}

	// set to not scheduled - to avoid scheduled device being published on
	d.txscheduled = false
}
