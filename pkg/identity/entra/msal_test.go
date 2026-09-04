package entra

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
)

// bufCache adapts a byte slice to MSAL's Marshaler/Unmarshaler.
type bufCache struct {
	data []byte
}

func (b *bufCache) Marshal() ([]byte, error) { return b.data, nil }
func (b *bufCache) Unmarshal(d []byte) error { b.data = append([]byte(nil), d...); return nil }

func TestFileCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", CacheFileName)
	fc := &fileCache{path: path}
	ctx := context.Background()

	if err := fc.Export(ctx, &bufCache{data: []byte(`{"tokens":1}`)}, cache.ExportHints{}); err != nil {
		t.Fatal(err)
	}
	in := &bufCache{}
	if err := fc.Replace(ctx, in, cache.ReplaceHints{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(in.data, []byte(`{"tokens":1}`)) {
		t.Fatalf("cache round trip = %q", in.data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// 0600 is best-effort on Windows; assert it where chmod semantics hold.
	if perm := info.Mode().Perm(); perm&0o077 != 0 && os.PathSeparator == '/' {
		t.Fatalf("cache file permissions = %o, want owner-only", perm)
	}
}

func TestFileCacheReplaceMissingFileIsNoop(t *testing.T) {
	fc := &fileCache{path: filepath.Join(t.TempDir(), "absent.json")}
	in := &bufCache{data: []byte("untouched")}
	if err := fc.Replace(context.Background(), in, cache.ReplaceHints{}); err != nil {
		t.Fatal(err)
	}
	if string(in.data) != "untouched" {
		t.Fatalf("a missing cache file must leave the in-memory cache alone, got %q", in.data)
	}
}
