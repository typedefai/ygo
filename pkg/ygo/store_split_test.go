package ygo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stringItem(client ClientID, clock uint64, s string) Node {
	return NewItemNode(&Item{
		ID:      ID{Client: client, Clock: clock},
		Content: &StringContent{Data: s},
		Parent:  &Parent{},
		Info:    NewItemFlags(0),
	})
}

func TestBlockStoreFind(t *testing.T) {
	bs := NewBlockStore()
	bs.Append(stringItem(1, 0, "hello")) // clocks 0-5
	bs.Append(stringItem(1, 5, "world")) // clocks 5-10

	node, ok := bs.Find(ID{Client: 1, Clock: 0})
	require.True(t, ok)
	assert.Equal(t, Clock(0), node.Clock())

	node, ok = bs.Find(ID{Client: 1, Clock: 7})
	require.True(t, ok)
	assert.Equal(t, Clock(5), node.Clock())

	_, ok = bs.Find(ID{Client: 1, Clock: 10})
	assert.False(t, ok)
	_, ok = bs.Find(ID{Client: 2, Clock: 0})
	assert.False(t, ok)
}

func TestBlockStoreAppendKeepsOrder(t *testing.T) {
	bs := NewBlockStore()
	bs.Append(stringItem(1, 5, "world"))
	bs.Append(stringItem(1, 0, "hello"))

	node, ok := bs.Find(ID{Client: 1, Clock: 2})
	require.True(t, ok)
	assert.Equal(t, Clock(0), node.Clock())

	list, _ := bs.Get(1)
	assert.Equal(t, Clock(10), list.GetState())
}

func TestGetItemCleanStartSplitsString(t *testing.T) {
	bs := NewBlockStore()
	bs.Append(stringItem(1, 0, "hello"))

	node, err := bs.GetItemCleanStart(ID{Client: 1, Clock: 2})
	require.NoError(t, err)

	item := node.AsItem()
	require.NotNil(t, item)
	assert.Equal(t, Clock(2), item.ID.Clock)
	assert.Equal(t, "llo", item.Content.(*StringContent).Data)
	require.NotNil(t, item.Origin)
	assert.Equal(t, ID{Client: 1, Clock: 1}, *item.Origin)

	list, _ := bs.Get(1)
	assert.Equal(t, 2, list.Len())
	assert.Equal(t, Clock(5), list.GetState())

	left, ok := list.Get(0)
	require.True(t, ok)
	assert.Equal(t, "he", left.AsItem().Content.(*StringContent).Data)
}

func TestGetItemCleanStartAlignedIsNoop(t *testing.T) {
	bs := NewBlockStore()
	bs.Append(stringItem(1, 0, "hello"))

	node, err := bs.GetItemCleanStart(ID{Client: 1, Clock: 0})
	require.NoError(t, err)
	assert.Equal(t, Clock(0), node.Clock())
	list, _ := bs.Get(1)
	assert.Equal(t, 1, list.Len())
}

func TestGetItemCleanEndSplitsString(t *testing.T) {
	bs := NewBlockStore()
	bs.Append(stringItem(1, 0, "hello"))

	node, err := bs.GetItemCleanEnd(ID{Client: 1, Clock: 1})
	require.NoError(t, err)
	assert.Equal(t, Clock(0), node.Clock())
	assert.Equal(t, "he", node.AsItem().Content.(*StringContent).Data)

	list, _ := bs.Get(1)
	assert.Equal(t, 2, list.Len())
	right, _ := list.Get(1)
	assert.Equal(t, Clock(2), right.Clock())
	assert.Equal(t, "llo", right.AsItem().Content.(*StringContent).Data)
}

func TestSplitGCSkip(t *testing.T) {
	bs := NewBlockStore()
	bs.Append(NewGCNode(ID{Client: 2, Clock: 0}, 8))
	bs.Append(NewSkipNode(ID{Client: 3, Clock: 0}, 6))

	gc, err := bs.GetItemCleanStart(ID{Client: 2, Clock: 3})
	require.NoError(t, err)
	assert.True(t, gc.IsGC())
	assert.Equal(t, Clock(3), gc.Clock())
	assert.Equal(t, uint64(5), gc.NodeLen())

	sk, err := bs.GetItemCleanEnd(ID{Client: 3, Clock: 2})
	require.NoError(t, err)
	assert.True(t, sk.IsSkip())
	assert.Equal(t, Clock(0), sk.Clock())
	assert.Equal(t, uint64(3), sk.NodeLen())
}

func TestGetItemCleanStartMissing(t *testing.T) {
	bs := NewBlockStore()
	_, err := bs.GetItemCleanStart(ID{Client: 9, Clock: 0})
	require.Error(t, err)
}

func TestStateVectorRoundTrip(t *testing.T) {
	sv := NewStateVector()
	sv.SetMax(1, 5)
	sv.SetMax(300, 7)

	data, err := EncodeStateVectorV1(sv)
	require.NoError(t, err)

	got, err := DecodeStateVector(data)
	require.NoError(t, err)
	assert.Equal(t, 2, got.Len())
	assert.Equal(t, Clock(5), got.Get(1))
	assert.Equal(t, Clock(7), got.Get(300))
}

func TestEncodeStateVectorFromUpdateStopsAtGap(t *testing.T) {
	bs := NewBlockStore()
	bs.Append(stringItem(1, 0, "ab"))
	bs.Append(NewSkipNode(ID{Client: 1, Clock: 2}, 3))
	bs.Append(stringItem(1, 5, "xyz"))

	sv := stateVectorFromUpdate(Update{blocks: map[ClientID][]Node{1: bs.clients[1].list}})
	assert.Equal(t, Clock(2), sv.Get(1))
}
