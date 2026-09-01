package cli

import (
	"github.com/perfectscale/poc-cli/internal/config"
	ucli "github.com/urfave/cli/v2"
)

func New(version string) *ucli.App {
	ucli.VersionPrinter = versionPrinter

	commands := []*ucli.Command{
		authCommand(),
		clustersCommand(),
		namespacesCommand(),
		workloadsCommand(),
		nodegroupsCommand(),
		unevictableCommand(),
		automationCommand(),
		updateCommand(),
		skillCommand(),
		completionCommand(),
		commandsCommand(),
	}
	attachRuntimeFlags(commands)

	app := &ucli.App{
		Name:                 config.BinaryName,
		Usage:                "Query Perfectscale public API data from the terminal with service-token auth",
		Version:              version,
		EnableBashCompletion: true,
		Description: withCommandName(`Perfectscale CLI uses public API service tokens generated from the Perfectscale UI.

Available commands:
  auth login|status|logout
  clusters list|get|emission
  namespaces list
  workloads list|summary|group-by namespace|group-by type|group-by optimization-policy|group-by risk-severity|group-by label|show|export|risky|labels|muted
  nodegroups list|get
  unevictable list|report|show|muted
  automation audit-logs
  update
  skill
  completion
  commands

Common short options:
  -p profile, -o output, -u public-api-url, -d debug
  -c cluster, -w period window, -n namespace, -t type
  -s sort, -r order, -T top, -B bottom
  -C min-cost / min-blocked-cost, -W min-waste
  -V view preset (workloads or nodegroups)
  -g node group name
  -i id (workload show, unevictable show pod uid)

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

Machine-readable output:
  -o json is a pretty-printed document; -o jsonl is a compact stream, one
  object per line. On failure, either prints a JSON error object (code/
  retryable) on stderr instead of a plain-text message, following the same
  pretty-vs-compact split. Exit codes are always stable and error-specific
  (usage/auth/not-found/conflict/validation/server/network/rate-limited),
  not just when output is json/jsonl. See README.md.

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
  {{cmd}} nodegroups list -c prod-a
  {{cmd}} nodegroups list -c prod-a --autoscaler-type karpenter --has-recommendations
  {{cmd}} nodegroups get -c prod-a -g clickhouse
  {{cmd}} unevictable list -c prod-a -n payments --reason pod_disruption_budget
  {{cmd}} unevictable report -c prod-a -C 5 -s blockedCostHourly -r desc
  {{cmd}} unevictable show -c prod-a -i a1b2c3d4
  {{cmd}} unevictable muted -c prod-a
  {{cmd}} automation audit-logs -c prod-a --since 24h
  {{cmd}} automation audit-logs --all -o jsonl
  {{cmd}} update
  {{cmd}} skill cursor
  {{cmd}} completion bash
  {{cmd}} commands -o json`),
		Flags:    runtimeFlags(),
		Commands: commands,
	}
	if app.Metadata == nil {
		app.Metadata = map[string]any{}
	}
	app.Metadata["version"] = version

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
			Name:    config.OutputFlagName,
			Aliases: []string{config.OutputFlagShortName},
			Usage:   "Output mode: table, json, or jsonl",
			EnvVars: []string{config.OutputEnvVar},
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
