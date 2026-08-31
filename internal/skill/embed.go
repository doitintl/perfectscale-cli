package skill

import "embed"

// FS is the Perfectscale agent skill tree, compiled into the binary.
// all: is required so .claude-plugin/ is included (default embed skips dotfiles).
//
//go:embed all:perfectscale
var FS embed.FS
