package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/perfectscale/poc-cli/internal/api"
	"github.com/perfectscale/poc-cli/internal/output"
	"github.com/perfectscale/poc-cli/internal/profile"
	ucli "github.com/urfave/cli/v2"
)

func automationCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "automation",
		Usage: "Inspect Perfectscale automation activity in your clusters",
		Subcommands: []*ucli.Command{
			{
				Name:  "audit-logs",
				Usage: "List automation audit log entries (cursor-paginated, last 30 days)",
				Description: withCommandName(`Examples:
  {{cmd}} automation audit-logs
  {{cmd}} automation audit-logs -c prod-a -c prod-b
  {{cmd}} automation audit-logs -c prod-a -n kube-system -n default
  {{cmd}} automation audit-logs --from 2026-04-01T00:00:00Z --to 2026-04-15T00:00:00Z
  {{cmd}} automation audit-logs --since 24h
  {{cmd}} automation audit-logs --all -o jsonl
  {{cmd}} automation audit-logs --page-size 200 --after BASE64CURSOR

Fetches automation audit log entries from the public API. The endpoint is
cursor-paginated and only returns events from the last 30 days.

Filters --cluster (-c) and --namespace (-n) are repeatable and are sent to the
server as cluster_uids[] and namespaces[] respectively. Cluster values may be
either UID or name; names are resolved to UIDs locally before the call.

Time range:
  --from / --to accept RFC3339 (e.g. 2026-04-01T00:00:00Z).
  --since accepts a relative duration (e.g. 24h, 7d, 30m) and sets --from to
  now-since when --from is not provided.

Pagination:
  --page-size sets server page size (1-5000, default 1000).
  --after / --before consume cursor tokens from a previous response.
  --all auto-paginates forward until has_next=false (capped by --page-cap).

Output schema:
  --output json: a single object wrapping entries and cursor pagination:
    {
      "entries": [ <entry>, ... ],
      "pagination": { "has_next": bool, "next": string, "has_prev": bool,
                      "prev": string, "page_size": int }
    }
  --output jsonl: one <entry> object per line (pagination cursors are not emitted).
  Each <entry>:
    {
      "started_at": string (RFC3339), "cluster_uid": string, "cluster_name": string,
      "namespace": string, "workload_id": string, "workload_name": string,
      "workload_type": string,
      "executed": string (regular-eviction | inplace-resize | cleanup),
      "labels": map[string]string,
      "container": {
        "name": string,
        "cpu": {
          "cpu_cores_request": int, "recommend_cpu_cores_request": int,
          "cpu_cores_limits": int, "recommend_cpu_cores_limits": int,
          "cpu_request_impact": int, "cpu_limit_impact": int,
          "cpu_request_change_percent": float64, "cpu_limit_change_percent": float64,
          "cpu_request_change_absolute": int, "cpu_limit_change_absolute": int
        },
        "memory": {
          "mem_mib_request": int, "recommend_mem_mib_request": int,
          "mem_mib_limits": int, "recommend_mem_mib_limits": int,
          "mem_mib_request_impact": int, "mem_mib_limit_impact": int,
          "mem_request_change_percent": float64, "mem_limit_change_percent": float64,
          "mem_mib_request_change_absolute": int, "mem_mib_limit_change_absolute": int
        }
      }
    }`),
				Flags: []ucli.Flag{
					&ucli.StringSliceFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Filter by cluster name or UID. Repeatable."},
					&ucli.StringSliceFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Filter by Kubernetes namespace. Repeatable."},
					&ucli.StringFlag{Name: "from", Usage: "Lower time bound (RFC3339, UTC). Defaults server-side to 30 days ago."},
					&ucli.StringFlag{Name: "to", Usage: "Upper time bound (RFC3339, UTC). Defaults server-side to now."},
					&ucli.StringFlag{Name: "since", Usage: "Relative duration applied as --from (e.g. 24h, 7d). Ignored if --from is set."},
					&ucli.IntFlag{Name: "page-size", Usage: "Server page size (1-5000)", Value: 1000},
					&ucli.StringFlag{Name: "after", Usage: "Cursor token from a previous response's pagination.next"},
					&ucli.StringFlag{Name: "before", Usage: "Cursor token from a previous response's pagination.prev"},
					&ucli.BoolFlag{Name: "all", Usage: "Auto-paginate forward until has_next=false"},
					&ucli.IntFlag{Name: "page-cap", Usage: "Safety cap on pages fetched when --all is used", Value: 50},
					&ucli.StringFlag{Name: "execution", Usage: "Client-side filter on executed type (regular-eviction, inplace-resize, cleanup)"},
				},
				Action: runAutomationAuditLogs,
			},
		},
	}
}

func runAutomationAuditLogs(c *ucli.Context) error {
	rt, err := NewRuntime(c)
	if err != nil {
		return err
	}

	data, err := rt.LoadProfile()
	if err != nil {
		return err
	}
	if data.AuthMode != profile.AuthModeServiceToken {
		return fmt.Errorf("profile %q uses unsupported auth mode %q; only service-token auth is supported now", data.Name, data.AuthMode)
	}

	token, err := rt.ResolveToken(c.Context, data)
	if err != nil {
		return err
	}

	input, err := buildAutomationAuditLogsInput(c)
	if err != nil {
		return err
	}

	if clusterArgs := c.StringSlice("cluster"); len(clusterArgs) > 0 {
		clusters, err := listClustersForProfile(c.Context, rt, data, token)
		if err != nil {
			return err
		}
		uids, err := resolveClusterUIDs(clusters, clusterArgs)
		if err != nil {
			return err
		}
		input.ClusterUIDs = uids
	}
	if namespaces := c.StringSlice("namespace"); len(namespaces) > 0 {
		input.Namespaces = namespaces
	}

	executionFilter := strings.TrimSpace(strings.ToLower(c.String("execution")))
	if executionFilter != "" {
		switch executionFilter {
		case "regular-eviction", "inplace-resize", "cleanup":
		default:
			return fmt.Errorf("--execution must be one of regular-eviction, inplace-resize, cleanup")
		}
	}

	var (
		entries    []api.AutomationLogEntry
		pagination api.AutomationLogPagination
	)
	if c.Bool("all") {
		entries, pagination, err = rt.API.ListAllAutomationAuditLogs(c.Context, data.PublicAPIURL, token, input, c.Int("page-cap"))
	} else {
		var page api.AutomationLogPage
		page, err = rt.API.ListAutomationAuditLogs(c.Context, data.PublicAPIURL, token, input)
		entries = page.Entries
		pagination = page.Pagination
	}
	if err != nil {
		return err
	}

	if executionFilter != "" {
		filtered := make([]api.AutomationLogEntry, 0, len(entries))
		for _, item := range entries {
			if strings.EqualFold(item.Executed, executionFilter) {
				filtered = append(filtered, item)
			}
		}
		entries = filtered
	}

	return renderAutomationAuditLogs(rt, entries, pagination)
}

func buildAutomationAuditLogsInput(c *ucli.Context) (api.AutomationAuditLogsInput, error) {
	input := api.AutomationAuditLogsInput{}

	if v := strings.TrimSpace(c.String("from")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return input, fmt.Errorf("--from must be RFC3339 (e.g. 2026-04-01T00:00:00Z): %w", err)
		}
		t = t.UTC()
		input.From = &t
	} else if since := strings.TrimSpace(c.String("since")); since != "" {
		dur, err := parseRelativeDuration(since)
		if err != nil {
			return input, fmt.Errorf("--since: %w", err)
		}
		t := time.Now().UTC().Add(-dur)
		input.From = &t
	}

	if v := strings.TrimSpace(c.String("to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return input, fmt.Errorf("--to must be RFC3339 (e.g. 2026-04-15T00:00:00Z): %w", err)
		}
		t = t.UTC()
		input.To = &t
	}

	if pageSize := c.Int("page-size"); pageSize > 0 {
		if pageSize > 5000 {
			return input, fmt.Errorf("--page-size must be <= 5000")
		}
		ps := pageSize
		input.PageSize = &ps
	}

	after := strings.TrimSpace(c.String("after"))
	before := strings.TrimSpace(c.String("before"))
	if after != "" && before != "" {
		return input, fmt.Errorf("--after and --before cannot be used together")
	}
	if after != "" {
		if c.Bool("all") {
			a := after
			input.After = &a
		} else {
			a := after
			input.After = &a
		}
	}
	if before != "" {
		b := before
		input.Before = &b
	}

	return input, nil
}

// parseRelativeDuration extends time.ParseDuration with the convenient "Nd"
// (days) suffix the rest of the CLI uses.
func parseRelativeDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(value, "d") {
		days := strings.TrimSuffix(value, "d")
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q", value)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	dur, err := time.ParseDuration(value)
	if err != nil || dur <= 0 {
		return 0, fmt.Errorf("invalid duration %q", value)
	}
	return dur, nil
}

func resolveClusterUIDs(clusters []api.Cluster, targets []string) ([]string, error) {
	out := make([]string, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		cluster, err := resolveClusterByNameOrUID(clusters, target)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[cluster.UID]; ok {
			continue
		}
		seen[cluster.UID] = struct{}{}
		out = append(out, cluster.UID)
	}
	return out, nil
}

func renderAutomationAuditLogs(rt *Runtime, entries []api.AutomationLogEntry, pagination api.AutomationLogPagination) error {
	writer := rt.Writer
	if writer == nil {
		writer = os.Stdout
	}

	switch rt.Config.Output {
	case "json":
		return output.WriteJSON(writer, map[string]any{
			"entries":    entries,
			"pagination": pagination,
		})
	case "jsonl":
		values := make([]any, 0, len(entries))
		for _, item := range entries {
			values = append(values, item)
		}
		return output.WriteJSONL(writer, values)
	default:
		headers := []string{"STARTED_AT", "CLUSTER", "NAMESPACE", "WORKLOAD", "TYPE", "CONTAINER", "EXECUTED", "CPU_REQ_Δ%", "MEM_REQ_Δ%"}
		rows := make([][]string, 0, len(entries))
		for _, item := range entries {
			rows = append(rows, []string{
				item.StartedAt.UTC().Format(time.RFC3339),
				item.ClusterName,
				item.Namespace,
				item.WorkloadName,
				item.WorkloadType,
				item.Container.Name,
				item.Executed,
				fmt.Sprintf("%.1f", item.Container.CPU.CPURequestChangePercent),
				fmt.Sprintf("%.1f", item.Container.Memory.MemRequestChangePercent),
			})
		}
		if err := output.WriteTable(writer, headers, rows); err != nil {
			return err
		}
		if pagination.HasNext && pagination.Next != "" {
			fmt.Fprintf(writer, "\n%d entries shown. More available — pass --after %s (or use --all)\n", len(entries), pagination.Next)
		}
		return nil
	}
}
