package main

import (
	"context"
	"log"
	"os"

	appcli "github.com/perfectscale/poc-cli/internal/cli"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	app := appcli.New(version, commit, buildDate)
	if err := app.RunContext(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
