package config

import (
	"strings"

	"github.com/perfectscale/poc-cli/internal/clierr"
)

const (
	BinaryName          = "pscli"
	DefaultProfileName  = "default"
	DefaultOutput       = "table"
	DefaultPublicAPIURL = "https://api.app.perfectscale.io/public/v1"

	// OutputEnvVar is the env var the "output" flag reads its default from.
	OutputEnvVar = "PERFECTSCALE_OUTPUT"

	// OutputFlagName and OutputFlagShortName name the "output" flag itself.
	// internal/cli/app.go's runtimeFlags() and OutputModeFromArgs below both
	// reference these, instead of each hardcoding "--output"/"-o" — so a
	// renamed alias can't make the two silently disagree.
	OutputFlagName      = "output"
	OutputFlagShortName = "o"
)

type Settings struct {
	Profile      string
	Output       string
	Debug        bool
	PublicAPIURL string
}

func NormalizeOutput(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "table", "":
		return "table", nil
	case "json":
		return "json", nil
	case "jsonl":
		return "jsonl", nil
	default:
		return "", clierr.Usage("unsupported output mode %q: must be one of table, json, jsonl", value)
	}
}

// OutputModeFromArgs resolves the requested output mode (table/json/jsonl)
// directly from raw CLI args and an env lookup, independent of urfave/cli's
// own parsing. It mirrors the "output" flag's own precedence (explicit flag
// beats OutputEnvVar) so main.go can pick an error-rendering format even for
// errors raised before any command runs (e.g. an unknown flag), without
// needing a constructed Runtime. Returns "" if unset or invalid.
func OutputModeFromArgs(args []string, lookupEnv func(string) string) string {
	value := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		name, val, hasVal := strings.Cut(arg, "=")
		if name != "--"+OutputFlagName && name != "-"+OutputFlagShortName {
			continue
		}
		if hasVal {
			value = val
			continue
		}
		if i+1 < len(args) {
			value = args[i+1]
			i++
		}
	}
	if value == "" {
		value = lookupEnv(OutputEnvVar)
	}
	if value == "" {
		return ""
	}
	normalized, err := NormalizeOutput(value)
	if err != nil {
		return ""
	}
	return normalized
}

func NormalizePublicAPIBaseURL(value string) string {
	base := strings.TrimRight(strings.TrimSpace(value), "/")
	if base == "" {
		return ""
	}
	if strings.HasSuffix(base, "/public/v1") {
		return base
	}
	return base + "/public/v1"
}
