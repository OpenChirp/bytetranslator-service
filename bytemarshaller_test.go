package main

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
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

func TestUint16Field(t *testing.T) {
	testcases := make([]uint16, 0)
	bmtype := FieldTypeUint16

	var i uint16
	for i = 0; i < math.MaxUint16; i += ((uint16(rand.Uint32()) % 1024) % (math.MaxUint16 - i)) + 1 {
		testcases = append(testcases, i)
	}

	Convey("Using a ByteMarshaller with one type "+bmtype.String(), t, func() {
		types := []FieldType{
			bmtype,
		}
		bm := NewByteMarshaller(types, FieldTypeFloat64, true)
		So(bm, ShouldNotBeNil)

		for _, i := range testcases {
			val := i
			Convey(fmt.Sprintf("Marshalling %d as %s and as a string should yield the same buffer", val, bmtype.String()), func() {
				payload, err := bm.Marshal([]interface{}{val})
				So(err, ShouldBeNil)

				strpayload, err := bm.Marshal([]interface{}{fmt.Sprint(val)})
				So(err, ShouldBeNil)
				So(bytes.Equal(payload, strpayload), ShouldBeTrue)
			})

			Convey(fmt.Sprintf("Marshalling %d and unmarshaling it should yield the original value", val), func() {
				payload, err := bm.Marshal([]interface{}{val})
				So(err, ShouldBeNil)

				retval, err := bm.Unmarshal(payload)
				So(err, ShouldBeNil)
				So(len(retval), ShouldEqual, 1)
				So(retval[0], ShouldEqual, val)
			})
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
