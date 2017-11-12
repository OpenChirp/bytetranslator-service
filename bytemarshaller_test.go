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

	// valt := reflect.TypeOf(v)

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
		t.Errorf("NewByteTranslator return nil")
	}

	payload, err := bm.Marshal(values)
	if err != nil {
		t.Logf("Marshal returned error: %v", err)
		return false
	}

	strpayload, err := bm.Marshal(valuesString)
	if err != nil {
		t.Logf("Marshal of string value returned error: %v", err)
		return false
	}

	if !bytes.Equal(payload, strpayload) {
		t.Logf("Marshal of value and Marshal of value as string yield different payloads")
		return false
	}

	retval, err := bm.Unmarshal(payload)
	if err != nil {
		t.Logf("Unmarshal returned error: %v", err)
		return false
	}

	// if len(retval) != 1 {
	// 	t.Logf("Unmarshal returned too many values (%d values)", len(retval))
	// 	return false
	// }

	if len(retval) != len(values) {
		t.Logf("Unmarshal returned invalid number of values (%d values)", len(retval))
		return false
	}

	// if retval[0] != val {
	// 	t.Logf("Marshalled and Unmarshalled values do not match (%v != %v)", retval[0], val)
	// 	return false
	// }

	// Check that the type conversion were done correctly
	for i, value := range retval {
		truthValue := TypeCast(values[i], types[i].GetGoType())
		t.Logf("As a %v value should be %v", types[i], truthValue)
		// if reflect.ValueOf(value) != reflect.ValueOf(truthValue) {
		if !reflect.DeepEqual(value, truthValue) {
			t.Logf("Marshalled and Unmarshalled values do not match (outvalue=%v truthvalue=%v)", value, truthValue)
			return false
		}
	}

	return true
}

func RunUniversalTester(t *testing.T, gotype reflect.Type) bool {
	return true
}

func TestUint8FieldExtended(t *testing.T) {
	tester := func(value uint8) bool {
		return UniversalTester(t, value)
	}
	Convey("Marshalling and unmarshalling given value with all cast possibilities", t, func() {
		if err := quick.Check(tester, nil); err != nil {
			t.Error(err)
		}
		So(1, ShouldEqual, 1)
	})
}

func TestInt8FieldExtended(t *testing.T) {
	tester := func(value int8) bool {
		return UniversalTester(t, value)
	}
	Convey("Marshalling and unmarshalling given value with all cast possibilities", t, func() {
		err := quick.Check(tester, nil)
		So(err, ShouldBeNil)
	})
}

func TestInt16FieldExtended(t *testing.T) {
	tester := func(value int16) bool {
		return UniversalTester(t, value)
	}
	if err := quick.Check(tester, nil); err != nil {
		t.Error(err)
	}
}

func TestUint16Field(t *testing.T) {
	bmtype := FieldTypeUint16
	types := []FieldType{
		bmtype,
	}
	bm := NewByteMarshaller(types, FieldTypeFloat64, true)
	if bm == nil {
		t.Errorf("NewByteTranslator return nil")
	}

	tester := func(val uint16) bool {
		payload, err := bm.Marshal([]interface{}{val})
		if err != nil {
			t.Logf("Marshal returned error: %v", err)
			return false
		}

		strpayload, err := bm.Marshal([]interface{}{fmt.Sprint(val)})
		if err != nil {
			t.Logf("Marshal of string value returned error: %v", err)
			return false
		}

		if !bytes.Equal(payload, strpayload) {
			t.Logf("Marshal of value and Marshal of value as string yield different payloads")
			return false
		}

		retval, err := bm.Unmarshal(payload)
		if err != nil {
			t.Logf("Unmarshal returned error: %v", err)
			return false
		}

		if len(retval) != 1 {
			t.Logf("Unmarshal returned too many values (%d values)", len(retval))
			return false
		}

		if retval[0] != val {
			t.Logf("Marshalled and Unmarshalled values do not match (%v != %v)", retval[0], val)
			return false
		}

		return true
	}

	Convey("Marshalling and unmarshalling the value should yield the original value", t, func() {
		if err := quick.Check(tester, nil); err != nil {
			t.Error(err)
		}
	})
}

func TestInt16Field(t *testing.T) {
	bmtype := FieldTypeInt16
	types := []FieldType{
		bmtype,
	}
	bm := NewByteMarshaller(types, FieldTypeFloat64, true)
	if bm == nil {
		t.Errorf("NewByteTranslator return nil")
	}

	tester := func(val int16) bool {
		payload, err := bm.Marshal([]interface{}{val})
		if err != nil {
			t.Logf("Marshal returned error: %v", err)
			return false
		}

		strpayload, err := bm.Marshal([]interface{}{fmt.Sprint(val)})
		if err != nil {
			t.Logf("Marshal of string value returned error: %v", err)
			return false
		}

		if !bytes.Equal(payload, strpayload) {
			t.Logf("Marshal of value and Marshal of value as string yield different payloads")
			return false
		}

		retval, err := bm.Unmarshal(payload)
		if err != nil {
			t.Logf("Unmarshal returned error: %v", err)
			return false
		}

		if len(retval) != 1 {
			t.Logf("Unmarshal returned too many values (%d values)", len(retval))
			return false
		}

		if retval[0] != val {
			t.Logf("Marshalled and Unmarshalled values do not match (%v != %v)", retval[0], val)
			return false
		}

		return true
	}

	Convey("Marshalling and unmarshalling the value should yield the original value", t, func() {
		if err := quick.Check(tester, nil); err != nil {
			t.Error(err)
		}
	})
}
