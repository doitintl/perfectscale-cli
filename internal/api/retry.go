package api

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// Retry tuning for HTTP 429 (rate limited) responses. The public API caps at
// 120 req/min and every cluster-scoped command costs 2 requests (name/UID
// resolve + actual call), so loops over many clusters hit 429s fast without
// this (PSD-9987).
const (
	maxRetryAttempts = 3
	retryBaseDelay   = 500 * time.Millisecond
	retryMaxDelay    = 8 * time.Second
)

// retryTransport retries requests that receive a 429 response, honoring
// Retry-After / Ratelimit-Reset when present and falling back to exponential
// backoff with jitter otherwise.
type retryTransport struct {
	wrapped    http.RoundTripper
	maxRetries int
	sleep      func(ctx context.Context, d time.Duration) error
}

func newRetryTransport(wrapped http.RoundTripper) *retryTransport {
	return &retryTransport{
		wrapped:    wrapped,
		maxRetries: maxRetryAttempts,
		sleep:      sleepContext,
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := drainBody(req)
	if err != nil {
		return nil, err
	}

	var resp *http.Response

	for attempt := 0; ; attempt++ {
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}

		resp, err = t.wrapped.RoundTrip(req)
		if err != nil || resp.StatusCode != http.StatusTooManyRequests || attempt == t.maxRetries {
			return resp, err
		}

		delay := retryDelay(resp, attempt)
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if sleepErr := t.sleep(req.Context(), delay); sleepErr != nil {
			return nil, sleepErr
		}
	}
}

// drainBody reads and closes req.Body so its bytes can be replayed on every
// retry attempt via a fresh io.NopCloser.
func drainBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}

	data, err := io.ReadAll(req.Body)
	_ = req.Body.Close()

	if err != nil {
		return nil, err
	}

	return data, nil
}

// retryDelay picks the wait before the next attempt: Retry-After (seconds or
// HTTP-date) takes priority, then Ratelimit-Reset (seconds), else exponential
// backoff keyed off the attempt number. Only the backoff fallback is capped
// at retryMaxDelay — a server-directed delay is honored as-is (floored at
// zero) since clamping it short would send the next request before the
// server's own advertised rate-limit window ends.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
		return floorDelay(d)
	}

	if d, ok := parseRateLimitReset(resp.Header.Get("Ratelimit-Reset")); ok {
		return floorDelay(d)
	}

	return capDelay(backoffDelay(attempt))
}

func parseRetryAfter(value string) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, true
	}

	if when, err := http.ParseTime(value); err == nil {
		return time.Until(when), true
	}

	return 0, false
}

func parseRateLimitReset(value string) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}

	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}

	return time.Duration(seconds) * time.Second, true
}

// backoffDelay computes an exponential delay with full jitter so concurrent
// retries don't line up on the same wall-clock tick.
func backoffDelay(attempt int) time.Duration {
	delay := retryBaseDelay * time.Duration(int64(1)<<uint(attempt))

	return time.Duration(rand.Int63n(int64(delay) + 1)) //nolint:gosec // jitter, not security-sensitive
}

func capDelay(d time.Duration) time.Duration {
	if d > retryMaxDelay {
		return retryMaxDelay
	}

	return floorDelay(d)
}

func floorDelay(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}

	return d
}
