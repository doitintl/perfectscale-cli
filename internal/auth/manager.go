package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/perfectscale/poc-cli/internal/config"
	"github.com/perfectscale/poc-cli/internal/profile"
)

type Manager struct {
	store *profile.Store
}

func NewManager(store *profile.Store) *Manager {
	return &Manager{store: store}
}

func (m *Manager) Load(name string) (*profile.Data, error) {
	return m.store.Load(name)
}

func (m *Manager) Save(data *profile.Data) error {
	return m.store.Save(data)
}

func (m *Manager) Delete(name string) error {
	return m.store.Delete(name)
}

func (m *Manager) EnsureAccessToken(ctx context.Context, data *profile.Data) (string, error) {
	if data == nil {
		return "", fmt.Errorf("profile is required")
	}
	if !data.NeedsRefresh(time.Now()) {
		return data.AccessToken, nil
	}

	if data.AuthMode != profile.AuthModeServiceToken {
		return "", fmt.Errorf("profile %q uses unsupported auth mode %q; only service-token auth is supported now, so run `%s auth login` again", data.Name, data.AuthMode, config.BinaryName)
	}
	if data.ClientID == "" || data.ClientSecret == "" {
		return "", fmt.Errorf("profile %q is missing service token credentials; run `%s auth login` again", data.Name, config.BinaryName)
	}

	tokens, err := ExchangeServiceToken(ctx, data.PublicAPIURL, data.ClientID, data.ClientSecret)
	if err != nil {
		return "", fmt.Errorf("refresh service token: %w", err)
	}
	ApplyTokenResponse(data, tokens)

	data.UpdatedAt = time.Now().UTC()
	if err := m.store.Save(data); err != nil {
		return "", err
	}

	return data.AccessToken, nil
}

func ApplyTokenResponse(data *profile.Data, tokens *OAuthTokenResponse) {
	data.AccessToken = tokens.AccessToken
	data.TokenType = tokens.TokenType
	if tokens.RefreshToken != "" {
		data.RefreshToken = tokens.RefreshToken
	}
	if tokens.ExpiresIn > 0 {
		data.ExpiresAt = time.Now().UTC().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	}
}
