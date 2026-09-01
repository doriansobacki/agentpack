package target

import (
	"bytes"
	"fmt"
	"strings"
)

// MergedMarkdown renders all packs' rules (and optionally memories) as one
// markdown document, one section per pack in resolution order. Targets whose
// tool consumes a single instruction file build on this.
func MergedMarkdown(packs []*Pack, includeMemories bool) string {
	var buf bytes.Buffer
	for _, p := range packs {
		if len(p.Rules) == 0 && (!includeMemories || len(p.Memories) == 0) {
			continue
		}
		fmt.Fprintf(&buf, "# %s\n\n", p.Name)
		if p.Description != "" {
			fmt.Fprintf(&buf, "%s\n\n", p.Description)
		}
		for _, f := range p.Rules {
			buf.WriteString(strings.TrimSpace(string(f.Content)))
			buf.WriteString("\n\n")
		}
		if includeMemories && len(p.Memories) > 0 {
			fmt.Fprintf(&buf, "## Memories (%s)\n\n", p.Name)
			for _, f := range p.Memories {
				buf.WriteString(strings.TrimSpace(string(f.Content)))
				buf.WriteString("\n\n")
			}
		}
	}
	return strings.TrimSpace(buf.String())
}
