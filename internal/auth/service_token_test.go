package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExchangeServiceToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/public/v1/auth/public_auth" {
				t.Fatalf("path = %s, want /public/v1/auth/public_auth", r.URL.Path)
			}
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("content-type = %q, want application/json", got)
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			payload := string(body)
			if !strings.Contains(payload, `"client_id":"client-id"`) {
				t.Fatalf("request body %q does not contain client_id", payload)
			}
			if !strings.Contains(payload, `"client_secret":"client-secret"`) {
				t.Fatalf("request body %q does not contain client_secret", payload)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"access_token":"token-123","expires_in":3600}}`))
		}))
		defer server.Close()

		tokens, err := ExchangeServiceToken(context.Background(), server.URL+"/public/v1", "client-id", "client-secret")
		if err != nil {
			t.Fatalf("ExchangeServiceToken() error = %v", err)
		}
		if tokens.AccessToken != "token-123" {
			t.Fatalf("AccessToken = %q, want token-123", tokens.AccessToken)
		}
		if tokens.ExpiresIn != 3600 {
			t.Fatalf("ExpiresIn = %d, want 3600", tokens.ExpiresIn)
		}
	})

	t.Run("success with flat response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"token-456","expires_in":1800}`))
		}))
		defer server.Close()

		tokens, err := ExchangeServiceToken(context.Background(), server.URL+"/public/v1", "client-id", "client-secret")
		if err != nil {
			t.Fatalf("ExchangeServiceToken() error = %v", err)
		}
		if tokens.AccessToken != "token-456" {
			t.Fatalf("AccessToken = %q, want token-456", tokens.AccessToken)
		}
		if tokens.ExpiresIn != 1800 {
			t.Fatalf("ExpiresIn = %d, want 1800", tokens.ExpiresIn)
		}
	})

	t.Run("http error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusUnauthorized)
		}))
		defer server.Close()

		_, err := ExchangeServiceToken(context.Background(), server.URL+"/public/v1", "client-id", "client-secret")
		if err == nil {
			t.Fatal("ExchangeServiceToken() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "status 401") {
			t.Fatalf("error %q does not mention the HTTP status", err)
		}
	})
}
