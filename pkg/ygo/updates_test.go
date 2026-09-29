package ygo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUpdate(t *testing.T) {
	u := NewUpdate()
	assert.Equal(t, 0, len(u.blocks))
	assert.Equal(t, 0, u.deleteSet.Len())
}

func TestUpdateAddNodeBlocksStateVector(t *testing.T) {
	u := NewUpdate()
	u.AddNode(NewSkipNode(ID{Client: 1, Clock: 0}, 2))
	u.AddNode(NewGCNode(ID{Client: 1, Clock: 2}, 3))
	u.AddNode(NewSkipNode(ID{Client: 2, Clock: 0}, 1))

	assert.Len(t, u.Blocks(1), 2)
	sv := u.StateVector()
	assert.Equal(t, Clock(5), sv.Get(1))
	assert.Equal(t, Clock(1), sv.Get(2))
}

func TestUpdateAddDeleteAndDeleteSet(t *testing.T) {
	u := NewUpdate()
	u.AddDelete(7, 11, 2)
	ds := u.DeleteSet()
	assert.Equal(t, 1, ds.Len())
	assert.Equal(t, []DeleteRange{{Clock: 11, Len: 2}}, ds.Get(7))
}

// TestUpdateEncodeDecodeV1LargeIDs exercises client IDs and clocks >= 64,
// which the old signed-VarInt ID encoder silently corrupted.
func TestUpdateEncodeDecodeV1LargeIDs(t *testing.T) {
	const bigClient = ClientID(4294900000)

	u := NewUpdate()
	u.AddNode(NewSkipNode(ID{Client: bigClient, Clock: 0}, 2))
	u.AddNode(NewGCNode(ID{Client: bigClient, Clock: 2}, 3))
	u.AddNode(NewSkipNode(ID{Client: 300, Clock: 0}, 1))
	u.AddDelete(bigClient, 4, 1)
	u.AddDelete(300, 0, 1)

	bin, err := u.EncodeV1()
	require.NoError(t, err)
	decoded, err := DecodeUpdateV1(bin)
	require.NoError(t, err)

	for _, client := range []ClientID{bigClient, 300} {
		want := u.Blocks(client)
		got := decoded.Blocks(client)
		require.Len(t, got, len(want))
		for i := range want {
			assert.Equal(t, want[i].NodeID(), got[i].NodeID())
			assert.Equal(t, want[i].NodeLen(), got[i].NodeLen())
			assert.Equal(t, want[i].IsGC(), got[i].IsGC())
			assert.Equal(t, want[i].IsSkip(), got[i].IsSkip())
		}
	}
	assert.Equal(t, u.DeleteSet().Get(bigClient), decoded.DeleteSet().Get(bigClient))
	assert.Equal(t, u.DeleteSet().Get(300), decoded.DeleteSet().Get(300))
}

func TestUpdateEncodeDecodeV1RoundTrip(t *testing.T) {
	u := NewUpdate()
	u.AddNode(NewSkipNode(ID{Client: 1, Clock: 0}, 2))
	u.AddNode(NewGCNode(ID{Client: 1, Clock: 2}, 3))
	u.AddNode(NewSkipNode(ID{Client: 2, Clock: 0}, 1))
	u.AddDelete(1, 4, 1)
	u.AddDelete(2, 0, 1)

	bin, err := u.EncodeV1()
	require.NoError(t, err)
	decoded, err := DecodeUpdateV1(bin)
	require.NoError(t, err)

	assert.Len(t, decoded.Blocks(1), 2)
	assert.Len(t, decoded.Blocks(2), 1)
	assert.Equal(t, u.DeleteSet().Get(1), decoded.DeleteSet().Get(1))
	assert.Equal(t, u.DeleteSet().Get(2), decoded.DeleteSet().Get(2))
}
