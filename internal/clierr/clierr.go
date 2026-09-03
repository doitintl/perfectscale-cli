// Package clierr classifies pscli errors into a stable exit code, a
// machine-readable code string, a retryable flag, human-readable hint text,
// and (for HTTP-backed errors) the API request id — the structured error
// contract from PSD-9883.
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
// the process exit code it maps to, whether retrying the same request might
// succeed, and a human-readable hint suggesting what to do about it. Hint is
// derived purely from the error class (unlike RequestID on HTTPStatusError,
// which is per-instance), so it's safe to hardcode per Code here.
type Info struct {
	Code      string
	ExitCode  int
	Retryable bool
	Hint      string
}

// Classified is implemented by errors that know their own classification.
type Classified interface {
	error
	ErrorInfo() Info
}

// HasRequestID is implemented by errors that can identify the specific API
// request that failed, e.g. HTTPStatusError. Kept separate from Classified
// since a request id is per-instance data, not part of an error's class.
type HasRequestID interface {
	error
	RequestID() string
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
	return New(Info{Code: "USAGE_ERROR", ExitCode: ExitUsage, Hint: "run `pscli <command> --help` for usage."}, format, a...)
}

// NotFound reports a client-side "no such resource" result, e.g. a cluster
// name/UID that didn't match anything in the list the CLI just fetched.
func NotFound(format string, a ...any) error {
	info := Info{
		Code:     "RESOURCE_NOT_FOUND",
		ExitCode: ExitNotFound,
		Hint:     "check the name/ID, or run the resource's `list` command to see what's available.",
	}
	return New(info, format, a...)
}

// Conflict reports a client-side ambiguity, e.g. a name matching more than
// one resource.
func Conflict(format string, a ...any) error {
	info := Info{
		Code:     "RESOURCE_CONFLICT",
		ExitCode: ExitConflict,
		Hint:     "the name matched more than one resource; use a more specific name or the resource's UID.",
	}
	return New(info, format, a...)
}

// HTTPStatusError is a failed HTTP response from the Perfectscale API or
// its auth endpoint. Its classification is derived from StatusCode.
type HTTPStatusError struct {
	Operation  string
	StatusCode int
	Body       string
	// ReqID is the X-Request-Id the API returned with this response, when
	// present. Surfaced via RequestID() so it can travel with the error
	// independently of Info, since it varies per call, not per error class.
	ReqID string
}

func (e *HTTPStatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s failed with status %d", e.Operation, e.StatusCode)
	}
	return fmt.Sprintf("%s failed with status %d: %s", e.Operation, e.StatusCode, e.Body)
}

func (e *HTTPStatusError) RequestID() string { return e.ReqID }

func (e *HTTPStatusError) ErrorInfo() Info {
	switch e.StatusCode {
	case 400, 422:
		return Info{Code: "VALIDATION_ERROR", ExitCode: ExitValidation, Hint: "check the request parameters against `--help`."}
	case 401:
		return Info{
			Code:     "AUTHENTICATION_FAILED",
			ExitCode: ExitAuthentication,
			Hint:     "run `pscli auth login` to refresh your credentials.",
		}
	case 403:
		return Info{
			Code:     "PERMISSION_DENIED",
			ExitCode: ExitAuthorization,
			Hint:     "the service token doesn't have permission for this action.",
		}
	case 404:
		return Info{
			Code:     "RESOURCE_NOT_FOUND",
			ExitCode: ExitNotFound,
			Hint:     "check the name/ID, or run the resource's `list` command to see what's available.",
		}
	case 409:
		return Info{
			Code:     "RESOURCE_CONFLICT",
			ExitCode: ExitConflict,
			Hint:     "the name matched more than one resource; use a more specific name or the resource's UID.",
		}
	case 429:
		return Info{
			Code:      "RATE_LIMITED",
			ExitCode:  ExitRateLimited,
			Retryable: true,
			Hint:      "wait a moment and retry, or reduce how often you're calling the API.",
		}
	default:
		if e.StatusCode >= 500 {
			return Info{
				Code:      "API_SERVER_ERROR",
				ExitCode:  ExitServer,
				Retryable: true,
				Hint:      "retry the request; if it persists, contact Perfectscale support.",
			}
		}
		return Info{Code: "API_ERROR", ExitCode: ExitGenericFailure}
	}
}

// cliFrameworkUsagePatterns are substrings unique to urfave/cli's own
// flag/argument-parsing errors (missing required flag, unknown flag,
// wrong-typed flag value). Their error types (errRequiredFlags, and Go's
// stdlib flag package's parse errors) are unexported, so — like dci-cli's
// own isUsageError for cobra/pflag — message matching is the only option.
// "no help topic for" is urfave/cli's message for an unrecognized
// command/subcommand name — the same class of mistake as an unrecognized
// flag, so it's classified as a usage error too.
var cliFrameworkUsagePatterns = []string{
	"required flag",
	"no such flag",
	"flag provided but not defined",
	"could not parse",
	"cannot use two forms of the same flag",
	"no help topic for",
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
		return Info{Code: "USAGE_ERROR", ExitCode: ExitUsage, Hint: "run `pscli <command> --help` for usage."}
	}

	var netErr net.Error
	var urlErr *url.Error
	if errors.As(err, &netErr) || errors.As(err, &urlErr) {
		return Info{
			Code:      "NETWORK_ERROR",
			ExitCode:  ExitNetwork,
			Retryable: true,
			Hint:      "check your network connection and the --public-api-url value, then retry.",
		}
	}

	return Info{Code: "CLI_ERROR", ExitCode: ExitGenericFailure}
}
