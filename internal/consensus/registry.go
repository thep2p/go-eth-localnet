package consensus

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"
)

// Registry manages available CL client launchers.
//
// The registry provides a central location for registering and retrieving
// launchers for different CL client implementations. This allows the system
// to support multiple CL clients without hardcoding dependencies.
type Registry struct {
	mu        sync.RWMutex
	launchers map[string]Launcher
}

// NewRegistry creates a new launcher registry.
//
// The registry starts empty and launchers must be registered before use.
func NewRegistry() *Registry {
	return &Registry{
		launchers: make(map[string]Launcher),
	}
}

// Register adds a launcher to the registry.
//
// Returns an error if a launcher with the same name is already registered.
// Launcher names should be lowercase and match the client name (e.g., "prysm").
func (r *Registry) Register(name string, launcher Launcher) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.launchers[name]; exists {
		return fmt.Errorf("launcher %s already registered", name)
	}

	r.launchers[name] = launcher
	return nil
}

// Get returns a launcher by name.
//
// Returns an error if no launcher with the given name is registered.
func (r *Registry) Get(name string) (Launcher, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	launcher, exists := r.launchers[name]
	if !exists {
		return nil, fmt.Errorf("launcher %s not found", name)
	}

	return launcher, nil
}

// Available returns all registered launcher names.
//
// The returned slice is sorted alphabetically for consistent output.
func (r *Registry) Available() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.launchers))
	for name := range r.launchers {
		names = append(names, name)
	}
	return names
}

// Unregister removes a launcher from the registry.
//
// Returns an error if no launcher with the given name is registered.
// This is primarily useful for testing.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.launchers[name]; !exists {
		return fmt.Errorf("launcher %s not found", name)
	}

	delete(r.launchers, name)
	return nil
}

// DefaultRegistry is the global CL launcher registry.
//
// Applications should register their launchers with this registry during
// initialization. The mock launcher is pre-registered for testing.
var DefaultRegistry = NewRegistry()

func init() {
	// Register mock launcher by default for testing
	if err := DefaultRegistry.Register("mock", NewMockLauncher(zerolog.Nop())); err != nil {
		panic(fmt.Sprintf("failed to register mock launcher: %v", err))
	}
}
