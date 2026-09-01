package cli

import (
	"testing"

	"github.com/perfectscale/poc-cli/internal/clierr"
)

func TestCompletionScripts(t *testing.T) {
	cases := []struct {
		shell  string
		marker string
	}{
		{shell: "bash", marker: "complete -o bashdefault"},
		{shell: "zsh", marker: "compdef _cli_zsh_autocomplete pscli"},
		{shell: "powershell", marker: "Register-ArgumentCompleter"},
		{shell: "fish", marker: "complete -c pscli"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.shell, func(t *testing.T) {
			output, err := runCLI(t, nil, "completion", tc.shell)
			if err != nil {
				t.Fatalf("runCLI(completion %s) error = %v", tc.shell, err)
			}

			assertContains(t, output, "pscli")
			assertContains(t, output, tc.marker)
		})
	}
}

func TestCompletionNoArgs(t *testing.T) {
	_, err := runCLI(t, nil, "completion")
	if err == nil {
		t.Fatal("expected usage error")
	}

	if clierr.Classify(err).ExitCode != clierr.ExitUsage {
		t.Fatalf("classify = %+v, want usage", clierr.Classify(err))
	}
}

func TestCompletionTooManyArgs(t *testing.T) {
	_, err := runCLI(t, nil, "completion", "bash", "zsh")
	if err == nil {
		t.Fatal("expected usage error")
	}

	if clierr.Classify(err).ExitCode != clierr.ExitUsage {
		t.Fatalf("classify = %+v, want usage", clierr.Classify(err))
	}
}

func TestCompletionUnknownShell(t *testing.T) {
	_, err := runCLI(t, nil, "completion", "tcsh")
	if err == nil {
		t.Fatal("expected usage error")
	}

	if clierr.Classify(err).ExitCode != clierr.ExitUsage {
		t.Fatalf("classify = %+v, want usage", clierr.Classify(err))
	}
}
