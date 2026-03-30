package results

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// capability_id: rainbond.worker.thirdcomponent.prober.manage-results-cache
func TestCacheOperations(t *testing.T) {
	updates := make(chan Update, 1)
	m := NewManager(updates)

	unsetID := "unset"
	setID := "set"

	_, found := m.Get(unsetID)
	assert.False(t, found, "unset result found")

	m.Set(setID, Success)
	result, found := m.Get(setID)
	assert.True(t, result == Success, "set result")
	assert.True(t, found, "set result found")
	select {
	case update := <-updates:
		assert.Equal(t, setID, update.EndpointID)
		assert.Equal(t, Success, update.Result)
	default:
		t.Fatal("expected probe cache update")
	}

	m.Remove(setID)
	_, found = m.Get(setID)
	assert.False(t, found, "removed result found")
}
