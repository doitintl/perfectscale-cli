package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/perfectscale/poc-cli/internal/clierr"
)

// SetEscapeHTML(false) on both encoders below: encoding/json's default
// HTML-escapes &, <, and > as unicode sequences, since it originally
// targeted embedding JSON in HTML. This is terminal/pipeline output, not
// HTML, so that escaping just makes strings containing those characters
// (e.g. `update`'s deb/rpm one-liner, which contains &&) unreadable in raw
// JSON for no benefit.

// WriteJSON writes value as a single JSON document. compact skips
// indentation: -o json is a full document meant to be read, so it stays
// pretty-printed; -o jsonl is a stream meant to be parsed line-by-line, so
// its scalar fallback (asSlice failing in RenderTableOrJSON) renders
// compact to match jsonl's own convention (see WriteJSONL, which is always
// compact by construction — one Encode() call per line).
func WriteJSON(w io.Writer, value any, compact bool) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if !compact {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(value); err != nil {
		return fmt.Errorf("marshal json output: %w", err)
	}
	return nil
}

func WriteJSONL(w io.Writer, values []any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, value := range values {
		if err := enc.Encode(value); err != nil {
			return fmt.Errorf("write jsonl output: %w", err)
		}
	}
	return nil
}

// ErrorEnvelope is the structured shape errors are printed in when the
// requested output mode is json/jsonl. Code, Retryable, and Hint come from
// clierr.Classify; RequestID comes from clierr.HasRequestID when the error
// implements it (HTTP-backed errors only).
type ErrorEnvelope struct {
	Error ErrorPayload `json:"error"`
}

type ErrorPayload struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
	Hint      string `json:"hint,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// WriteJSONError writes err as a JSON object, following the same
// pretty-vs-compact convention as WriteJSON: compact under -o jsonl,
// pretty-printed under -o json.
func WriteJSONError(w io.Writer, err error, compact bool) error {
	info := clierr.Classify(err)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if !compact {
		enc.SetIndent("", "  ")
	}
	payload := ErrorEnvelope{Error: ErrorPayload{Code: info.Code, Message: err.Error(), Retryable: info.Retryable, Hint: info.Hint}}
	var withID clierr.HasRequestID
	if errors.As(err, &withID) {
		payload.Error.RequestID = withID.RequestID()
	}
	if encodeErr := enc.Encode(payload); encodeErr != nil {
		return fmt.Errorf("marshal json error output: %w", encodeErr)
	}
	return nil
}
