package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/perfectscale/poc-cli/internal/api"
	"github.com/perfectscale/poc-cli/internal/auth"
	"github.com/perfectscale/poc-cli/internal/config"
	"github.com/perfectscale/poc-cli/internal/output"
	"github.com/perfectscale/poc-cli/internal/profile"
	ucli "github.com/urfave/cli/v2"
)

type Runtime struct {
	Config config.Settings
	Store  *profile.Store
	Auth   *auth.Manager
	API    *api.Client
	Writer io.Writer
}

type WorkloadFilters struct {
	Namespace string
	Name      string
	Type      string
	MinCost   float64
	MinWaste  float64
}

type NamespaceFilters struct {
	Namespace string
}

func NewRuntime(c *ucli.Context) (*Runtime, error) {
	outputMode, err := config.NormalizeOutput(stringFlagValue(c, "output"))
	if err != nil {
		return nil, err
	}

	store, err := profile.NewStore("")
	if err != nil {
		return nil, err
	}

	settings := config.Settings{
		Profile:      stringFlagValue(c, "profile"),
		Output:       outputMode,
		Debug:        boolFlagValue(c, "debug"),
		PublicAPIURL: config.NormalizePublicAPIBaseURL(stringFlagValue(c, "public-api-url")),
	}

	return &Runtime{
		Config: settings,
		Store:  store,
		Auth:   auth.NewManager(store),
		API:    api.NewClient(),
		Writer: c.App.Writer,
	}, nil
}

func isFlagSetInLineage(c *ucli.Context, name string) bool {
	for _, ctx := range c.Lineage() {
		if ctx.IsSet(name) {
			return true
		}
	}
	return false
}

func stringFlagValue(c *ucli.Context, name string) string {
	for _, ctx := range c.Lineage() {
		if ctx.IsSet(name) {
			return ctx.String(name)
		}
	}
	return c.String(name)
}

func boolFlagValue(c *ucli.Context, name string) bool {
	for _, ctx := range c.Lineage() {
		if ctx.IsSet(name) {
			return ctx.Bool(name)
		}
	}
	return c.Bool(name)
}

func (r *Runtime) LoadProfile() (*profile.Data, error) {
	return r.Auth.Load(r.Config.Profile)
}

func (r *Runtime) ResolveToken(ctx context.Context, data *profile.Data) (string, error) {
	return r.Auth.EnsureAccessToken(ctx, data)
}

func (r *Runtime) RenderTableOrJSON(value any, headers []string, rows [][]string) error {
	writer := r.Writer
	if writer == nil {
		writer = os.Stdout
	}
	switch r.Config.Output {
	case "json":
		return output.WriteJSON(writer, value)
	case "jsonl":
		jsonValues, err := asSlice(value)
		if err != nil {
			return output.WriteJSON(writer, value)
		}
		return output.WriteJSONL(writer, jsonValues)
	default:
		return output.WriteTable(writer, headers, rows)
	}
}

func asSlice(value any) ([]any, error) {
	switch typed := value.(type) {
	case []api.Cluster:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, nil
	case []api.Workload:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, nil
	case []api.NamespaceSummary:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, nil
	case []api.ClusterEmissionEntry:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, nil
	case []api.WorkloadGroupSummary:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, nil
	case []api.WorkloadLabelSummary:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("jsonl output is only supported for list commands")
	}
}

func resolveClusterByNameOrUID(clusters []api.Cluster, target string) (api.Cluster, error) {
	if strings.TrimSpace(target) == "" {
		return api.Cluster{}, fmt.Errorf("--cluster is required")
	}

	target = strings.TrimSpace(target)
	var matches []api.Cluster
	for _, cluster := range clusters {
		if strings.EqualFold(cluster.UID, target) || strings.EqualFold(cluster.Name, target) {
			matches = append(matches, cluster)
		}
	}

	if len(matches) == 0 {
		return api.Cluster{}, fmt.Errorf("cluster %q not found", target)
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, item := range matches {
			names = append(names, fmt.Sprintf("%s (%s)", item.Name, item.UID))
		}
		sort.Strings(names)
		return api.Cluster{}, fmt.Errorf("cluster %q is ambiguous: %s", target, strings.Join(names, ", "))
	}

	return matches[0], nil
}

func normalizePeriod(period string) (string, error) {
	if period == "" {
		period = "30d"
	}
	if period != "30d" {
		return "", fmt.Errorf("only --period 30d is supported because the public workloads API is fixed to a 30 day window")
	}
	return "30d", nil
}

func filterWorkloads(items []api.Workload, filters WorkloadFilters) []api.Workload {
	filtered := make([]api.Workload, 0, len(items))
	for _, item := range items {
		if filters.Namespace != "" && !strings.EqualFold(item.Namespace, filters.Namespace) {
			continue
		}
		if filters.Name != "" && !strings.Contains(strings.ToLower(item.Name), strings.ToLower(filters.Name)) {
			continue
		}
		if filters.Type != "" && !strings.EqualFold(item.Type, filters.Type) {
			continue
		}
		if item.Cost < filters.MinCost {
			continue
		}
		if item.Waste < filters.MinWaste {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func fetchWorkloadsForProfile(ctx context.Context, rt *Runtime, data *profile.Data, token string, clusterUID string, normalizedPeriod string) ([]api.Workload, error) {
	if data.AuthMode != profile.AuthModeServiceToken {
		return nil, fmt.Errorf("profile %q uses unsupported auth mode %q; only service-token auth is supported now", data.Name, data.AuthMode)
	}
	return rt.API.ListPublicWorkloads(ctx, data.PublicAPIURL, token, clusterUID)
}

func summarizeNamespaces(cluster api.Cluster, workloads []api.Workload) []api.NamespaceSummary {
	if len(workloads) == 0 {
		return []api.NamespaceSummary{}
	}

	aggregates := make(map[string]*api.NamespaceSummary, len(workloads))
	for _, item := range workloads {
		name := item.Namespace
		summary, ok := aggregates[name]
		if !ok {
			summary = &api.NamespaceSummary{
				ClusterUID:  cluster.UID,
				ClusterName: cluster.Name,
				Name:        name,
				Period:      item.Period,
			}
			aggregates[name] = summary
		}
		summary.Workloads++
		summary.TotalCost += item.Cost
		summary.TotalWaste += item.Waste
	}

	out := make([]api.NamespaceSummary, 0, len(aggregates))
	for _, item := range aggregates {
		out = append(out, *item)
	}
	return out
}

func filterNamespaceSummaries(items []api.NamespaceSummary, filters NamespaceFilters) []api.NamespaceSummary {
	if strings.TrimSpace(filters.Namespace) == "" {
		return items
	}

	target := strings.ToLower(strings.TrimSpace(filters.Namespace))
	filtered := make([]api.NamespaceSummary, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), target) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func sortWorkloads(items []api.Workload, sortBy string, order string) {
	desc := strings.EqualFold(order, "desc")
	sort.Slice(items, func(i, j int) bool {
		var primary int
		switch sortBy {
		case "cost":
			primary = compareFloat64(items[i].Cost, items[j].Cost)
		case "waste":
			primary = compareFloat64(items[i].Waste, items[j].Waste)
		default:
			primary = strings.Compare(strings.ToLower(items[i].Name), strings.ToLower(items[j].Name))
		}
		if primary != 0 {
			if desc {
				return primary > 0
			}
			return primary < 0
		}

		tieBreak := strings.Compare(strings.ToLower(items[i].Namespace), strings.ToLower(items[j].Namespace))
		if tieBreak == 0 {
			tieBreak = strings.Compare(strings.ToLower(items[i].Type), strings.ToLower(items[j].Type))
		}
		if tieBreak == 0 {
			tieBreak = strings.Compare(items[i].ID, items[j].ID)
		}
		return tieBreak < 0
	})
}

func sortNamespaceSummaries(items []api.NamespaceSummary, sortBy string, order string) {
	desc := strings.EqualFold(order, "desc")
	sort.Slice(items, func(i, j int) bool {
		var primary int
		switch sortBy {
		case "workloads":
			primary = compareInt(items[i].Workloads, items[j].Workloads)
		case "cost":
			primary = compareFloat64(items[i].TotalCost, items[j].TotalCost)
		case "waste":
			primary = compareFloat64(items[i].TotalWaste, items[j].TotalWaste)
		default:
			primary = strings.Compare(strings.ToLower(items[i].Name), strings.ToLower(items[j].Name))
		}
		if primary != 0 {
			if desc {
				return primary > 0
			}
			return primary < 0
		}

		tieBreak := strings.Compare(strings.ToLower(items[i].Name), strings.ToLower(items[j].Name))
		if tieBreak == 0 {
			tieBreak = compareFloat64(items[i].TotalWaste, items[j].TotalWaste)
		}
		if tieBreak == 0 {
			tieBreak = compareFloat64(items[i].TotalCost, items[j].TotalCost)
		}
		return tieBreak < 0
	})
}

func limitWorkloads(items []api.Workload, top int, bottom int) ([]api.Workload, error) {
	return limitItems(items, top, bottom)
}

func limitNamespaceSummaries(items []api.NamespaceSummary, top int, bottom int) ([]api.NamespaceSummary, error) {
	return limitItems(items, top, bottom)
}

func limitItems[T any](items []T, top int, bottom int) ([]T, error) {
	if top > 0 && bottom > 0 {
		return nil, fmt.Errorf("--top and --bottom cannot be used together")
	}
	if top < 0 {
		return nil, fmt.Errorf("--top must be a positive integer")
	}
	if bottom < 0 {
		return nil, fmt.Errorf("--bottom must be a positive integer")
	}
	if top > 0 {
		if top >= len(items) {
			return items, nil
		}
		return items[:top], nil
	}
	if bottom == 0 || bottom >= len(items) {
		return items, nil
	}
	return items[len(items)-bottom:], nil
}

func formatTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func indicatorLabel(item *api.Indicator) string {
	if item == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s/%d", item.Type, item.Name, item.Severity)
}

func compareFloat64(left float64, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareInt(left int, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
