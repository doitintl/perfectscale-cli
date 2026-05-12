package cli

import (
	"fmt"
	"strings"

	"github.com/perfectscale/poc-cli/internal/api"
	ucli "github.com/urfave/cli/v2"
)

const (
	workloadViewDefault  = "default"
	workloadViewCapacity = "capacity"
	workloadViewUsage    = "usage"
	workloadViewPolicy   = "policy"
	workloadViewRisk     = "risk"
	workloadViewAll      = "all"
)

func normalizeWorkloadView(view string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(view))
	if normalized == "" {
		return workloadViewDefault, nil
	}

	switch normalized {
	case workloadViewDefault, workloadViewCapacity, workloadViewUsage, workloadViewPolicy, workloadViewRisk, workloadViewAll:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported --view %q: must be one of default, capacity, usage, policy, risk, all", view)
	}
}

func applyImplicitOutputForWorkloadView(c *ucli.Context, rt *Runtime, view string) {
	rt.Config.Output = implicitOutputForWorkloadView(view, isFlagSetInLineage(c, "output"), rt.Config.Output)
}

func implicitOutputForWorkloadView(view string, explicit bool, current string) string {
	if explicit {
		return current
	}
	if view == workloadViewAll {
		return "jsonl"
	}
	return current
}

func renderWorkloadListView(rt *Runtime, workloads []api.Workload, view string) error {
	headers, rows := workloadListRows(workloads, view)
	return rt.RenderTableOrJSON(workloads, headers, rows)
}

func workloadListRows(workloads []api.Workload, view string) ([]string, [][]string) {
	switch view {
	case workloadViewCapacity:
		headers := []string{"NAME", "NAMESPACE", "TYPE", "REPL_MAX", "REPL_AVG", "CONTAINERS", "CPU_REQ", "CPU_LIMIT", "MEM_REQ_MIB", "MEM_LIMIT_MIB", "REC_CPU_REQ", "REC_MEM_REQ_MIB"}
		rows := make([][]string, 0, len(workloads))
		for _, item := range workloads {
			rows = append(rows, []string{
				item.Name,
				item.Namespace,
				item.Type,
				fmt.Sprintf("%d", item.ReplicasCounts.MaxCount),
				fmt.Sprintf("%d", item.ReplicasCounts.AvgCount),
				fmt.Sprintf("%d", item.Derived.ContainerCount),
				formatMetric(item.Derived.CurrentCPURequestCoresTotal),
				formatMetric(item.Derived.CurrentCPULimitCoresTotal),
				formatMetric(item.Derived.CurrentMemoryRequestMiBTotal),
				formatMetric(item.Derived.CurrentMemoryLimitMiBTotal),
				formatMetric(item.Derived.RecommendedCPURequestCoresTotal),
				formatMetric(item.Derived.RecommendedMemoryRequestMiBTotal),
			})
		}
		return headers, rows
	case workloadViewUsage:
		headers := []string{"NAME", "NAMESPACE", "TYPE", "CONTAINERS", "CPU_P90_SUM", "CPU_P95_SUM", "CPU_P100_SUM", "MEM_P90_MIB_SUM", "MEM_P95_MIB_SUM", "MEM_P100_MIB_SUM", "RUNNING_MIN"}
		rows := make([][]string, 0, len(workloads))
		for _, item := range workloads {
			rows = append(rows, []string{
				item.Name,
				item.Namespace,
				item.Type,
				fmt.Sprintf("%d", item.Derived.ContainerCount),
				formatMetric(item.Derived.CPUUsageP90CoresSum),
				formatMetric(item.Derived.CPUUsageP95CoresSum),
				formatMetric(item.Derived.CPUUsageP100CoresSum),
				formatMetric(item.Derived.MemoryUsageP90MiBSum),
				formatMetric(item.Derived.MemoryUsageP95MiBSum),
				formatMetric(item.Derived.MemoryUsageP100MiBSum),
				fmt.Sprintf("%d", item.RunningMinutes),
			})
		}
		return headers, rows
	case workloadViewPolicy:
		headers := []string{"NAME", "NAMESPACE", "TYPE", "POLICY", "POLICY_WINDOW", "CPU_POLICY", "MEM_POLICY", "MEM_REQ_EQ_LIMIT", "RESILIENCE", "MUTED", "MUTE_EXPIRES"}
		rows := make([][]string, 0, len(workloads))
		for _, item := range workloads {
			rows = append(rows, []string{
				item.Name,
				item.Namespace,
				item.Type,
				item.OptimizationPolicy,
				item.OptimizationPolicyTimeWindow,
				item.CPUOptimizationPolicy,
				item.MemoryOptimizationPolicy,
				fmt.Sprintf("%t", item.MemoryRequestEqualsLimit),
				item.ResilienceLevel,
				fmt.Sprintf("%t", item.MuteStatus.IsMuted),
				formatTime(item.MuteStatus.Expires),
			})
		}
		return headers, rows
	case workloadViewRisk:
		headers := []string{"NAME", "NAMESPACE", "TYPE", "RISK_SEVERITY", "RISK_COUNT", "WASTE_COUNT", "RISK_INDICATOR", "MAX_INDICATOR", "COST", "WASTE"}
		rows := make([][]string, 0, len(workloads))
		for _, item := range workloads {
			rows = append(rows, []string{
				item.Name,
				item.Namespace,
				item.Type,
				fmt.Sprintf("%d", workloadRiskSeverity(item)),
				fmt.Sprintf("%d", item.Derived.RiskIndicatorsCount),
				fmt.Sprintf("%d", item.Derived.WasteIndicatorsCount),
				indicatorLabel(workloadRiskIndicator(item)),
				indicatorLabel(item.MaxIndicator),
				fmt.Sprintf("%.2f", item.Cost),
				fmt.Sprintf("%.2f", item.Waste),
			})
		}
		return headers, rows
	case workloadViewAll:
		headers := []string{"ID", "NAME", "NAMESPACE", "TYPE", "PERIOD", "REPL_MAX", "REPL_AVG", "POLICY", "CPU_POLICY", "MEM_POLICY", "MUTED", "CONTAINERS", "CPU_REQ", "MEM_REQ_MIB", "CPU_P95_SUM", "MEM_P95_MIB_SUM", "COST", "WASTE", "POTENTIAL_SAVINGS", "LAST_SEEN", "MAX_INDICATOR", "LABELS"}
		rows := make([][]string, 0, len(workloads))
		for _, item := range workloads {
			rows = append(rows, []string{
				item.ID,
				item.Name,
				item.Namespace,
				item.Type,
				item.Period,
				fmt.Sprintf("%d", item.ReplicasCounts.MaxCount),
				fmt.Sprintf("%d", item.ReplicasCounts.AvgCount),
				item.OptimizationPolicy,
				item.CPUOptimizationPolicy,
				item.MemoryOptimizationPolicy,
				fmt.Sprintf("%t", item.MuteStatus.IsMuted),
				fmt.Sprintf("%d", item.Derived.ContainerCount),
				formatMetric(item.Derived.CurrentCPURequestCoresTotal),
				formatMetric(item.Derived.CurrentMemoryRequestMiBTotal),
				formatMetric(item.Derived.CPUUsageP95CoresSum),
				formatMetric(item.Derived.MemoryUsageP95MiBSum),
				fmt.Sprintf("%.2f", item.Cost),
				fmt.Sprintf("%.2f", item.Waste),
				fmt.Sprintf("%.2f", item.PotentialSavings),
				formatTime(item.LastSeen),
				indicatorLabel(item.MaxIndicator),
				formatLabelMap(item.WorkloadLabels),
			})
		}
		return headers, rows
	default:
		headers := []string{"NAME", "NAMESPACE", "TYPE", "COST", "WASTE", "PERIOD", "LAST_SEEN", "MAX_INDICATOR"}
		rows := make([][]string, 0, len(workloads))
		for _, item := range workloads {
			rows = append(rows, []string{
				item.Name,
				item.Namespace,
				item.Type,
				fmt.Sprintf("%.2f", item.Cost),
				fmt.Sprintf("%.2f", item.Waste),
				item.Period,
				formatTime(item.LastSeen),
				indicatorLabel(item.MaxIndicator),
			})
		}
		return headers, rows
	}
}

func formatMetric(value float64) string {
	return fmt.Sprintf("%.2f", value)
}
