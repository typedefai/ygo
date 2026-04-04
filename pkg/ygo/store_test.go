package ygo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDocStore(t *testing.T) {
	s := NewDocStore()
	assert.NotNil(t, s)
	assert.NotNil(t, s.blocks)
}

func TestDocStore_GetStateVector_Empty(t *testing.T) {
	s := NewDocStore()
	sv := s.GetStateVector()
	assert.True(t, sv.IsEmpty())
}

func TestDocStore_AppendNode(t *testing.T) {
	s := NewDocStore()
	s.AppendNode(NewSkipNode(ID{Client: 3, Clock: 0}, 4))
	sv := s.GetStateVector()
	assert.Equal(t, Clock(4), sv.Get(3))
}
