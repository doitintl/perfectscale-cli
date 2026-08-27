package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/perfectscale/poc-cli/internal/api"
	"github.com/perfectscale/poc-cli/internal/clierr"
	"github.com/perfectscale/poc-cli/internal/output"
	ucli "github.com/urfave/cli/v2"
)

const unevictableSortBlockedCostHourly = "blockedCostHourly"

func unevictableCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "unevictable",
		Usage: "Inspect pods that autoscalers can't evict, why, and what they cost",
		Subcommands: []*ucli.Command{
			{
				Name:  "list",
				Usage: "List unevictable pods for a cluster",
				Description: withCommandName(`Examples:
  {{cmd}} unevictable list -c prod-a
  {{cmd}} unevictable list -c prod-a -n payments --reason pod_disruption_budget
  {{cmd}} unevictable list -c prod-a -C 5 -s blockedCostHourly -r desc

Served from the latest pre-computed snapshot for the cluster (no request-time
recompute). Returns a distinct error if the snapshot is still processing (202)
or failed to process (422) instead of a generic HTTP error.

-n, --reason, -g, and -C are server-side filters (AND-combined), not applied
client-side. --mute controls muted-finding visibility (default: exclude).
-s only accepts blockedCostHourly, the only server-side sort key today.

Pagination:
  --page-size sets server page size (1-500, default 50).
  --page-token consumes an opaque cursor from a previous response's
  meta.pagination.next (--output json) or the table-mode footer hint.
  --all auto-paginates forward until no next cursor remains (capped by
  --page-cap). Snapshot metadata (snapshot time, algorithm version, summary)
  is unavailable in --all mode since it auto-paginates.

Output schema (--output json):
  { "pods": [ <pod>, ... ], "pagination": {"next","prev","page_size"},
    "snapshot_time": string, "algorithm_version": string,
    "summary": {"total_pods","unevictable_pods","mute","total_nodes","autoscaler_type"} }
  <pod>:
    { "name","namespace","id","workload":{"id","name","type"},
      "reasons":[{"reason","reason_code","details","mute","muted_by_rule",
                  "remediation":{"fix_summary","risk","confidence",
                                 "current_spec","recommended_spec","yaml_diff"}}],
      "phase","start_time","labels","annotations",
      "spec":{"node","node_group","priority","node_selector","affinity",
              "tolerations","containers","volumes",
              "topology_spread_constraints","owner_references"},
      "blocked_node_count","blocked_nodes","blocked_cost_hourly",
      "cluster_uid","mute" }
  --output jsonl emits one <pod> object per line (no pagination cursor or
  snapshot metadata; use --output json or the table-mode footer for those).`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query"},
					&ucli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Server-side filter: exact namespace match"},
					&ucli.StringFlag{Name: "reason", Usage: "Server-side filter: exact reason code match (e.g. pod_disruption_budget)"},
					&ucli.StringFlag{Name: "node-group", Aliases: []string{"g"}, Usage: "Server-side filter: exact node group match"},
					&ucli.Float64Flag{Name: "min-blocked-cost", Aliases: []string{"C"}, Usage: "Server-side filter: minimum blocked cost per hour"},
					&ucli.StringFlag{Name: "mute", Usage: "Muted-finding visibility: exclude, include, or only", Value: "exclude"},
					&ucli.StringFlag{Name: "sort", Aliases: []string{"s"}, Usage: "Sort field: blockedCostHourly (the only supported value)"},
					&ucli.StringFlag{Name: "order", Aliases: []string{"r"}, Usage: "Sort direction: asc or desc", Value: "desc"},
					&ucli.IntFlag{Name: "page-size", Usage: "Server page size (1-500)", Value: 50},
					&ucli.StringFlag{Name: "page-token", Usage: "Opaque cursor from a previous response's pagination.next"},
					&ucli.BoolFlag{Name: "all", Usage: "Auto-paginate forward until no next cursor remains"},
					&ucli.IntFlag{Name: "page-cap", Usage: "Safety cap on pages fetched when --all is used", Value: 50},
				},
				Action: runUnevictableList,
			},
			{
				Name:  "report",
				Usage: "Per-pod unevictable report — one row per pod, with all its reasons combined",
				Description: withCommandName(`Examples:
  {{cmd}} unevictable report -c prod-a
  {{cmd}} unevictable report -c prod-a -n payments -C 5

Same filter/sort/mute/pagination flags as "unevictable list", except --reason:
the report endpoint's filter schema doesn't support reasonCode.

Output schema (--output json):
  { "rows": [ <row>, ... ], "pagination": {...}, "snapshot_time": string,
    "algorithm_version": string, "summary": {...} }
  <row>:
    { "name","id","workload":{"id","name","type"},"namespace","labels",
      "node","node_group","reasons":[...same shape as "unevictable list"...],
      "mute","priority","blocked_cost_hourly" }
  --output jsonl emits one <row> object per line.`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query"},
					&ucli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Server-side filter: exact namespace match"},
					&ucli.StringFlag{Name: "node-group", Aliases: []string{"g"}, Usage: "Server-side filter: exact node group match"},
					&ucli.Float64Flag{Name: "min-blocked-cost", Aliases: []string{"C"}, Usage: "Server-side filter: minimum blocked cost per hour"},
					&ucli.StringFlag{Name: "mute", Usage: "Muted-finding visibility: exclude, include, or only", Value: "exclude"},
					&ucli.StringFlag{Name: "sort", Aliases: []string{"s"}, Usage: "Sort field: blockedCostHourly (the only supported value)"},
					&ucli.StringFlag{Name: "order", Aliases: []string{"r"}, Usage: "Sort direction: asc or desc", Value: "desc"},
					&ucli.IntFlag{Name: "page-size", Usage: "Server page size (1-500)", Value: 50},
					&ucli.StringFlag{Name: "page-token", Usage: "Opaque cursor from a previous response's pagination.next"},
					&ucli.BoolFlag{Name: "all", Usage: "Auto-paginate forward until no next cursor remains"},
					&ucli.IntFlag{Name: "page-cap", Usage: "Safety cap on pages fetched when --all is used", Value: 50},
				},
				Action: runUnevictableReport,
			},
			{
				Name:  "show",
				Usage: "Show full detail for a single unevictable pod",
				Description: withCommandName(`Examples:
  {{cmd}} unevictable show -c prod-a -i a1b2c3d4

Includes remediation detail (fix summary, risk, confidence, current/recommended
spec, unified diff where available) and sibling pod names, which the list and
report endpoints don't populate.

Output schema (--output json):
  Same <pod> object shape as one entry from "unevictable list", plus
  "sibling_pod_names".`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query"},
					&ucli.StringFlag{Name: "id", Aliases: []string{"i"}, Usage: "Pod UID to fetch"},
				},
				Action: runUnevictableShow,
			},
			{
				Name:  "muted",
				Usage: "List workloads with an active mute (dismissal) rule",
				Description: withCommandName(`Examples:
  {{cmd}} unevictable muted -c prod-a

Read-only: mute/dismissal rules can only be created or removed via the web
app or the user API — this CLI has no corresponding write command.

Output schema (--output json):
  { "workloads": [ <workload>, ... ], "pagination": {...} }
  <workload>:
    { "cluster_uid","id","namespace","workload_name","note","created_by",
      "create_time","update_time" }
  --output jsonl emits one <workload> object per line.`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query"},
					&ucli.IntFlag{Name: "page-size", Usage: "Server page size (1-500)", Value: 50},
					&ucli.StringFlag{Name: "page-token", Usage: "Opaque cursor from a previous response's pagination.next"},
					&ucli.BoolFlag{Name: "all", Usage: "Auto-paginate forward until no next cursor remains"},
					&ucli.IntFlag{Name: "page-cap", Usage: "Safety cap on pages fetched when --all is used", Value: 50},
				},
				Action: runUnevictableMuted,
			},
		},
	}
}

func runUnevictableList(c *ucli.Context) error {
	opts, err := buildUnevictableListOptions(c)
	if err != nil {
		return err
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	cluster, err := resources.resolveCluster(c.Context, c.String("cluster"))
	if err != nil {
		return err
	}

	autoPaginated := c.Bool("all")

	var page api.UnevictablePodPage
	if autoPaginated {
		page, err = resources.Runtime.API.ListAllPublicUnevictablePods(c.Context, resources.Profile.PublicAPIURL, resources.Token, cluster.UID, opts, c.Int("page-cap"))
	} else {
		page, err = resources.Runtime.API.ListPublicUnevictablePods(c.Context, resources.Profile.PublicAPIURL, resources.Token, cluster.UID, opts)
	}
	if err != nil {
		return err
	}

	return renderUnevictableList(resources.Runtime, page.Pods, page, autoPaginated)
}

func runUnevictableReport(c *ucli.Context) error {
	opts, err := buildUnevictableListOptions(c)
	if err != nil {
		return err
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	cluster, err := resources.resolveCluster(c.Context, c.String("cluster"))
	if err != nil {
		return err
	}

	autoPaginated := c.Bool("all")

	var page api.UnevictableReportPage
	if autoPaginated {
		page, err = resources.Runtime.API.ListAllPublicUnevictableReport(c.Context, resources.Profile.PublicAPIURL, resources.Token, cluster.UID, opts, c.Int("page-cap"))
	} else {
		page, err = resources.Runtime.API.GetPublicUnevictableReport(c.Context, resources.Profile.PublicAPIURL, resources.Token, cluster.UID, opts)
	}
	if err != nil {
		return err
	}

	return renderUnevictableReport(resources.Runtime, page.Rows, page, autoPaginated)
}

func runUnevictableShow(c *ucli.Context) error {
	podUID := strings.TrimSpace(c.String("id"))
	if podUID == "" {
		return clierr.Usage("--id (-i) is required")
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	cluster, err := resources.resolveCluster(c.Context, c.String("cluster"))
	if err != nil {
		return err
	}

	pod, err := resources.Runtime.API.GetPublicUnevictablePod(c.Context, resources.Profile.PublicAPIURL, resources.Token, cluster.UID, podUID)
	if err != nil {
		return err
	}

	return renderUnevictableShow(resources.Runtime, *pod)
}

func runUnevictableMuted(c *ucli.Context) error {
	pageSize := c.Int("page-size")
	if pageSize > 500 {
		return clierr.Usage("--page-size must be <= 500")
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	cluster, err := resources.resolveCluster(c.Context, c.String("cluster"))
	if err != nil {
		return err
	}

	var page api.UnevictableMutedWorkloadPage
	if c.Bool("all") {
		page, err = resources.Runtime.API.ListAllPublicUnevictableMutedWorkloads(c.Context, resources.Profile.PublicAPIURL, resources.Token, cluster.UID, c.Int("page-cap"))
	} else {
		var size *int
		if pageSize > 0 {
			size = &pageSize
		}
		var token *string
		if pageToken := strings.TrimSpace(c.String("page-token")); pageToken != "" {
			token = &pageToken
		}
		page, err = resources.Runtime.API.ListPublicUnevictableMutedWorkloads(c.Context, resources.Profile.PublicAPIURL, resources.Token, cluster.UID, size, token)
	}
	if err != nil {
		return err
	}

	return renderUnevictableMuted(resources.Runtime, page.Workloads, page.Pagination)
}

// buildUnevictableListOptions is shared by "list" and "report". "report"
// simply never registers a --reason flag, since its filter schema doesn't
// support reasonCode — urfave/cli rejects the flag at parse time, so there's
// nothing to reject here.
func buildUnevictableListOptions(c *ucli.Context) (api.UnevictableListOptions, error) {
	opts := api.UnevictableListOptions{}

	if namespace := strings.TrimSpace(c.String("namespace")); namespace != "" {
		opts.Namespace = &namespace
	}

	if reason := strings.TrimSpace(c.String("reason")); reason != "" {
		opts.Reason = &reason
	}

	if nodeGroup := strings.TrimSpace(c.String("node-group")); nodeGroup != "" {
		opts.NodeGroup = &nodeGroup
	}

	if c.IsSet("min-blocked-cost") {
		minBlockedCost := c.Float64("min-blocked-cost")
		opts.MinBlockedCost = &minBlockedCost
	}

	if mute := strings.TrimSpace(c.String("mute")); mute != "" {
		switch mute {
		case "exclude", "include", "only":
			opts.Mute = &mute
		default:
			return opts, clierr.Usage("--mute must be one of exclude, include, only")
		}
	}

	if sortBy := strings.TrimSpace(c.String("sort")); sortBy != "" {
		if sortBy != unevictableSortBlockedCostHourly {
			return opts, clierr.Usage("--sort must be %q, the only supported value", unevictableSortBlockedCostHourly)
		}
		opts.SortBy = &sortBy

		order := strings.TrimSpace(c.String("order"))
		switch order {
		case "asc", "desc":
			opts.SortOrder = &order
		default:
			return opts, clierr.Usage("--order must be asc or desc")
		}
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

func renderUnevictableList(rt *Runtime, pods []api.UnevictablePod, page api.UnevictablePodPage, autoPaginated bool) error {
	writer := rt.Writer
	if writer == nil {
		writer = os.Stdout
	}

	switch rt.Config.Output {
	case "json":
		return output.WriteJSON(writer, map[string]any{
			"pods":              pods,
			"pagination":        page.Pagination,
			"snapshot_time":     page.SnapshotTime,
			"algorithm_version": page.AlgorithmVersion,
			"summary":           page.Summary,
		}, false)
	case "jsonl":
		values := make([]any, 0, len(pods))
		for _, item := range pods {
			values = append(values, item)
		}
		return output.WriteJSONL(writer, values)
	default:
		headers, rows := unevictablePodRows(pods)
		if err := output.WriteTable(writer, headers, rows); err != nil {
			return err
		}
		if autoPaginated {
			fmt.Fprintf(writer, "\n%d pods shown.\n", len(pods))
			return nil
		}
		fmt.Fprintf(writer, "\n%d pods shown. %s\n", len(pods), unevictableSnapshotFooter(page.SnapshotTime, page.AlgorithmVersion, page.Summary))
		if page.Pagination.Next != nil && *page.Pagination.Next != "" {
			fmt.Fprintf(writer, "More available — pass --page-token %s (or use --all)\n", *page.Pagination.Next)
		}
		return nil
	}
}

func unevictablePodRows(pods []api.UnevictablePod) ([]string, [][]string) {
	headers := []string{"NAME", "NAMESPACE", "REASON_CODES", "BLOCKED_NODES", "COST_HOURLY", "MUTE"}
	rows := make([][]string, 0, len(pods))
	for _, item := range pods {
		rows = append(rows, []string{
			item.Name,
			item.Namespace,
			unevictableReasonCodes(item.Reasons),
			fmt.Sprintf("%d", item.BlockedNodeCount),
			fmt.Sprintf("%.4f", item.BlockedCostHourly),
			fmt.Sprintf("%t", item.Mute),
		})
	}
	return headers, rows
}

func renderUnevictableReport(rt *Runtime, rows []api.UnevictableReportRow, page api.UnevictableReportPage, autoPaginated bool) error {
	writer := rt.Writer
	if writer == nil {
		writer = os.Stdout
	}

	switch rt.Config.Output {
	case "json":
		return output.WriteJSON(writer, map[string]any{
			"rows":              rows,
			"pagination":        page.Pagination,
			"snapshot_time":     page.SnapshotTime,
			"algorithm_version": page.AlgorithmVersion,
			"summary":           page.Summary,
		}, false)
	case "jsonl":
		values := make([]any, 0, len(rows))
		for _, item := range rows {
			values = append(values, item)
		}
		return output.WriteJSONL(writer, values)
	default:
		headers, tableRows := unevictableReportRows(rows)
		if err := output.WriteTable(writer, headers, tableRows); err != nil {
			return err
		}
		if autoPaginated {
			fmt.Fprintf(writer, "\n%d rows shown.\n", len(rows))
			return nil
		}
		fmt.Fprintf(writer, "\n%d rows shown. %s\n", len(rows), unevictableSnapshotFooter(page.SnapshotTime, page.AlgorithmVersion, page.Summary))
		if page.Pagination.Next != nil && *page.Pagination.Next != "" {
			fmt.Fprintf(writer, "More available — pass --page-token %s (or use --all)\n", *page.Pagination.Next)
		}
		return nil
	}
}

func unevictableReportRows(rows []api.UnevictableReportRow) ([]string, [][]string) {
	headers := []string{"NAME", "NAMESPACE", "NODE", "NODE_GROUP", "PRIORITY", "REASON_CODES", "COST_HOURLY", "MUTE"}
	tableRows := make([][]string, 0, len(rows))
	for _, item := range rows {
		node := item.Node
		if node == "" {
			node = "-"
		}
		tableRows = append(tableRows, []string{
			item.Name,
			item.Namespace,
			node,
			item.NodeGroup,
			fmt.Sprintf("%d", item.Priority),
			unevictableReasonCodes(item.Reasons),
			fmt.Sprintf("%.4f", item.BlockedCostHourly),
			fmt.Sprintf("%t", item.Mute),
		})
	}
	return headers, tableRows
}

func renderUnevictableShow(rt *Runtime, pod api.UnevictablePod) error {
	rows := [][]string{
		{"name", pod.Name},
		{"namespace", pod.Namespace},
		{"id", pod.ID},
		{"workload.id", pod.Workload.ID},
		{"workload.name", pod.Workload.Name},
		{"workload.type", pod.Workload.Type},
		{"phase", pod.Phase},
		{"start_time", formatTime(&pod.StartTime)},
		{"node", pod.Spec.Node},
		{"node_group", pod.Spec.NodeGroup},
		{"priority", fmt.Sprintf("%d", pod.Spec.Priority)},
		{"blocked_node_count", fmt.Sprintf("%d", pod.BlockedNodeCount)},
		{"blocked_nodes", strings.Join(pod.BlockedNodes, ", ")},
		{"blocked_cost_hourly", fmt.Sprintf("%.4f", pod.BlockedCostHourly)},
		{"mute", fmt.Sprintf("%t", pod.Mute)},
		{"sibling_pod_names", strings.Join(pod.SiblingPodNames, ", ")},
		{"labels", formatLabelMap(pod.Labels)},
		{"annotations", formatLabelMap(pod.Annotations)},
	}

	for i, reason := range pod.Reasons {
		prefix := fmt.Sprintf("reasons[%d]", i)
		rows = append(rows,
			[]string{prefix + ".reason", reason.Reason},
			[]string{prefix + ".reason_code", reason.ReasonCode},
			[]string{prefix + ".details", reason.Details},
			[]string{prefix + ".mute", fmt.Sprintf("%t", reason.Mute)},
			[]string{prefix + ".remediation.fix_summary", reason.Remediation.FixSummary},
			[]string{prefix + ".remediation.risk", reason.Remediation.Risk},
			[]string{prefix + ".remediation.confidence", reason.Remediation.Confidence},
		)
		if reason.MutedByRule != nil {
			rows = append(rows,
				[]string{prefix + ".muted_by_rule.created_by", reason.MutedByRule.CreatedBy},
				[]string{prefix + ".muted_by_rule.create_time", formatTime(&reason.MutedByRule.CreateTime)},
			)
		}
	}

	return rt.RenderTableOrJSON(pod, []string{"FIELD", "VALUE"}, rows)
}

func renderUnevictableMuted(rt *Runtime, workloads []api.UnevictableMutedWorkload, pagination api.CursorPagination) error {
	writer := rt.Writer
	if writer == nil {
		writer = os.Stdout
	}

	switch rt.Config.Output {
	case "json":
		return output.WriteJSON(writer, map[string]any{
			"workloads":  workloads,
			"pagination": pagination,
		}, false)
	case "jsonl":
		values := make([]any, 0, len(workloads))
		for _, item := range workloads {
			values = append(values, item)
		}
		return output.WriteJSONL(writer, values)
	default:
		headers := []string{"WORKLOAD", "NAMESPACE", "NOTE", "CREATED_BY", "CREATED_AT", "UPDATED_AT"}
		rows := make([][]string, 0, len(workloads))
		for _, item := range workloads {
			workloadLabel := item.WorkloadName
			if workloadLabel == "" {
				workloadLabel = item.ID
			}
			rows = append(rows, []string{
				workloadLabel,
				item.Namespace,
				item.Note,
				item.CreatedBy,
				formatTime(&item.CreateTime),
				formatTime(&item.UpdateTime),
			})
		}
		if err := output.WriteTable(writer, headers, rows); err != nil {
			return err
		}
		if pagination.Next != nil && *pagination.Next != "" {
			fmt.Fprintf(writer, "\n%d muted workloads shown. More available — pass --page-token %s (or use --all)\n", len(workloads), *pagination.Next)
		}
		return nil
	}
}

func unevictableReasonCodes(reasons []api.UnevictableReason) string {
	if len(reasons) == 0 {
		return "-"
	}
	codes := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		code := reason.ReasonCode
		if code == "" {
			code = reason.Reason
		}
		codes = append(codes, code)
	}
	return strings.Join(codes, ",")
}

func unevictableSnapshotFooter(snapshotTime time.Time, algorithmVersion string, summary api.UnevictableSummary) string {
	return fmt.Sprintf(
		"Snapshot: %s (algorithm %s). Summary: total=%d unevictable=%d muted=%d nodes=%d%s",
		formatTime(&snapshotTime), algorithmVersion,
		summary.TotalPods, summary.UnevictablePods, summary.Mute, summary.TotalNodes,
		unevictableAutoscalerSuffix(summary.AutoscalerType),
	)
}

func unevictableAutoscalerSuffix(autoscalerType string) string {
	if autoscalerType == "" {
		return ""
	}
	return " autoscaler=" + autoscalerType
}
