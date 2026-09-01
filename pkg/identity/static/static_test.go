package static_test

import (
	"context"
	"strings"
	"testing"

	"github.com/doriansobacki/agentpack/pkg/identity"
	"github.com/doriansobacki/agentpack/pkg/identity/static"
)

func resolve(t *testing.T, email string, users map[string][]string) (*identity.Identity, error) {
	t.Helper()
	return static.New().Resolve(context.Background(), identity.Request{
		Email:       email,
		StaticUsers: users,
	})
}

func TestResolveCaseInsensitive(t *testing.T) {
	id, err := resolve(t, "Alice@Example.com", map[string][]string{
		"alice@example.com": {"team-a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(id.Groups) != 1 || id.Groups[0] != "team-a" {
		t.Fatalf("groups = %v", id.Groups)
	}
}

func TestResolveUnknownEmailErrors(t *testing.T) {
	_, err := resolve(t, "typo@example.com", map[string][]string{
		"alice@example.com": {"team-a"},
	})
	if err == nil || !strings.Contains(err.Error(), "not listed") {
		t.Fatalf("expected unknown-email error, got %v", err)
	}
}

func TestResolveCaseDuplicateEntriesError(t *testing.T) {
	_, err := resolve(t, "alice@example.com", map[string][]string{
		"alice@example.com": {"team-a"},
		"Alice@Example.com": {"team-b"},
	})
	if err == nil || !strings.Contains(err.Error(), "differing only in case") {
		t.Fatalf("expected duplicate-entries error, got %v", err)
	}
}

func TestResolveEmptyEmailErrors(t *testing.T) {
	_, err := resolve(t, "", map[string][]string{"a@b.c": nil})
	if err == nil {
		t.Fatal("expected an error for empty email")
	}
}
