package output

import (
	"bytes"
	"errors"
	"testing"

	"github.com/perfectscale/poc-cli/internal/clierr"
)

func TestWriteJSONError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		compact bool
		want    string
	}{
		{
			name:    "unclassified error falls back to CLI_ERROR, compact",
			err:     errors.New("boom"),
			compact: true,
			want:    `{"error":{"code":"CLI_ERROR","message":"boom","retryable":false}}` + "\n",
		},
		{
			name:    "classified error carries its code and retryable flag, compact",
			err:     clierr.NotFound("cluster %q not found", "prod-a"),
			compact: true,
			want:    `{"error":{"code":"RESOURCE_NOT_FOUND","message":"cluster \"prod-a\" not found","retryable":false}}` + "\n",
		},
		{
			name:    "pretty-printed when not compact",
			err:     clierr.NotFound("cluster %q not found", "prod-a"),
			compact: false,
			want: "{\n  \"error\": {\n    \"code\": \"RESOURCE_NOT_FOUND\",\n" +
				"    \"message\": \"cluster \\\"prod-a\\\" not found\",\n    \"retryable\": false\n  }\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteJSONError(&buf, tt.err, tt.compact); err != nil {
				t.Fatalf("WriteJSONError() error = %v", err)
			}
			if buf.String() != tt.want {
				t.Fatalf("WriteJSONError() output = %q, want %q", buf.String(), tt.want)
			}
		})
	}
}
