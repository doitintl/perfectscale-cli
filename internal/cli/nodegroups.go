package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/perfectscale/poc-cli/internal/api"
	"github.com/perfectscale/poc-cli/internal/clierr"
	"github.com/perfectscale/poc-cli/internal/output"
	ucli "github.com/urfave/cli/v2"
)

func nodegroupsCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "nodegroups",
		Usage: "List and inspect InfraFit node groups for a cluster",
		Subcommands: []*ucli.Command{
			{
				Name:  "list",
				Usage: "List node groups with utilization, cost, and recommendation summaries",
				Description: withCommandName(`Examples:
  {{cmd}} nodegroups list -c prod-a
  {{cmd}} nodegroups list -c prod-a --autoscaler-type karpenter --has-recommendations
  {{cmd}} nodegroups list -c prod-a --all -o jsonl

Server-side filters (not client-side): --autoscaler-type, --has-recommendations,
--include-muted. See Pagination below.

Views (-V, --view):
  default: NAME, AUTOSCALER, NODES_AVG, PODS_AVG, COST_HOURLY, COST_IDLE,
           CPU_REQ_AVG, MEM_REQ_AVG_GIB, REC_TYPE, REC_COUNT, TOP_INSTANCE_TYPE
    REC_TYPE/REC_COUNT summarize the recommendation (discriminated union:
    "standard" or "karpenter"). TOP_INSTANCE_TYPE is the #1-ranked replacement
    instance type, populated only for "standard" — always "-" for "karpenter"
    (those are NodePool config diffs, not a ranked instance-type list).
    Use -o json/jsonl for full recommendation detail.
  gpu: NAME, AUTOSCALER, NODES_AVG, GPU_ARCH, GPU_REQ_UNITS_AVG,
       GPU_USED_UNITS_AVG, GPU_USED_MEM_AVG_MIB
    "-" in every GPU column for node groups with no GPU.

Pagination:
  --page-size (1-500, default 50); --page-token consumes a cursor from a prior
  response's meta.pagination.next (-o json, or the table footer hint); --all
  auto-paginates forward (capped by --page-cap), always at max page size since
  the backend recomputes the full node-group set on every request.

Output schema (--output json):
  { "node_groups": [ <node group>, ... ], "pagination": {"next","prev","page_size"} }
  <node group>:
    { "id": string, "autoscaler_type": string, "architectures": [string],
      "reservations": [string], "nodes": {"min","max","avg"},
      "pods": {"capacity","allocatable","avg_count"}, "running_minutes": int,
      "cpu": {"idle","requested":{avg,min,max,p80,p95,p99,p999},"used":{...}},
      "mem": {...same shape as cpu...},
      "gpu": {"architectures":[string],"sharing_type":[string],"idle_memory_mib","idle_units",
              "requested":{avg_memory_mib,min_memory_mib,max_memory_mib,p80..p999_memory_mib,
                           avg_units,min_units,max_units,p80..p999_units},"used":{...}}
             (omitted entirely for node groups/types with no GPU),
      "cost": {"hourly","timeframe","idle_cpu","idle_mem","idle_gpu","idle_total"},
      "labels": map[string]string,
      "node_types": [ {...per-instance-type breakdown, same shape as a node group...} ],
      "recommendations": {
        "type": "standard"|"karpenter", "has_changes": bool,
        "node_type_recommendations": [ {"id","instance_type","instance_family",
          "hourly_cost","estimated_savings","estimated_savings_pct","node_count"}, ... ]
          (only for type="standard"; estimated_savings is a MONTHLY dollar figure —
           current monthly forecast minus recommended monthly forecast, not hourly
           and not the same basis as this node group's own cost.timeframe below),
        "changes": [...], "current_config": {...}, "recommended_config": {...}
          (only for type="karpenter")
      }
    }
  --output jsonl emits one <node group> object per line (no pagination cursor;
  use --output json or the table-mode footer to get it).`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query"},
					&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "ISO-8601 duration window for utilization/cost figures", Value: "P30D"},
					&ucli.StringFlag{Name: "view", Aliases: []string{"V"}, Usage: "Table view preset: default or gpu", Value: "default"},
					&ucli.StringFlag{Name: "autoscaler-type", Usage: "Server-side filter by autoscaler type (e.g. karpenter, cluster_autoscaler)"},
					&ucli.BoolFlag{Name: "has-recommendations", Usage: "Server-side filter: only node groups with an actionable recommendation"},
					&ucli.BoolFlag{Name: "include-muted", Usage: "Include muted recommendations"},
					&ucli.IntFlag{Name: "recommendation-limit", Usage: "Cap node-type recommendations per node group (1-20)", Value: 3},
					&ucli.IntFlag{Name: "page-size", Usage: "Server page size (1-500)", Value: 50},
					&ucli.StringFlag{Name: "page-token", Usage: "Opaque cursor from a previous response's pagination.next"},
					&ucli.BoolFlag{Name: "all", Usage: "Auto-paginate forward until no next cursor remains"},
					&ucli.IntFlag{Name: "page-cap", Usage: "Safety cap on pages fetched when --all is used", Value: 50},
				},
				Action: runNodegroupsList,
			},
			{
				Name:  "get",
				Usage: "Show a single node group with full recommendation detail",
				Description: withCommandName(`Examples:
  {{cmd}} nodegroups get -c prod-a -g clickhouse

Table output lists each recommended change below the summary fields: Karpenter
gets one row per NodePool field changed, showing the recommended value and why
(rationale); standard gets one row per recommended instance type, with cost/
savings (no rationale upstream). Use -o json for the full nested payload,
including the current value and the raw NodePool config diff.

Output schema (--output json):
  Same object shape as one entry from "nodegroups list".`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query"},
					&ucli.StringFlag{Name: "node-group", Aliases: []string{"g"}, Usage: "Node group name"},
					&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "ISO-8601 duration window for utilization/cost figures", Value: "P30D"},
					&ucli.IntFlag{Name: "recommendation-limit", Usage: "Cap node-type recommendations returned (1-20)", Value: 3},
				},
				Action: runNodegroupsGet,
			},
		},
	}
}

func runNodegroupsList(c *ucli.Context) error {
	view, err := normalizeNodegroupsView(c.String("view"))
	if err != nil {
		return err
	}

	opts, err := buildNodeGroupListOptions(c)
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

	var page api.NodeGroupPage
	if c.Bool("all") {
		page, err = resources.Runtime.API.ListAllPublicNodeGroups(c.Context, resources.Profile.PublicAPIURL, resources.Token, cluster.UID, opts, c.Int("page-cap"))
	} else {
		page, err = resources.Runtime.API.ListPublicNodeGroups(c.Context, resources.Profile.PublicAPIURL, resources.Token, cluster.UID, opts)
	}
	if err != nil {
		return err
	}

	return renderNodegroupsList(resources.Runtime, page.NodeGroups, page.Pagination, view)
}

const (
	nodegroupsViewDefault = "default"
	nodegroupsViewGPU     = "gpu"
)

func normalizeNodegroupsView(view string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(view))
	if normalized == "" {
		return nodegroupsViewDefault, nil
	}

	switch normalized {
	case nodegroupsViewDefault, nodegroupsViewGPU:
		return normalized, nil
	default:
		return "", clierr.Usage("unsupported --view %q: must be one of default, gpu", view)
	}
}

func runNodegroupsGet(c *ucli.Context) error {
	nodeGroupName := strings.TrimSpace(c.String("node-group"))
	if nodeGroupName == "" {
		return clierr.Usage("--node-group (-g) is required")
	}

	limit := c.Int("recommendation-limit")
	if limit > 20 {
		return clierr.Usage("--recommendation-limit must be <= 20")
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	cluster, err := resources.resolveCluster(c.Context, c.String("cluster"))
	if err != nil {
		return err
	}

	group, err := resources.Runtime.API.GetPublicNodeGroup(c.Context, resources.Profile.PublicAPIURL, resources.Token, cluster.UID, nodeGroupName, c.String("period"), limit)
	if err != nil {
		return err
	}

	return renderNodegroupGet(resources.Runtime, *group)
}

func buildNodeGroupListOptions(c *ucli.Context) (api.NodeGroupListOptions, error) {
	opts := api.NodeGroupListOptions{}

	if period := strings.TrimSpace(c.String("period")); period != "" {
		opts.Period = &period
	}

	if autoscalerType := strings.TrimSpace(c.String("autoscaler-type")); autoscalerType != "" {
		opts.AutoscalerType = &autoscalerType
	}

	if c.IsSet("has-recommendations") {
		hasRecommendations := c.Bool("has-recommendations")
		opts.HasRecommendations = &hasRecommendations
	}

	if c.IsSet("include-muted") {
		includeMuted := c.Bool("include-muted")
		opts.IncludeMuted = &includeMuted
	}

	if limit := c.Int("recommendation-limit"); limit > 0 {
		if limit > 20 {
			return opts, clierr.Usage("--recommendation-limit must be <= 20")
		}
		opts.RecommendationLimit = &limit
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

func renderNodegroupsList(rt *Runtime, groups []api.NodeGroup, pagination api.CursorPagination, view string) error {
	writer := rt.Writer
	if writer == nil {
		writer = os.Stdout
	}

	switch rt.Config.Output {
	case "json":
		return output.WriteJSON(writer, map[string]any{
			"node_groups": groups,
			"pagination":  pagination,
		}, false)
	case "jsonl":
		values := make([]any, 0, len(groups))
		for _, item := range groups {
			values = append(values, item)
		}
		return output.WriteJSONL(writer, values)
	default:
		headers, rows := nodegroupListRows(groups, view)
		if err := output.WriteTable(writer, headers, rows); err != nil {
			return err
		}
		if pagination.Next != nil && *pagination.Next != "" {
			fmt.Fprintf(writer, "\n%d node groups shown. More available — pass --page-token %s (or use --all)\n", len(groups), *pagination.Next)
		}
		return nil
	}
}

func nodegroupListRows(groups []api.NodeGroup, view string) ([]string, [][]string) {
	switch view {
	case nodegroupsViewGPU:
		headers := []string{"NAME", "AUTOSCALER", "NODES_AVG", "GPU_ARCH", "GPU_REQ_UNITS_AVG", "GPU_USED_UNITS_AVG", "GPU_USED_MEM_AVG_MIB"}
		rows := make([][]string, 0, len(groups))
		for _, item := range groups {
			rows = append(rows, []string{
				item.ID,
				item.AutoscalerType,
				fmt.Sprintf("%.2f", item.Nodes.Avg),
				gpuArchLabel(item.GPU),
				gpuMetric(item.GPU, func(g *api.GPUUtilization) float64 { return g.Requested.AvgUnits }),
				gpuMetric(item.GPU, func(g *api.GPUUtilization) float64 { return g.Used.AvgUnits }),
				gpuMetric(item.GPU, func(g *api.GPUUtilization) float64 { return g.Used.AvgMemoryMiB }),
			})
		}
		return headers, rows
	default:
		headers := []string{"NAME", "AUTOSCALER", "NODES_AVG", "PODS_AVG", "COST_HOURLY", "COST_IDLE", "CPU_REQ_AVG", "MEM_REQ_AVG_GIB", "REC_TYPE", "REC_COUNT", "TOP_INSTANCE_TYPE"}
		rows := make([][]string, 0, len(groups))
		for _, item := range groups {
			rows = append(rows, []string{
				item.ID,
				item.AutoscalerType,
				fmt.Sprintf("%.2f", item.Nodes.Avg),
				fmt.Sprintf("%.2f", item.Pods.AvgCount),
				fmt.Sprintf("%.4f", item.Cost.Hourly),
				fmt.Sprintf("%.2f", item.Cost.IdleTotal),
				fmt.Sprintf("%.2f", item.CPU.Requested.Avg),
				fmt.Sprintf("%.3f", mibToGiB(item.Mem.Requested.Avg)),
				nodeGroupRecommendationsType(item.Recommendations),
				fmt.Sprintf("%d", nodeGroupRecommendationsCount(item.Recommendations)),
				nodeGroupTopInstanceType(item.Recommendations),
			})
		}
		return headers, rows
	}
}

func gpuArchLabel(gpu *api.GPUUtilization) string {
	if gpu == nil || len(gpu.Architectures) == 0 {
		return "-"
	}
	return strings.Join(gpu.Architectures, ",")
}

func gpuMetric(gpu *api.GPUUtilization, extract func(*api.GPUUtilization) float64) string {
	if gpu == nil {
		return "-"
	}
	return fmt.Sprintf("%.3f", extract(gpu))
}

func renderNodegroupGet(rt *Runtime, group api.NodeGroup) error {
	rows := [][]string{
		{"id", group.ID},
		{"autoscaler_type", group.AutoscalerType},
		{"architectures", strings.Join(group.Architectures, ", ")},
		{"reservations", strings.Join(group.Reservations, ", ")},
		{"nodes_min", fmt.Sprintf("%d", group.Nodes.Min)},
		{"nodes_max", fmt.Sprintf("%d", group.Nodes.Max)},
		{"nodes_avg", fmt.Sprintf("%.2f", group.Nodes.Avg)},
		{"pods_capacity", fmt.Sprintf("%d", group.Pods.Capacity)},
		{"pods_allocatable", fmt.Sprintf("%d", group.Pods.Allocatable)},
		{"pods_avg_count", fmt.Sprintf("%.2f", group.Pods.AvgCount)},
		{"running_minutes", fmt.Sprintf("%d", group.RunningMinutes)},
		{"cpu_requested_avg", fmt.Sprintf("%.2f", group.CPU.Requested.Avg)},
		{"cpu_used_avg", fmt.Sprintf("%.2f", group.CPU.Used.Avg)},
		{"mem_requested_avg_gib", fmt.Sprintf("%.3f", mibToGiB(group.Mem.Requested.Avg))},
		{"mem_used_avg_gib", fmt.Sprintf("%.3f", mibToGiB(group.Mem.Used.Avg))},
		{"cost_hourly", fmt.Sprintf("%.4f", group.Cost.Hourly)},
		{"cost_timeframe", fmt.Sprintf("%.2f", group.Cost.Timeframe)},
		{"cost_idle_total", fmt.Sprintf("%.2f", group.Cost.IdleTotal)},
		{"node_types", fmt.Sprintf("%d", len(group.NodeTypes))},
		{"labels", formatLabelMap(group.Labels)},
		{"recommendations.type", nodeGroupRecommendationsType(group.Recommendations)},
		{"recommendations.has_changes", fmt.Sprintf("%t", group.Recommendations.HasChanges)},
		{"recommendations.count", fmt.Sprintf("%d", nodeGroupRecommendationsCount(group.Recommendations))},
	}
	rows = append(rows, nodeGroupRecommendationRows(group.Recommendations)...)

	return rt.RenderTableOrJSON(group, []string{"FIELD", "VALUE"}, rows)
}

// nodeGroupRecommendationRows expands each recommendation into its own row so table
// mode shows the actual change, not just a count; -o json still has the full payload
// (current value, raw NodePool config diff for karpenter). Karpenter rows show the
// recommended value and rationale (what + why); standard node-type recommendations
// have no rationale upstream, so they show cost/savings instead.
func nodeGroupRecommendationRows(rec api.NodeGroupRecommendations) [][]string {
	var rows [][]string

	switch rec.Type {
	case api.NodeGroupRecommendationsTypeKarpenter:
		for i, change := range rec.Changes {
			field := fmt.Sprintf("recommendation[%d] %s", i+1, change.Title)
			value := recommendationValueString(change.RecommendedValue)
			if change.Rationale != "" {
				value = fmt.Sprintf("%s — %s", value, change.Rationale)
			}
			rows = append(rows, []string{field, value})
		}
	case api.NodeGroupRecommendationsTypeStandard:
		for i, nodeType := range rec.NodeTypeRecs {
			field := fmt.Sprintf("recommendation[%d] %s", i+1, nodeType.InstanceType)
			value := fmt.Sprintf("$%.4f/hr, save $%.2f (%.1f%%), %d node(s)",
				nodeType.HourlyCost, nodeType.EstimatedSavings, nodeType.EstimatedSavingsPct, nodeType.NodeCount)
			rows = append(rows, []string{field, value})
		}
	}

	return rows
}

// recommendationValueString renders a Karpenter change's current/recommended value
// (string, number, bool, array, object, or nil per the public API) for table display.
func recommendationValueString(v any) string {
	switch value := v.(type) {
	case nil:
		return "-"
	case string:
		return value
	default:
		b, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(b)
	}
}

func mibToGiB(value float64) float64 {
	return value / 1024
}

func nodeGroupRecommendationsType(rec api.NodeGroupRecommendations) string {
	if rec.Type == "" {
		return "-"
	}
	return rec.Type
}

func nodeGroupRecommendationsCount(rec api.NodeGroupRecommendations) int {
	switch rec.Type {
	case api.NodeGroupRecommendationsTypeStandard:
		return len(rec.NodeTypeRecs)
	case api.NodeGroupRecommendationsTypeKarpenter:
		return len(rec.Changes)
	default:
		return 0
	}
}

func nodeGroupTopInstanceType(rec api.NodeGroupRecommendations) string {
	if rec.Type == api.NodeGroupRecommendationsTypeStandard && len(rec.NodeTypeRecs) > 0 {
		return rec.NodeTypeRecs[0].InstanceType
	}
	return "-"
}
