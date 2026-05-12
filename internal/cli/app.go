package cli

import (
	"fmt"

	"github.com/perfectscale/poc-cli/internal/config"
	ucli "github.com/urfave/cli/v2"
)

func New(version string, commit string, buildDate string) *ucli.App {
	commands := []*ucli.Command{
		authCommand(),
		clustersCommand(),
		namespacesCommand(),
		workloadsCommand(),
		automationCommand(),
	}
	attachRuntimeFlags(commands)

	app := &ucli.App{
		Name:    config.BinaryName,
		Usage:   "Query Perfectscale public API data from the terminal with service-token auth",
		Version: fmt.Sprintf("%s (commit=%s built=%s)", version, commit, buildDate),
		Description: withCommandName(`Perfectscale CLI uses public API service tokens generated from the Perfectscale UI.

Available commands:
  auth login|status|logout
  clusters list|get|emission
  namespaces list
  workloads list|summary|group-by namespace|group-by type|group-by optimization-policy|group-by risk-severity|group-by label|show|export|risky|labels|muted
  automation audit-logs

Common short options:
  -p profile, -o output, -u public-api-url, -d debug
  -c cluster, -w period window, -n namespace, -t type
  -s sort, -r order, -T top, -B bottom
  -C min-cost, -W min-waste
  -V workload view preset

Workload list views (--view, -V):
  default
    Cost, waste, and max-indicator overview.
  capacity
    Replicas plus current and recommended request/limit totals.
  usage
    Summed container usage percentiles for each workload.
  policy
    Optimization policy, resilience, and mute state.
  risk
    Risk severity, risk counts, and waste counts.
  all
    The broadest enriched workload view. Defaults to jsonl unless output is explicitly set.

Output modes (--output, -o):
  table
    Human-friendly terminal output.
  json
    One JSON document, useful for structured inspection.
  jsonl
    One JSON object per line for list commands, useful for agents and pipelines.

Examples:
  {{cmd}} auth login
  {{cmd}} auth login -s -i ps_xxx -k ps_yyy
  {{cmd}} clusters list
  {{cmd}} clusters get -c prod-a
  {{cmd}} clusters emission -c prod-a -s value -r desc
  {{cmd}} namespaces list -c prod-a -s workloads -r desc
  {{cmd}} workloads list -c prod-a -w 30d -W 50 -s waste -r desc
  {{cmd}} workloads list -c prod-a -V usage
  {{cmd}} workloads summary -c prod-a
  {{cmd}} workloads group-by namespace -c prod-a -s waste -r desc -T 10
  {{cmd}} workloads group-by optimization-policy -c prod-a -s waste -r desc
  {{cmd}} workloads group-by risk-severity -c prod-a -s workloads -r desc
  {{cmd}} workloads group-by label -c prod-a -k team -s waste -r desc
  {{cmd}} workloads list -c prod-a -V all
  {{cmd}} -o jsonl workloads list -c prod-a -V all -s waste -r desc -T 10
  {{cmd}} workloads show -c prod-a -i workload-123
  {{cmd}} workloads risky -c prod-a -S 2 -T 10
  {{cmd}} workloads labels -c prod-a -k app -s waste -r desc -T 20
  {{cmd}} workloads export -c prod-a -F workloads.csv
  {{cmd}} automation audit-logs -c prod-a --since 24h
  {{cmd}} automation audit-logs --all -o jsonl`),
		Flags:    runtimeFlags(),
		Commands: commands,
	}

	return app
}

func runtimeFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.StringFlag{
			Name:    "profile",
			Aliases: []string{"p"},
			Usage:   "Profile name used for stored credentials and defaults",
			EnvVars: []string{"PERFECTSCALE_PROFILE"},
			Value:   config.DefaultProfileName,
		},
		&ucli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output mode: table, json, or jsonl",
			EnvVars: []string{"PERFECTSCALE_OUTPUT"},
			Value:   config.DefaultOutput,
		},
		&ucli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Usage:   "Enable verbose debugging output",
			EnvVars: []string{"PERFECTSCALE_DEBUG"},
			Value:   false,
		},
		&ucli.StringFlag{
			Name:    "public-api-url",
			Aliases: []string{"u"},
			Usage:   "Base URL for the Perfectscale public API",
			EnvVars: []string{"PERFECTSCALE_PUBLIC_API_URL"},
			Value:   config.DefaultPublicAPIURL,
		},
	}
}

func attachRuntimeFlags(commands []*ucli.Command) {
	for _, command := range commands {
		command.Flags = append(command.Flags, runtimeFlags()...)
		if len(command.Subcommands) > 0 {
			attachRuntimeFlags(command.Subcommands)
		}
	}
}
