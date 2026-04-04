package ygo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDocPublisher(t *testing.T) {
	p := NewDocPublisher()
	assert.NotNil(t, p)
}
