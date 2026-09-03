package clierr

import (
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestClassifyConstructors(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantCode      string
		wantExit      int
		wantRetryable bool
		wantHint      bool
	}{
		{name: "usage", err: Usage("bad flag"), wantCode: "USAGE_ERROR", wantExit: ExitUsage, wantHint: true},
		{name: "not found", err: NotFound("cluster %q not found", "a"), wantCode: "RESOURCE_NOT_FOUND", wantExit: ExitNotFound, wantHint: true},
		{name: "conflict", err: Conflict("ambiguous"), wantCode: "RESOURCE_CONFLICT", wantExit: ExitConflict, wantHint: true},
		{name: "unclassified", err: errors.New("boom"), wantCode: "CLI_ERROR", wantExit: ExitGenericFailure},
		{
			name: "wrapped classified error still classifies", err: fmt.Errorf("wrap: %w", Usage("bad flag")),
			wantCode: "USAGE_ERROR", wantExit: ExitUsage, wantHint: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := Classify(tt.err)
			if info.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", info.Code, tt.wantCode)
			}
			if info.ExitCode != tt.wantExit {
				t.Errorf("ExitCode = %d, want %d", info.ExitCode, tt.wantExit)
			}
			if info.Retryable != tt.wantRetryable {
				t.Errorf("Retryable = %v, want %v", info.Retryable, tt.wantRetryable)
			}
			if hasHint := info.Hint != ""; hasHint != tt.wantHint {
				t.Errorf("Hint = %q, want non-empty: %v", info.Hint, tt.wantHint)
			}
		})
	}
}

func TestClassifyCLIFrameworkUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "required flag", err: errors.New(`Required flag "node-group" not set`)},
		{name: "required flags plural", err: errors.New(`Required flags "id, secret" not set`)},
		{name: "no such flag", err: errors.New("no such flag -bogus")},
		{name: "flag provided but not defined", err: errors.New("flag provided but not defined: -bogus")},
		{name: "could not parse", err: errors.New(`could not parse "abc" as int value from flag -top: strconv.ParseInt`)},
		{name: "duplicate flag forms", err: errors.New("Cannot use two forms of the same flag: agent no-agent")},
		{name: "unknown command", err: errors.New("No help topic for 'badcommand'")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := Classify(tt.err)
			if info.Code != "USAGE_ERROR" || info.ExitCode != ExitUsage {
				t.Fatalf("Classify(%q) = %+v, want USAGE_ERROR/%d", tt.err, info, ExitUsage)
			}
		})
	}
}

func TestClassifyNetworkError(t *testing.T) {
	err := fmt.Errorf("dial: %w", &net.DNSError{Err: "no such host", Name: "example.invalid"})
	info := Classify(err)
	if info.Code != "NETWORK_ERROR" || info.ExitCode != ExitNetwork || !info.Retryable {
		t.Fatalf("Classify(network error) = %+v, want NETWORK_ERROR/%d/retryable", info, ExitNetwork)
	}
}

func TestHTTPStatusErrorClassification(t *testing.T) {
	tests := []struct {
		status        int
		wantCode      string
		wantExit      int
		wantRetryable bool
		wantHint      bool
	}{
		{status: 400, wantCode: "VALIDATION_ERROR", wantExit: ExitValidation, wantHint: true},
		{status: 422, wantCode: "VALIDATION_ERROR", wantExit: ExitValidation, wantHint: true},
		{status: 401, wantCode: "AUTHENTICATION_FAILED", wantExit: ExitAuthentication, wantHint: true},
		{status: 403, wantCode: "PERMISSION_DENIED", wantExit: ExitAuthorization, wantHint: true},
		{status: 404, wantCode: "RESOURCE_NOT_FOUND", wantExit: ExitNotFound, wantHint: true},
		{status: 409, wantCode: "RESOURCE_CONFLICT", wantExit: ExitConflict, wantHint: true},
		{status: 429, wantCode: "RATE_LIMITED", wantExit: ExitRateLimited, wantRetryable: true, wantHint: true},
		{status: 500, wantCode: "API_SERVER_ERROR", wantExit: ExitServer, wantRetryable: true, wantHint: true},
		{status: 418, wantCode: "API_ERROR", wantExit: ExitGenericFailure},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status %d", tt.status), func(t *testing.T) {
			err := &HTTPStatusError{Operation: "get thing", StatusCode: tt.status, Body: "details"}
			info := Classify(err)
			if info.Code != tt.wantCode || info.ExitCode != tt.wantExit || info.Retryable != tt.wantRetryable {
				t.Fatalf("Classify(status %d) = %+v, want {%s %d %v}", tt.status, info, tt.wantCode, tt.wantExit, tt.wantRetryable)
			}
			if hasHint := info.Hint != ""; hasHint != tt.wantHint {
				t.Fatalf("Classify(status %d).Hint = %q, want non-empty: %v", tt.status, info.Hint, tt.wantHint)
			}
			wantMsg := fmt.Sprintf("get thing failed with status %d: details", tt.status)
			if err.Error() != wantMsg {
				t.Fatalf("Error() = %q, want %q", err.Error(), wantMsg)
			}
		})
	}
}

func TestHTTPStatusErrorNoBody(t *testing.T) {
	err := &HTTPStatusError{Operation: "get thing", StatusCode: 500}
	want := "get thing failed with status 500"
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestHTTPStatusErrorRequestID(t *testing.T) {
	err := &HTTPStatusError{Operation: "get thing", StatusCode: 404, ReqID: "req-abc-123"}
	if got := err.RequestID(); got != "req-abc-123" {
		t.Fatalf("RequestID() = %q, want %q", got, "req-abc-123")
	}

	var withID HasRequestID
	if !errors.As(error(err), &withID) {
		t.Fatal("HTTPStatusError does not satisfy HasRequestID via errors.As")
	}
	if got := withID.RequestID(); got != "req-abc-123" {
		t.Fatalf("errors.As RequestID() = %q, want %q", got, "req-abc-123")
	}

	noID := &HTTPStatusError{Operation: "get thing", StatusCode: 404}
	if got := noID.RequestID(); got != "" {
		t.Fatalf("RequestID() with no ReqID set = %q, want empty", got)
	}
}
