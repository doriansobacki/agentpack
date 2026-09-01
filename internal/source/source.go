// Package source fetches the org config repository: a local directory is
// used in place, a git URL is cloned into (and thereafter pulled in) a local
// cache directory.
package source

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// NormalizeLocal reports whether src is a local directory and, if so,
// returns its absolute path. It is the single definition of "the source is
// a local directory", shared by login (which persists the source) and Fetch.
func NormalizeLocal(src string) (string, bool) {
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return src, false
	}
	abs, err := filepath.Abs(src)
	if err != nil {
		return src, false
	}
	return abs, true
}

// Fetch makes the org config available locally and returns its directory.
func Fetch(src, cacheDir string) (string, error) {
	if dir, ok := NormalizeLocal(src); ok {
		return dir, nil
	}

	sum := sha256.Sum256([]byte(src))
	dest := filepath.Join(cacheDir, "repo-"+hex.EncodeToString(sum[:6]))

	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		if out, err := gitRun(dest, "pull", "--ff-only"); err != nil {
			return "", fmt.Errorf("updating %s: %w\n%s", src, err, out)
		}
		return dest, nil
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	if out, err := gitRun("", "clone", "--depth", "1", src, dest); err != nil {
		return "", fmt.Errorf("cloning %s: %w\n%s", src, err, out)
	}
	return dest, nil
}

func gitRun(dir string, args ...string) (string, error) {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
