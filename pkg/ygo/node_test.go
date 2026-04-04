package ygo

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeConstructorsAndAccessors(t *testing.T) {
	gc := NewGCNode(ID{Client: 1, Clock: 2}, 3)
	skip := NewSkipNode(ID{Client: 5, Clock: 7}, 11)

	name := "text"
	item := &Item{
		ID:      ID{Client: 9, Clock: 10},
		Parent:  &Parent{Named: &name},
		Content: &StringContent{Data: "ab"},
	}
	inode := NewItemNode(item)

	assert.True(t, gc.IsGC())
	assert.True(t, skip.IsSkip())
	assert.True(t, inode.IsItem())

	assert.Equal(t, ClientID(1), gc.Client())
	assert.Equal(t, Clock(2), gc.Clock())
	assert.Equal(t, uint64(3), gc.NodeLen())
	assert.True(t, gc.IsDeleted())

	assert.Equal(t, ClientID(5), skip.Client())
	assert.Equal(t, Clock(7), skip.Clock())
	assert.Equal(t, uint64(11), skip.NodeLen())
	assert.False(t, skip.IsDeleted())

	assert.Equal(t, ClientID(9), inode.Client())
	assert.Equal(t, Clock(10), inode.Clock())
	assert.Equal(t, uint64(2), inode.NodeLen())
	assert.NotNil(t, inode.AsItem())
	assert.False(t, inode.IsDeleted())
	lid := inode.LastID()
	require.NotNil(t, lid)
	assert.Equal(t, ID{Client: 9, Clock: 11}, *lid)
}

func TestNodeWriteReadGCAndSkip(t *testing.T) {
	nodes := []Node{
		NewGCNode(ID{Client: 1, Clock: 0}, 2),
		NewSkipNode(ID{Client: 2, Clock: 3}, 4),
	}
	for _, n := range nodes {
		enc := NewEncoderV1()
		err := n.WriteNode(&enc)
		require.NoError(t, err)

		dec := NewDecoderV1(bytes.NewReader(enc.ToBytes()))
		decoded, err := ReadNode(&dec, n.NodeID())
		require.NoError(t, err)
		assert.Equal(t, n.NodeID(), decoded.NodeID())
		assert.Equal(t, n.NodeLen(), decoded.NodeLen())
	}
}

func TestNodeWriteReadItem(t *testing.T) {
	name := "text"
	i := &Item{
		ID:      ID{Client: 3, Clock: 5},
		Parent:  &Parent{Named: &name},
		Content: &StringContent{Data: "hello"},
	}
	n := NewItemNode(i)

	enc := NewEncoderV1()
	err := n.WriteNode(&enc)
	require.NoError(t, err)

	dec := NewDecoderV1(bytes.NewReader(enc.ToBytes()))
	decoded, err := ReadNode(&dec, i.ID)
	require.NoError(t, err)
	di := decoded.AsItem()
	require.NotNil(t, di)
	assert.Equal(t, i.ID, di.ID)
	assert.Equal(t, uint64(5), di.Len())
	assert.Equal(t, "hello", di.Content.(*StringContent).Data)
}
