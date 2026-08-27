// Package clierr classifies pscli errors into a stable exit code, a
// machine-readable code string, and a retryable hint. It's the first slice
// of the structured-error-contract work (PSD-9883): exit codes and
// classification now, without yet committing to hint text or a
// request-id/HTTP-status envelope.
package clierr

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Exit codes. Numbering mirrors dci-cli's contract so DoiT's CLIs agree on
// what a given code means for a caller that only checks $?.
const (
	ExitSuccess        = 0
	ExitGenericFailure = 1
	ExitUsage          = 2
	ExitAuthentication = 10
	ExitAuthorization  = 11
	ExitNotFound       = 20
	ExitConflict       = 21
	ExitValidation     = 30
	ExitServer         = 40
	ExitNetwork        = 41
	ExitRateLimited    = 50
)

// Info is the classification for an error: a stable machine-readable code,
// the process exit code it maps to, and whether retrying the same request
// might succeed.
type Info struct {
	Code      string
	ExitCode  int
	Retryable bool
}

// Classified is implemented by errors that know their own classification.
type Classified interface {
	error
	ErrorInfo() Info
}

type classifiedError struct {
	msg  string
	info Info
}

func (e *classifiedError) Error() string   { return e.msg }
func (e *classifiedError) ErrorInfo() Info { return e.info }

// New builds an error with an explicit classification, for cases that don't
// fit Usage/NotFound/Conflict/HTTPStatusError.
func New(info Info, format string, a ...any) error {
	return &classifiedError{msg: fmt.Sprintf(format, a...), info: info}
}

// Usage reports a client-side flag/argument mistake caught before any
// network call — the pscli-level equivalent of a shell "bad usage" exit.
func Usage(format string, a ...any) error {
	return New(Info{Code: "USAGE_ERROR", ExitCode: ExitUsage}, format, a...)
}

// NotFound reports a client-side "no such resource" result, e.g. a cluster
// name/UID that didn't match anything in the list the CLI just fetched.
func NotFound(format string, a ...any) error {
	return New(Info{Code: "RESOURCE_NOT_FOUND", ExitCode: ExitNotFound}, format, a...)
}

// Conflict reports a client-side ambiguity, e.g. a name matching more than
// one resource.
func Conflict(format string, a ...any) error {
	return New(Info{Code: "RESOURCE_CONFLICT", ExitCode: ExitConflict}, format, a...)
}

// HTTPStatusError is a failed HTTP response from the Perfectscale API or
// its auth endpoint. Its classification is derived from StatusCode.
type HTTPStatusError struct {
	Operation  string
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s failed with status %d", e.Operation, e.StatusCode)
	}
	return fmt.Sprintf("%s failed with status %d: %s", e.Operation, e.StatusCode, e.Body)
}

func (e *HTTPStatusError) ErrorInfo() Info {
	switch e.StatusCode {
	case 400, 422:
		return Info{Code: "VALIDATION_ERROR", ExitCode: ExitValidation}
	case 401:
		return Info{Code: "AUTHENTICATION_FAILED", ExitCode: ExitAuthentication}
	case 403:
		return Info{Code: "PERMISSION_DENIED", ExitCode: ExitAuthorization}
	case 404:
		return Info{Code: "RESOURCE_NOT_FOUND", ExitCode: ExitNotFound}
	case 409:
		return Info{Code: "RESOURCE_CONFLICT", ExitCode: ExitConflict}
	case 429:
		return Info{Code: "RATE_LIMITED", ExitCode: ExitRateLimited, Retryable: true}
	default:
		if e.StatusCode >= 500 {
			return Info{Code: "API_SERVER_ERROR", ExitCode: ExitServer, Retryable: true}
		}
		return Info{Code: "API_ERROR", ExitCode: ExitGenericFailure}
	}
}

// cliFrameworkUsagePatterns are substrings unique to urfave/cli's own
// flag/argument-parsing errors (missing required flag, unknown flag,
// wrong-typed flag value). Their error types (errRequiredFlags, and Go's
// stdlib flag package's parse errors) are unexported, so — like dci-cli's
// own isUsageError for cobra/pflag — message matching is the only option.
var cliFrameworkUsagePatterns = []string{
	"required flag",
	"no such flag",
	"flag provided but not defined",
	"could not parse",
	"cannot use two forms of the same flag",
}

func isCLIFrameworkUsageError(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, pattern := range cliFrameworkUsagePatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

// Classify derives Info for any error: an explicit Classified wins, then a
// recognized urfave/cli parsing error, then a network-transport failure,
// then a generic fallback that preserves today's exit-1 behavior for
// everything else.
func Classify(err error) Info {
	var classified Classified
	if errors.As(err, &classified) {
		return classified.ErrorInfo()
	}

	if isCLIFrameworkUsageError(err) {
		return Info{Code: "USAGE_ERROR", ExitCode: ExitUsage}
	}

	var netErr net.Error
	var urlErr *url.Error
	if errors.As(err, &netErr) || errors.As(err, &urlErr) {
		return Info{Code: "NETWORK_ERROR", ExitCode: ExitNetwork, Retryable: true}
	}

	return Info{Code: "CLI_ERROR", ExitCode: ExitGenericFailure}
}
