package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStoreSaveLoadDelete(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "profiles")
	store, err := NewStore(baseDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	now := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	input := &Data{
		SchemaVersion: 1,
		Name:          "team/prod",
		AuthMode:      AuthModeServiceToken,
		PublicAPIURL:  "https://api.app.perfectscale.io/public/v1",
		AccessToken:   "access-token",
		ExpiresAt:     now.Add(time.Hour),
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	path := store.Path(input.Name)
	if !strings.HasSuffix(path, "team_prod.json") {
		t.Fatalf("store.Path() = %q, want sanitized suffix team_prod.json", path)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != filePerm {
		t.Fatalf("file perms = %o, want %o", info.Mode().Perm(), filePerm)
	}

	loaded, err := store.Load(input.Name)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Name != input.Name {
		t.Fatalf("loaded.Name = %q, want %q", loaded.Name, input.Name)
	}
	if loaded.AuthMode != input.AuthMode {
		t.Fatalf("loaded.AuthMode = %q, want %q", loaded.AuthMode, input.AuthMode)
	}
	if loaded.AccessToken != input.AccessToken {
		t.Fatalf("loaded.AccessToken = %q, want %q", loaded.AccessToken, input.AccessToken)
	}

	if err := store.Delete(input.Name); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Load(input.Name); err == nil {
		t.Fatal("Load() after Delete() error = nil, want non-nil")
	}
}

func TestStoreLoadMissingProfile(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "profiles"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	_, err = store.Load("missing")
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "run `pscli auth login` first") {
		t.Fatalf("error = %q, want actionable guidance", err)
	}
}
