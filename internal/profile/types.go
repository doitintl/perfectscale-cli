package profile

import "time"

type AuthMode string

const (
	AuthModeServiceToken AuthMode = "service_token"
)

type Data struct {
	SchemaVersion int       `json:"schema_version"`
	Name          string    `json:"name"`
	AuthMode      AuthMode  `json:"auth_mode"`
	PublicAPIURL  string    `json:"public_api_url"`
	AccessToken   string    `json:"access_token,omitempty"`
	RefreshToken  string    `json:"refresh_token,omitempty"`
	TokenType     string    `json:"token_type,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	ClientID      string    `json:"client_id,omitempty"`
	ClientSecret  string    `json:"client_secret,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (p *Data) NeedsRefresh(now time.Time) bool {
	if p == nil || p.AccessToken == "" {
		return true
	}
	if p.ExpiresAt.IsZero() {
		return false
	}
	return now.After(p.ExpiresAt.Add(-time.Minute))
}
