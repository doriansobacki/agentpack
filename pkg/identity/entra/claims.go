package entra

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// idTokenClaims are the ID-token claims this provider consumes. Groups holds
// group object IDs; Entra never puts display names in the token.
type idTokenClaims struct {
	// PreferredUsername is usually the user's email/UPN in Entra tenants.
	PreferredUsername string `json:"preferred_username"`
	// UPN is the user principal name, emitted by some claim configurations.
	UPN string `json:"upn"`
	// Groups is the `groups` claim: object IDs of the user's groups. nil when
	// the claim is absent (not configured on the app, or omitted for overage).
	Groups []string `json:"groups"`
	// ClaimNames is `_claim_names`, set by Entra when a claim was moved out of
	// the token (groups overage: past ~200 groups the token names a Graph
	// endpoint in `_claim_sources` instead of embedding the claim).
	ClaimNames map[string]string `json:"_claim_names"`
}

// parseIDTokenClaims decodes the claims from a compact-serialized JWT. The
// signature is not verified: the token was just handed to us by the identity
// provider over TLS, and we consume it locally rather than accept it as proof
// from a third party.
func parseIDTokenClaims(raw string) (*idTokenClaims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a compact JWT (%d segments, want 3)", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding claims segment: %w", err)
	}
	claims := &idTokenClaims{}
	if err := json.Unmarshal(payload, claims); err != nil {
		return nil, fmt.Errorf("unmarshaling claims: %w", err)
	}
	return claims, nil
}

// email returns the user's email, preferring preferred_username over upn.
func (c *idTokenClaims) email() string {
	if c.PreferredUsername != "" {
		return c.PreferredUsername
	}
	return c.UPN
}

// groupsIncomplete reports whether the token's groups claim cannot be trusted
// as the full membership list: either the tenant signaled overage via
// _claim_names, or the claim is absent entirely.
func (c *idTokenClaims) groupsIncomplete() bool {
	if _, overage := c.ClaimNames["groups"]; overage {
		return true
	}
	return c.Groups == nil
}
