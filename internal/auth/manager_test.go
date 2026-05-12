package auth

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/perfectscale/poc-cli/internal/profile"
)

func TestEnsureAccessTokenRejectsLegacyUserProfile(t *testing.T) {
	store, err := profile.NewStore(filepath.Join(t.TempDir(), "profiles"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	manager := NewManager(store)
	data := &profile.Data{
		SchemaVersion: 1,
		Name:          "default",
		AuthMode:      profile.AuthMode("user_token"),
		AccessToken:   "expired-token",
		ExpiresAt:     time.Now().UTC().Add(-5 * time.Minute),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	_, err = manager.EnsureAccessToken(context.Background(), data)
	if err == nil {
		t.Fatal("EnsureAccessToken() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "only service-token auth is supported now") {
		t.Fatalf("error = %q, want legacy profile guidance", err)
	}
}
