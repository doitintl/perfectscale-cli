package output

import (
	"encoding/json"
	"fmt"
	"io"
)

func WriteJSON(w io.Writer, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json output: %w", err)
	}
	_, err = fmt.Fprintln(w, string(body))
	return err
}

func WriteJSONL(w io.Writer, values []any) error {
	enc := json.NewEncoder(w)
	for _, value := range values {
		if err := enc.Encode(value); err != nil {
			return fmt.Errorf("write jsonl output: %w", err)
		}
	}
	return nil
}
