// Package entra implements the identity provider backed by Microsoft Entra ID
// (formerly Azure AD). Users authenticate via the OAuth 2.0 device
// authorization grant; the resolved email comes from the ID token, and group
// membership comes from the token's `groups` claim, falling back to Microsoft
// Graph when the tenant emits a groups-overage token.
//
// The resolved identity's groups are the union of group object IDs and (when
// a Graph lookup ran) group display names, so the org manifest's `groups:`
// keys may use either form. Display names are friendlier; object IDs work
// offline from token claims alone. A Graph lookup runs when the token has no
// usable `groups` claim (overage, or the claim is not configured on the app
// registration) or when the manifest sets `resolve_display_names: true`.
//
// Manifest configuration:
//
//	identity:
//	  provider: entra
//	  tenant_id: <tenant guid>
//	  client_id: <app registration guid>
//	  resolve_display_names: true   # optional, default false
package entra

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/doriansobacki/agentpack/internal/config"
	"github.com/doriansobacki/agentpack/pkg/identity"
)

// Name is this provider's identifier in the org manifest's identity: section.
const Name = "entra"

// CacheFileName is the token-cache file under the agentpack home directory.
const CacheFileName = "entra-token-cache.json"

// scopes requested on every token acquisition. User.Read covers the Graph
// /me/memberOf fallback for tenants that grant it (or GroupMember.Read.All).
var scopes = []string{"openid", "profile", "email", "User.Read"}

// Provider resolves identity and group membership from Microsoft Entra ID.
type Provider struct {
	// newClient builds the token-acquiring client; tests substitute a fake.
	newClient func(tenantID, clientID string) (client, error)
	// graphBaseURL is Microsoft Graph's base URL; tests point it at httptest.
	graphBaseURL string
	// stdout receives the device-code instruction message for the user.
	stdout io.Writer
}

// New returns the Entra provider.
func New() *Provider {
	return &Provider{
		newClient: func(tenantID, clientID string) (client, error) {
			return newMSALClient(tenantID, clientID, filepath.Join(config.Home(), CacheFileName))
		},
		graphBaseURL: graphBaseURL,
		stdout:       os.Stdout,
	}
}

// Name implements identity.Provider.
func (*Provider) Name() string { return Name }

// options are the provider-specific keys from the org manifest.
type options struct {
	TenantID            string
	ClientID            string
	ResolveDisplayNames bool
}

// parseOptions validates the manifest's identity options for this provider.
func parseOptions(raw map[string]any) (*options, error) {
	opts := &options{}
	var err error
	if opts.TenantID, err = stringOption(raw, "tenant_id"); err != nil {
		return nil, err
	}
	if opts.ClientID, err = stringOption(raw, "client_id"); err != nil {
		return nil, err
	}
	if v, ok := raw["resolve_display_names"]; ok {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("entra provider: identity option resolve_display_names must be a boolean, got %T", v)
		}
		opts.ResolveDisplayNames = b
	}
	return opts, nil
}

// stringOption returns the named option, erroring when it is missing, empty,
// or not a string.
func stringOption(raw map[string]any, key string) (string, error) {
	v, ok := raw[key]
	if !ok {
		return "", fmt.Errorf("entra provider: identity option %s is required in the org manifest", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("entra provider: identity option %s must be a string, got %T", key, v)
	}
	if s == "" {
		return "", fmt.Errorf("entra provider: identity option %s must not be empty", key)
	}
	return s, nil
}

// Resolve implements identity.Provider. It tries a silent token acquisition
// from the local cache first; when that fails and the request is interactive,
// it falls back to the device-code flow, printing the verification URL and
// user code. Non-interactive requests never prompt: they fail with an
// instruction to sign in interactively instead.
func (p *Provider) Resolve(ctx context.Context, req identity.Request) (*identity.Identity, error) {
	opts, err := parseOptions(req.Options)
	if err != nil {
		return nil, err
	}
	cl, err := p.newClient(opts.TenantID, opts.ClientID)
	if err != nil {
		return nil, fmt.Errorf("entra provider: creating client: %w", err)
	}

	res, err := cl.AcquireSilent(ctx, scopes)
	if err != nil {
		if !req.Interactive {
			return nil, fmt.Errorf("entra provider: no valid cached sign-in and this run is non-interactive; run `agentpack login` or `agentpack sync` from a terminal to sign in")
		}
		res, err = cl.AcquireDeviceCode(ctx, scopes, func(message string) {
			fmt.Fprintln(p.stdout, message)
		})
		if err != nil {
			return nil, fmt.Errorf("entra provider: device-code sign-in failed: %w", err)
		}
	}

	claims, err := parseIDTokenClaims(res.IDTokenRaw)
	if err != nil {
		return nil, fmt.Errorf("entra provider: parsing ID token: %w", err)
	}
	email := claims.email()
	if email == "" {
		return nil, fmt.Errorf("entra provider: ID token carries neither preferred_username nor upn; cannot determine the user's email")
	}

	groups := claims.Groups
	if opts.ResolveDisplayNames || claims.groupsIncomplete() {
		ids, names, err := fetchGraphGroups(ctx, p.graphBaseURL, res.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("entra provider: reading group membership from Microsoft Graph: %w", err)
		}
		groups = unionGroups(groups, ids, names)
	}
	return &identity.Identity{Email: email, Groups: groups}, nil
}

// unionGroups merges group lists, preserving order and dropping duplicates.
func unionGroups(lists ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range lists {
		for _, g := range list {
			if !seen[g] {
				seen[g] = true
				out = append(out, g)
			}
		}
	}
	return out
}
