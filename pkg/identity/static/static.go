// Package static implements the built-in identity provider that reads group
// membership from the org manifest's `users:` map. It is the zero-setup
// default; organizations with an IdP should switch to a directory-backed
// provider once one is configured.
package static

import (
	"context"
	"fmt"
	"strings"

	"github.com/doriansobacki/agentpack/pkg/identity"
)

// Provider resolves groups from the org manifest's users map.
type Provider struct{}

// New returns the static provider.
func New() *Provider { return &Provider{} }

// Name implements identity.Provider.
func (*Provider) Name() string { return "static" }

// Resolve implements identity.Provider. Email matching is case-insensitive.
func (*Provider) Resolve(_ context.Context, req identity.Request) (*identity.Identity, error) {
	if req.Email == "" {
		return nil, fmt.Errorf("static provider: no email configured; run `agentpack login <email>`")
	}
	want := strings.ToLower(req.Email)
	for email, groups := range req.StaticUsers {
		if strings.ToLower(email) == want {
			return &identity.Identity{Email: req.Email, Groups: groups}, nil
		}
	}
	// Unknown users still get the identity — they receive only the packages
	// mapped to the wildcard group. The syncer surfaces a warning.
	return &identity.Identity{Email: req.Email, Groups: nil}, nil
}
