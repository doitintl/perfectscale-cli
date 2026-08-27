package config

import "testing"

func TestOutputModeFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{name: "nothing set", args: []string{"pscli", "clusters", "list"}, want: ""},
		{
			name: "long flag with separate value",
			args: []string{"pscli", "--output", "jsonl", "clusters", "list"},
			want: "jsonl",
		},
		{
			name: "long flag with equals",
			args: []string{"pscli", "--output=json", "clusters", "list"},
			want: "json",
		},
		{
			name: "short flag with separate value",
			args: []string{"pscli", "-o", "json", "clusters", "list"},
			want: "json",
		},
		{
			name: "short flag with equals",
			args: []string{"pscli", "-o=jsonl", "clusters", "list"},
			want: "jsonl",
		},
		{
			name: "last occurrence wins",
			args: []string{"pscli", "-o", "table", "clusters", "list", "-o", "jsonl"},
			want: "jsonl",
		},
		{
			name: "stops scanning at -- terminator",
			args: []string{"pscli", "clusters", "list", "--", "-o", "jsonl"},
			want: "",
		},
		{
			name: "falls back to env var when flag absent",
			args: []string{"pscli", "clusters", "list"},
			env:  map[string]string{"PERFECTSCALE_OUTPUT": "json"},
			want: "json",
		},
		{
			name: "explicit flag beats env var",
			args: []string{"pscli", "-o", "table", "clusters", "list"},
			env:  map[string]string{"PERFECTSCALE_OUTPUT": "json"},
			want: "table",
		},
		{
			name: "invalid value normalizes to empty",
			args: []string{"pscli", "-o", "csv", "clusters", "list"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(name string) string { return tt.env[name] }
			got := OutputModeFromArgs(tt.args, lookup)
			if got != tt.want {
				t.Fatalf("OutputModeFromArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}
