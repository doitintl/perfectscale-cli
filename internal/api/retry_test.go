package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func noopSleep(context.Context, time.Duration) error { return nil }

type stubTransport struct {
	responses []*http.Response
	errs      []error
	calls     int
}

func (t *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	i := t.calls
	t.calls++

	if i < len(t.errs) && t.errs[i] != nil {
		return nil, t.errs[i]
	}

	return t.responses[i], nil
}

func newStatusResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       http.NoBody,
		Header:     http.Header{},
	}
}

func TestRetryTransportRetriesOn429ThenSucceeds(t *testing.T) {
	t.Parallel()

	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := newRetryTransport(http.DefaultTransport)
	transport.sleep = noopSleep

	client := &http.Client{Transport: transport}

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if hits != 3 {
		t.Fatalf("hits = %d, want 3", hits)
	}
}

func TestRetryTransportExhaustsRetries(t *testing.T) {
	t.Parallel()

	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	transport := newRetryTransport(http.DefaultTransport)
	transport.sleep = noopSleep

	client := &http.Client{Transport: transport}

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want 429", resp.StatusCode)
	}
	if want := maxRetryAttempts + 1; hits != want {
		t.Fatalf("hits = %d, want %d", hits, want)
	}
}

func TestRetryTransportNoRetryOnNon429(t *testing.T) {
	t.Parallel()

	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	transport := newRetryTransport(http.DefaultTransport)
	transport.sleep = noopSleep

	client := &http.Client{Transport: transport}

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if hits != 1 {
		t.Fatalf("hits = %d, want 1 (no retry on 500)", hits)
	}
}

func TestRetryTransportNoRetryOnTransportError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	stub := &stubTransport{errs: []error{wantErr}, responses: []*http.Response{nil}}
	transport := &retryTransport{wrapped: stub, maxRetries: maxRetryAttempts, sleep: noopSleep}

	req, err := http.NewRequest(http.MethodGet, "http://example.invalid", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	_, err = transport.RoundTrip(req)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if stub.calls != 1 {
		t.Fatalf("calls = %d, want 1", stub.calls)
	}
}

func TestRetryTransportContextCancelledDuringWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	transport := newRetryTransport(http.DefaultTransport)
	transport.sleep = func(context.Context, time.Duration) error { return context.Canceled }

	client := &http.Client{Transport: transport}

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	_, err = client.Do(req)
	if err == nil {
		t.Fatal("Do() error = nil, want context cancellation error")
	}
}

func TestRetryTransportReplaysRequestBody(t *testing.T) {
	t.Parallel()

	hits := 0
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		bodies = append(bodies, string(buf))
		if hits <= 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := newRetryTransport(http.DefaultTransport)
	transport.sleep = noopSleep

	client := &http.Client{Transport: transport}

	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	for i, body := range bodies {
		if body != "payload" {
			t.Fatalf("bodies[%d] = %q, want %q", i, body, "payload")
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  time.Duration
		ok    bool
	}{
		{name: "empty", value: "", want: 0, ok: false},
		{name: "seconds", value: "5", want: 5 * time.Second, ok: true},
		{name: "invalid", value: "not-a-value", want: 0, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseRetryAfter(tt.value)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Fatalf("got = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("http_date", func(t *testing.T) {
		t.Parallel()

		when := time.Now().Add(10 * time.Second).UTC()
		got, ok := parseRetryAfter(when.Format(http.TimeFormat))
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if got < 8*time.Second || got > 11*time.Second {
			t.Fatalf("got = %v, want ~10s", got)
		}
	})
}

func TestParseRateLimitReset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  time.Duration
		ok    bool
	}{
		{name: "empty", value: "", want: 0, ok: false},
		{name: "seconds", value: "3", want: 3 * time.Second, ok: true},
		{name: "invalid", value: "soon", want: 0, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseRateLimitReset(tt.value)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Fatalf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetryDelayPrefersRetryAfterThenRateLimitReset(t *testing.T) {
	t.Parallel()

	t.Run("retry_after_wins", func(t *testing.T) {
		t.Parallel()

		resp := newStatusResponse(http.StatusTooManyRequests)
		resp.Header.Set("Retry-After", "2")
		resp.Header.Set("Ratelimit-Reset", "9")

		if got := retryDelay(resp, 0); got != 2*time.Second {
			t.Fatalf("got = %v, want 2s", got)
		}
	})

	t.Run("ratelimit_reset_fallback", func(t *testing.T) {
		t.Parallel()

		resp := newStatusResponse(http.StatusTooManyRequests)
		resp.Header.Set("Ratelimit-Reset", "4")

		if got := retryDelay(resp, 0); got != 4*time.Second {
			t.Fatalf("got = %v, want 4s", got)
		}
	})

	t.Run("backoff_when_no_headers", func(t *testing.T) {
		t.Parallel()

		resp := newStatusResponse(http.StatusTooManyRequests)

		got := retryDelay(resp, 0)
		if got < 0 || got > retryBaseDelay {
			t.Fatalf("got = %v, want in [0, %v]", got, retryBaseDelay)
		}
	})
}

func TestCapDelay(t *testing.T) {
	t.Parallel()

	if got := capDelay(-1 * time.Second); got != 0 {
		t.Fatalf("capDelay(negative) = %v, want 0", got)
	}
	if got := capDelay(100 * time.Second); got != retryMaxDelay {
		t.Fatalf("capDelay(large) = %v, want %v", got, retryMaxDelay)
	}
	if got := capDelay(time.Second); got != time.Second {
		t.Fatalf("capDelay(1s) = %v, want 1s", got)
	}
}
