//go:build !go1.24

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGo123StartupDefersAssuranceJanitorCleanup(t *testing.T) {
	home := t.TempDir()
	assuranceDirectory := filepath.Join(home, "artifacts", "assurance")
	if err := os.MkdirAll(assuranceDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(assuranceDirectory, "orphan.json")
	if err := os.WriteFile(orphan, []byte("preserve until safe cleanup is available"), 0o600); err != nil {
		t.Fatal(err)
	}

	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	data, err := os.ReadFile(orphan)
	if err != nil {
		t.Fatalf("startup removed deferred assurance orphan: %v", err)
	}
	if string(data) != "preserve until safe cleanup is available" {
		t.Fatalf("deferred assurance orphan content = %q", data)
	}
}
