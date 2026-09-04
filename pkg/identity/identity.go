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

	"github.com/doriansobacki/agentpack/internal/registry"
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
	// Interactive reports whether the caller is attached to a user who can
	// answer prompts. Providers that authenticate interactively (device-code
	// flows) must not prompt when it is false — e.g. a sync running from a
	// scheduler — and should return an error directing the user to sign in
	// interactively instead.
	Interactive bool
}

// Provider resolves the current user's identity and group memberships.
type Provider interface {
	// Name is the identifier used in the org manifest (`identity.provider`).
	Name() string
	// Resolve returns the user's identity, or an error if the user cannot
	// be identified (unknown email, failed auth, ...).
	Resolve(ctx context.Context, req Request) (*Identity, error)
}

var providers = registry.New[Provider]("identity provider")

// Register makes a provider available by name. Registering the same name
// twice panics: it is a programmer error in wiring, not a runtime condition.
func Register(p Provider) { providers.Register(p) }

// Get returns the provider registered under name.
func Get(name string) (Provider, error) { return providers.Get(name) }

// Names lists registered providers, sorted.
func Names() []string { return providers.Names() }
