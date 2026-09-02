// Package registry provides the shared name-keyed registry backing both
// extension points (identity providers and targets), so the concurrency and
// error behavior lives in exactly one place.
package registry

import (
	"fmt"
	"sort"
	"sync"
)

// Named is anything registrable by name.
type Named interface{ Name() string }

// Registry is a concurrency-safe map of names to implementations.
type Registry[T Named] struct {
	kind  string
	mu    sync.RWMutex
	items map[string]T
}

// New returns an empty registry; kind names the extension point in messages
// (e.g. "identity provider", "target").
func New[T Named](kind string) *Registry[T] {
	return &Registry[T]{kind: kind, items: map[string]T{}}
}

// Register adds an implementation. Registering the same name twice panics:
// it is a programmer error in wiring, not a runtime condition.
func (r *Registry[T]) Register(item T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[item.Name()]; exists {
		panic(fmt.Sprintf("%s %q registered twice", r.kind, item.Name()))
	}
	r.items[item.Name()] = item
}

// Get returns the implementation registered under name.
func (r *Registry[T]) Get(name string) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[name]
	if !ok {
		var zero T
		return zero, fmt.Errorf("unknown %s %q (available: %v)", r.kind, name, r.names())
	}
	return item, nil
}

// Names lists registered names, sorted.
func (r *Registry[T]) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.names()
}

func (r *Registry[T]) names() []string {
	names := make([]string, 0, len(r.items))
	for n := range r.items {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
