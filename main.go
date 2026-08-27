package main

import (
	"context"
	"log"
	"os"

	appcli "github.com/perfectscale/poc-cli/internal/cli"
	"github.com/perfectscale/poc-cli/internal/clierr"
	"github.com/perfectscale/poc-cli/internal/config"
	"github.com/perfectscale/poc-cli/internal/output"
)

var version = "dev"

func main() {
	log.SetFlags(0)

	app := appcli.New(version)
	if err := app.RunContext(context.Background(), os.Args); err != nil {
		exitCode := clierr.Classify(err).ExitCode

		outputMode := config.OutputModeFromArgs(os.Args, os.Getenv)
		if outputMode == "json" || outputMode == "jsonl" {
			if writeErr := output.WriteJSONError(os.Stderr, err, outputMode == "jsonl"); writeErr != nil {
				log.Print(writeErr)
			}
		} else {
			log.Print(err)
		}
		os.Exit(exitCode)
	}
}
