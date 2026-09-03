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
			name:    "classified error carries its code, retryable flag, and hint, compact",
			err:     clierr.NotFound("cluster %q not found", "prod-a"),
			compact: true,
			want: `{"error":{"code":"RESOURCE_NOT_FOUND","message":"cluster \"prod-a\" not found","retryable":false,` +
				`"hint":"check the name/ID, or run the resource's ` + "`list`" + ` command to see what's available."}}` + "\n",
		},
		{
			name:    "pretty-printed when not compact",
			err:     clierr.NotFound("cluster %q not found", "prod-a"),
			compact: false,
			want: "{\n  \"error\": {\n    \"code\": \"RESOURCE_NOT_FOUND\",\n" +
				"    \"message\": \"cluster \\\"prod-a\\\" not found\",\n    \"retryable\": false,\n" +
				"    \"hint\": \"check the name/ID, or run the resource's `list` command to see what's available.\"\n  }\n}\n",
		},
		{
			name:    "http status error carries request id",
			err:     &clierr.HTTPStatusError{Operation: "get thing", StatusCode: 500, ReqID: "req-123"},
			compact: true,
			want: `{"error":{"code":"API_SERVER_ERROR","message":"get thing failed with status 500","retryable":true,` +
				`"hint":"retry the request; if it persists, contact Perfectscale support.","request_id":"req-123"}}` + "\n",
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
