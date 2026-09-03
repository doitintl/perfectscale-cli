package main

import (
	"context"
	"errors"
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
		info := clierr.Classify(err)

		outputMode := config.OutputModeFromArgs(os.Args, os.Getenv)
		if outputMode == "json" || outputMode == "jsonl" {
			if writeErr := output.WriteJSONError(os.Stderr, err, outputMode == "jsonl"); writeErr != nil {
				log.Print(writeErr)
			}
		} else {
			log.Print(err)
			if info.Hint != "" {
				log.Print("hint: " + info.Hint)
			}
			var withID clierr.HasRequestID
			if errors.As(err, &withID) && withID.RequestID() != "" {
				log.Print("request id: " + withID.RequestID())
			}
		}
		os.Exit(info.ExitCode)
	}
}
