package resolve_test

import (
	"reflect"
	"testing"

	"github.com/doriansobacki/agentpack/internal/orgcfg"
	"github.com/doriansobacki/agentpack/internal/resolve"
)

func manifest() *orgcfg.Manifest {
	return &orgcfg.Manifest{
		Groups: map[string][]string{
			"*":              {"org-baseline"},
			"team-a":         {"team-a-core"},
			"team-a/backend": {"dotnet", "org-baseline"}, // duplicate on purpose
		},
	}
}

func TestPackagesWildcardOnly(t *testing.T) {
	pkgs, unknown := resolve.Packages(manifest(), nil)
	if !reflect.DeepEqual(pkgs, []string{"org-baseline"}) {
		t.Fatalf("got %v", pkgs)
	}
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown groups: %v", unknown)
	}
}

func TestPackagesOrderAndDedupe(t *testing.T) {
	pkgs, _ := resolve.Packages(manifest(), []string{"team-a", "team-a/backend"})
	want := []string{"org-baseline", "team-a-core", "dotnet"}
	if !reflect.DeepEqual(pkgs, want) {
		t.Fatalf("got %v, want %v", pkgs, want)
	}
}

func TestPackagesUnknownGroup(t *testing.T) {
	pkgs, unknown := resolve.Packages(manifest(), []string{"team-a", "ghosts"})
	if !reflect.DeepEqual(unknown, []string{"ghosts"}) {
		t.Fatalf("unknown = %v", unknown)
	}
	want := []string{"org-baseline", "team-a-core"}
	if !reflect.DeepEqual(pkgs, want) {
		t.Fatalf("got %v, want %v", pkgs, want)
	}
}

func TestWildcardInUserGroupsIsIgnored(t *testing.T) {
	pkgs, unknown := resolve.Packages(manifest(), []string{"*"})
	if !reflect.DeepEqual(pkgs, []string{"org-baseline"}) || len(unknown) != 0 {
		t.Fatalf("got %v, unknown %v", pkgs, unknown)
	}
}
