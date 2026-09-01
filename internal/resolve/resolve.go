// Package resolve turns a user's groups into an ordered, deduplicated
// package list based on the org manifest.
package resolve

import "github.com/doriansobacki/agentpack/internal/orgcfg"

// Wildcard is the group every user implicitly belongs to.
const Wildcard = "*"

// Packages returns the package IDs for the given groups: wildcard packages
// first, then each group's packages in the order the groups were given.
// Duplicates keep their first position. Groups missing from the manifest are
// returned separately so the caller can warn about them.
func Packages(m *orgcfg.Manifest, groups []string) (packages []string, unknownGroups []string) {
	seen := map[string]bool{}
	add := func(ids []string) {
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				packages = append(packages, id)
			}
		}
	}

	add(m.Groups[Wildcard])
	for _, g := range groups {
		if g == Wildcard {
			continue
		}
		ids, ok := m.Groups[g]
		if !ok {
			unknownGroups = append(unknownGroups, g)
			continue
		}
		add(ids)
	}
	return packages, unknownGroups
}
