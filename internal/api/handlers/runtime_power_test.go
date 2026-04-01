package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindByContainerID_FullID(t *testing.T) {
	r := NewRuntimeStore()
	r.Set("node-1", "abc123def456abc123def456abc123def456abc123def456abc123def456abc1", 100)

	ref, ok := r.FindByContainerID("abc123def456abc123def456abc123def456abc123def456abc123def456abc1")

	assert.True(t, ok)
	assert.Equal(t, "node-1", ref.NodeID)
}

func TestFindByContainerID_ShortID(t *testing.T) {
	r := NewRuntimeStore()
	r.Set("node-1", "abc123def456abc123def456abc123def456abc123def456abc123def456abc1", 100)

	ref, ok := r.FindByContainerID("abc123def456")

	assert.True(t, ok)
	assert.Equal(t, "node-1", ref.NodeID)
}

func TestFindByContainerID_Miss(t *testing.T) {
	r := NewRuntimeStore()
	r.Set("node-1", "abc123def456abc123def456abc123def456abc123def456abc123def456abc1", 100)

	_, ok := r.FindByContainerID("000000000000000000000000000000000000000000000000000000000000000")

	assert.False(t, ok)
}

func TestFindByContainerID_EmptyStore(t *testing.T) {
	r := NewRuntimeStore()

	_, ok := r.FindByContainerID("abc123def456")

	assert.False(t, ok)
}
