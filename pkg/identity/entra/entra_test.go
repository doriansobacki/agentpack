package entra

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doriansobacki/agentpack/pkg/identity"
)

// makeIDToken builds an unsigned compact JWT carrying the given claims, the
// same shape parseIDTokenClaims consumes from a real token.
func makeIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc(payload) + "." + enc([]byte("sig"))
}

// fakeClient implements client for tests.
type fakeClient struct {
	silent       authResult
	silentErr    error
	device       authResult
	deviceErr    error
	deviceCalled bool
}

func (f *fakeClient) AcquireSilent(context.Context, []string) (authResult, error) {
	return f.silent, f.silentErr
}

func (f *fakeClient) AcquireDeviceCode(_ context.Context, _ []string, prompt func(string)) (authResult, error) {
	f.deviceCalled = true
	prompt("To sign in, open https://microsoft.com/devicelogin and enter the code ABC-123")
	return f.device, f.deviceErr
}

// newTestProvider wires a Provider to the fake client and an optional Graph
// test server URL.
func newTestProvider(fake *fakeClient, graphURL string) (*Provider, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return &Provider{
		newClient:    func(_, _ string) (client, error) { return fake, nil },
		graphBaseURL: graphURL,
		stdout:       out,
	}, out
}

func testOptions() map[string]any {
	return map[string]any{
		"tenant_id": "00000000-0000-0000-0000-000000000001",
		"client_id": "00000000-0000-0000-0000-000000000002",
	}
}

func TestParseOptionsErrors(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]any
		want string
	}{
		{"missing tenant_id", map[string]any{"client_id": "x"}, "tenant_id is required"},
		{"missing client_id", map[string]any{"tenant_id": "x"}, "client_id is required"},
		{"non-string tenant_id", map[string]any{"tenant_id": 42, "client_id": "x"}, "tenant_id must be a string"},
		{"empty client_id", map[string]any{"tenant_id": "x", "client_id": ""}, "client_id must not be empty"},
		{"non-bool resolve_display_names", map[string]any{"tenant_id": "x", "client_id": "y", "resolve_display_names": "yes"}, "resolve_display_names must be a boolean"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseOptions(tc.raw)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestParseOptionsResolveDisplayNamesDefaultsFalse(t *testing.T) {
	opts, err := parseOptions(testOptions())
	if err != nil {
		t.Fatal(err)
	}
	if opts.ResolveDisplayNames {
		t.Fatal("resolve_display_names should default to false")
	}
}

func TestClaimsEmailPrefersPreferredUsername(t *testing.T) {
	token := makeIDToken(t, map[string]any{
		"preferred_username": "alice@example.com",
		"upn":                "alice.upn@example.com",
	})
	claims, err := parseIDTokenClaims(token)
	if err != nil {
		t.Fatal(err)
	}
	if got := claims.email(); got != "alice@example.com" {
		t.Fatalf("email = %q", got)
	}
}

func TestClaimsEmailFallsBackToUPN(t *testing.T) {
	token := makeIDToken(t, map[string]any{"upn": "bob@example.com"})
	claims, err := parseIDTokenClaims(token)
	if err != nil {
		t.Fatal(err)
	}
	if got := claims.email(); got != "bob@example.com" {
		t.Fatalf("email = %q", got)
	}
}

func TestClaimsGroups(t *testing.T) {
	token := makeIDToken(t, map[string]any{
		"preferred_username": "a@example.com",
		"groups":             []string{"g1", "g2"},
	})
	claims, err := parseIDTokenClaims(token)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims.Groups) != 2 || claims.Groups[0] != "g1" || claims.Groups[1] != "g2" {
		t.Fatalf("groups = %v", claims.Groups)
	}
	if claims.groupsIncomplete() {
		t.Fatal("groups present without overage should be complete")
	}
}

func TestClaimsOverageDetection(t *testing.T) {
	overage := makeIDToken(t, map[string]any{
		"preferred_username": "a@example.com",
		"_claim_names":       map[string]string{"groups": "src1"},
		"_claim_sources":     map[string]any{"src1": map[string]string{"endpoint": "https://graph.example.com"}},
	})
	claims, err := parseIDTokenClaims(overage)
	if err != nil {
		t.Fatal(err)
	}
	if !claims.groupsIncomplete() {
		t.Fatal("overage token should report incomplete groups")
	}

	absent := makeIDToken(t, map[string]any{"preferred_username": "a@example.com"})
	claims, err = parseIDTokenClaims(absent)
	if err != nil {
		t.Fatal(err)
	}
	if !claims.groupsIncomplete() {
		t.Fatal("token without a groups claim should report incomplete groups")
	}

	empty := makeIDToken(t, map[string]any{"preferred_username": "a@example.com", "groups": []string{}})
	claims, err = parseIDTokenClaims(empty)
	if err != nil {
		t.Fatal(err)
	}
	if claims.groupsIncomplete() {
		t.Fatal("an explicit empty groups claim is complete: the user is in no groups")
	}
}

func TestParseIDTokenClaimsMalformed(t *testing.T) {
	for _, raw := range []string{"", "onesegment", "a.b", "a.!!!.c"} {
		if _, err := parseIDTokenClaims(raw); err == nil {
			t.Fatalf("expected an error for %q", raw)
		}
	}
}

func TestResolveSilentSuccess(t *testing.T) {
	fake := &fakeClient{silent: authResult{
		AccessToken: "at",
		IDTokenRaw: makeIDToken(t, map[string]any{
			"preferred_username": "alice@example.com",
			"groups":             []string{"g1", "g2"},
		}),
	}}
	p, out := newTestProvider(fake, "http://graph.invalid")
	id, err := p.Resolve(context.Background(), identity.Request{Options: testOptions()})
	if err != nil {
		t.Fatal(err)
	}
	if id.Email != "alice@example.com" {
		t.Fatalf("email = %q", id.Email)
	}
	if len(id.Groups) != 2 || id.Groups[0] != "g1" {
		t.Fatalf("groups = %v", id.Groups)
	}
	if fake.deviceCalled {
		t.Fatal("device-code flow ran despite a silent success")
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestResolveNonInteractiveNeverPrompts(t *testing.T) {
	fake := &fakeClient{silentErr: fmt.Errorf("no account in the token cache")}
	p, out := newTestProvider(fake, "http://graph.invalid")
	_, err := p.Resolve(context.Background(), identity.Request{Options: testOptions(), Interactive: false})
	if err == nil || !strings.Contains(err.Error(), "non-interactive") || !strings.Contains(err.Error(), "agentpack login") {
		t.Fatalf("error = %v, want a non-interactive sign-in instruction", err)
	}
	if fake.deviceCalled {
		t.Fatal("device-code flow must not run when Interactive is false")
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestResolveDeviceCodeFallback(t *testing.T) {
	fake := &fakeClient{
		silentErr: fmt.Errorf("no account in the token cache"),
		device: authResult{
			AccessToken: "at",
			IDTokenRaw: makeIDToken(t, map[string]any{
				"preferred_username": "alice@example.com",
				"groups":             []string{"g1"},
			}),
		},
	}
	p, out := newTestProvider(fake, "http://graph.invalid")
	id, err := p.Resolve(context.Background(), identity.Request{Options: testOptions(), Interactive: true})
	if err != nil {
		t.Fatal(err)
	}
	if !fake.deviceCalled {
		t.Fatal("device-code flow did not run")
	}
	if !strings.Contains(out.String(), "devicelogin") || !strings.Contains(out.String(), "ABC-123") {
		t.Fatalf("device-code instructions not printed: %q", out.String())
	}
	if id.Email != "alice@example.com" {
		t.Fatalf("email = %q", id.Email)
	}
}

func TestResolveMissingOptions(t *testing.T) {
	p, _ := newTestProvider(&fakeClient{}, "http://graph.invalid")
	_, err := p.Resolve(context.Background(), identity.Request{Options: map[string]any{}})
	if err == nil || !strings.Contains(err.Error(), "tenant_id is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveMissingEmailClaim(t *testing.T) {
	fake := &fakeClient{silent: authResult{
		IDTokenRaw: makeIDToken(t, map[string]any{"groups": []string{"g1"}}),
	}}
	p, _ := newTestProvider(fake, "http://graph.invalid")
	_, err := p.Resolve(context.Background(), identity.Request{Options: testOptions()})
	if err == nil || !strings.Contains(err.Error(), "preferred_username") {
		t.Fatalf("error = %v", err)
	}
}

// newGraphServer serves two pages of /me/memberOf results, asserting the
// bearer token and the $select projection on the way through.
func newGraphServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer at" {
			t.Errorf("Authorization = %q", got)
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1.0/me/memberOf") && r.URL.Query().Get("page") == "":
			if got := r.URL.Query().Get("$select"); got != "id,displayName" {
				t.Errorf("$select = %q", got)
			}
			fmt.Fprintf(w, `{"@odata.nextLink":%q,"value":[{"id":"gid-1","displayName":"Team A"},{"id":"gid-2","displayName":"Team B"}]}`,
				srv.URL+"/v1.0/me/memberOf?page=2")
		case r.URL.Query().Get("page") == "2":
			fmt.Fprint(w, `{"value":[{"id":"gid-3","displayName":"Team C"},{"id":"role-1"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveGraphFallbackOnOverage(t *testing.T) {
	srv := newGraphServer(t)
	fake := &fakeClient{silent: authResult{
		AccessToken: "at",
		IDTokenRaw: makeIDToken(t, map[string]any{
			"preferred_username": "alice@example.com",
			"_claim_names":       map[string]string{"groups": "src1"},
		}),
	}}
	p, _ := newTestProvider(fake, srv.URL)
	id, err := p.Resolve(context.Background(), identity.Request{Options: testOptions()})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"gid-1", "gid-2", "gid-3", "role-1", "Team A", "Team B", "Team C"}
	if len(id.Groups) != len(want) {
		t.Fatalf("groups = %v, want %v", id.Groups, want)
	}
	for i, g := range want {
		if id.Groups[i] != g {
			t.Fatalf("groups = %v, want %v", id.Groups, want)
		}
	}
}

func TestResolveDisplayNamesUnionWithTokenGroups(t *testing.T) {
	srv := newGraphServer(t)
	fake := &fakeClient{silent: authResult{
		AccessToken: "at",
		IDTokenRaw: makeIDToken(t, map[string]any{
			"preferred_username": "alice@example.com",
			"groups":             []string{"gid-1", "gid-2"},
		}),
	}}
	opts := testOptions()
	opts["resolve_display_names"] = true
	p, _ := newTestProvider(fake, srv.URL)
	id, err := p.Resolve(context.Background(), identity.Request{Options: opts})
	if err != nil {
		t.Fatal(err)
	}
	// Token IDs first, then Graph IDs (deduplicated), then display names.
	want := []string{"gid-1", "gid-2", "gid-3", "role-1", "Team A", "Team B", "Team C"}
	if len(id.Groups) != len(want) {
		t.Fatalf("groups = %v, want %v", id.Groups, want)
	}
	for i, g := range want {
		if id.Groups[i] != g {
			t.Fatalf("groups = %v, want %v", id.Groups, want)
		}
	}
}

func TestResolveGraphErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":{"code":"Authorization_RequestDenied"}}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	fake := &fakeClient{silent: authResult{
		AccessToken: "at",
		IDTokenRaw:  makeIDToken(t, map[string]any{"preferred_username": "a@example.com"}),
	}}
	p, _ := newTestProvider(fake, srv.URL)
	_, err := p.Resolve(context.Background(), identity.Request{Options: testOptions()})
	if err == nil || !strings.Contains(err.Error(), "Microsoft Graph") {
		t.Fatalf("error = %v", err)
	}
}
