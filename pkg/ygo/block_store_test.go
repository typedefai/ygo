package ygo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBlockStore(t *testing.T) {
	bs := NewBlockStore()
	assert.NotNil(t, bs)
	assert.NotNil(t, bs.clients)
}

func TestClientBlockListGetStateEmpty(t *testing.T) {
	list := ClientBlockList{}
	assert.Equal(t, Clock(0), list.GetState())
}

func TestClientBlockListAppendGetLenAndState(t *testing.T) {
	list := ClientBlockList{}
	n1 := NewSkipNode(ID{Client: 1, Clock: 0}, 3)
	n2 := NewGCNode(ID{Client: 1, Clock: 3}, 5)
	list.Append(n1)
	list.Append(n2)

	assert.Equal(t, 2, list.Len())
	got, ok := list.Get(1)
	assert.True(t, ok)
	assert.True(t, got.IsGC())
	_, ok = list.Get(2)
	assert.False(t, ok)

	assert.Equal(t, Clock(8), list.GetState())
}

func TestBlockStoreAppendAndGet(t *testing.T) {
	bs := NewBlockStore()
	n := NewSkipNode(ID{Client: 7, Clock: 10}, 2)
	bs.Append(n)

	list, ok := bs.Get(7)
	assert.True(t, ok)
	assert.Equal(t, 1, list.Len())
}

func TestBlockStoreGetStateVector(t *testing.T) {
	bs := NewBlockStore()
	bs.Append(NewSkipNode(ID{Client: 1, Clock: 0}, 2))
	bs.Append(NewSkipNode(ID{Client: 2, Clock: 0}, 1))
	bs.Append(NewGCNode(ID{Client: 1, Clock: 2}, 3))

	sv := bs.GetStateVector()
	assert.Equal(t, 2, sv.Len())
	assert.Equal(t, Clock(5), sv.Get(1))
	assert.Equal(t, Clock(1), sv.Get(2))
}

func TestNewStateVectorFromNilStore(t *testing.T) {
	sv := NewStateVectorFrom(nil)
	assert.True(t, sv.IsEmpty())
}

func TestStateVectorMethods(t *testing.T) {
	sv := NewStateVector()
	assert.True(t, sv.IsEmpty())

	sv.SetMax(1, 10)
	assert.Equal(t, 1, sv.Len())
	assert.Equal(t, Clock(10), sv.Get(1))
	assert.True(t, sv.Contains(ID{Client: 1, Clock: 10}))

	sv.IncreaseBy(1, 2)
	assert.Equal(t, Clock(12), sv.Get(1))

	sv.SetMin(1, 5)
	assert.Equal(t, Clock(5), sv.Get(1))

	other := NewStateVector()
	other.SetMax(1, 8)
	other.SetMax(2, 3)
	sv.Merge(&other)
	assert.Equal(t, Clock(8), sv.Get(1))
	assert.Equal(t, Clock(3), sv.Get(2))
}

func TestStateVectorEncode(t *testing.T) {
	sv := NewStateVector()
	sv.SetMax(1, 2)
	sv.SetMax(2, 3)

	enc := NewEncoderV1()
	err := sv.Encode(&enc)
	assert.NoError(t, err)
	assert.NotEmpty(t, enc.ToBytes())
}
