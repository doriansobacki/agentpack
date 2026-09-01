// Package static implements the built-in identity provider that reads group
// membership from the org manifest's `users:` map. It is the zero-setup
// default; organizations with an IdP should switch to a directory-backed
// provider once one is configured.
package static

import (
	"context"
	"fmt"
	"sort"
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
// An email absent from the users map is an error, per the Provider contract:
// returning an empty identity instead would make a typo'd login silently
// resolve to wildcard-only packs and prune the user's team content.
func (*Provider) Resolve(_ context.Context, req identity.Request) (*identity.Identity, error) {
	if req.Email == "" {
		return nil, fmt.Errorf("static provider: no email configured; run `agentpack login <email>`")
	}
	var matches []string
	for email := range req.StaticUsers {
		if strings.EqualFold(email, req.Email) {
			matches = append(matches, email)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("static provider: %s is not listed in the org manifest's users: map", req.Email)
	case 1:
		return &identity.Identity{Email: req.Email, Groups: req.StaticUsers[matches[0]]}, nil
	default:
		sort.Strings(matches)
		return nil, fmt.Errorf("static provider: users: contains entries differing only in case (%s); remove the duplicates", strings.Join(matches, ", "))
	}
}
