package ygo_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"riguz.com/ygo/pkg/ygo"
)

func roundTripAny(t *testing.T, a ygo.Any) {
	t.Helper()
	encoder := ygo.NewEncoderV1()
	err := ygo.WriteAny(&encoder, a)
	assert.NoError(t, err)

	data := encoder.ToBytes()
	decoder := ygo.NewDecoderV1(bytes.NewReader(data))
	got, err := ygo.ReadAny(&decoder)
	assert.NoError(t, err)
	assert.True(t, a.Equal(got), "expected %+v, got %+v", a, got)
}

func TestAny_Undefined(t *testing.T) {
	roundTripAny(t, ygo.UndefinedAny())
}

func TestAny_Null(t *testing.T) {
	roundTripAny(t, ygo.NullAny())
}

func TestAny_Integer(t *testing.T) {
	roundTripAny(t, ygo.IntegerAny(0))
	roundTripAny(t, ygo.IntegerAny(42))
	roundTripAny(t, ygo.IntegerAny(-100))
}

func TestAny_Float32(t *testing.T) {
	roundTripAny(t, ygo.Float32Any(3.14))
}

func TestAny_Float64(t *testing.T) {
	roundTripAny(t, ygo.Float64Any(3.141592653589793))
}

func TestAny_BigInt64(t *testing.T) {
	roundTripAny(t, ygo.BigInt64Any(1234567890123456789))
	roundTripAny(t, ygo.BigInt64Any(-1234567890123456789))
}

func TestAny_Bool(t *testing.T) {
	roundTripAny(t, ygo.FalseAny())
	roundTripAny(t, ygo.TrueAny())
	roundTripAny(t, ygo.BoolAny(true))
	roundTripAny(t, ygo.BoolAny(false))
}

func TestAny_String(t *testing.T) {
	roundTripAny(t, ygo.StringAny(""))
	roundTripAny(t, ygo.StringAny("hello"))
	roundTripAny(t, ygo.StringAny("Hello,中国！𐐷"))
}

func TestAny_Object(t *testing.T) {
	roundTripAny(t, ygo.ObjectAny(map[string]ygo.Any{}))
	roundTripAny(t, ygo.ObjectAny(map[string]ygo.Any{
		"key":  ygo.IntegerAny(42),
		"name": ygo.StringAny("test"),
	}))
}

func TestAny_Array(t *testing.T) {
	roundTripAny(t, ygo.ArrayAny([]ygo.Any{}))
	roundTripAny(t, ygo.ArrayAny([]ygo.Any{
		ygo.IntegerAny(1),
		ygo.StringAny("two"),
		ygo.TrueAny(),
	}))
}

func TestAny_Binary(t *testing.T) {
	roundTripAny(t, ygo.BinaryAny([]uint8{}))
	roundTripAny(t, ygo.BinaryAny([]uint8{1, 2, 3, 4, 5}))
}

func TestAny_Nested(t *testing.T) {
	nested := ygo.ObjectAny(map[string]ygo.Any{
		"arr": ygo.ArrayAny([]ygo.Any{
			ygo.IntegerAny(1),
			ygo.ObjectAny(map[string]ygo.Any{
				"inner": ygo.BigInt64Any(42),
			}),
		}),
		"bin": ygo.BinaryAny([]uint8{0xFF}),
	})
	roundTripAny(t, nested)
}

func TestAny_Equal(t *testing.T) {
	assert.True(t, ygo.UndefinedAny().Equal(ygo.UndefinedAny()))
	assert.False(t, ygo.UndefinedAny().Equal(ygo.NullAny()))
	assert.True(t, ygo.IntegerAny(42).Equal(ygo.IntegerAny(42)))
	assert.False(t, ygo.IntegerAny(42).Equal(ygo.IntegerAny(43)))
	assert.True(t, ygo.BinaryAny([]uint8{1, 2}).Equal(ygo.BinaryAny([]uint8{1, 2})))
	assert.False(t, ygo.BinaryAny([]uint8{1, 2}).Equal(ygo.BinaryAny([]uint8{1, 3})))
}

func TestReadMultipleAny(t *testing.T) {
	anys := []ygo.Any{
		ygo.IntegerAny(1),
		ygo.StringAny("hello"),
		ygo.BigInt64Any(42),
	}

	encoder := ygo.NewEncoderV1()
	err := ygo.WriteMultipleAny(&encoder, anys)
	assert.NoError(t, err)

	decoder := ygo.NewDecoderV1(bytes.NewReader(encoder.ToBytes()))
	got, err := ygo.ReadMultipleAny(&decoder)
	assert.NoError(t, err)
	assert.Equal(t, len(anys), len(got))
	for i := range anys {
		assert.True(t, anys[i].Equal(got[i]))
	}
}
