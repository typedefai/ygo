package lib0_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"gotest.tools/assert"
	"riguz.com/ygo/internal/lib0"
)

func TestRead_uint8array(t *testing.T) {
	var tests = []struct {
		hex         string
		len         uint
		expectedErr string
		expected    []byte
	}{
		{"3f4c5b2a", 0, "", []byte{}},
		{"3f4c5b2a", 1, "", []byte{0x3f}},
		{"3f4c5b2a", 4, "", []byte{0x3f, 0x4c, 0x5b, 0x2a}},
		{"3f4c5b2a", 5, "EOF", nil},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("read uint8array:%d", tt.len), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))

			arr, err := r.ReadUint8Array(tt.len)
			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, hex.EncodeToString(tt.expected), hex.EncodeToString(arr))
			}
		})
	}
}

func TestRead_uint8(t *testing.T) {
	buf, _ := hex.DecodeString("3f4c5b2a")
	r := lib0.NewBufferRead(bytes.NewBuffer(buf))

	var tests = []struct {
		expectedErr string
		expected    uint8
	}{
		{"", 0x3f},
		{"", 0x4c},
		{"", 0x5b},
		{"", 0x2a},
		{"EOF", 0},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("read uint8:%d", tt.expected), func(t *testing.T) {
			value, err := r.ReadUint8()
			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, tt.expected, value)
			}
		})
	}
}

func TestRead_uint16(t *testing.T) {
	var tests = []struct {
		expected uint16
		hex      string
	}{
		{0, "00000000"},
		{1, "01000000"},
		{255, "ff000000"},
		{65535, "ffff0000"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("read uint16:%d", tt.expected), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))
			value, err := r.ReadUint16()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRead_uint32(t *testing.T) {
	var tests = []struct {
		expected uint32
		hex      string
	}{
		{0, "00000000"},
		{1, "01000000"},
		{255, "ff000000"},
		{65535, "ffff0000"},
		{4294967295, "ffffffff"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("read uint32:%d", tt.expected), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))
			value, err := r.ReadUint32()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRead_uint32be(t *testing.T) {
	var tests = []struct {
		expected uint32
		hex      string
	}{
		{0, "00000000"},
		{1, "00000001"},
		{255, "000000ff"},
		{65535, "0000ffff"},
		{4294967295, "ffffffff"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("read uint32be:%d", tt.expected), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))
			value, err := r.ReadUint32BigEndian()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRead_uint64(t *testing.T) {
	var tests = []struct {
		expected uint64
		hex      string
	}{
		{0, "0000000000000000"},
		{1, "0000000000000001"},
		{255, "00000000000000ff"},
		{65535, "000000000000ffff"},
		{4294967295, "00000000ffffffff"},
		{18446744073709551615, "ffffffffffffffff"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("read uint64:%d", tt.expected), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))
			value, err := r.ReadUint64()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRead_float32(t *testing.T) {
	var tests = []struct {
		expected float32
		hex      string
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
		t.Run(fmt.Sprintf("read float32:%f", tt.expected), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))
			value, err := r.ReadFloat32()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRead_float64(t *testing.T) {
	var tests = []struct {
		expected float64
		hex      string
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
		t.Run(fmt.Sprintf("read float64:%f", tt.expected), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))
			value, err := r.ReadFloat64()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRead_int64(t *testing.T) {
	var tests = []struct {
		expected int64
		hex      string
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
		t.Run(fmt.Sprintf("read int64:%d", tt.expected), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))
			value, err := r.ReadInt64()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRead_varUInt(t *testing.T) {
	var tests = []struct {
		expected uint64
		hex      string
	}{
		{0, "00"},
		{1, "01"},
		{255, "ff01"},
		{65535, "ffff03"},
		{4294967295, "ffffffff0f"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("read varUInt:%d", tt.expected), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))
			value, err := r.ReadVarUint()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRead_varIntArray(t *testing.T) {
	buf, _ := hex.DecodeString("04000180fe")
	r := lib0.NewBufferRead(bytes.NewBuffer(buf))
	value, err := r.ReadVarUint8Array()
	assert.NilError(t, err)
	assert.Equal(t, hex.EncodeToString([]byte{0, 1, 128, 254}), hex.EncodeToString(value))
}

func TestRead_varInt(t *testing.T) {
	var tests = []struct {
		expected int64
		hex      string
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
		t.Run(fmt.Sprintf("read varInt:%d", tt.expected), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))
			value, err := r.ReadVarInt()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRead_varString(t *testing.T) {
	var tests = []struct {
		expected string
		hex      string
	}{
		{"Hello World!", "0c48656c6c6f20576f726c6421"},
		{"", "00"},
		{"你好，世界！", "12e4bda0e5a5bdefbc8ce4b896e7958cefbc81"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("read varInt:%s", tt.expected), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))
			value, err := r.ReadVarString()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRead_anyBasic(t *testing.T) {
	var tests = []struct {
		expected any
		hex      string
	}{
		{lib0.Undefined{}, "7f"},
		{nil, "7e"},
		{int64(0), "7d00"},
		{int64(1), "7d01"},
		{int64(-1), "7d41"},
		{int64(2147483647), "7dbfffffff0f"},
		{lib0.BigInt(-9223372036854775808), "7a8000000000000000"},
		{float32(1.9999998807907104), "7c3fffffff"},
		{float64(1.7976931348623157e+308), "7b7fefffffffffffff"},
		{true, "78"},
		{false, "79"},
		{"Hello world!", "770c48656c6c6f20776f726c6421"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("read any:%s", tt.expected), func(t *testing.T) {
			buf, _ := hex.DecodeString(tt.hex)
			r := lib0.NewBufferRead(bytes.NewBuffer(buf))
			value, err := r.ReadAny()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestRead_anyByteArray(t *testing.T) {
	buf, _ := hex.DecodeString("74042a3b4c9d")
	r := lib0.NewBufferRead(bytes.NewBuffer(buf))
	value, err := r.ReadAny()
	assert.NilError(t, err)
	arr, ok := value.([]uint8)
	assert.Equal(t, true, ok)
	assert.Equal(t, hex.EncodeToString([]uint8{0x2a, 0x3b, 0x4c, 0x9d}), hex.EncodeToString(arr))
}

func TestRead_anyMap(t *testing.T) {
	buf, _ := hex.DecodeString("7600")
	r := lib0.NewBufferRead(bytes.NewBuffer(buf))
	value, err := r.ReadAny()
	assert.NilError(t, err)
	obj, ok := value.(map[string]any)
	assert.Equal(t, true, ok)
	assert.Equal(t, 0, len(obj))

	buf, _ = hex.DecodeString("7602046e616d6577064a2e204d6573036167657d12")
	r = lib0.NewBufferRead(bytes.NewBuffer(buf))
	value, err = r.ReadAny()
	assert.NilError(t, err)
	obj, ok = value.(map[string]any)
	assert.Equal(t, true, ok)
	assert.Equal(t, "J. Mes", obj["name"])
	assert.Equal(t, int64(18), obj["age"])
}

func TestRead_anyAnyArray(t *testing.T) {
	buf, _ := hex.DecodeString("75037f7dbf037ccf000000")
	r := lib0.NewBufferRead(bytes.NewBuffer(buf))
	value, err := r.ReadAny()
	assert.NilError(t, err)
	arr, ok := value.([]any)
	assert.Equal(t, true, ok)
	assert.Equal(t, lib0.Undefined{}, arr[0])
	assert.Equal(t, int64(255), arr[1])
	assert.Equal(t, float32(-2147483648), arr[2])
}
