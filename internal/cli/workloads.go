package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/perfectscale/poc-cli/internal/api"
	"github.com/perfectscale/poc-cli/internal/profile"
	ucli "github.com/urfave/cli/v2"
)

func workloadsCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "workloads",
		Usage: "Query workload cost, waste, risk, labels, and export data",
		Subcommands: []*ucli.Command{
			{
				Name:  "list",
				Usage: "List workloads for a cluster with filtering and sorting",
				Description: withCommandName(`Examples:
  {{cmd}} workloads list -c prod-a -w 30d -s waste -r desc -T 10
  {{cmd}} workloads list -c prod-a -w 30d -s waste -r asc -B 10
  {{cmd}} workloads list -c prod-a -n kube-system -w 30d
  {{cmd}} workloads list -c prod-a --view usage
  {{cmd}} workloads list -c prod-a --view all

Views:
  default
    Cost, waste, and max-indicator overview.
  capacity
    Replicas plus current and recommended request/limit totals.
  usage
    Summed container usage percentiles for the workload.
  policy
    Optimization policy, resilience, and mute state.
  risk
    Risk severity, risk counts, and waste counts.
  all
    The broadest workload view. When output is not explicitly set, this view defaults to jsonl so agents can consume the full enriched workload objects.

Filters such as namespace, name, type, min-cost, and min-waste are applied client-side.
The public workloads API is fixed to a 30 day window, so only --period 30d is supported.

Output schema (--output json):
  Array of Workload objects. All views produce the same JSON shape; --view only
  controls which columns appear in table mode.
  {
    "id": string, "name": string, "namespace": string, "type": string,
    "period": string, "cost": float64, "waste": float64,
    "potential_savings": float64, "cost_per_hour": float64,
    "running_minutes": int, "first_seen": string (RFC3339),
    "last_seen": string (RFC3339),
    "replicas_counts": { "max_count": int, "avg_count": int },
    "resilience_level": string, "optimization_policy": string,
    "optimization_policy_time_window": string,
    "cpu_optimization_policy": string, "memory_optimization_policy": string,
    "memory_request_equals_limit": bool,
    "mute_status": { "is_muted": bool, "expires": string (RFC3339) },
    "max_indicator": { "name": string, "type": string, "severity": int },
    "indicators": [{ "name": string, "type": string, "severity": int }],
    "workload_labels": map[string]string,
    "containers": [{
      "name": string, "running_minutes": int,
      "resources": {
        "current":      { "cpu_request_cores": float64, "cpu_limit_cores": float64, "memory_request_mib": float64, "memory_limit_mib": float64 },
        "recommended":  { "cpu_request_cores": float64, "cpu_limit_cores": float64, "memory_request_mib": float64, "memory_limit_mib": float64 }
      },
      "usage": {
        "cpu_cores":  { "p90": float64, "p95": float64, "p100": float64 },
        "memory_mib": { "p90": float64, "p95": float64, "p100": float64 }
      },
      "indicators": [{ "name": string, "type": string, "severity": int }]
    }],
    "derived": {
      "container_count": int, "indicators_count": int,
      "risk_indicators_count": int, "waste_indicators_count": int,
      "current_cpu_request_cores_total": float64, "current_cpu_limit_cores_total": float64,
      "current_memory_request_mib_total": float64, "current_memory_limit_mib_total": float64,
      "recommended_cpu_request_cores_total": float64, "recommended_memory_request_mib_total": float64,
      "cpu_usage_p90_cores_sum": float64, "cpu_usage_p95_cores_sum": float64,
      "cpu_usage_p100_cores_sum": float64,
      "memory_usage_p90_mib_sum": float64, "memory_usage_p95_mib_sum": float64,
      "memory_usage_p100_mib_sum": float64
    }
  }`),
				Flags: append(commonWorkloadSelectionFlags(), append(commonWorkloadListSortFlags(), append(commonTopBottomFlags(),
					&ucli.StringFlag{Name: "view", Aliases: []string{"V"}, Usage: "Output view preset: default, capacity, usage, policy, risk, or all", Value: "default"},
				)...)...),
				Action: runWorkloadsList,
			},
			{
				Name:  "summary",
				Usage: "Show aggregate cost, waste, and risk counters for a cluster",
				Description: withCommandName(`Examples:
  {{cmd}} workloads summary -c prod-a
  {{cmd}} workloads summary -c prod-a -n kube-system

This command summarizes the workload list after client-side filters are applied.

Output schema (--output json):
  {
    "cluster_uid": string, "cluster_name": string, "period": string,
    "workloads": int, "namespaces": int, "types": int,
    "muted_workloads": int, "risky_workloads": int, "waste_workloads": int,
    "total_cost": float64, "total_waste": float64, "total_potential_saving": float64,
    "top_namespace": string, "top_namespace_waste": float64,
    "top_type": string, "top_type_waste": float64
  }`),
				Flags:  commonWorkloadSelectionFlags(),
				Action: runWorkloadsSummary,
			},
			{
				Name:  "group-by",
				Usage: "Aggregate workloads by namespace, type, optimization policy, risk severity, or label value",
				Description: withCommandName(`Available group-by options:
  namespace
    Group by Kubernetes namespace.
  type
    Group by workload type such as Deployment or StatefulSet.
  optimization-policy
    Group by the Perfectscale optimization policy on the workload.
  risk-severity
    Group by the highest risk severity found on the workload. Bucket 0 means no risk indicators.
  label
    Group by the value of one workload label key. This mode requires --key or -k.

Examples:
  {{cmd}} workloads group-by namespace -c prod-a -s waste -r desc -T 10
  {{cmd}} workloads group-by optimization-policy -c prod-a -s workloads -r desc
  {{cmd}} workloads group-by risk-severity -c prod-a
  {{cmd}} workloads group-by label -c prod-a -k team -s waste -r desc

Output schema (--output json):
  Every group-by subcommand returns an array of:
    { "cluster_uid": string, "cluster_name": string, "field": string,
      "key": string, "period": string, "workloads": int,
      "muted_workloads": int, "risky_workloads": int, "waste_workloads": int,
      "total_cost": float64, "total_waste": float64 }`),
				Subcommands: []*ucli.Command{
					{
						Name:  "namespace",
						Usage: "Group workloads by namespace",
						Description: withCommandName(`Examples:
  {{cmd}} workloads group-by namespace -c prod-a
  {{cmd}} workloads group-by namespace -c prod-a -s waste -r desc -T 10

Output schema (--output json):
  Array of group summaries (field="namespace", key=namespace name):
    { "cluster_uid": string, "cluster_name": string, "field": string,
      "key": string, "period": string, "workloads": int,
      "muted_workloads": int, "risky_workloads": int, "waste_workloads": int,
      "total_cost": float64, "total_waste": float64 }`),
						Flags: append(commonWorkloadSelectionFlags(), append(groupByFlags(), commonTopBottomFlags()...)...),
						Action: func(c *ucli.Context) error {
							return runWorkloadsGroupBy(c, "namespace")
						},
					},
					{
						Name:  "type",
						Usage: "Group workloads by workload type",
						Description: withCommandName(`Examples:
  {{cmd}} workloads group-by type -c prod-a
  {{cmd}} workloads group-by type -c prod-a -s workloads -r desc

Output schema (--output json):
  Array of group summaries (field="type", key=workload type):
    { "cluster_uid": string, "cluster_name": string, "field": string,
      "key": string, "period": string, "workloads": int,
      "muted_workloads": int, "risky_workloads": int, "waste_workloads": int,
      "total_cost": float64, "total_waste": float64 }`),
						Flags: append(commonWorkloadSelectionFlags(), append(groupByFlags(), commonTopBottomFlags()...)...),
						Action: func(c *ucli.Context) error {
							return runWorkloadsGroupBy(c, "type")
						},
					},
					{
						Name:  "optimization-policy",
						Usage: "Group workloads by Perfectscale optimization policy",
						Description: withCommandName(`Examples:
  {{cmd}} workloads group-by optimization-policy -c prod-a
  {{cmd}} workloads group-by optimization-policy -c prod-a -s waste -r desc -T 10

Output schema (--output json):
  Array of group summaries (field="optimization-policy", key=policy name):
    { "cluster_uid": string, "cluster_name": string, "field": string,
      "key": string, "period": string, "workloads": int,
      "muted_workloads": int, "risky_workloads": int, "waste_workloads": int,
      "total_cost": float64, "total_waste": float64 }`),
						Flags: append(commonWorkloadSelectionFlags(), append(groupByFlags(), commonTopBottomFlags()...)...),
						Action: func(c *ucli.Context) error {
							return runWorkloadsGroupBy(c, "optimization-policy")
						},
					},
					{
						Name:  "risk-severity",
						Usage: "Group workloads by their highest risk severity bucket",
						Description: withCommandName(`Examples:
  {{cmd}} workloads group-by risk-severity -c prod-a
  {{cmd}} workloads group-by risk-severity -c prod-a -s workloads -r desc

Bucket 0 means the workload has no risk indicators.

Output schema (--output json):
  Array of group summaries (field="risk-severity", key=severity bucket as string):
    { "cluster_uid": string, "cluster_name": string, "field": string,
      "key": string, "period": string, "workloads": int,
      "muted_workloads": int, "risky_workloads": int, "waste_workloads": int,
      "total_cost": float64, "total_waste": float64 }`),
						Flags: append(commonWorkloadSelectionFlags(), append(groupByFlags(), commonTopBottomFlags()...)...),
						Action: func(c *ucli.Context) error {
							return runWorkloadsGroupBy(c, "risk-severity")
						},
					},
					{
						Name:  "label",
						Usage: "Group workloads by the value of one workload label key",
						Description: withCommandName(`Examples:
  {{cmd}} workloads group-by label -c prod-a -k team
  {{cmd}} workloads group-by label -c prod-a -k app -s waste -r desc -T 10

This command groups workloads by the value of the selected label key. Workloads missing that label are grouped under <missing>.

Output schema (--output json):
  Array of group summaries (field="label", key=label value, or "<missing>"):
    { "cluster_uid": string, "cluster_name": string, "field": string,
      "key": string, "period": string, "workloads": int,
      "muted_workloads": int, "risky_workloads": int, "waste_workloads": int,
      "total_cost": float64, "total_waste": float64 }`),
						Flags: append(commonWorkloadSelectionFlags(), append(groupByFlags(), append(commonTopBottomFlags(),
							&ucli.StringFlag{Name: "key", Aliases: []string{"k"}, Usage: "Label key to group by", Required: true},
						)...)...),
						Action: func(c *ucli.Context) error {
							return runWorkloadsGroupBy(c, "label")
						},
					},
				},
			},
			{
				Name:  "show",
				Usage: "Show a single workload in detail",
				Description: withCommandName(`Examples:
  {{cmd}} workloads show -c prod-a -i workload-123
  {{cmd}} workloads show -c prod-a -m api -n backend

Use --id when you want an exact workload match. Use --name for an exact workload name, and add --namespace when multiple workloads share the same name.

Output schema (--output json):
  A single, flattened object (note: flatter than the "workloads list" object):
  {
    "id": string, "name": string, "namespace": string, "type": string,
    "period": string, "cost": float64, "waste": float64,
    "potential_savings": float64, "cost_per_hour": float64,
    "running_minutes": int, "first_seen": string (RFC3339 or ""),
    "last_seen": string (RFC3339 or ""),
    "replicas_max_count": int, "replicas_avg_count": int,
    "resilience_level": string, "optimization_policy": string,
    "optimization_policy_time_window": string,
    "cpu_optimization_policy": string, "memory_optimization_policy": string,
    "memory_request_equals_limit": bool,
    "is_muted": bool, "mute_expires_at": string (RFC3339 or ""),
    "max_indicator":  { "name": string, "type": string, "severity": int } or null,
    "risk_indicator": { "name": string, "type": string, "severity": int } or null,
    "workload_labels": map[string]string,
    "indicators": [{ "name": string, "type": string, "severity": int }],
    "containers": [ /* same container shape as "workloads list" */ ]
  }`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query", Required: true},
					&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "Time window: 30d", Value: "30d"},
					&ucli.StringFlag{Name: "id", Aliases: []string{"i"}, Usage: "Exact workload ID to show"},
					&ucli.StringFlag{Name: "name", Aliases: []string{"m"}, Usage: "Exact workload name to show"},
					&ucli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Optional namespace to disambiguate --name"},
				},
				Action: runWorkloadsShow,
			},
			{
				Name:  "export",
				Usage: "Export workload rows as CSV",
				Description: withCommandName(`Examples:
  {{cmd}} workloads export -c prod-a
  {{cmd}} workloads export -c prod-a -n kube-system -s waste -r desc -T 25
  {{cmd}} workloads export -c prod-a -F workloads.csv

The export format is CSV in v1. When --file is omitted, CSV is written to stdout.

Output schema (CSV; the --output mode is ignored for export):
  Header row, then one row per workload, with columns:
    id,name,namespace,type,period,cost,waste,potential_savings,cost_per_hour,
    running_minutes,is_muted,mute_expires_at,max_indicator
  cost/waste/potential_savings use 2 decimals, cost_per_hour uses 4,
  mute_expires_at is RFC3339 ("" when not muted), and max_indicator is
  "<type>/<name>/<severity>" ("" when none).`),
				Flags: append(commonWorkloadSelectionFlags(), append(commonWorkloadListSortFlags(), append(commonTopBottomFlags(),
					&ucli.StringFlag{Name: "format", Aliases: []string{"f"}, Usage: "Export format. Only csv is supported in v1", Value: "csv"},
					&ucli.StringFlag{Name: "file", Aliases: []string{"F"}, Usage: "Optional file path. Defaults to stdout when omitted"},
				)...)...),
				Action: runWorkloadsExport,
			},
			{
				Name:  "risky",
				Usage: "List workloads that have risk indicators",
				Description: withCommandName(`Examples:
  {{cmd}} workloads risky -c prod-a
  {{cmd}} workloads risky -c prod-a -S 2 -s severity -r desc -T 10

Risky workloads are identified from public workload indicators and container indicators.

Output schema (--output json):
  Array of full Workload objects (same shape as "workloads list"). Inspect
  each object's "max_indicator"/"indicators" and "derived.risk_indicators_count"
  for risk detail.`),
				Flags: append(commonWorkloadSelectionFlags(), append(commonTopBottomFlags(),
					&ucli.IntFlag{Name: "min-severity", Aliases: []string{"S"}, Usage: "Only include workloads with a risk severity at or above this value", Value: 1},
					&ucli.StringFlag{Name: "sort", Aliases: []string{"s"}, Usage: "Sort by one of: severity, name, cost, waste", Value: "severity"},
					&ucli.StringFlag{Name: "order", Aliases: []string{"r"}, Usage: "Sort order: asc or desc", Value: "desc"},
				)...),
				Action: runWorkloadsRisky,
			},
			{
				Name:  "labels",
				Usage: "List distinct workload labels and their cost/waste footprint",
				Description: withCommandName(`Examples:
  {{cmd}} workloads labels -c prod-a
  {{cmd}} workloads labels -c prod-a -k app -s waste -r desc -T 20
  {{cmd}} workloads labels -c prod-a -v production

This command aggregates the workload label map into distinct key/value rows.

Output schema (--output json):
  Array of:
    { "cluster_uid": string, "cluster_name": string, "period": string,
      "key": string, "value": string, "workloads": int,
      "total_cost": float64, "total_waste": float64 }`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query", Required: true},
					&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "Time window: 30d", Value: "30d"},
					&ucli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Filter workloads by namespace before aggregating labels"},
					&ucli.StringFlag{Name: "name", Aliases: []string{"m"}, Usage: "Filter workloads by workload name substring before aggregating labels"},
					&ucli.StringFlag{Name: "type", Aliases: []string{"t"}, Usage: "Filter workloads by workload type before aggregating labels"},
					&ucli.Float64Flag{Name: "min-cost", Aliases: []string{"C"}, Usage: "Only include workloads with cost at or above this value before aggregating labels"},
					&ucli.Float64Flag{Name: "min-waste", Aliases: []string{"W"}, Usage: "Only include workloads with waste at or above this value before aggregating labels"},
					&ucli.StringFlag{Name: "key", Aliases: []string{"k"}, Usage: "Filter label rows by key substring"},
					&ucli.StringFlag{Name: "value", Aliases: []string{"v"}, Usage: "Filter label rows by value substring"},
					&ucli.StringFlag{Name: "sort", Aliases: []string{"s"}, Usage: "Sort by one of: key, value, workloads, cost, waste", Value: "key"},
					&ucli.StringFlag{Name: "order", Aliases: []string{"r"}, Usage: "Sort order: asc or desc", Value: "asc"},
					&ucli.IntFlag{Name: "top", Aliases: []string{"T"}, Usage: "Return only the first N label rows after filtering and sorting"},
					&ucli.IntFlag{Name: "bottom", Aliases: []string{"B"}, Usage: "Return only the last N label rows after filtering and sorting"},
				},
				Action: runWorkloadsLabels,
			},
			{
				Name:  "muted",
				Usage: "List workloads that are currently muted",
				Description: withCommandName(`Examples:
  {{cmd}} workloads muted -c prod-a
  {{cmd}} workloads muted -c prod-a -s expires -r asc
  {{cmd}} workloads muted -c prod-a -n kube-system -T 10

Output schema (--output json):
  Array of full Workload objects (same shape as "workloads list"). Mute detail
  is in each object's "mute_status": { "is_muted": bool, "expires": string (RFC3339) }.`),
				Flags: append(commonWorkloadSelectionFlags(), append(commonTopBottomFlags(),
					&ucli.StringFlag{Name: "sort", Aliases: []string{"s"}, Usage: "Sort by one of: expires, name, cost, waste", Value: "expires"},
					&ucli.StringFlag{Name: "order", Aliases: []string{"r"}, Usage: "Sort order: asc or desc", Value: "asc"},
				)...),
				Action: runWorkloadsMuted,
			},
		},
	}
}

func commonWorkloadSelectionFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query", Required: true},
		&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "Time window: 30d", Value: "30d"},
		&ucli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Filter workloads by namespace"},
		&ucli.StringFlag{Name: "name", Aliases: []string{"m"}, Usage: "Filter workloads by workload name substring"},
		&ucli.StringFlag{Name: "type", Aliases: []string{"t"}, Usage: "Filter workloads by workload type"},
		&ucli.Float64Flag{Name: "min-cost", Aliases: []string{"C"}, Usage: "Only include workloads with cost at or above this value"},
		&ucli.Float64Flag{Name: "min-waste", Aliases: []string{"W"}, Usage: "Only include workloads with waste at or above this value"},
	}
}

func commonWorkloadListSortFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.StringFlag{Name: "sort", Aliases: []string{"s"}, Usage: "Sort by one of: name, cost, waste", Value: "name"},
		&ucli.StringFlag{Name: "order", Aliases: []string{"r"}, Usage: "Sort order: asc or desc", Value: "asc"},
	}
}

func commonTopBottomFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.IntFlag{Name: "top", Aliases: []string{"T"}, Usage: "Return only the first N rows after filtering and sorting"},
		&ucli.IntFlag{Name: "bottom", Aliases: []string{"B"}, Usage: "Return only the last N rows after filtering and sorting"},
	}
}

func groupByFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.StringFlag{Name: "sort", Aliases: []string{"s"}, Usage: "Sort by one of: key, workloads, muted, risky, cost, waste", Value: "key"},
		&ucli.StringFlag{Name: "order", Aliases: []string{"r"}, Usage: "Sort order: asc or desc", Value: "asc"},
	}
}

func workloadFiltersFromContext(c *ucli.Context) WorkloadFilters {
	return WorkloadFilters{
		Namespace: c.String("namespace"),
		Name:      c.String("name"),
		Type:      c.String("type"),
		MinCost:   c.Float64("min-cost"),
		MinWaste:  c.Float64("min-waste"),
	}
}

func runWorkloadsList(c *ucli.Context) error {
	resources, _, workloads, err := loadFilteredWorkloads(c)
	if err != nil {
		return err
	}
	view, err := normalizeWorkloadView(c.String("view"))
	if err != nil {
		return err
	}

	sortWorkloads(workloads, c.String("sort"), c.String("order"))
	workloads, err = limitWorkloads(workloads, c.Int("top"), c.Int("bottom"))
	if err != nil {
		return err
	}
	applyImplicitOutputForWorkloadView(c, resources.Runtime, view)

	return renderWorkloadListView(resources.Runtime, workloads, view)
}

func runWorkloadsSummary(c *ucli.Context) error {
	resources, cluster, workloads, err := loadFilteredWorkloads(c)
	if err != nil {
		return err
	}

	summary := summarizeWorkloads(cluster, workloads)
	rows := [][]string{
		{"cluster_uid", summary.ClusterUID},
		{"cluster_name", summary.ClusterName},
		{"period", summary.Period},
		{"workloads", fmt.Sprintf("%d", summary.Workloads)},
		{"namespaces", fmt.Sprintf("%d", summary.Namespaces)},
		{"types", fmt.Sprintf("%d", summary.Types)},
		{"muted_workloads", fmt.Sprintf("%d", summary.MutedWorkloads)},
		{"risky_workloads", fmt.Sprintf("%d", summary.RiskyWorkloads)},
		{"waste_workloads", fmt.Sprintf("%d", summary.WasteWorkloads)},
		{"total_cost", fmt.Sprintf("%.2f", summary.TotalCost)},
		{"total_waste", fmt.Sprintf("%.2f", summary.TotalWaste)},
		{"total_potential_saving", fmt.Sprintf("%.2f", summary.TotalPotentialSaving)},
		{"top_namespace", summary.TopNamespace},
		{"top_namespace_waste", fmt.Sprintf("%.2f", summary.TopNamespaceWaste)},
		{"top_type", summary.TopType},
		{"top_type_waste", fmt.Sprintf("%.2f", summary.TopTypeWaste)},
	}

	return resources.Runtime.RenderTableOrJSON(summary, []string{"FIELD", "VALUE"}, rows)
}

func runWorkloadsGroupBy(c *ucli.Context, field string) error {
	resources, cluster, workloads, err := loadFilteredWorkloads(c)
	if err != nil {
		return err
	}

	var items []api.WorkloadGroupSummary
	switch field {
	case "label":
		items = groupWorkloadsByLabel(cluster, workloads, c.String("key"))
	default:
		items = groupWorkloads(cluster, workloads, field)
	}
	sortWorkloadGroups(items, c.String("sort"), c.String("order"))
	items, err = limitWorkloadGroups(items, c.Int("top"), c.Int("bottom"))
	if err != nil {
		return err
	}

	headers := []string{"KEY", "WORKLOADS", "MUTED", "RISKY", "COST", "WASTE", "PERIOD"}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.Key,
			fmt.Sprintf("%d", item.Workloads),
			fmt.Sprintf("%d", item.MutedWorkloads),
			fmt.Sprintf("%d", item.RiskyWorkloads),
			fmt.Sprintf("%.2f", item.TotalCost),
			fmt.Sprintf("%.2f", item.TotalWaste),
			item.Period,
		})
	}

	return resources.Runtime.RenderTableOrJSON(items, headers, rows)
}

func runWorkloadsShow(c *ucli.Context) error {
	resources, _, workloads, err := loadFilteredWorkloads(c)
	if err != nil {
		return err
	}

	workload, err := resolveWorkload(workloads, c.String("id"), c.String("name"), c.String("namespace"))
	if err != nil {
		return err
	}

	payload := map[string]any{
		"id":                              workload.ID,
		"name":                            workload.Name,
		"namespace":                       workload.Namespace,
		"type":                            workload.Type,
		"period":                          workload.Period,
		"cost":                            workload.Cost,
		"waste":                           workload.Waste,
		"potential_savings":               workload.PotentialSavings,
		"cost_per_hour":                   workload.CostPerHour,
		"running_minutes":                 workload.RunningMinutes,
		"first_seen":                      formatTime(workload.FirstSeen),
		"last_seen":                       formatTime(workload.LastSeen),
		"replicas_max_count":              workload.ReplicasCounts.MaxCount,
		"replicas_avg_count":              workload.ReplicasCounts.AvgCount,
		"resilience_level":                workload.ResilienceLevel,
		"optimization_policy":             workload.OptimizationPolicy,
		"optimization_policy_time_window": workload.OptimizationPolicyTimeWindow,
		"cpu_optimization_policy":         workload.CPUOptimizationPolicy,
		"memory_optimization_policy":      workload.MemoryOptimizationPolicy,
		"memory_request_equals_limit":     workload.MemoryRequestEqualsLimit,
		"is_muted":                        workload.MuteStatus.IsMuted,
		"mute_expires_at":                 formatTime(workload.MuteStatus.Expires),
		"max_indicator":                   workload.MaxIndicator,
		"risk_indicator":                  workloadRiskIndicator(workload),
		"workload_labels":                 workload.WorkloadLabels,
		"indicators":                      workload.Indicators,
		"containers":                      workload.Containers,
	}

	rows := [][]string{
		{"id", workload.ID},
		{"name", workload.Name},
		{"namespace", workload.Namespace},
		{"type", workload.Type},
		{"period", workload.Period},
		{"cost", fmt.Sprintf("%.2f", workload.Cost)},
		{"waste", fmt.Sprintf("%.2f", workload.Waste)},
		{"potential_savings", fmt.Sprintf("%.2f", workload.PotentialSavings)},
		{"cost_per_hour", fmt.Sprintf("%.4f", workload.CostPerHour)},
		{"running_minutes", fmt.Sprintf("%d", workload.RunningMinutes)},
		{"first_seen", formatTime(workload.FirstSeen)},
		{"last_seen", formatTime(workload.LastSeen)},
		{"replicas_max_count", fmt.Sprintf("%d", workload.ReplicasCounts.MaxCount)},
		{"replicas_avg_count", fmt.Sprintf("%d", workload.ReplicasCounts.AvgCount)},
		{"resilience_level", workload.ResilienceLevel},
		{"optimization_policy", workload.OptimizationPolicy},
		{"optimization_policy_time_window", workload.OptimizationPolicyTimeWindow},
		{"cpu_optimization_policy", workload.CPUOptimizationPolicy},
		{"memory_optimization_policy", workload.MemoryOptimizationPolicy},
		{"memory_request_equals_limit", fmt.Sprintf("%t", workload.MemoryRequestEqualsLimit)},
		{"is_muted", fmt.Sprintf("%t", workload.MuteStatus.IsMuted)},
		{"mute_expires_at", formatTime(workload.MuteStatus.Expires)},
		{"max_indicator", indicatorLabel(workload.MaxIndicator)},
		{"risk_indicator", indicatorLabel(workloadRiskIndicator(workload))},
		{"labels", formatLabelMap(workload.WorkloadLabels)},
		{"containers", fmt.Sprintf("%d", len(workload.Containers))},
	}

	return resources.Runtime.RenderTableOrJSON(payload, []string{"FIELD", "VALUE"}, rows)
}

func runWorkloadsExport(c *ucli.Context) error {
	format := strings.ToLower(strings.TrimSpace(c.String("format")))
	if format == "" {
		format = "csv"
	}
	if format != "csv" {
		return fmt.Errorf("unsupported --format %q: only csv is supported in v1", c.String("format"))
	}

	resources, _, workloads, err := loadFilteredWorkloads(c)
	if err != nil {
		return err
	}

	sortWorkloads(workloads, c.String("sort"), c.String("order"))
	workloads, err = limitWorkloads(workloads, c.Int("top"), c.Int("bottom"))
	if err != nil {
		return err
	}

	writer, closeFn, err := csvWriterForPath(c.String("file"), resources.Runtime.Writer)
	if err != nil {
		return err
	}
	defer func() {
		_ = closeFn()
	}()

	if err := writeWorkloadsCSV(writer, workloads); err != nil {
		return err
	}

	if strings.TrimSpace(c.String("file")) != "" {
		fmt.Fprintf(resources.Runtime.Writer, "Wrote %d workloads to %s\n", len(workloads), c.String("file"))
	}
	return nil
}

func runWorkloadsRisky(c *ucli.Context) error {
	resources, _, workloads, err := loadFilteredWorkloads(c)
	if err != nil {
		return err
	}

	minSeverity := c.Int("min-severity")
	if minSeverity < 1 {
		return fmt.Errorf("--min-severity must be at least 1")
	}

	workloads = filterRiskyWorkloads(workloads, minSeverity)
	sortRiskyWorkloads(workloads, c.String("sort"), c.String("order"))
	workloads, err = limitWorkloads(workloads, c.Int("top"), c.Int("bottom"))
	if err != nil {
		return err
	}

	headers := []string{"NAME", "NAMESPACE", "TYPE", "SEVERITY", "RISK_INDICATOR", "COST", "WASTE", "LAST_SEEN"}
	rows := make([][]string, 0, len(workloads))
	for _, item := range workloads {
		rows = append(rows, []string{
			item.Name,
			item.Namespace,
			item.Type,
			fmt.Sprintf("%d", workloadRiskSeverity(item)),
			indicatorLabel(workloadRiskIndicator(item)),
			fmt.Sprintf("%.2f", item.Cost),
			fmt.Sprintf("%.2f", item.Waste),
			formatTime(item.LastSeen),
		})
	}

	return resources.Runtime.RenderTableOrJSON(workloads, headers, rows)
}

func runWorkloadsLabels(c *ucli.Context) error {
	resources, cluster, workloads, err := loadFilteredWorkloads(c)
	if err != nil {
		return err
	}

	items := summarizeWorkloadLabels(cluster, workloads, c.String("key"), c.String("value"))
	sortWorkloadLabels(items, c.String("sort"), c.String("order"))
	items, err = limitWorkloadLabels(items, c.Int("top"), c.Int("bottom"))
	if err != nil {
		return err
	}

	headers := []string{"KEY", "VALUE", "WORKLOADS", "COST", "WASTE", "PERIOD"}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.Key,
			item.Value,
			fmt.Sprintf("%d", item.Workloads),
			fmt.Sprintf("%.2f", item.TotalCost),
			fmt.Sprintf("%.2f", item.TotalWaste),
			item.Period,
		})
	}

	return resources.Runtime.RenderTableOrJSON(items, headers, rows)
}

func runWorkloadsMuted(c *ucli.Context) error {
	resources, _, workloads, err := loadFilteredWorkloads(c)
	if err != nil {
		return err
	}

	workloads = filterMutedWorkloads(workloads)
	sortMutedWorkloads(workloads, c.String("sort"), c.String("order"))
	workloads, err = limitWorkloads(workloads, c.Int("top"), c.Int("bottom"))
	if err != nil {
		return err
	}

	headers := []string{"NAME", "NAMESPACE", "TYPE", "MUTE_EXPIRES_AT", "COST", "WASTE", "LAST_SEEN"}
	rows := make([][]string, 0, len(workloads))
	for _, item := range workloads {
		rows = append(rows, []string{
			item.Name,
			item.Namespace,
			item.Type,
			formatTime(item.MuteStatus.Expires),
			fmt.Sprintf("%.2f", item.Cost),
			fmt.Sprintf("%.2f", item.Waste),
			formatTime(item.LastSeen),
		})
	}

	return resources.Runtime.RenderTableOrJSON(workloads, headers, rows)
}

func loadFilteredWorkloads(c *ucli.Context) (*commandResources, api.Cluster, []api.Workload, error) {
	normalizedPeriod, err := normalizePeriod(c.String("period"))
	if err != nil {
		return nil, api.Cluster{}, nil, err
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return nil, api.Cluster{}, nil, err
	}

	cluster, workloads, err := resources.loadWorkloads(c.Context, c.String("cluster"), normalizedPeriod)
	if err != nil {
		return nil, api.Cluster{}, nil, err
	}

	workloads = filterWorkloads(workloads, workloadFiltersFromContext(c))
	return resources, cluster, workloads, nil
}

func formatLabelMap(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, labels[key]))
	}
	return strings.Join(parts, ", ")
}

func listClustersForProfile(ctx context.Context, rt *Runtime, data *profile.Data, token string) ([]api.Cluster, error) {
	if data.AuthMode != profile.AuthModeServiceToken {
		return nil, fmt.Errorf("profile %q uses unsupported auth mode %q; only service-token auth is supported now", data.Name, data.AuthMode)
	}
	return rt.API.ListPublicClusters(ctx, data.PublicAPIURL, token)
}
