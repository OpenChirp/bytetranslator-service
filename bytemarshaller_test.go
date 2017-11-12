package main

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"testing"
	"testing/quick"

	. "github.com/smartystreets/goconvey/convey"
)

const (
	debugMessages                   = true
	quitOnPayloadEquivalenceFailure = false
)

func TestIntegerStuff(t *testing.T) {
	Convey("Given some integer with a starting value", t, func() {
		x := 1

		Convey("When the integer is incremented", func() {
			x++

			Convey("The value should be greater by one", func() {
				So(x, ShouldEqual, 2)
			})
			Convey("The value should be two", func() {
				So(x, ShouldEqual, 2)
			})
		})
	})
}

func TestUint8Field(t *testing.T) {
	types := []FieldType{
		FieldTypeUint8,
	}
	bm := NewByteMarshaller(types, FieldTypeFloat64, true)
	Convey("Given an uint8 value ", t, func() {
		var i uint8
		for i = 0; i < math.MaxUint8; i++ {
			val := i
			Convey("When we marshal and unmarshal "+fmt.Sprint(val), func() {
				values := []interface{}{val}
				payload, err := bm.Marshal(values)
				So(err, ShouldBeNil)

				retval, err := bm.Unmarshal(payload)
				So(err, ShouldBeNil)
				Convey("The return values should be the same", func() {
					So(len(retval), ShouldEqual, 1)
					So(retval[0], ShouldEqual, val)
				})
			})

			Convey("When we marshal as a string and unmarshal "+fmt.Sprint(val), func() {
				values := []interface{}{fmt.Sprint(val)}
				payload, err := bm.Marshal(values)
				So(err, ShouldBeNil)

				retval, err := bm.Unmarshal(payload)
				So(err, ShouldBeNil)
				Convey("The return values should be the same", func() {
					So(len(retval), ShouldEqual, 1)
					So(retval[0], ShouldEqual, val)
				})
			})
		}
	})
}

func TestInt8Field(t *testing.T) {
	types := []FieldType{
		FieldTypeInt8,
	}
	bm := NewByteMarshaller(types, FieldTypeFloat64, true)
	Convey("Given an int8 value ", t, func() {
		var i int8
		for i = math.MinInt8; i < math.MaxInt8; i++ {
			val := i
			Convey("When we marshal and unmarshal "+fmt.Sprint(val), func() {
				values := []interface{}{val}
				payload, err := bm.Marshal(values)
				So(err, ShouldBeNil)

				retval, err := bm.Unmarshal(payload)
				So(err, ShouldBeNil)
				Convey("The return values should be the same", func() {
					So(len(retval), ShouldEqual, 1)
					So(retval[0], ShouldEqual, val)
				})
			})

			Convey("When we marshal as a string and unmarshal "+fmt.Sprint(val), func() {
				values := []interface{}{fmt.Sprint(val)}
				payload, err := bm.Marshal(values)
				So(err, ShouldBeNil)

				retval, err := bm.Unmarshal(payload)
				So(err, ShouldBeNil)
				Convey("The return values should be the same", func() {
					So(len(retval), ShouldEqual, 1)
					So(retval[0], ShouldEqual, val)
				})
			})
		}
	})
}

func TypeCast(v interface{}, dest reflect.Type) interface{} {
	return reflect.ValueOf(v).Convert(dest).Interface()
}

// We convert the value into a float64 and check it's sign
func IsNegative(val interface{}) bool {
	var f float64
	f = reflect.ValueOf(val).Convert(reflect.TypeOf(f)).Interface().(float64)
	return f < 0
}

func UniversalTester(t *testing.T, val interface{}) bool {
	defaultByteMarshallerType := FieldTypeFloat64

	t.Log("Testing", reflect.TypeOf(val), val)

	// Here we will try to convert the given value/type to each of the possible
	// other types using Marshal and a field for each possible type
	// We want to verify that the conversion followed the same rules
	// as it would if we type casted it

	types := []FieldType{
		FieldTypeUint8,
		FieldTypeUint16,
		FieldTypeUint32,
		FieldTypeUint64,
		FieldTypeInt8,
		FieldTypeInt16,
		FieldTypeInt32,
		FieldTypeInt64,
		FieldTypeFloat32,
		FieldTypeFloat64,
	}

	values := []interface{}{
		val,
		val,
		val,
		val,
		val,
		val,
		val,
		val,
		val,
		val,
	}

	valuesString := []interface{}{
		fmt.Sprint(val),
		fmt.Sprint(val),
		fmt.Sprint(val),
		fmt.Sprint(val),
		fmt.Sprint(val),
		fmt.Sprint(val),
		fmt.Sprint(val),
		fmt.Sprint(val),
		fmt.Sprint(val),
		fmt.Sprint(val),
	}

	bm := NewByteMarshaller(types, defaultByteMarshallerType, true)
	if bm == nil {
		t.Errorf("Err: NewByteTranslator return nil")
	}

	payload, err := bm.Marshal(values)
	if err != nil {
		t.Logf("Err: Marshal of std types returned error: %v", err)
		return false
	}

	strpayload, err := bm.Marshal(valuesString)
	if err != nil {
		t.Logf("Err: Marshal of string value returned error: %v", err)
		t.Logf("Standard Types: %v", values)
		t.Logf("String Types:   %v", valuesString)
		return false
	}

	// Check that marshalled payload are byte equivalent
	if !bytes.Equal(payload, strpayload) {
		t.Logf("Err: Marshalling values from standard types vs. marshalling from strings yieldied different payloads")
		t.Logf("std types payload: %v", payload)
		t.Logf("strings payload:   %v", strpayload)

		if quitOnPayloadEquivalenceFailure {
			return false
		}
	}

	/*
		Try to unmarshal the marshalled payload from the standard values
		and check that the output values match the input marshalled values.
	*/
	retval, err := bm.Unmarshal(payload)
	if err != nil {
		t.Logf("Err: Unmarshal returned error: %v", err)
		return false
	}

	if len(retval) != len(values) {
		t.Logf("Err: Unmarshal returned invalid number of values (%d values)", len(retval))
		return false
	}

	// Check that the type conversion were done correctly
	for i, value := range retval {
		truthValue := TypeCast(values[i], types[i].GetGoType())
		if debugMessages {
			t.Logf("As a %v value should be %v", types[i], truthValue)
		}
		if !reflect.DeepEqual(value, truthValue) {
			t.Logf("Err: Marshalled and Unmarshalled values do not match (outvalue=%v truthvalue=%v)", value, truthValue)
			t.Logf("As a %v value should have been %v", types[i], truthValue)
			return false
		}
	}

	/*
		Try to unmarshal the marshalled payload from the string values
		and check that the output values match the input marshalled values.

		This is only useful if you have disabled the "return false" when
		checking that the two marshalled payload are byte equivalent.
		In otherwords, set quitOnPayloadEquivalenceFailure to false
	*/
	if !quitOnPayloadEquivalenceFailure {
		retval, err := bm.Unmarshal(strpayload)
		if err != nil {
			t.Logf("Unmarshal returned error: %v", err)
			return false
		}

		if len(retval) != len(values) {
			t.Logf("Unmarshal returned invalid number of values (%d values)", len(retval))
			return false
		}

		// Check that the type conversion were done correctly
		for i, value := range retval {
			truthValue := TypeCast(values[i], types[i].GetGoType())
			if debugMessages {
				t.Logf("As a %v value should be %v", types[i], truthValue)
			}
			if !reflect.DeepEqual(value, truthValue) {
				t.Logf("As a %v value should have been %v", types[i], truthValue)
				t.Logf("Std Value: %v = %f", values[i], values[i])
				t.Logf("Str Value: %v", valuesString[i])
				t.Logf("Marshalled and Unmarshalled values do not match (outvalue=%v truthvalue=%v)", value, truthValue)
				return false
			}
		}
	}

	return true
}

func TestAllTypes(t *testing.T) {
	alltypes := []FieldType{
		FieldTypeUint8,
		FieldTypeUint16,
		FieldTypeUint32,
		FieldTypeUint64,
		FieldTypeInt8,
		FieldTypeInt16,
		FieldTypeInt32,
		FieldTypeInt64,
		FieldTypeFloat32,
		FieldTypeFloat64,
	}

	alltesters := []interface{}{
		func(value uint8) bool {
			return UniversalTester(t, value)
		},
		func(value uint16) bool {
			return UniversalTester(t, value)
		},
		func(value uint32) bool {
			return UniversalTester(t, value)
		},
		func(value uint64) bool {
			return UniversalTester(t, value)
		},
		func(value int8) bool {
			return UniversalTester(t, value)
		},
		func(value int16) bool {
			return UniversalTester(t, value)
		},
		func(value int32) bool {
			return UniversalTester(t, value)
		},
		func(value int64) bool {
			return UniversalTester(t, value)
		},
		func(value float32) bool {
			return UniversalTester(t, value)
		},
		func(value float64) bool {
			return UniversalTester(t, value)
		},
	}

	Convey("Testing marshalling and unmarshalling of all types", t, func() {
		for i, bmtype := range alltypes {
			Convey("Trying "+bmtype.String()+" input types", func() {
				err := quick.Check(alltesters[i], nil)
				So(err, ShouldBeNil)
			})
		}

	})
}
