package cli

import (
	"strings"

	"github.com/perfectscale/poc-cli/internal/config"
)

func withCommandName(text string) string {
	return strings.ReplaceAll(text, "{{cmd}}", config.BinaryName)
}
