package cli

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/perfectscale/poc-cli/internal/api"
	"github.com/perfectscale/poc-cli/internal/profile"
	ucli "github.com/urfave/cli/v2"
)

type commandResources struct {
	Runtime *Runtime
	Profile *profile.Data
	Token   string
}

func loadCommandResources(c *ucli.Context) (*commandResources, error) {
	rt, err := NewRuntime(c)
	if err != nil {
		return nil, err
	}

	data, err := rt.LoadProfile()
	if err != nil {
		return nil, err
	}

	token, err := rt.ResolveToken(c.Context, data)
	if err != nil {
		return nil, err
	}

	return &commandResources{
		Runtime: rt,
		Profile: data,
		Token:   token,
	}, nil
}

func (r *commandResources) resolveCluster(ctx context.Context, target string) (api.Cluster, error) {
	clusters, err := listClustersForProfile(ctx, r.Runtime, r.Profile, r.Token)
	if err != nil {
		return api.Cluster{}, err
	}
	return resolveClusterByNameOrUID(clusters, target)
}

func (r *commandResources) loadClusterDetail(ctx context.Context, target string, period string) (api.Cluster, *api.ClusterDetail, error) {
	cluster, err := r.resolveCluster(ctx, target)
	if err != nil {
		return api.Cluster{}, nil, err
	}

	detail, err := r.Runtime.API.GetPublicCluster(ctx, r.Profile.PublicAPIURL, r.Token, cluster.UID, period)
	if err != nil {
		return api.Cluster{}, nil, err
	}

	return cluster, detail, nil
}

func (r *commandResources) loadWorkloads(ctx context.Context, target string, period string) (api.Cluster, []api.Workload, error) {
	cluster, err := r.resolveCluster(ctx, target)
	if err != nil {
		return api.Cluster{}, nil, err
	}

	workloads, err := fetchWorkloadsForProfile(ctx, r.Runtime, r.Profile, r.Token, cluster.UID, period)
	if err != nil {
		return api.Cluster{}, nil, err
	}

	return cluster, workloads, nil
}

func clusterEmissionEntries(cluster api.Cluster, detail *api.ClusterDetail) []api.ClusterEmissionEntry {
	if detail == nil || len(detail.Emission) == 0 {
		return []api.ClusterEmissionEntry{}
	}

	items := make([]api.ClusterEmissionEntry, 0, len(detail.Emission))
	for metric, value := range detail.Emission {
		items = append(items, api.ClusterEmissionEntry{
			ClusterUID:  cluster.UID,
			ClusterName: cluster.Name,
			Metric:      metric,
			Value:       value,
		})
	}
	return items
}

func sortEmissionEntries(items []api.ClusterEmissionEntry, sortBy string, order string) {
	desc := strings.EqualFold(order, "desc")
	sort.Slice(items, func(i, j int) bool {
		var primary int
		switch sortBy {
		case "value":
			primary = compareFloat64(items[i].Value, items[j].Value)
		default:
			primary = strings.Compare(strings.ToLower(items[i].Metric), strings.ToLower(items[j].Metric))
		}
		if primary != 0 {
			if desc {
				return primary > 0
			}
			return primary < 0
		}
		return strings.Compare(strings.ToLower(items[i].Metric), strings.ToLower(items[j].Metric)) < 0
	})
}

func summarizeWorkloads(cluster api.Cluster, workloads []api.Workload) api.WorkloadSummary {
	summary := api.WorkloadSummary{
		ClusterUID:  cluster.UID,
		ClusterName: cluster.Name,
		Period:      "30d",
	}
	if len(workloads) == 0 {
		return summary
	}

	namespaces := make(map[string]struct{}, len(workloads))
	types := make(map[string]struct{}, len(workloads))
	namespaceWaste := make(map[string]float64)
	typeWaste := make(map[string]float64)

	summary.Period = workloads[0].Period
	for _, item := range workloads {
		summary.Workloads++
		summary.TotalCost += item.Cost
		summary.TotalWaste += item.Waste
		summary.TotalPotentialSaving += item.PotentialSavings
		if item.Namespace != "" {
			namespaces[item.Namespace] = struct{}{}
			namespaceWaste[item.Namespace] += item.Waste
		}
		if item.Type != "" {
			types[item.Type] = struct{}{}
			typeWaste[item.Type] += item.Waste
		}
		if item.MuteStatus.IsMuted {
			summary.MutedWorkloads++
		}
		if workloadRiskSeverity(item) > 0 {
			summary.RiskyWorkloads++
		}
		if item.Waste > 0 {
			summary.WasteWorkloads++
		}
	}

	summary.Namespaces = len(namespaces)
	summary.Types = len(types)
	summary.TopNamespace, summary.TopNamespaceWaste = highestWasteKey(namespaceWaste)
	summary.TopType, summary.TopTypeWaste = highestWasteKey(typeWaste)

	return summary
}

func highestWasteKey(values map[string]float64) (string, float64) {
	if len(values) == 0 {
		return "", 0
	}

	bestKey := ""
	bestValue := 0.0
	first := true
	for key, value := range values {
		if first || value > bestValue || (value == bestValue && strings.ToLower(key) < strings.ToLower(bestKey)) {
			bestKey = key
			bestValue = value
			first = false
		}
	}
	return bestKey, bestValue
}

func groupWorkloads(cluster api.Cluster, workloads []api.Workload, field string) []api.WorkloadGroupSummary {
	if len(workloads) == 0 {
		return []api.WorkloadGroupSummary{}
	}

	aggregates := make(map[string]*api.WorkloadGroupSummary, len(workloads))
	for _, item := range workloads {
		key := workloadGroupKey(item, field)
		group, ok := aggregates[key]
		if !ok {
			group = &api.WorkloadGroupSummary{
				ClusterUID:  cluster.UID,
				ClusterName: cluster.Name,
				Field:       field,
				Key:         key,
				Period:      item.Period,
			}
			aggregates[key] = group
		}
		group.Workloads++
		group.TotalCost += item.Cost
		group.TotalWaste += item.Waste
		if item.MuteStatus.IsMuted {
			group.MutedWorkloads++
		}
		if workloadRiskSeverity(item) > 0 {
			group.RiskyWorkloads++
		}
		if item.Waste > 0 {
			group.WasteWorkloads++
		}
	}

	out := make([]api.WorkloadGroupSummary, 0, len(aggregates))
	for _, item := range aggregates {
		out = append(out, *item)
	}
	return out
}

func workloadGroupKey(item api.Workload, field string) string {
	switch field {
	case "type":
		if strings.TrimSpace(item.Type) == "" {
			return "<unknown>"
		}
		return item.Type
	case "optimization-policy":
		if strings.TrimSpace(item.OptimizationPolicy) == "" {
			return "<unknown>"
		}
		return item.OptimizationPolicy
	case "risk-severity":
		return fmt.Sprintf("%d", workloadRiskSeverity(item))
	default:
		if strings.TrimSpace(item.Namespace) == "" {
			return "<unknown>"
		}
		return item.Namespace
	}
}

func groupWorkloadsByLabel(cluster api.Cluster, workloads []api.Workload, labelKey string) []api.WorkloadGroupSummary {
	labelKey = strings.TrimSpace(labelKey)
	if labelKey == "" {
		return []api.WorkloadGroupSummary{}
	}

	aggregates := make(map[string]*api.WorkloadGroupSummary, len(workloads))
	for _, item := range workloads {
		key := "<missing>"
		for candidateKey, candidateValue := range item.WorkloadLabels {
			if !strings.EqualFold(candidateKey, labelKey) {
				continue
			}
			if strings.TrimSpace(candidateValue) != "" {
				key = candidateValue
			}
			break
		}

		group, ok := aggregates[key]
		if !ok {
			group = &api.WorkloadGroupSummary{
				ClusterUID:  cluster.UID,
				ClusterName: cluster.Name,
				Field:       "label:" + labelKey,
				Key:         key,
				Period:      item.Period,
			}
			aggregates[key] = group
		}
		group.Workloads++
		group.TotalCost += item.Cost
		group.TotalWaste += item.Waste
		if item.MuteStatus.IsMuted {
			group.MutedWorkloads++
		}
		if workloadRiskSeverity(item) > 0 {
			group.RiskyWorkloads++
		}
		if item.Waste > 0 {
			group.WasteWorkloads++
		}
	}

	out := make([]api.WorkloadGroupSummary, 0, len(aggregates))
	for _, item := range aggregates {
		out = append(out, *item)
	}
	return out
}

func sortWorkloadGroups(items []api.WorkloadGroupSummary, sortBy string, order string) {
	desc := strings.EqualFold(order, "desc")
	sort.Slice(items, func(i, j int) bool {
		var primary int
		switch sortBy {
		case "workloads":
			primary = compareInt(items[i].Workloads, items[j].Workloads)
		case "muted":
			primary = compareInt(items[i].MutedWorkloads, items[j].MutedWorkloads)
		case "risky":
			primary = compareInt(items[i].RiskyWorkloads, items[j].RiskyWorkloads)
		case "cost":
			primary = compareFloat64(items[i].TotalCost, items[j].TotalCost)
		case "waste":
			primary = compareFloat64(items[i].TotalWaste, items[j].TotalWaste)
		default:
			primary = strings.Compare(strings.ToLower(items[i].Key), strings.ToLower(items[j].Key))
		}
		if primary != 0 {
			if desc {
				return primary > 0
			}
			return primary < 0
		}
		return strings.Compare(strings.ToLower(items[i].Key), strings.ToLower(items[j].Key)) < 0
	})
}

func limitWorkloadGroups(items []api.WorkloadGroupSummary, top int, bottom int) ([]api.WorkloadGroupSummary, error) {
	return limitItems(items, top, bottom)
}

func resolveWorkload(workloads []api.Workload, id string, name string, namespace string) (api.Workload, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	namespace = strings.TrimSpace(namespace)

	if id != "" && name != "" {
		return api.Workload{}, fmt.Errorf("--id and --name cannot be used together")
	}
	if id == "" && name == "" {
		return api.Workload{}, fmt.Errorf("either --id or --name is required")
	}

	matches := make([]api.Workload, 0, 4)
	for _, item := range workloads {
		if namespace != "" && !strings.EqualFold(item.Namespace, namespace) {
			continue
		}
		if id != "" && strings.EqualFold(item.ID, id) {
			matches = append(matches, item)
			continue
		}
		if name != "" && strings.EqualFold(item.Name, name) {
			matches = append(matches, item)
		}
	}

	if len(matches) == 0 {
		if id != "" {
			return api.Workload{}, fmt.Errorf("workload id %q not found", id)
		}
		return api.Workload{}, fmt.Errorf("workload name %q not found", name)
	}
	if len(matches) > 1 {
		options := make([]string, 0, len(matches))
		for _, item := range matches {
			options = append(options, fmt.Sprintf("%s/%s (%s)", item.Namespace, item.Name, item.ID))
		}
		sort.Strings(options)
		return api.Workload{}, fmt.Errorf("workload match is ambiguous: %s", strings.Join(options, ", "))
	}

	return matches[0], nil
}

func filterRiskyWorkloads(items []api.Workload, minSeverity int) []api.Workload {
	out := make([]api.Workload, 0, len(items))
	for _, item := range items {
		if workloadRiskSeverity(item) < minSeverity {
			continue
		}
		out = append(out, item)
	}
	return out
}

func sortRiskyWorkloads(items []api.Workload, sortBy string, order string) {
	desc := strings.EqualFold(order, "desc")
	sort.Slice(items, func(i, j int) bool {
		var primary int
		switch sortBy {
		case "cost":
			primary = compareFloat64(items[i].Cost, items[j].Cost)
		case "waste":
			primary = compareFloat64(items[i].Waste, items[j].Waste)
		case "name":
			primary = strings.Compare(strings.ToLower(items[i].Name), strings.ToLower(items[j].Name))
		default:
			primary = compareInt(workloadRiskSeverity(items[i]), workloadRiskSeverity(items[j]))
		}
		if primary != 0 {
			if desc {
				return primary > 0
			}
			return primary < 0
		}

		if tieBreak := compareFloat64(items[i].Waste, items[j].Waste); tieBreak != 0 {
			return tieBreak > 0
		}
		if tieBreak := compareFloat64(items[i].Cost, items[j].Cost); tieBreak != 0 {
			return tieBreak > 0
		}
		return strings.Compare(strings.ToLower(items[i].Name), strings.ToLower(items[j].Name)) < 0
	})
}

func filterMutedWorkloads(items []api.Workload) []api.Workload {
	out := make([]api.Workload, 0, len(items))
	for _, item := range items {
		if item.MuteStatus.IsMuted {
			out = append(out, item)
		}
	}
	return out
}

func sortMutedWorkloads(items []api.Workload, sortBy string, order string) {
	desc := strings.EqualFold(order, "desc")
	sort.Slice(items, func(i, j int) bool {
		var primary int
		switch sortBy {
		case "cost":
			primary = compareFloat64(items[i].Cost, items[j].Cost)
		case "waste":
			primary = compareFloat64(items[i].Waste, items[j].Waste)
		case "name":
			primary = strings.Compare(strings.ToLower(items[i].Name), strings.ToLower(items[j].Name))
		default:
			primary = compareTimePtr(items[i].MuteStatus.Expires, items[j].MuteStatus.Expires)
		}
		if primary != 0 {
			if desc {
				return primary > 0
			}
			return primary < 0
		}
		return strings.Compare(strings.ToLower(items[i].Name), strings.ToLower(items[j].Name)) < 0
	})
}

func summarizeWorkloadLabels(cluster api.Cluster, workloads []api.Workload, keyFilter string, valueFilter string) []api.WorkloadLabelSummary {
	normalizedKeyFilter := strings.ToLower(strings.TrimSpace(keyFilter))
	normalizedValueFilter := strings.ToLower(strings.TrimSpace(valueFilter))

	aggregates := map[string]*api.WorkloadLabelSummary{}
	for _, item := range workloads {
		for key, value := range item.WorkloadLabels {
			if normalizedKeyFilter != "" && !strings.Contains(strings.ToLower(key), normalizedKeyFilter) {
				continue
			}
			if normalizedValueFilter != "" && !strings.Contains(strings.ToLower(value), normalizedValueFilter) {
				continue
			}

			compositeKey := key + "\x00" + value
			summary, ok := aggregates[compositeKey]
			if !ok {
				summary = &api.WorkloadLabelSummary{
					ClusterUID:  cluster.UID,
					ClusterName: cluster.Name,
					Period:      item.Period,
					Key:         key,
					Value:       value,
				}
				aggregates[compositeKey] = summary
			}
			summary.Workloads++
			summary.TotalCost += item.Cost
			summary.TotalWaste += item.Waste
		}
	}

	out := make([]api.WorkloadLabelSummary, 0, len(aggregates))
	for _, item := range aggregates {
		out = append(out, *item)
	}
	return out
}

func sortWorkloadLabels(items []api.WorkloadLabelSummary, sortBy string, order string) {
	desc := strings.EqualFold(order, "desc")
	sort.Slice(items, func(i, j int) bool {
		var primary int
		switch sortBy {
		case "value":
			primary = strings.Compare(strings.ToLower(items[i].Value), strings.ToLower(items[j].Value))
		case "workloads":
			primary = compareInt(items[i].Workloads, items[j].Workloads)
		case "cost":
			primary = compareFloat64(items[i].TotalCost, items[j].TotalCost)
		case "waste":
			primary = compareFloat64(items[i].TotalWaste, items[j].TotalWaste)
		default:
			primary = strings.Compare(strings.ToLower(items[i].Key), strings.ToLower(items[j].Key))
		}
		if primary != 0 {
			if desc {
				return primary > 0
			}
			return primary < 0
		}
		if tieBreak := strings.Compare(strings.ToLower(items[i].Key), strings.ToLower(items[j].Key)); tieBreak != 0 {
			return tieBreak < 0
		}
		return strings.Compare(strings.ToLower(items[i].Value), strings.ToLower(items[j].Value)) < 0
	})
}

func limitWorkloadLabels(items []api.WorkloadLabelSummary, top int, bottom int) ([]api.WorkloadLabelSummary, error) {
	return limitItems(items, top, bottom)
}

func workloadRiskSeverity(item api.Workload) int {
	best := 0
	for _, indicator := range allWorkloadIndicators(item) {
		if !strings.EqualFold(indicator.Type, "risk") {
			continue
		}
		if indicator.Severity > best {
			best = indicator.Severity
		}
	}
	return best
}

func workloadRiskIndicator(item api.Workload) *api.Indicator {
	return maxIndicatorByType(item, "risk")
}

func allWorkloadIndicators(item api.Workload) []api.Indicator {
	total := len(item.Indicators)
	for _, container := range item.Containers {
		total += len(container.Indicators)
	}
	if total == 0 {
		return nil
	}

	out := make([]api.Indicator, 0, total)
	out = append(out, item.Indicators...)
	for _, container := range item.Containers {
		out = append(out, container.Indicators...)
	}
	return out
}

func maxIndicatorByType(item api.Workload, indicatorType string) *api.Indicator {
	var best *api.Indicator
	for _, indicator := range allWorkloadIndicators(item) {
		if !strings.EqualFold(indicator.Type, indicatorType) {
			continue
		}
		if best == nil || indicator.Severity > best.Severity || (indicator.Severity == best.Severity && strings.ToLower(indicator.Name) < strings.ToLower(best.Name)) {
			candidate := indicator
			best = &candidate
		}
	}
	return best
}

func compareTimePtr(left *time.Time, right *time.Time) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return 1
	case right == nil:
		return -1
	default:
		return compareTime(*left, *right)
	}
}

func compareTime(left time.Time, right time.Time) int {
	switch {
	case left.Before(right):
		return -1
	case left.After(right):
		return 1
	default:
		return 0
	}
}

func csvWriterForPath(path string, defaultWriter io.Writer) (io.Writer, func() error, error) {
	if strings.TrimSpace(path) == "" {
		if defaultWriter == nil {
			defaultWriter = os.Stdout
		}
		return defaultWriter, func() error { return nil }, nil
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create export file: %w", err)
	}
	return file, file.Close, nil
}

func writeWorkloadsCSV(writer io.Writer, workloads []api.Workload) error {
	csvWriter := csv.NewWriter(writer)
	headers := []string{
		"id",
		"name",
		"namespace",
		"type",
		"period",
		"cost",
		"waste",
		"potential_savings",
		"cost_per_hour",
		"running_minutes",
		"is_muted",
		"mute_expires_at",
		"max_indicator",
	}
	if err := csvWriter.Write(headers); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, item := range workloads {
		record := []string{
			item.ID,
			item.Name,
			item.Namespace,
			item.Type,
			item.Period,
			fmt.Sprintf("%.2f", item.Cost),
			fmt.Sprintf("%.2f", item.Waste),
			fmt.Sprintf("%.2f", item.PotentialSavings),
			fmt.Sprintf("%.4f", item.CostPerHour),
			fmt.Sprintf("%d", item.RunningMinutes),
			fmt.Sprintf("%t", item.MuteStatus.IsMuted),
			formatTime(item.MuteStatus.Expires),
			indicatorLabel(item.MaxIndicator),
		}
		if err := csvWriter.Write(record); err != nil {
			return fmt.Errorf("write csv record: %w", err)
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}
