package ygo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDocOptions_DefaultsAndOptions(t *testing.T) {
	opts, err := NewDocOptions(WithClientId(42), WithGuid("g-1"), WithGc(true))
	require.NoError(t, err)
	assert.Equal(t, uint64(42), opts.ClientId)
	assert.Equal(t, "g-1", opts.Guid)
	assert.True(t, opts.Gc)
}

func TestNewDocAndAccessors(t *testing.T) {
	d, err := NewDoc()
	require.NoError(t, err)
	assert.NotNil(t, d)
	assert.NotNil(t, d.Store())
	assert.NotNil(t, d.Publisher())
	assert.NotEqual(t, uint64(0), d.ClientID())
	assert.Equal(t, d.ClientID(), d.Options().ClientId)
}

func TestNewDocWithOptions(t *testing.T) {
	d := NewDocWithOptions(DocOptions{ClientId: 99, Guid: "abc", Gc: false})
	assert.Equal(t, uint64(99), d.ClientID())
	assert.Equal(t, "abc", d.Options().Guid)
	assert.NotNil(t, d.Store())
	assert.NotNil(t, d.Publisher())
}
