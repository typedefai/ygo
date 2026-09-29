package ygo_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"riguz.com/ygo/pkg/ygo"
)

func contentRoundTrip(t *testing.T, content ygo.ItemContent) {
	t.Helper()
	encoder := ygo.NewEncoderV1()
	err := encoder.WriteUint8(content.GetRefNumber())
	require.NoError(t, err)
	err = content.Write(&encoder, 0)
	require.NoError(t, err)

	data := encoder.ToBytes()
	decoder := ygo.NewDecoderV1(bytes.NewReader(data))
	tag, err := decoder.ReadUint8()
	require.NoError(t, err)

	decoded, err := ygo.ReadContent(&decoder, tag)
	require.NoError(t, err)
	assert.Equal(t, content.GetRefNumber(), decoded.GetRefNumber())
	assert.Equal(t, content.ClockLen(), decoded.ClockLen())
	assert.Equal(t, content.IsCountable(), decoded.IsCountable())
}

func TestDeletedContent_RoundTrip(t *testing.T) {
	c := &ygo.DeletedContent{Len: 5}
	assert.Equal(t, uint8(1), c.GetRefNumber())
	assert.Equal(t, false, c.IsCountable())
	assert.Equal(t, uint64(5), c.ClockLen())
	contentRoundTrip(t, c)
}

func TestDeletedContent_Split(t *testing.T) {
	c := &ygo.DeletedContent{Len: 10}
	left, right, err := c.Split(3)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), left.ClockLen())
	assert.Equal(t, uint64(7), right.ClockLen())
}

func TestJsonContent_RoundTrip(t *testing.T) {
	c := &ygo.JsonContent{Data: []ygo.Any{
		ygo.StringAny("hello"),
		ygo.UndefinedAny(),
		ygo.IntegerAny(42),
	}}
	assert.Equal(t, uint8(2), c.GetRefNumber())
	assert.Equal(t, true, c.IsCountable())
	assert.Equal(t, uint64(3), c.ClockLen())
	contentRoundTrip(t, c)
}

// TestJsonContent_WireLayout pins Yjs ContentJSON encoding: each element is a
// lib0 Any payload, not a VarString. Regression guard for the previous
// `[]string` + WriteVarString implementation.
func TestJsonContent_WireLayout(t *testing.T) {
	c := &ygo.JsonContent{Data: []ygo.Any{ygo.StringAny("a")}}
	e := ygo.NewEncoderV1()
	require.NoError(t, e.WriteUint8(c.GetRefNumber()))
	require.NoError(t, c.Write(&e, 0))
	assert.Equal(t, "0201770161", hex.EncodeToString(e.ToBytes()))
}

func TestJsonContent_Split(t *testing.T) {
	c := &ygo.JsonContent{Data: []ygo.Any{
		ygo.StringAny("a"), ygo.StringAny("b"), ygo.StringAny("c"), ygo.StringAny("d"),
	}}
	left, right, err := c.Split(2)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), left.ClockLen())
	assert.Equal(t, uint64(2), right.ClockLen())
	assert.Equal(t, ygo.StringAny("a"), left.(*ygo.JsonContent).Data[0])
	assert.Equal(t, ygo.StringAny("c"), right.(*ygo.JsonContent).Data[0])
}

func TestBinaryContent_RoundTrip(t *testing.T) {
	c := &ygo.BinaryContent{Data: []byte{1, 2, 3, 4, 5}}
	assert.Equal(t, uint8(3), c.GetRefNumber())
	assert.Equal(t, true, c.IsCountable())
	assert.Equal(t, uint64(1), c.ClockLen())
	contentRoundTrip(t, c)
}

func TestBinaryContent_SplitNotSupported(t *testing.T) {
	c := &ygo.BinaryContent{Data: []byte{1}}
	_, _, err := c.Split(1)
	assert.Error(t, err)
}

func TestStringContent_RoundTrip(t *testing.T) {
	c := &ygo.StringContent{Data: "hello world"}
	assert.Equal(t, uint8(4), c.GetRefNumber())
	assert.Equal(t, true, c.IsCountable())
	assert.Equal(t, uint64(11), c.ClockLen())
	contentRoundTrip(t, c)
}

func TestStringContent_UTF16Length(t *testing.T) {
	// emoji takes 2 UTF-16 code units
	c := &ygo.StringContent{Data: "a😀b"}
	assert.Equal(t, uint64(4), c.ClockLen()) // a=1, 😀=2, b=1
}

func TestStringContent_Split(t *testing.T) {
	c := &ygo.StringContent{Data: "hello"}
	left, right, err := c.Split(3)
	require.NoError(t, err)
	assert.Equal(t, "hel", left.(*ygo.StringContent).Data)
	assert.Equal(t, "lo", right.(*ygo.StringContent).Data)
}

func TestStringContent_SplitUTF16(t *testing.T) {
	// "a😀b" — split at offset 3 (after the emoji which is 2 UTF-16 units)
	c := &ygo.StringContent{Data: "a😀b"}
	left, right, err := c.Split(3)
	require.NoError(t, err)
	assert.Equal(t, "a😀", left.(*ygo.StringContent).Data)
	assert.Equal(t, "b", right.(*ygo.StringContent).Data)
}

func TestEmbedContent_RoundTrip(t *testing.T) {
	c := &ygo.EmbedContent{Data: ygo.StringAny("{\"key\":\"value\"}")}
	assert.Equal(t, uint8(5), c.GetRefNumber())
	assert.Equal(t, true, c.IsCountable())
	assert.Equal(t, uint64(1), c.ClockLen())
	contentRoundTrip(t, c)
}

func TestFormatContent_RoundTrip(t *testing.T) {
	c := &ygo.FormatContent{Key: "bold", Value: ygo.StringAny("true")}
	assert.Equal(t, uint8(6), c.GetRefNumber())
	assert.Equal(t, false, c.IsCountable())
	assert.Equal(t, uint64(1), c.ClockLen())
	contentRoundTrip(t, c)
}

func TestTypeContent_RoundTrip(t *testing.T) {
	c := &ygo.TypeContent{TypeRef: ygo.NewYTypeRef(ygo.YTypeText, nil)}
	assert.Equal(t, uint8(7), c.GetRefNumber())
	assert.Equal(t, true, c.IsCountable())
	assert.Equal(t, uint64(1), c.ClockLen())
	contentRoundTrip(t, c)
}

func TestTypeContent_XMLElement_RoundTrip(t *testing.T) {
	tag := "div"
	c := &ygo.TypeContent{TypeRef: ygo.NewYTypeRef(ygo.YTypeXMLElement, &tag)}
	contentRoundTrip(t, c)
}

func TestTypeContent_UnknownType(t *testing.T) {
	encoder := ygo.NewEncoderV1()
	_ = encoder.WriteUint8(7) // type ref number
	_ = encoder.WriteVarUint64(999)
	data := encoder.ToBytes()
	decoder := ygo.NewDecoderV1(bytes.NewReader(data))
	_, _ = decoder.ReadUint8()
	_, err := ygo.ReadContent(&decoder, 7)
	assert.Error(t, err)
}

func TestAnyContent_RoundTrip(t *testing.T) {
	c := &ygo.AnyContent{Data: []ygo.Any{
		ygo.IntegerAny(42),
		ygo.StringAny("test"),
		ygo.TrueAny(),
	}}
	assert.Equal(t, uint8(8), c.GetRefNumber())
	assert.Equal(t, true, c.IsCountable())
	assert.Equal(t, uint64(3), c.ClockLen())
	contentRoundTrip(t, c)
}

func TestAnyContent_Split(t *testing.T) {
	c := &ygo.AnyContent{Data: []ygo.Any{
		ygo.IntegerAny(1),
		ygo.IntegerAny(2),
		ygo.IntegerAny(3),
	}}
	left, right, err := c.Split(2)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), left.ClockLen())
	assert.Equal(t, uint64(1), right.ClockLen())
}

func TestDocContent_RoundTrip(t *testing.T) {
	c := &ygo.DocContent{Guid: "test-guid-123", Opts: ygo.NullAny()}
	assert.Equal(t, uint8(9), c.GetRefNumber())
	assert.Equal(t, true, c.IsCountable())
	assert.Equal(t, uint64(1), c.ClockLen())
	contentRoundTrip(t, c)
}

func TestReadContent_UnknownTag(t *testing.T) {
	encoder := ygo.NewEncoderV1()
	data := encoder.ToBytes()
	decoder := ygo.NewDecoderV1(bytes.NewReader(data))
	_, err := ygo.ReadContent(&decoder, 99)
	assert.Error(t, err)
}
