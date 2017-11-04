// Craig Hesling
// September 9, 2017
//
// This library is used to encode/decode bytes in buffers where position and
// type matter.
package main

import (
	"encoding/binary"
	"fmt"
	"math"
)

type FieldType uint8

const (
	// The field type is unknown. Handlers will ignore these fields.
	FieldTypeUnknown FieldType = iota
	FieldTypeUint8
	FieldTypeUint16
	FieldTypeUint32
	FieldTypeUint64
	FieldTypeInt8
	FieldTypeInt16
	FieldTypeInt32
	FieldTypeInt64
	FieldTypeFloat32
	FieldTypeFloat64
)

func ParseFieldType(name string) FieldType {
	switch name {
	case "uint8":
		return FieldTypeUint8
	case "uint16":
		return FieldTypeUint16
	case "uint32":
		return FieldTypeUint32
	case "uint64":
		return FieldTypeUint64
	case "int8":
		return FieldTypeInt8
	case "int16":
		return FieldTypeInt16
	case "int32":
		return FieldTypeInt32
	case "int64":
		return FieldTypeInt64
	case "float32":
		return FieldTypeFloat32
	case "float64":
		return FieldTypeFloat64
	default:
		return FieldTypeUnknown
	}
}

var fieldTypeInfo = map[FieldType]struct {
	str   string
	bytes int
}{
	FieldTypeUint8:   {"uint8", 1},
	FieldTypeUint16:  {"uint16", 2},
	FieldTypeUint32:  {"uint32", 4},
	FieldTypeUint64:  {"uint64", 8},
	FieldTypeInt8:    {"int8", 1},
	FieldTypeInt16:   {"int16", 2},
	FieldTypeInt32:   {"int32", 4},
	FieldTypeInt64:   {"int64", 8},
	FieldTypeFloat32: {"float32", 4},
	FieldTypeFloat64: {"float64", 8},
	FieldTypeUnknown: {"unknown", 0},
}

func (ft FieldType) String() string {
	return fieldTypeInfo[ft].str
}

func (ft FieldType) ByteCount() int {
	return fieldTypeInfo[ft].bytes
}

func (ft FieldType) Unmarshal(bytes []byte, order binary.ByteOrder) interface{} {
	var u8 uint8
	var u16 uint16
	var u32 uint32
	var u64 uint64

	if len(bytes) < ft.ByteCount() {
		return nil
	}
	if ft == FieldTypeUnknown {
		return nil
	}

	switch ft {
	case FieldTypeUint8:
		fallthrough
	case FieldTypeInt8:
		u8 = uint8(bytes[0])

	case FieldTypeUint16:
		fallthrough
	case FieldTypeInt16:
		u16 = order.Uint16(bytes)

	case FieldTypeUint32:
		fallthrough
	case FieldTypeInt32:
		fallthrough
	case FieldTypeFloat32:
		u32 = order.Uint32(bytes)

	case FieldTypeUint64:
		fallthrough
	case FieldTypeInt64:
		fallthrough
	case FieldTypeFloat64:
		u64 = order.Uint64(bytes)
	}

	switch ft {
	case FieldTypeUint8:
		return uint8(u8)
	case FieldTypeUint16:
		return uint16(u16)
	case FieldTypeUint32:
		return uint32(u32)
	case FieldTypeUint64:
		return uint64(u64)
	case FieldTypeInt8:
		return int8(u8)
	case FieldTypeInt16:
		return int16(u16)
	case FieldTypeInt32:
		return int32(u32)
	case FieldTypeInt64:
		return int64(u64)
	case FieldTypeFloat32:
		return math.Float32frombits(u32)
	case FieldTypeFloat64:
		return math.Float64frombits(u64)
	}
	return nil
}

// ByteMarshaller handles types related to positions.
// Setting defaultType to UnknownType would mean the field is omitted
type ByteMarshaller struct {
	order       binary.ByteOrder
	types       []FieldType
	defaultType FieldType
}

func NewByteMarshaller(types []FieldType, defaultType FieldType, littleEndian bool) *ByteMarshaller {
	// TODO: This should probably cache identical byte marshallers for identical configs
	if littleEndian {
		return &ByteMarshaller{binary.LittleEndian, types, defaultType}
	}
	return &ByteMarshaller{binary.BigEndian, types, defaultType}
}

func (m *ByteMarshaller) Marshal(values ...interface{}) {
	//TODO: Implement
}

// Unmarshal tries to decode as many known types as possible from the byte array.
// This function may return an error, which can be considered a warning.
func (m *ByteMarshaller) Unmarshal(buffer []byte) ([]interface{}, error) {
	values := make([]interface{}, 0)
	buf := buffer[:]

	// walk through expected types/fields
	for _, t := range m.types {
		if bcount := t.ByteCount(); len(buf) >= bcount {
			v := t.Unmarshal(buf, m.order)
			values = append(values, v)
			buf = buf[bcount:]
		} else if len(buf) == 0 {
			// packet smaller than expected, but we are finished decoding
			return values, nil
		} else {
			// this is basically a warning
			return values, fmt.Errorf("Tailing bytes detected")
		}
	}

	// try to pull out as many defaultTypes as possible
	for len(buf) > 0 {
		if bcount := m.defaultType.ByteCount(); len(buf) >= bcount {
			v := m.defaultType.Unmarshal(buf, m.order)
			values = append(values, v)
			buf = buf[bcount:]
		} else {
			// this is basically a warning
			return values, fmt.Errorf("Tailing bytes detected")
		}
	}

	return values, nil
}
