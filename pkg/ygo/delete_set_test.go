package ygo

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDeleteSet(t *testing.T) {
	ds := NewDeleteSet()
	assert.Equal(t, 0, ds.Len())
}

func TestDeleteSetAddGetLen(t *testing.T) {
	ds := NewDeleteSet()
	ds.Add(1, 10, 5)
	ds.Add(1, 20, 1)
	ds.Add(2, 3, 2)
	ds.Add(2, 99, 0)

	assert.Equal(t, 2, ds.Len())
	assert.Len(t, ds.Get(1), 2)
	assert.Len(t, ds.Get(2), 1)
}

func TestDeleteSetEncodeDecode(t *testing.T) {
	ds := NewDeleteSet()
	ds.Add(2, 3, 2)
	ds.Add(1, 10, 5)

	enc := NewEncoderV1()
	err := ds.Encode(&enc)
	require.NoError(t, err)

	dec := NewDecoderV1(bytes.NewReader(enc.ToBytes()))
	decoded, err := DecodeDeleteSet(&dec)
	require.NoError(t, err)

	assert.Equal(t, 2, decoded.Len())
	assert.Equal(t, ds.Get(1), decoded.Get(1))
	assert.Equal(t, ds.Get(2), decoded.Get(2))
}
