package cli

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/perfectscale/poc-cli/internal/api"
	"github.com/perfectscale/poc-cli/internal/clierr"
	"github.com/perfectscale/poc-cli/internal/output"
	ucli "github.com/urfave/cli/v2"
)

const realizedSavingsDateLayout = "2006-01-02"

// realizedSavingsPeriod echoes the range into JSON, since the default dates are otherwise invisible there.
type realizedSavingsPeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}

var (
	realizedSavingsClusterSortKeys   = []string{"name", "totalCost", "realizedSavings", "realizedSavingsPct", "pricingSource", "cpuCoreHoursSaved", "memoryGbHoursSaved"}
	realizedSavingsNamespaceSortKeys = []string{"namespace", "totalCost", "realizedSavings", "realizedSavingsPct", "cpuCoreHoursSaved", "memoryGbHoursSaved"}
	realizedSavingsWorkloadSortKeys  = []string{"name", "type", "totalCost", "realizedSavings", "realizedSavingsPct", "cpuCoreHoursSaved", "memoryGbHoursSaved"}
)

const realizedSavingsRangeHelp = `Date range:
  --from is inclusive and --to is exclusive, both YYYY-MM-DD in UTC. Without
  them the range is the last 30 days, today included. --from can go back to
  the 1st of the month 13 months ago. Data older than 6 months is kept per
  calendar month, so a range starting more than 6 months back must start on a
  1st and end on a 1st or in the current month; otherwise the API answers 422
  partial_month_range and its detail names the range to send instead.

Only automated savings are counted, the same numbers as the Realized Savings
pages in the Perfectscale UI.`

const realizedSavingsListHelp = `Pagination:
  --page-size (1-500, default 50); --page-token consumes a cursor from a prior
  response's pagination.next (-o json, or the table footer hint) and needs the
  same dates, scope and sort; --all auto-paginates forward (capped by --page-cap).

Money fields are USD. GB is 10^9 bytes. pct is saved / (saved + used) × 100.
-o json wraps the rows with "from", "to" (exclusive) and "pagination"
{"next","prev","page_size"}; -o jsonl emits one row per line, without them.`

type realizedSavingsScope int

const (
	realizedSavingsTenantScope realizedSavingsScope = iota
	realizedSavingsClusterScope
	realizedSavingsNamespaceScope
)

func realizedSavingsRangeFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.StringFlag{Name: "from", Usage: "Start date, inclusive (YYYY-MM-DD, UTC). Defaults to 30 days before --to"},
		&ucli.StringFlag{Name: "to", Usage: "End date, exclusive (YYYY-MM-DD, UTC). Defaults to tomorrow, so today is included"},
	}
}

func realizedSavingsListFlags(scope realizedSavingsScope) []ucli.Flag {
	var flags []ucli.Flag
	if scope >= realizedSavingsClusterScope {
		flags = append(flags, &ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID (required)"})
	}
	if scope >= realizedSavingsNamespaceScope {
		flags = append(flags, &ucli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Namespace name, matched exactly (required)"})
	}
	return append(append(flags, realizedSavingsRangeFlags()...),
		&ucli.StringFlag{Name: "sort", Aliases: []string{"s"}, Usage: "Server-side sort key (see description)"},
		&ucli.StringFlag{Name: "order", Aliases: []string{"r"}, Usage: "Sort order: asc or desc"},
		&ucli.IntFlag{Name: "page-size", Usage: "Server page size (1-500)", Value: 50},
		&ucli.StringFlag{Name: "page-token", Usage: "Opaque cursor from a previous response's pagination.next"},
		&ucli.BoolFlag{Name: "all", Usage: "Auto-paginate forward until no next cursor remains"},
		&ucli.IntFlag{Name: "page-cap", Usage: "Safety cap on pages fetched when --all is used", Value: 50},
	)
}

func realizedSavingsCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "realized-savings",
		Usage: "Show how much automation saved, for the tenant, a cluster, a namespace or a workload",
		Subcommands: []*ucli.Command{
			{
				Name:  "summary",
				Usage: "Show realized savings totals for the tenant, or narrow with -c, -n and -i",
				Description: withCommandName(`Examples:
  {{cmd}} realized-savings summary
  {{cmd}} realized-savings summary -c prod-a
  {{cmd}} realized-savings summary -c prod-a -n payments
  {{cmd}} realized-savings summary -c prod-a -n payments -i payments-deployment-api
  {{cmd}} realized-savings summary --from 2026-10-01 --to 2026-11-01 -o json

Scope: no flags is the whole tenant, deleted clusters included; -c a cluster;
-c -n a namespace of it; -c -n -i one workload, where -i is the id column of
"realized-savings workloads".

` + realizedSavingsRangeHelp + `

Output schema (--output json):
  tenant, cluster, namespace:
    { "from", "to", "total_cost", "baseline_cost", "realized_savings",
      "realized_savings_pct", "automated_cost", "not_automated_cost",
      "resiliency_issues_addressed",
      "cluster": {"cluster_uid","name","cloud","region","pricing_source"} (with -c) }
    baseline_cost = total_cost + realized_savings, and
    automated_cost + not_automated_cost = total_cost.
  workload:
    { "from", "to", "total_cost", "baseline_cost", "realized_savings",
      "realized_savings_pct", "cpu_core_hours_saved": {"absolute","pct"},
      "memory_gb_hours_saved": {"absolute","pct"} }
  Money fields are USD.`),
				Flags: append([]ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID; omit for the whole tenant"},
					&ucli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Namespace of the cluster; needs -c"},
					&ucli.StringFlag{Name: "id", Aliases: []string{"i"}, Usage: "Workload id from \"realized-savings workloads\"; needs -c and -n"},
				}, realizedSavingsRangeFlags()...),
				Action: runRealizedSavingsSummary,
			},
			{
				Name:  "clusters",
				Usage: "List realized savings per cluster",
				Description: withCommandName(`Examples:
  {{cmd}} realized-savings clusters
  {{cmd}} realized-savings clusters --from 2026-10-01 --to 2026-11-01 -s totalCost -r desc
  {{cmd}} realized-savings clusters --all -o jsonl

Deleted clusters are included; their name is their UID.

` + realizedSavingsRangeHelp + `

Sorting (server-side):
  -s one of ` + strings.Join(realizedSavingsClusterSortKeys, ", ") + `.
  Default realizedSavings. The two hours keys sort by their pct. -r asc or desc
  (default desc).

` + realizedSavingsListHelp + `

Row schema:
  { "cluster_uid", "name", "pricing_source", "transmission_status",
    "upgrade_status", "total_cost", "realized_savings", "realized_savings_pct",
    "cpu_core_hours_saved": {"absolute","pct"},
    "memory_gb_hours_saved": {"absolute","pct"} }
  -o json puts the rows under "clusters".`),
				Flags:  realizedSavingsListFlags(realizedSavingsTenantScope),
				Action: runRealizedSavingsClusters,
			},
			{
				Name:  "namespaces",
				Usage: "List realized savings per namespace of a cluster",
				Description: withCommandName(`Examples:
  {{cmd}} realized-savings namespaces -c prod-a
  {{cmd}} realized-savings namespaces -c prod-a --from 2026-10-01 --to 2026-11-01 -s totalCost
  {{cmd}} realized-savings namespaces -c prod-a --all -o jsonl

-c is required. A deleted cluster is a 404.

` + realizedSavingsRangeHelp + `

Sorting (server-side):
  -s one of ` + strings.Join(realizedSavingsNamespaceSortKeys, ", ") + `.
  Default realizedSavings. The two hours keys sort by their pct. -r asc or desc
  (default desc).

` + realizedSavingsListHelp + `

Row schema:
  { "cluster_uid", "namespace", "total_cost", "realized_savings",
    "realized_savings_pct", "cpu_core_hours_saved": {"absolute","pct"},
    "memory_gb_hours_saved": {"absolute","pct"} }
  -o json puts the rows under "namespaces".`),
				Flags:  realizedSavingsListFlags(realizedSavingsClusterScope),
				Action: runRealizedSavingsNamespaces,
			},
			{
				Name:  "workloads",
				Usage: "List realized savings per workload of a namespace",
				Description: withCommandName(`Examples:
  {{cmd}} realized-savings workloads -c prod-a -n payments
  {{cmd}} realized-savings workloads -c prod-a -n payments -s type -r asc
  {{cmd}} realized-savings workloads -c prod-a -n payments --all -o jsonl

-c and -n are required. A namespace with nothing in range is an empty list.
type is one of deployment, stateful_set, daemon_set, cron_job, job, rollout,
scaled_job, unknown. Pass the id column to "realized-savings summary -i".

` + realizedSavingsRangeHelp + `

Sorting (server-side):
  -s one of ` + strings.Join(realizedSavingsWorkloadSortKeys, ", ") + `.
  Default realizedSavings. type sorts by Kubernetes kind (CronJob before
  Deployment). The two hours keys sort by their pct. -r asc or desc (default desc).

` + realizedSavingsListHelp + `

Row schema:
  { "cluster_uid", "namespace", "id", "name", "type", "total_cost",
    "realized_savings", "realized_savings_pct",
    "cpu_core_hours_saved": {"absolute","pct"},
    "memory_gb_hours_saved": {"absolute","pct"} }
  -o json puts the rows under "workloads".`),
				Flags:  realizedSavingsListFlags(realizedSavingsNamespaceScope),
				Action: runRealizedSavingsWorkloads,
			},
		},
	}
}

func runRealizedSavingsSummary(c *ucli.Context) error {
	dates, err := parseRealizedSavingsRange(c, time.Now())
	if err != nil {
		return err
	}
	namespace := strings.TrimSpace(c.String("namespace"))
	workloadID := strings.TrimSpace(c.String("id"))
	clusterTarget := strings.TrimSpace(c.String("cluster"))
	if namespace != "" && clusterTarget == "" {
		return clierr.Usage("--namespace (-n) needs --cluster (-c)")
	}
	if workloadID != "" && namespace == "" {
		return clierr.Usage("--id (-i) needs --cluster (-c) and --namespace (-n)")
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	clusterUID := ""
	if clusterTarget != "" {
		cluster, err := resources.resolveCluster(c.Context, clusterTarget)
		if err != nil {
			return err
		}
		clusterUID = cluster.UID
	}

	period := newRealizedSavingsPeriod(dates)
	rows := [][]string{{"from", period.From}, {"to (exclusive)", period.To}}

	if workloadID != "" {
		summary, err := resources.Runtime.API.GetPublicRealizedSavingsWorkloadSummary(c.Context, resources.Profile.PublicAPIURL, resources.Token, clusterUID, namespace, workloadID, dates)
		if err != nil {
			return err
		}
		rows = append(rows,
			[]string{"workload_id", workloadID},
			[]string{"realized_savings_usd", fmt.Sprintf("%.2f", summary.RealizedSavings)},
			[]string{"realized_savings_pct", fmt.Sprintf("%.2f", summary.RealizedSavingsPct)},
			[]string{"total_cost_usd", fmt.Sprintf("%.2f", summary.TotalCost)},
			[]string{"baseline_cost_usd", fmt.Sprintf("%.2f", summary.BaselineCost)},
			[]string{"cpu_core_hours_saved", formatRealizedSavingsHours(summary.CPUCoreHoursSaved)},
			[]string{"memory_gb_hours_saved", formatRealizedSavingsHours(summary.MemoryGBHoursSaved)},
		)
		return resources.Runtime.RenderTableOrJSON(struct {
			realizedSavingsPeriod
			api.RealizedSavingsWorkloadSummary
		}{period, summary}, []string{"FIELD", "VALUE"}, rows)
	}

	summary, err := resources.Runtime.API.GetPublicRealizedSavingsSummary(c.Context, resources.Profile.PublicAPIURL, resources.Token, clusterUID, namespace, dates)
	if err != nil {
		return err
	}
	if cluster := summary.Cluster; cluster != nil {
		rows = append(rows, []string{"cluster", cluster.Name}, []string{"cluster_uid", cluster.ClusterUID})
	}
	if namespace != "" {
		rows = append(rows, []string{"namespace", namespace})
	}
	rows = append(rows,
		[]string{"realized_savings_usd", fmt.Sprintf("%.2f", summary.RealizedSavings)},
		[]string{"realized_savings_pct", fmt.Sprintf("%.2f", summary.RealizedSavingsPct)},
		[]string{"total_cost_usd", fmt.Sprintf("%.2f", summary.TotalCost)},
		[]string{"baseline_cost_usd", fmt.Sprintf("%.2f", summary.BaselineCost)},
		[]string{"automated_cost_usd", fmt.Sprintf("%.2f", summary.AutomatedCost)},
		[]string{"not_automated_cost_usd", fmt.Sprintf("%.2f", summary.NotAutomatedCost)},
		[]string{"resiliency_issues_addressed", fmt.Sprintf("%d", summary.ResiliencyIssuesAddressed)},
	)
	return resources.Runtime.RenderTableOrJSON(struct {
		realizedSavingsPeriod
		api.RealizedSavingsSummary
	}{period, summary}, []string{"FIELD", "VALUE"}, rows)
}

func runRealizedSavingsClusters(c *ucli.Context) error {
	return runRealizedSavingsTable(c, "clusters", realizedSavingsTenantScope, realizedSavingsClusterSortKeys, (*api.Client).ListPublicRealizedSavingsClusters,
		[]string{"NAME", "REALIZED_SAVINGS_USD", "SAVINGS_PCT", "TOTAL_COST_USD", "CPU_CORE_H_SAVED", "MEM_GB_H_SAVED", "PRICING_SOURCE", "TRANSMISSION"},
		func(item api.RealizedSavingsCluster) []string {
			return append(append([]string{item.Name}, realizedSavingsMoneyCells(item.RealizedSavings, item.RealizedSavingsPct, item.TotalCost, item.CPUCoreHoursSaved, item.MemoryGBHoursSaved)...),
				item.PricingSource, item.TransmissionStatus)
		})
}

func runRealizedSavingsNamespaces(c *ucli.Context) error {
	return runRealizedSavingsTable(c, "namespaces", realizedSavingsClusterScope, realizedSavingsNamespaceSortKeys, (*api.Client).ListPublicRealizedSavingsNamespaces,
		[]string{"NAMESPACE", "REALIZED_SAVINGS_USD", "SAVINGS_PCT", "TOTAL_COST_USD", "CPU_CORE_H_SAVED", "MEM_GB_H_SAVED"},
		func(item api.RealizedSavingsNamespace) []string {
			return append([]string{item.Namespace}, realizedSavingsMoneyCells(item.RealizedSavings, item.RealizedSavingsPct, item.TotalCost, item.CPUCoreHoursSaved, item.MemoryGBHoursSaved)...)
		})
}

func runRealizedSavingsWorkloads(c *ucli.Context) error {
	return runRealizedSavingsTable(c, "workloads", realizedSavingsNamespaceScope, realizedSavingsWorkloadSortKeys, (*api.Client).ListPublicRealizedSavingsWorkloads,
		[]string{"NAME", "TYPE", "REALIZED_SAVINGS_USD", "SAVINGS_PCT", "TOTAL_COST_USD", "CPU_CORE_H_SAVED", "MEM_GB_H_SAVED", "ID"},
		func(item api.RealizedSavingsWorkload) []string {
			return append(append([]string{item.Name, item.Type}, realizedSavingsMoneyCells(item.RealizedSavings, item.RealizedSavingsPct, item.TotalCost, item.CPUCoreHoursSaved, item.MemoryGBHoursSaved)...),
				item.ID)
		})
}

type realizedSavingsLister[T any] func(*api.Client, context.Context, string, string, api.RealizedSavingsListOptions) (api.RealizedSavingsPage[T], error)

func runRealizedSavingsTable[T any](c *ucli.Context, key string, scope realizedSavingsScope, sortKeys []string, list realizedSavingsLister[T], headers []string, row func(T) []string) error {
	opts, err := buildRealizedSavingsListOptions(c, sortKeys, time.Now())
	if err != nil {
		return err
	}
	if scope >= realizedSavingsNamespaceScope {
		if opts.Namespace = strings.TrimSpace(c.String("namespace")); opts.Namespace == "" {
			return clierr.Usage("--namespace (-n) is required")
		}
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}
	if scope >= realizedSavingsClusterScope {
		cluster, err := resources.resolveCluster(c.Context, c.String("cluster"))
		if err != nil {
			return err
		}
		opts.ClusterUID = cluster.UID
	}

	listPage := func(opts api.RealizedSavingsListOptions) (api.RealizedSavingsPage[T], error) {
		return list(resources.Runtime.API, c.Context, resources.Profile.PublicAPIURL, resources.Token, opts)
	}
	var page api.RealizedSavingsPage[T]
	if c.Bool("all") {
		page, err = api.ListAllRealizedSavings(opts, c.Int("page-cap"), listPage)
	} else {
		page, err = listPage(opts)
	}
	if err != nil {
		return err
	}

	writer := resources.Runtime.Writer
	if writer == nil {
		writer = os.Stdout
	}
	switch resources.Runtime.Config.Output {
	case "json":
		period := newRealizedSavingsPeriod(opts.Range)
		return output.WriteJSON(writer, map[string]any{"from": period.From, "to": period.To, key: page.Rows, "pagination": page.Pagination}, false)
	case "jsonl":
		values := make([]any, 0, len(page.Rows))
		for _, item := range page.Rows {
			values = append(values, item)
		}
		return output.WriteJSONL(writer, values)
	default:
		rows := make([][]string, 0, len(page.Rows))
		for _, item := range page.Rows {
			rows = append(rows, row(item))
		}
		if err := output.WriteTable(writer, headers, rows); err != nil {
			return err
		}
		if next := page.Pagination.Next; next != nil && *next != "" {
			fmt.Fprintf(writer, "\n%d %s shown. More available — pass --page-token %s (or use --all)\n", len(page.Rows), key, *next)
		}
		return nil
	}
}

func buildRealizedSavingsListOptions(c *ucli.Context, sortKeys []string, now time.Time) (api.RealizedSavingsListOptions, error) {
	dates, err := parseRealizedSavingsRange(c, now)
	if err != nil {
		return api.RealizedSavingsListOptions{}, err
	}
	opts := api.RealizedSavingsListOptions{Range: dates}

	if sortBy := strings.TrimSpace(c.String("sort")); sortBy != "" {
		if !slices.Contains(sortKeys, sortBy) {
			return opts, clierr.Usage("unsupported --sort %q: must be one of %s", sortBy, strings.Join(sortKeys, ", "))
		}
		opts.SortBy = &sortBy
	}

	if order := strings.ToLower(strings.TrimSpace(c.String("order"))); order != "" {
		if order != "asc" && order != "desc" {
			return opts, clierr.Usage("unsupported --order %q: must be asc or desc", order)
		}
		opts.SortOrder = &order
	}

	if pageSize := c.Int("page-size"); pageSize > 0 {
		if pageSize > 500 {
			return opts, clierr.Usage("--page-size must be <= 500")
		}
		opts.PageSize = &pageSize
	}

	if pageToken := strings.TrimSpace(c.String("page-token")); pageToken != "" {
		opts.PageToken = &pageToken
	}

	return opts, nil
}

func parseRealizedSavingsRange(c *ucli.Context, now time.Time) (api.RealizedSavingsRange, error) {
	parseDate := func(flag string, fallback time.Time) (time.Time, error) {
		value := strings.TrimSpace(c.String(flag))
		if value == "" {
			return fallback, nil
		}
		parsed, err := time.Parse(realizedSavingsDateLayout, value)
		if err != nil {
			return parsed, clierr.Usage("--%s must be YYYY-MM-DD, got %q", flag, value)
		}
		return parsed, nil
	}

	var dates api.RealizedSavingsRange
	var err error
	if dates.End, err = parseDate("to", now.UTC().Truncate(24*time.Hour).AddDate(0, 0, 1)); err != nil {
		return dates, err
	}
	if dates.Start, err = parseDate("from", dates.End.AddDate(0, 0, -30)); err != nil {
		return dates, err
	}
	if !dates.End.After(dates.Start) {
		return dates, clierr.Usage("--to must be after --from (--to is exclusive)")
	}
	return dates, nil
}

func newRealizedSavingsPeriod(dates api.RealizedSavingsRange) realizedSavingsPeriod {
	return realizedSavingsPeriod{From: dates.Start.Format(realizedSavingsDateLayout), To: dates.End.Format(realizedSavingsDateLayout)}
}

func realizedSavingsMoneyCells(saved float64, savedPct float64, totalCost float64, cpu api.RealizedSavingsHours, memory api.RealizedSavingsHours) []string {
	return []string{
		fmt.Sprintf("%.2f", saved),
		fmt.Sprintf("%.2f", savedPct),
		fmt.Sprintf("%.2f", totalCost),
		formatRealizedSavingsHours(cpu),
		formatRealizedSavingsHours(memory),
	}
}

func formatRealizedSavingsHours(hours api.RealizedSavingsHours) string {
	return fmt.Sprintf("%.1f (%d%%)", hours.Absolute, hours.Pct)
}
