package ygo_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"gotest.tools/assert"
	"riguz.com/ygo/pkg/ygo"
)

func TestWriteClientIsVarUint(t *testing.T) {
	var tests = []struct {
		client   ygo.ClientID
		expected string
	}{
		{0, "00"},
		{63, "3f"},
		{64, "40"},
		{65, "41"},
		{300, "ac02"},
		{1000, "e807"},
		{4294900000, "a0f2fbff0f"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write client:%d", tt.client), func(t *testing.T) {
			e := ygo.NewEncoderV1()
			err := e.WriteClient(tt.client)
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(e.ToBytes()))
		})
	}
}

func TestWriteIdIsVarUint(t *testing.T) {
	var tests = []struct {
		id       ygo.ID
		expected string
	}{
		{ygo.ID{Client: 1, Clock: 65}, "0141"},
		{ygo.ID{Client: 300, Clock: 128}, "ac028001"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write id:%v", tt.id), func(t *testing.T) {
			e := ygo.NewEncoderV1()
			err := e.WriteId(tt.id)
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(e.ToBytes()))
		})
	}
}

// TestClientAndIdRoundTrip guards the VarInt/VarUint mismatch that silently
// corrupted every client ID or clock >= 64 when the encoder used WriteVarInt
// while the decoder used ReadVarUint.
// TestEmptyV2Layout pins the column framing: a feature byte, nine
// length-prefixed columns, and a raw (un-prefixed) trailing "rest" column.
// The previous code length-prefixed the rest column as well.
func TestEmptyV2Layout(t *testing.T) {
	e := ygo.NewEncoderV2()
	b, err := e.ToBytes()
	assert.NilError(t, err)
	assert.Equal(t, "0000000000000100000000", hex.EncodeToString(b))
}

func TestClientAndIdRoundTrip(t *testing.T) {
	for _, c := range []ygo.ClientID{0, 63, 64, 65, 300, 1000, 4294900000} {
		e := ygo.NewEncoderV1()
		assert.NilError(t, e.WriteClient(c))
		d := ygo.NewDecoderV1(bytes.NewReader(e.ToBytes()))
		got, err := d.ReadClient()
		assert.NilError(t, err)
		assert.Equal(t, c, got)
	}
	for _, id := range []ygo.ID{{Client: 1, Clock: 65}, {Client: 300, Clock: 128}} {
		e := ygo.NewEncoderV1()
		assert.NilError(t, e.WriteId(id))
		d := ygo.NewDecoderV1(bytes.NewReader(e.ToBytes()))
		got, err := d.ReadLeftId()
		assert.NilError(t, err)
		assert.Equal(t, id, got)
	}
}

func TestIntDiffOptRleEncoder_write(t *testing.T) {
	var tests = []struct {
		numbers  []uint32
		expected string
	}{
		{[]uint32{}, ""},
		{[]uint32{1, 2, 3, 2}, "030142"},
		{[]uint32{1, 2, 3, 2, 2, 2, 2, 2, 2, 2}, "0301420104"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write IntDiffOptRleEncoder:%s", tt.expected), func(t *testing.T) {
			encoder := ygo.NewIntDiffOptRleEncoder()
			for _, e := range tt.numbers {
				err := encoder.Write(e)
				assert.NilError(t, err)
			}
			result, err := encoder.ToBytes()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(result))
		})
	}
}

func TestUIntOptRleEncoder_write(t *testing.T) {
	var tests = []struct {
		numbers  []uint64
		expected string
	}{
		{[]uint64{}, ""},
		{[]uint64{1, 2, 3, 3, 3}, "01024301"},
		{[]uint64{1, 2, 3, 65535, 18273719133}, "010203bfff079dcd96938801"},
		// A run of zeros must encode -0 (0x40) as the run marker; collapsing it
		// to +0 (0x00) makes the decoder read the count as the next value.
		{[]uint64{0, 0}, "4000"},
		{[]uint64{0, 0, 0}, "4001"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write UIntOptRleEncoder:%s", tt.expected), func(t *testing.T) {
			encoder := ygo.NewUIntOptRleEncoder()
			for _, e := range tt.numbers {
				err := encoder.Write(e)
				assert.NilError(t, err)
			}
			result, err := encoder.ToBytes()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(result))
		})
	}
}

func TestRleEncoder_write(t *testing.T) {
	var tests = []struct {
		numbers  []uint8
		expected string
	}{
		{[]uint8{}, ""},
		{[]uint8{1, 1, 1, 7}, "010207"},
		{[]uint8{1, 2, 3, 255}, "010002000300ff"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write RleEncoder:%s", tt.expected), func(t *testing.T) {
			encoder := ygo.NewRleEncoder()
			for _, e := range tt.numbers {
				err := encoder.Write(e)
				assert.NilError(t, err)
			}
			result := encoder.ToBytes()
			assert.Equal(t, tt.expected, hex.EncodeToString(result))
		})
	}
}

func TestStringEncoder_write(t *testing.T) {
	var tests = []struct {
		str      string
		expected string
	}{
		{"", "0000"},
		{"abc", "0361626303"},
		{"Hello world!", "0c48656c6c6f20776f726c64210c"},
		{"𐐷", "04f09090b702"},
		{"Hello,中国！𐐷𐐷𐐷", "1b48656c6c6f2ce4b8ade59bbdefbc81f09090b7f09090b7f09090b70f"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("write StringEncoder:%s", tt.expected), func(t *testing.T) {
			encoder := ygo.NewStringEncoder()
			err := encoder.Write(&tt.str)
			assert.NilError(t, err)
			result, err := encoder.ToBytes()
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, hex.EncodeToString(result))
		})
	}
}
