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
	"reflect"
	"sort"
	"strconv"
)

const (
	alwaysIncludeAllFields = false
	defaultFieldEmptyValue = 0
)

const (
	defaultBufferSizeMultiplier = 4
)

type NumberType uint8

const (
	NumberTypeUnknown NumberType = iota
	NumberTypeUnsignedInt
	NumberTypeSignedInt
	NumberTypeFloat
)

func (nt NumberType) String() string {
	switch nt {
	case NumberTypeUnsignedInt:
		return "UnsignedInt"
	case NumberTypeSignedInt:
		return "SignedInt"
	case NumberTypeFloat:
		return "Float"
	default:
		return "Unknown"
	}
}

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

func FieldTypeFromGo(t reflect.Type) FieldType {
	return ParseFieldType(t.String())
}

var fieldTypeInfo = map[FieldType]struct {
	str    string
	bytes  int
	gotype reflect.Type
}{
	FieldTypeUint8:   {"uint8", 1, reflect.TypeOf(uint8(0))},
	FieldTypeUint16:  {"uint16", 2, reflect.TypeOf(uint16(0))},
	FieldTypeUint32:  {"uint32", 4, reflect.TypeOf(uint32(0))},
	FieldTypeUint64:  {"uint64", 8, reflect.TypeOf(uint64(0))},
	FieldTypeInt8:    {"int8", 1, reflect.TypeOf(int8(0))},
	FieldTypeInt16:   {"int16", 2, reflect.TypeOf(int16(0))},
	FieldTypeInt32:   {"int32", 4, reflect.TypeOf(int32(0))},
	FieldTypeInt64:   {"int64", 8, reflect.TypeOf(int64(0))},
	FieldTypeFloat32: {"float32", 4, reflect.TypeOf(float32(0))},
	FieldTypeFloat64: {"float64", 8, reflect.TypeOf(float64(0))},
	FieldTypeUnknown: {"unknown", 0, reflect.TypeOf(nil)},
}

func (ft FieldType) String() string {
	return fieldTypeInfo[ft].str
}

func (ft FieldType) ByteCount() int {
	return fieldTypeInfo[ft].bytes
}

func (ft FieldType) GetGoType() reflect.Type {
	return fieldTypeInfo[ft].gotype
}

func (ft FieldType) GetNumberType() NumberType {
	switch ft {
	case FieldTypeUint8:
		fallthrough
	case FieldTypeUint16:
		fallthrough
	case FieldTypeUint32:
		fallthrough
	case FieldTypeUint64:
		return NumberTypeUnsignedInt

	case FieldTypeInt8:
		fallthrough
	case FieldTypeInt16:
		fallthrough
	case FieldTypeInt32:
		fallthrough
	case FieldTypeInt64:
		return NumberTypeSignedInt

	case FieldTypeFloat32:
		fallthrough
	case FieldTypeFloat64:
		return NumberTypeFloat

	default:
		return NumberTypeUnknown
	}
}

func (ft FieldType) Marshal(buffer []byte, value interface{}, order binary.ByteOrder) int {
	var err error
	var i64 int64
	var u64 uint64
	var f64 float64

	if ft == FieldTypeUnknown {
		return 0
	}

	// All types other than floats can fit into a u64 without loosing data.
	// This would look relatively clean, but floats introduce different type
	// casting scheme.
	// Since floats take on different pos/neg values when type casted
	// to unsigned or signed, we need to actually cast it to a u64 and an i64
	// up front.
	// To reduce clutter, we will simply do all three type conversion for every
	// innput type.
	// Another assumption is that a float64 can always represent a float32.
	// If this is untrue, this routine needs to be revised.
	switch value.(type) {
	case uint8:
		u64 = uint64(value.(uint8))
		i64 = int64(u64)
		f64 = float64(value.(uint8))
	case uint16:
		u64 = uint64(value.(uint16))
		i64 = int64(u64)
		f64 = float64(value.(uint16))
	case uint32:
		u64 = uint64(value.(uint32))
		i64 = int64(u64)
		f64 = float64(value.(uint32))
	case uint64:
		u64 = uint64(value.(uint64))
		i64 = int64(u64)
		f64 = float64(value.(uint64))

	case int8:
		u64 = uint64(int64(value.(int8)))
		i64 = int64(u64)
		f64 = float64(value.(int8))
	case int16:
		u64 = uint64(int64(value.(int16)))
		i64 = int64(u64)
		f64 = float64(value.(int16))
	case int32:
		u64 = uint64(int64(value.(int32)))
		i64 = int64(u64)
		f64 = float64(value.(int32))
	case int64:
		u64 = uint64(int64(value.(int64)))
		i64 = int64(u64)
		f64 = float64(value.(int64))

	case float32:
		f64 = float64(value.(float32))
		u64 = uint64(f64)
		i64 = int64(f64)
	case float64:
		f64 = float64(value.(float64))
		u64 = uint64(f64)
		i64 = int64(f64)

	case string:
		// Separate different categories, so we can have a different priority
		// parsed types
		switch ft {
		case FieldTypeUint8:
			fallthrough
		case FieldTypeUint16:
			fallthrough
		case FieldTypeUint32:
			fallthrough
		case FieldTypeUint64:
			// Since we strive to parse/convert values at all cost,
			// we will try the correct type first and they successively try
			// other possible types.
			// This technique is not perfect. For example, values that are
			// out of range may fall into the last category
			if u64, err = strconv.ParseUint(value.(string), 10, 64); err == nil {
				i64 = int64(u64)
				f64 = float64(u64)
			} else if i64, err = strconv.ParseInt(value.(string), 10, 64); err == nil {
				f64 = float64(i64)
				u64 = uint64(i64)
			} else if f64, err = strconv.ParseFloat(value.(string), 64); err == nil {
				u64 = uint64(f64)
				i64 = int64(f64)
			}

		case FieldTypeInt8:
			fallthrough
		case FieldTypeInt16:
			fallthrough
		case FieldTypeInt32:
			fallthrough
		case FieldTypeInt64:
			if i64, err = strconv.ParseInt(value.(string), 10, 64); err == nil {
				f64 = float64(i64)
				u64 = uint64(i64)
			} else if u64, err = strconv.ParseUint(value.(string), 10, 64); err == nil {
				i64 = int64(u64)
				f64 = float64(u64)
			} else if f64, err = strconv.ParseFloat(value.(string), 64); err == nil {
				u64 = uint64(f64)
				i64 = int64(f64)
			}

		case FieldTypeFloat32:
			fallthrough
		case FieldTypeFloat64:
			if f64, err = strconv.ParseFloat(value.(string), 64); err == nil {
				u64 = uint64(f64)
				i64 = int64(f64)
			} else if u64, err = strconv.ParseUint(value.(string), 10, 64); err == nil {
				i64 = int64(u64)
				f64 = float64(u64)
			} else if i64, err = strconv.ParseInt(value.(string), 10, 64); err == nil {
				f64 = float64(i64)
				u64 = uint64(i64)
			}
		}
	default:
		return 0
	}

	// If we failed to parse the string
	if err != nil {
		return 0
	}

	switch ft {
	case FieldTypeUint8:
		buffer[0] = byte(uint8(u64))
	case FieldTypeInt8:
		// Again, i64 could yield a different result from u64
		// if the original type has a float
		buffer[0] = byte(uint8(i64))

	case FieldTypeUint16:
		order.PutUint16(buffer, uint16(u64))
	case FieldTypeInt16:
		order.PutUint16(buffer, uint16(i64))

	case FieldTypeUint32:
		order.PutUint32(buffer, uint32(u64))
	case FieldTypeInt32:
		order.PutUint32(buffer, uint32(i64))

	case FieldTypeUint64:
		order.PutUint64(buffer, uint64(u64))
	case FieldTypeInt64:
		order.PutUint64(buffer, uint64(i64))

	case FieldTypeFloat32:
		order.PutUint32(buffer, math.Float32bits(float32(f64)))
	case FieldTypeFloat64:
		order.PutUint64(buffer, math.Float64bits(float64(f64)))
	default:
		return 0
	}

	return ft.ByteCount()
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

// This function will always create the byte buffer even when an error is passed.
func (m *ByteMarshaller) MarshalValues(values map[int][]byte) []byte {
	findicies := make([]int, len(values))
	index := 0
	for findex, _ := range values {
		findicies[index] = findex
		index++
	}

	var minfindex int = 0
	var maxfindex int

	if len(values) == 0 {
		maxfindex = -1
	} else {
		sort.Ints(findicies)

		maxfindex = findicies[len(findicies)-1]
	}

	buffer := make([]byte, 0, ((maxfindex+1)-minfindex)*defaultBufferSizeMultiplier)

	var findex int
	for findex = minfindex; findex <= maxfindex; findex++ {
		if bytes, ok := values[findex]; ok {
			buffer = append(buffer, bytes...)
		} else {
			bytes, _ := m.MarshalValue(defaultFieldEmptyValue, findex)
			buffer = append(buffer, bytes...)
		}
	}

	if alwaysIncludeAllFields {
		for ; findex < len(m.types); findex++ {
			bytes, _ := m.MarshalValue(defaultFieldEmptyValue, findex)
			buffer = append(buffer, bytes...)
		}
	}

	return buffer
}

// This function will always create the byte buffer even when an error is passed.
func (m *ByteMarshaller) MarshalValue(value interface{}, findex int) ([]byte, error) {
	var err error

	/* Calculate size of buffer */
	size := 0
	if 0 <= findex && findex < len(m.types) {
		size += m.types[findex].ByteCount()
	} else {
		size += m.defaultType.ByteCount()
	}

	buffer := make([]byte, size)

	/* Marshal field at index findex */
	if 0 <= findex && findex < len(m.types) {
		count := m.types[findex].Marshal(buffer, value, m.order)
		if count == 0 {
			// this means there was a parse error or FieldType==Unknown
			err = fmt.Errorf("Parsing error or unknown data type")
		}
	} else {
		count := m.defaultType.Marshal(buffer, value, m.order)
		if count == 0 {
			// this means there was a parse error or FieldType==Unknown
			err = fmt.Errorf("Parsing error or unknown data type")
		}
	}
	return buffer, err
}

// This function will always create a byte buffer even when an error is passed.
func (m *ByteMarshaller) Marshal(values []interface{}) ([]byte, error) {
	var err error
	var findex int

	/* Calculate size of buffer */
	size := 0
	for findex, _ = range values {
		if findex < len(m.types) {
			size += m.types[findex].ByteCount()
		} else {
			size += m.defaultType.ByteCount()
		}
	}
	if alwaysIncludeAllFields {
		for ; findex < len(m.types); findex++ {
			size += m.defaultType.ByteCount()
		}
	}

	buffer := make([]byte, size)

	/* Marshal each field */
	var bindex int
	var value interface{}
	for findex, value = range values {
		if findex < len(m.types) {
			count := m.types[findex].Marshal(buffer[bindex:], value, m.order)
			if count == 0 {
				// this means there was a parse error or FieldType==Unknown
				err = fmt.Errorf("Parsing error or unknown data type")
				count = m.types[findex].ByteCount()
			}
			bindex += count
		} else {
			count := m.defaultType.Marshal(buffer[bindex:], value, m.order)
			if count == 0 {
				// this means there was a parse error or FieldType==Unknown
				err = fmt.Errorf("Parsing error or unknown data type")
				count = m.defaultType.ByteCount()
			}
			bindex += count
		}
	}
	if alwaysIncludeAllFields {
		for ; findex < len(m.types); findex++ {
			count := m.defaultType.Marshal(buffer[bindex:], defaultFieldEmptyValue, m.order)
			if count == 0 {
				// this means there was a parse error or FieldType==Unknown
				err = fmt.Errorf("Parsing error or unknown data type")
				count = m.defaultType.ByteCount()
			}
			bindex += count
		}
	}
	return buffer, err
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
