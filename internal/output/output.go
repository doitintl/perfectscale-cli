package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// SetEscapeHTML(false) on both encoders below: encoding/json's default
// HTML-escapes &, <, and > as unicode sequences, since it originally
// targeted embedding JSON in HTML. This is terminal/pipeline output, not
// HTML, so that escaping just makes strings containing those characters
// (e.g. `update`'s deb/rpm one-liner, which contains &&) unreadable in raw
// JSON for no benefit.

func WriteJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
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
