package killswitch

import (
	"context"
	"sync"
)

// Killswitch is used to signal that a feature needs to be forcibly disabled.
//
// Practically, this is used to disable a feature when something is believed
// to be wrong with it.
type Killswitch interface {
	Enabled(ctx context.Context, feature string) bool
}

// Memory is a simple in-memory killswitch implementation.
type Memory struct {
	mut   sync.RWMutex
	state map[string]bool
}

// NewMemory allocates a new Memory.
func NewMemory() *Memory {
	return &Memory{state: map[string]bool{}}
}

// Set sets the killswitch for a given feature.
func (m *Memory) Set(feature string) {
	m.mut.Lock()
	defer m.mut.Unlock()
	m.state[feature] = true
}

// Enabled implements the Killswitch interface.
func (m *Memory) Enabled(ctx context.Context, feature string) bool {
	m.mut.Lock()
	defer m.mut.Unlock()
	return m.state[feature]
}
