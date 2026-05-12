package config

import (
	"fmt"
	"strings"
)

const (
	BinaryName          = "pscli"
	DefaultProfileName  = "default"
	DefaultOutput       = "table"
	DefaultPublicAPIURL = "https://api.app.perfectscale.io/public/v1"
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
		return "", fmt.Errorf("unsupported output mode %q: must be one of table, json, jsonl", value)
	}
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
