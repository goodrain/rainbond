package results

import (
	"sync"
)

// Manager provides a probe results cache and channel of updates.
type Manager interface {
	// Get returns the cached result for the endpoint.
	Get(endpointID string) (Result, bool)
	// Set sets the cached result for the endpoint.
	Set(endpointID string, result Result)
	// Remove clears the cached result for the endpoint.
	Remove(endpointID string)
}

// Result is the type for probe results.
type Result int

const (
	// Unknown is encoded as -1 (type Result)
	Unknown Result = iota - 1

	// Success is encoded as 0 (type Result)
	Success

	// Failure is encoded as 1 (type Result)
	Failure
)

func (r Result) String() string {
	switch r {
	case Success:
		return "Success"
	case Failure:
		return "Failure"
	default:
		return "UNKNOWN"
	}
}

// ToPrometheusType translates a Result to a form which is better understood by prometheus.
func (r Result) ToPrometheusType() float64 {
	switch r {
	case Success:
		return 0
	case Failure:
		return 1
	default:
		return -1
	}
}

// Update is an enum of the types of updates sent over the Updates channel.
type Update struct {
	EndpointID string
	Result     Result
}

// Manager implementation.
type manager struct {
	// guards the cache
	sync.RWMutex
	// map of endpoint ID -> probe Result
	cache map[string]Result
	// channel of updates
	updates chan Update
}

var _ Manager = &manager{}

// NewManager creates and returns an empty results manager.
func NewManager(updates chan Update) Manager {
	return &manager{
		cache:   make(map[string]Result),
		updates: updates,
	}
}

func (m *manager) Get(id string) (Result, bool) {
	m.RLock()
	defer m.RUnlock()
	result, found := m.cache[id]
	return result, found
}

func (m *manager) Set(id string, result Result) {
	if m.setInternal(id, result) {
		m.updates <- Update{EndpointID: id, Result: result}
	}
}

func (m *manager) setInternal(id string, result Result) bool {
	m.Lock()
	defer m.Unlock()
	prev, exists := m.cache[id]
	if !exists || prev != result {
		m.cache[id] = result
		return true
	}
	return false
}

func (m *manager) Remove(id string) {
	m.Lock()
	defer m.Unlock()
	delete(m.cache, id)
}
