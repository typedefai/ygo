package lib0_test

import (
	"encoding/hex"
	"fmt"
	"math"
	"testing"

	"gotest.tools/assert"
	"riguz.com/ygo/internal/lib0"
)

/*
	import * as enc from "lib0/encoding";

	const toHexString = (bytes) => {
	return Array.from(bytes, (byte) => {
		return ("0" + (byte & 0xff).toString(16)).slice(-2);
	}).join("");
	};

	var encoder = enc.createEncoder();
	enc.writeUint8(encoder, 1);
	var result = enc.toUint8Array(encoder);
	console.log(toHexString(result));
*/

func TestWrite_uint8Array(t *testing.T) {
	w := lib0.NewBufferWrite()

	err := w.WriteUint8Array([]byte{0, 1, 128, 254})

	assert.NilError(t, err)
	assert.Equal(t, "000180fe", hex.EncodeToString(w.ToBytes()))
}

func TestWrite_varUint8Array(t *testing.T) {
	w := lib0.NewBufferWrite()

	err := w.WriteVarUint8Array([]byte{0, 1, 128, 254})

	assert.NilError(t, err)
	assert.Equal(t, "04000180fe", hex.EncodeToString(w.ToBytes()))
}

func TestWrite_uint8(t *testing.T) {
	var tests = []struct {
		number   uint8
		expected string
	}{
		{1, "01"},
		{123, "7b"},
		{255, "ff"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write uint8:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteUint8(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_uint16(t *testing.T) {
	var tests = []struct {
		number   uint16
		expected string
	}{
		{0, "0000"},
		{1, "0100"},
		{255, "ff00"},
		{65535, "ffff"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write uint16:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteUint16(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_uint32(t *testing.T) {
	var tests = []struct {
		number   uint32
		expected string
	}{
		{0, "00000000"},
		{1, "01000000"},
		{255, "ff000000"},
		{65535, "ffff0000"},
		{4294967295, "ffffffff"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write uint32:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteUint32(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_uint32_be(t *testing.T) {
	var tests = []struct {
		number   uint32
		expected string
	}{
		{0, "00000000"},
		{1, "00000001"},
		{255, "000000ff"},
		{65535, "0000ffff"},
		{4294967295, "ffffffff"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write uint32be:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteUint32BigEndian(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_float32(t *testing.T) {
	var tests = []struct {
		number   float32
		expected string
	}{
		{0, "00000000"},
		{1, "3f800000"},
		{255, "437f0000"},
		{65535, "477fff00"},
		{4294967295, "4f800000"},
		{123.45, "42f6e666"},
		{982990.1, "496ffce2"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write float32:%f", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteFloat32(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_float64(t *testing.T) {
	var tests = []struct {
		number   float64
		expected string
	}{
		{0, "0000000000000000"},
		{1, "3ff0000000000000"},
		{255, "406fe00000000000"},
		{65535, "40efffe000000000"},
		{4294967295, "41efffffffe00000"},
		{18446744073709552000, "43f0000000000000"},
		{123.45, "405edccccccccccd"},
		{982990.1, "412dff9c33333333"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write float32:%f", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteFloat64(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_uint64(t *testing.T) {
	var tests = []struct {
		number   uint64
		expected string
	}{
		{0, "0000000000000000"},
		{1, "0000000000000001"},
		{255, "00000000000000ff"},
		{65535, "000000000000ffff"},
		{4294967295, "00000000ffffffff"},
		{18446744073709551615, "ffffffffffffffff"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write uint64:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteUint64(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_int64(t *testing.T) {
	var tests = []struct {
		number   int64
		expected string
	}{
		{0, "0000000000000000"},
		{1, "0000000000000001"},
		{-1, "ffffffffffffffff"},
		{255, "00000000000000ff"},
		{65535, "000000000000ffff"},
		{-65535, "ffffffffffff0001"},
		{-9223372036854775808, "8000000000000000"},
		{9223372036854775807, "7fffffffffffffff"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write int64:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteInt64(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_varUint(t *testing.T) {
	var tests = []struct {
		number   uint
		expected string
	}{
		{0, "00"},
		{1, "01"},
		{255, "ff01"},
		{65535, "ffff03"},
		{4294967295, "ffffffff0f"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("writeVarUint:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteVarUint(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_varUint8(t *testing.T) {
	var tests = []struct {
		number   uint8
		expected string
	}{
		{0, "00"},
		{1, "01"},
		{255, "ff01"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("writeVarUint8:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteVarUint8(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_varUint16(t *testing.T) {
	var tests = []struct {
		number   uint16
		expected string
	}{
		{0, "00"},
		{1, "01"},
		{255, "ff01"},
		{65535, "ffff03"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("writeVarUint16:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteVarUint16(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_varUint32(t *testing.T) {
	var tests = []struct {
		number   uint32
		expected string
	}{
		{0, "00"},
		{1, "01"},
		{255, "ff01"},
		{65535, "ffff03"},
		{4294967295, "ffffffff0f"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("writeVarUint32:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteVarUint32(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_varUint64(t *testing.T) {
	var tests = []struct {
		number   uint64
		expected string
	}{
		{0, "00"},
		{1, "01"},
		{255, "ff01"},
		{65535, "ffff03"},
		{4294967295, "ffffffff0f"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("writeVarUint64:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteVarUint64(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_varInt(t *testing.T) {
	var tests = []struct {
		number   int
		expected string
	}{
		{0, "00"},
		{1, "01"},
		{255, "bf03"},
		{65535, "bfff07"},
		{4294967295, "bfffffff1f"},
		{-1, "41"},
		{-255, "ff03"},
		{-65535, "ffff07"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("writeVarInt:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteVarInt(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_varInt8(t *testing.T) {
	var tests = []struct {
		number   int8
		expected string
	}{
		{0, "00"},
		{1, "01"},
		{-1, "41"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("writeVarInt8:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteVarInt8(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_varInt16(t *testing.T) {
	var tests = []struct {
		number   int16
		expected string
	}{
		{0, "00"},
		{1, "01"},
		{255, "bf03"},
		{-1, "41"},
		{-255, "ff03"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("writeVarInt16:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteVarInt16(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_varInt32(t *testing.T) {
	var tests = []struct {
		number   int32
		expected string
	}{
		{0, "00"},
		{1, "01"},
		{255, "bf03"},
		{65535, "bfff07"},
		{-1, "41"},
		{-255, "ff03"},
		{-65535, "ffff07"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("writeVarInt32:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteVarInt32(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_varInt64(t *testing.T) {
	var tests = []struct {
		number   int64
		expected string
	}{
		{0, "00"},
		{1, "01"},
		{255, "bf03"},
		{65535, "bfff07"},
		{4294967295, "bfffffff1f"},
		{-1, "41"},
		{-255, "ff03"},
		{-65535, "ffff07"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("writeVarInt64:%d", tt.number), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteVarInt64(tt.number)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_varString(t *testing.T) {
	var tests = []struct {
		str      string
		expected string
	}{
		{"Hello World!", "0c48656c6c6f20576f726c6421"},
		{"", "00"},
		{"你好，世界！", "12e4bda0e5a5bdefbc8ce4b896e7958cefbc81"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write var string:%s", tt.str), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteVarString(&tt.str)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_any(t *testing.T) {
	var tests = []struct {
		a        any
		expected string
	}{
		{lib0.Undefined{}, "7f"},
		{nil, "7e"},
		{0, "7d00"},
		{1, "7d01"},
		{-1, "7d41"},
		{2147483647, "7dbfffffff0f"},
		{-9223372036854775808, "7a8000000000000000"},
		{float32(1.9999998807907104), "7c3fffffff"},
		{float64(1.7976931348623157e+308), "7b7fefffffffffffff"},
		{true, "78"},
		{false, "79"},
		{"Hello world!", "770c48656c6c6f20776f726c6421"},
		{[]uint8{0x2a, 0x3b, 0x4c, 0x9d}, "74042a3b4c9d"},
		{map[string]any{}, "7600"},
		// Keys are emitted in sorted order (Go maps are unordered; the decoded
		// object is identical regardless of key order).
		{map[string]any{
			"name": "J. Mes",
			"age":  18,
		}, "7602036167657d12046e616d6577064a2e204d6573"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write var any:%v = %v", tt.a, tt.expected), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteAny(tt.a)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestWrite_anyNumber(t *testing.T) {
	var tests = []struct {
		a        float64
		expected string
	}{
		{2147483647, "7dbfffffff0f"},
		{-2147483647, "7dffffffff0f"},
		// lib0's threshold is |data| <= BITS31, so -2^31 is out of integer
		// range and takes the float32 path (tag 124) — verified against
		// lib0@0.2.99 by testutil/gen_lib0_vectors.js.
		{-2147483648, "7ccf000000"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write var any:%v = %v", tt.a, tt.expected), func(t *testing.T) {
			w := lib0.NewBufferWrite()

			err := w.WriteAny(tt.a)

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

// TestWrite_anyNumberDispatch pins lib0's value-based number dispatch:
// integer in int32 range -> 125; otherwise float32 if lossless -> 124, else
// float64 -> 123; beyond float64's exact integer range -> BigInt 122.
func TestWrite_anyNumberDispatch(t *testing.T) {
	var tests = []struct {
		name     string
		a        any
		expected string
	}{
		{"int8_neg", int8(-1), "7d41"},
		{"int32_min", int32(math.MinInt32), "7ccf000000"},
		{"int64_2p40", int64(1 << 40), "7c53800000"},
		{"float64_2p40", float64(1 << 40), "7c53800000"},
		{"int64_neg2p40", int64(-(1 << 40)), "7cd3800000"},
		{"int64_2p40p1", int64((1 << 40) + 1), "7b4270000000001000"},
		{"float32_1p5", float32(1.5), "7c3fc00000"},
		{"float64_1p5", float64(1.5), "7c3fc00000"},
		{"float64_1p1", float64(1.1), "7b3ff199999999999a"},
		{"int64_2p53", int64(1 << 53), "7c5a000000"},
		{"int64_2p53p1", int64((1 << 53) + 1), "7a0020000000000001"},
		{"bigint_neg1", lib0.BigInt(-1), "7affffffffffffffff"},
		{"uint64_max", uint64(math.MaxUint64), "7b43f0000000000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := lib0.NewBufferWrite()
			err := w.WriteAny(tt.a)
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}

// TestWriteNegVarUint pins the sign-magnitude negative encoding used by
// UIntOptRle runs, including the -0 case that plain WriteVarInt64 cannot express.
func TestWriteNegVarUint(t *testing.T) {
	var tests = []struct {
		v        uint64
		expected string
	}{
		{0, "40"},    // -0
		{5, "45"},    // -5
		{63, "7f"},   // -63
		{64, "c001"}, // -64
		{128, "c002"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("neg varuint:%d", tt.v), func(t *testing.T) {
			w := lib0.NewBufferWrite()
			err := w.WriteNegVarUint(tt.v)
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(w.ToBytes()))
		})
	}
}
