// Package identity defines the extension point for resolving who the current
// user is and which groups they belong to.
//
// Built-in providers live in subpackages (e.g. the static provider, which
// reads group membership from the org manifest). External providers — an IdP
// like Microsoft Entra, Okta, or a custom directory service — implement
// Provider and register themselves, either in-tree or from a custom binary
// that imports this package.
package identity

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Identity is the resolved user: an email plus the groups they belong to.
type Identity struct {
	Email  string
	Groups []string
}

// Request carries everything a provider may need to resolve an identity.
type Request struct {
	// Email is the locally configured email (`agentpack login <email>`).
	// Interactive providers (device-code flows) may ignore it and derive
	// the identity from the authentication result instead.
	Email string
	// Options are the provider-specific options from the org manifest's
	// `identity:` section, passed through verbatim.
	Options map[string]any
	// StaticUsers is the `users:` map from the org manifest (email -> groups).
	// Only the static provider consumes it; IdP-backed providers should use
	// their own directory instead.
	StaticUsers map[string][]string
}

// Provider resolves the current user's identity and group memberships.
type Provider interface {
	// Name is the identifier used in the org manifest (`identity.provider`).
	Name() string
	// Resolve returns the user's identity, or an error if the user cannot
	// be identified (unknown email, failed auth, ...).
	Resolve(ctx context.Context, req Request) (*Identity, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Provider{}
)

// Register makes a provider available by name. Registering the same name
// twice panics: it is a programmer error in wiring, not a runtime condition.
func Register(p Provider) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[p.Name()]; exists {
		panic(fmt.Sprintf("identity: provider %q registered twice", p.Name()))
	}
	registry[p.Name()] = p
}

// Get returns the provider registered under name.
func Get(name string) (Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("identity: unknown provider %q (available: %v)", name, Names())
	}
	return p, nil
}

// Names lists registered providers, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
