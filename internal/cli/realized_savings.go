package cli

import (
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

func newRealizedSavingsPeriod(dates api.RealizedSavingsRange) realizedSavingsPeriod {
	return realizedSavingsPeriod{From: dates.Start.Format(realizedSavingsDateLayout), To: dates.End.Format(realizedSavingsDateLayout)}
}

var realizedSavingsClusterSortKeys = []string{"name", "totalCost", "realizedSavings", "realizedSavingsPct", "pricingSource", "cpuCoreHoursSaved", "memoryGbHoursSaved"}

const realizedSavingsRangeHelp = `Date range:
  --from is inclusive and --to is exclusive, both YYYY-MM-DD in UTC. Without
  them the range is the last 30 days, today included. --from can go back to
  the 1st of the month 13 months ago. Data older than 6 months is kept per
  calendar month, so a range starting more than 6 months back must start on a
  1st and end on a 1st or in the current month; otherwise the API answers 422
  partial_month_range and its detail names the range to send instead.

Only automated savings are counted, the same numbers as the Realized Savings
pages in the Perfectscale UI.`

func realizedSavingsRangeFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.StringFlag{Name: "from", Usage: "Start date, inclusive (YYYY-MM-DD, UTC). Defaults to 30 days before --to"},
		&ucli.StringFlag{Name: "to", Usage: "End date, exclusive (YYYY-MM-DD, UTC). Defaults to tomorrow, so today is included"},
	}
}

func realizedSavingsCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "realized-savings",
		Usage: "Show how much automation saved, for the tenant or per cluster",
		Subcommands: []*ucli.Command{
			{
				Name:  "summary",
				Usage: "Show realized savings totals for the tenant, or for one cluster with -c",
				Description: withCommandName(`Examples:
  {{cmd}} realized-savings summary
  {{cmd}} realized-savings summary -c prod-a
  {{cmd}} realized-savings summary --from 2026-10-01 --to 2026-11-01 -o json

Without -c the totals cover every cluster of the tenant, deleted ones included.

` + realizedSavingsRangeHelp + `

Output schema (--output json):
  { "from", "to", "total_cost", "baseline_cost", "realized_savings", "realized_savings_pct",
    "automated_cost", "not_automated_cost", "resiliency_issues_addressed",
    "cluster": {"cluster_uid","name","cloud","region","pricing_source"} (only with -c) }
  Money fields are USD. baseline_cost = total_cost + realized_savings, and
  automated_cost + not_automated_cost = total_cost.`),
				Flags: append([]ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID; omit for the whole tenant"},
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
  -s one of name, totalCost, realizedSavings (default), realizedSavingsPct,
  pricingSource, cpuCoreHoursSaved, memoryGbHoursSaved. The two hours keys
  sort by their pct. -r asc or desc (default desc).

Pagination:
  --page-size (1-500, default 50); --page-token consumes a cursor from a prior
  response's pagination.next (-o json, or the table footer hint) and needs the
  same dates and sort; --all auto-paginates forward (capped by --page-cap).

Output schema (--output json):
  { "from", "to", "clusters": [ <cluster>, ... ], "pagination": {"next","prev","page_size"} }
  <cluster>:
    { "cluster_uid", "name", "pricing_source", "transmission_status",
      "upgrade_status", "total_cost", "realized_savings", "realized_savings_pct",
      "cpu_core_hours_saved": {"absolute","pct"},
      "memory_gb_hours_saved": {"absolute","pct"} }
  Money fields are USD. GB is 10^9 bytes. pct is saved / (saved + used) × 100.
  from/to echo the range sent (to is exclusive). --output jsonl emits one
  <cluster> object per line, without the range or the pagination cursor.`),
				Flags: append(realizedSavingsRangeFlags(),
					&ucli.StringFlag{Name: "sort", Aliases: []string{"s"}, Usage: "Server-side sort key (see description)"},
					&ucli.StringFlag{Name: "order", Aliases: []string{"r"}, Usage: "Sort order: asc or desc"},
					&ucli.IntFlag{Name: "page-size", Usage: "Server page size (1-500)", Value: 50},
					&ucli.StringFlag{Name: "page-token", Usage: "Opaque cursor from a previous response's pagination.next"},
					&ucli.BoolFlag{Name: "all", Usage: "Auto-paginate forward until no next cursor remains"},
					&ucli.IntFlag{Name: "page-cap", Usage: "Safety cap on pages fetched when --all is used", Value: 50},
				),
				Action: runRealizedSavingsClusters,
			},
		},
	}
}

func runRealizedSavingsSummary(c *ucli.Context) error {
	dates, err := parseRealizedSavingsRange(c, time.Now())
	if err != nil {
		return err
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	clusterUID := ""
	if target := strings.TrimSpace(c.String("cluster")); target != "" {
		cluster, err := resources.resolveCluster(c.Context, target)
		if err != nil {
			return err
		}
		clusterUID = cluster.UID
	}

	summary, err := resources.Runtime.API.GetPublicRealizedSavingsSummary(c.Context, resources.Profile.PublicAPIURL, resources.Token, clusterUID, dates)
	if err != nil {
		return err
	}

	var rows [][]string
	if cluster := summary.Cluster; cluster != nil {
		rows = append(rows, []string{"cluster", cluster.Name}, []string{"cluster_uid", cluster.ClusterUID})
	}
	rows = append(rows,
		[]string{"from", dates.Start.Format(realizedSavingsDateLayout)},
		[]string{"to (exclusive)", dates.End.Format(realizedSavingsDateLayout)},
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
	}{newRealizedSavingsPeriod(dates), summary}, []string{"FIELD", "VALUE"}, rows)
}

func runRealizedSavingsClusters(c *ucli.Context) error {
	opts, err := buildRealizedSavingsClusterListOptions(c, time.Now())
	if err != nil {
		return err
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	var page api.RealizedSavingsClusterPage
	if c.Bool("all") {
		page, err = resources.Runtime.API.ListAllPublicRealizedSavingsClusters(c.Context, resources.Profile.PublicAPIURL, resources.Token, opts, c.Int("page-cap"))
	} else {
		page, err = resources.Runtime.API.ListPublicRealizedSavingsClusters(c.Context, resources.Profile.PublicAPIURL, resources.Token, opts)
	}
	if err != nil {
		return err
	}

	return renderRealizedSavingsClusters(resources.Runtime, newRealizedSavingsPeriod(opts.Range), page)
}

func buildRealizedSavingsClusterListOptions(c *ucli.Context, now time.Time) (api.RealizedSavingsClusterListOptions, error) {
	dates, err := parseRealizedSavingsRange(c, now)
	if err != nil {
		return api.RealizedSavingsClusterListOptions{}, err
	}
	opts := api.RealizedSavingsClusterListOptions{Range: dates}

	if sortBy := strings.TrimSpace(c.String("sort")); sortBy != "" {
		if !slices.Contains(realizedSavingsClusterSortKeys, sortBy) {
			return opts, clierr.Usage("unsupported --sort %q: must be one of %s", sortBy, strings.Join(realizedSavingsClusterSortKeys, ", "))
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
	today := now.UTC().Truncate(24 * time.Hour)
	dates := api.RealizedSavingsRange{End: today.AddDate(0, 0, 1)}

	if value := strings.TrimSpace(c.String("to")); value != "" {
		end, err := time.Parse(realizedSavingsDateLayout, value)
		if err != nil {
			return dates, clierr.Usage("--to must be YYYY-MM-DD, got %q", value)
		}
		dates.End = end
	}

	dates.Start = dates.End.AddDate(0, 0, -30)
	if value := strings.TrimSpace(c.String("from")); value != "" {
		start, err := time.Parse(realizedSavingsDateLayout, value)
		if err != nil {
			return dates, clierr.Usage("--from must be YYYY-MM-DD, got %q", value)
		}
		dates.Start = start
	}

	if !dates.End.After(dates.Start) {
		return dates, clierr.Usage("--to must be after --from (--to is exclusive)")
	}
	return dates, nil
}

func renderRealizedSavingsClusters(rt *Runtime, period realizedSavingsPeriod, page api.RealizedSavingsClusterPage) error {
	writer := rt.Writer
	if writer == nil {
		writer = os.Stdout
	}

	switch rt.Config.Output {
	case "json":
		return output.WriteJSON(writer, map[string]any{
			"from":       period.From,
			"to":         period.To,
			"clusters":   page.Clusters,
			"pagination": page.Pagination,
		}, false)
	case "jsonl":
		values := make([]any, 0, len(page.Clusters))
		for _, item := range page.Clusters {
			values = append(values, item)
		}
		return output.WriteJSONL(writer, values)
	default:
		headers := []string{"NAME", "REALIZED_SAVINGS_USD", "SAVINGS_PCT", "TOTAL_COST_USD", "CPU_CORE_H_SAVED", "MEM_GB_H_SAVED", "PRICING_SOURCE", "TRANSMISSION"}
		rows := make([][]string, 0, len(page.Clusters))
		for _, item := range page.Clusters {
			rows = append(rows, []string{
				item.Name,
				fmt.Sprintf("%.2f", item.RealizedSavings),
				fmt.Sprintf("%.2f", item.RealizedSavingsPct),
				fmt.Sprintf("%.2f", item.TotalCost),
				fmt.Sprintf("%.1f (%d%%)", item.CPUCoreHoursSaved.Absolute, item.CPUCoreHoursSaved.Pct),
				fmt.Sprintf("%.1f (%d%%)", item.MemoryGBHoursSaved.Absolute, item.MemoryGBHoursSaved.Pct),
				item.PricingSource,
				item.TransmissionStatus,
			})
		}
		if err := output.WriteTable(writer, headers, rows); err != nil {
			return err
		}
		if next := page.Pagination.Next; next != nil && *next != "" {
			fmt.Fprintf(writer, "\n%d clusters shown. More available — pass --page-token %s (or use --all)\n", len(page.Clusters), *next)
		}
		return nil
	}
}
