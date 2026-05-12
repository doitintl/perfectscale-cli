package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type PublicAuthResponse struct {
	Data OAuthTokenResponse `json:"data"`
}

type OAuthTokenResponse struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func ExchangeServiceToken(ctx context.Context, publicAPIBaseURL string, clientID string, clientSecret string) (*OAuthTokenResponse, error) {
	payload := map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode public auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(publicAPIBaseURL, "/")+"/auth/public_auth", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create public auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute public auth request: %w", err)
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read public auth response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("public auth failed with status %d: %s", res.StatusCode, strings.TrimSpace(string(resBody)))
	}

	var wrapper PublicAuthResponse
	if err := json.Unmarshal(resBody, &wrapper); err == nil && wrapper.Data.AccessToken != "" {
		return &wrapper.Data, nil
	}

	var direct OAuthTokenResponse
	if err := json.Unmarshal(resBody, &direct); err != nil {
		return nil, fmt.Errorf("decode public auth response: %w", err)
	}
	if direct.AccessToken == "" {
		return nil, fmt.Errorf("decode public auth response: access_token missing")
	}

	return &direct, nil
}
