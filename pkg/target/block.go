package target

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Managed-block markers. Content between them is owned by agentpack and
// rewritten on every sync; everything outside them is preserved verbatim.
const (
	BeginMarker = "<!-- agentpack:begin -->"
	EndMarker   = "<!-- agentpack:end -->"
)

const blockNotice = "<!-- Managed by agentpack. Everything between the markers is overwritten on sync; edit outside them. -->"

// UpsertManagedBlock inserts or replaces the agentpack-managed block in the
// file at path, creating the file (and parent directories) if needed. Content
// outside the markers is preserved. Literal marker strings inside the content
// are defanged: an embedded marker would make the next sync splice the file
// at the wrong position and leak old block content into the user's portion.
func UpsertManagedBlock(path, content string) error {
	content = strings.ReplaceAll(content, BeginMarker, "(agentpack begin marker)")
	content = strings.ReplaceAll(content, EndMarker, "(agentpack end marker)")
	block := BeginMarker + "\n" + blockNotice + "\n\n" + strings.TrimSpace(content) + "\n" + EndMarker

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(existing)

	begin := strings.Index(text, BeginMarker)
	end := strings.Index(text, EndMarker)
	var out string
	switch {
	case begin >= 0 && end > begin:
		out = text[:begin] + block + text[end+len(EndMarker):]
	case begin >= 0 || end >= 0:
		return fmt.Errorf("%s: found a lone agentpack marker; fix or remove it manually", path)
	case strings.TrimSpace(text) == "":
		out = block + "\n"
	default:
		out = strings.TrimRight(text, "\n") + "\n\n" + block + "\n"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// RemoveManagedBlock deletes the agentpack-managed block from the file at
// path, if present. The file itself is never deleted.
func RemoveManagedBlock(path string) error {
	existing, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	text := string(existing)
	begin := strings.Index(text, BeginMarker)
	end := strings.Index(text, EndMarker)
	if begin < 0 || end < begin {
		return nil
	}
	out := strings.TrimRight(text[:begin], "\n") + text[end+len(EndMarker):]
	out = strings.TrimSpace(out)
	if out != "" {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0o644)
}
