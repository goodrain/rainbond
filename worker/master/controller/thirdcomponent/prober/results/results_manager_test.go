package results

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheOperations(t *testing.T) {
	m := NewManager(make(chan Update))

	unsetID := "unset"
	setID := "set"

	_, found := m.Get(unsetID)
	assert.False(t, found, "unset result found")

	m.Set(setID, Success)
	result, found := m.Get(setID)
	assert.True(t, result == Success, "set result")
	assert.True(t, found, "set result found")

	m.Remove(setID)
	_, found = m.Get(setID)
	assert.False(t, found, "removed result found")
}
