package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/perfectscale/poc-cli/internal/api"
)

func TestNormalizePeriod(t *testing.T) {
	tests := []struct {
		name    string
		period  string
		want    string
		wantErr string
	}{
		{name: "default", period: "", want: "30d"},
		{name: "explicit 30d", period: "30d", want: "30d"},
		{name: "reject 7d", period: "7d", wantErr: "only --period 30d is supported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePeriod(tt.period)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("normalizePeriod() error = nil, want non-nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizePeriod() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizePeriod() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterAndSortWorkloads(t *testing.T) {
	items := []api.Workload{
		{ID: "b", Name: "api", Namespace: "prod", Type: "deployment", Cost: 10, Waste: 7},
		{ID: "a", Name: "api", Namespace: "kube-system", Type: "deployment", Cost: 10, Waste: 7},
		{ID: "c", Name: "worker", Namespace: "prod", Type: "daemonset", Cost: 5, Waste: 3},
	}

	filtered := filterWorkloads(items, WorkloadFilters{
		Name:     "api",
		MinCost:  9,
		MinWaste: 7,
	})
	if len(filtered) != 2 {
		t.Fatalf("len(filtered) = %d, want 2", len(filtered))
	}

	sortWorkloads(filtered, "cost", "desc")
	if filtered[0].ID != "a" || filtered[1].ID != "b" {
		t.Fatalf("sorted IDs = [%s %s], want [a b]", filtered[0].ID, filtered[1].ID)
	}

	sortWorkloads(filtered, "name", "asc")
	if filtered[0].Namespace != "kube-system" {
		t.Fatalf("ascending tie-break order mismatch: got namespace %q first", filtered[0].Namespace)
	}
}

func TestLimitWorkloads(t *testing.T) {
	items := []api.Workload{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
	}

	topTwo, err := limitWorkloads(items, 2, 0)
	if err != nil {
		t.Fatalf("limitWorkloads(top) error = %v", err)
	}
	if len(topTwo) != 2 {
		t.Fatalf("len(topTwo) = %d, want 2", len(topTwo))
	}
	if topTwo[0].ID != "a" || topTwo[1].ID != "b" {
		t.Fatalf("topTwo IDs = [%s %s], want [a b]", topTwo[0].ID, topTwo[1].ID)
	}

	bottomTwo, err := limitWorkloads(items, 0, 2)
	if err != nil {
		t.Fatalf("limitWorkloads(bottom) error = %v", err)
	}
	if len(bottomTwo) != 2 {
		t.Fatalf("len(bottomTwo) = %d, want 2", len(bottomTwo))
	}
	if bottomTwo[0].ID != "b" || bottomTwo[1].ID != "c" {
		t.Fatalf("bottomTwo IDs = [%s %s], want [b c]", bottomTwo[0].ID, bottomTwo[1].ID)
	}

	allItems, err := limitWorkloads(items, 0, 0)
	if err != nil {
		t.Fatalf("limitWorkloads(0,0) error = %v", err)
	}
	if len(allItems) != 3 {
		t.Fatalf("len(allItems) = %d, want 3", len(allItems))
	}

	if _, err := limitWorkloads(items, -1, 0); err == nil {
		t.Fatal("limitWorkloads(-1,0) error = nil, want non-nil")
	}
	if _, err := limitWorkloads(items, 0, -1); err == nil {
		t.Fatal("limitWorkloads(0,-1) error = nil, want non-nil")
	}
	if _, err := limitWorkloads(items, 1, 1); err == nil {
		t.Fatal("limitWorkloads(1,1) error = nil, want non-nil")
	}
}

func TestSummarizeFilterSortAndLimitNamespaces(t *testing.T) {
	cluster := api.Cluster{UID: "cluster-1", Name: "prod-a"}
	workloads := []api.Workload{
		{ID: "a", Namespace: "prod", Cost: 10, Waste: 3, Period: "30d"},
		{ID: "b", Namespace: "prod", Cost: 20, Waste: 5, Period: "30d"},
		{ID: "c", Namespace: "kube-system", Cost: 5, Waste: 8, Period: "30d"},
	}

	summaries := summarizeNamespaces(cluster, workloads)
	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}

	filtered := filterNamespaceSummaries(summaries, NamespaceFilters{Namespace: "prod"})
	if len(filtered) != 1 {
		t.Fatalf("len(filtered) = %d, want 1", len(filtered))
	}
	if filtered[0].Name != "prod" {
		t.Fatalf("filtered[0].Name = %q, want %q", filtered[0].Name, "prod")
	}
	if filtered[0].Workloads != 2 {
		t.Fatalf("filtered[0].Workloads = %d, want 2", filtered[0].Workloads)
	}
	if filtered[0].TotalCost != 30 {
		t.Fatalf("filtered[0].TotalCost = %v, want 30", filtered[0].TotalCost)
	}
	if filtered[0].TotalWaste != 8 {
		t.Fatalf("filtered[0].TotalWaste = %v, want 8", filtered[0].TotalWaste)
	}

	sortNamespaceSummaries(summaries, "workloads", "desc")
	if summaries[0].Name != "prod" {
		t.Fatalf("summaries[0].Name = %q, want %q", summaries[0].Name, "prod")
	}

	limited, err := limitNamespaceSummaries(summaries, 1, 0)
	if err != nil {
		t.Fatalf("limitNamespaceSummaries() error = %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("len(limited) = %d, want 1", len(limited))
	}
}

func TestAsSlice(t *testing.T) {
	values, err := asSlice([]api.Cluster{{UID: "cluster-1", Name: "prod-a"}})
	if err != nil {
		t.Fatalf("asSlice() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("len(values) = %d, want 1", len(values))
	}

	values, err = asSlice([]api.NamespaceSummary{{ClusterUID: "cluster-1", Name: "prod", Workloads: 2}})
	if err != nil {
		t.Fatalf("asSlice(namespace summaries) error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("len(namespace summary values) = %d, want 1", len(values))
	}

	if _, err := asSlice(struct{}{}); err == nil {
		t.Fatal("asSlice() error = nil for scalar value, want non-nil")
	}
}

func TestSummarizeWorkloadsAndGroups(t *testing.T) {
	expires := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	cluster := api.Cluster{UID: "cluster-1", Name: "prod-a"}
	workloads := []api.Workload{
		{
			ID:               "a",
			Name:             "api",
			Namespace:        "backend",
			Type:             "Deployment",
			Period:           "30d",
			Cost:             50,
			Waste:            20,
			PotentialSavings: 10,
			MuteStatus:       api.MuteStatus{IsMuted: true, Expires: &expires},
			Indicators:       []api.Indicator{{Name: "risk-1", Type: "risk", Severity: 2}},
		},
		{
			ID:        "b",
			Name:      "worker",
			Namespace: "backend",
			Type:      "Deployment",
			Period:    "30d",
			Cost:      10,
			Waste:     0,
		},
		{
			ID:        "c",
			Name:      "db",
			Namespace: "data",
			Type:      "StatefulSet",
			Period:    "30d",
			Cost:      30,
			Waste:     15,
			Containers: []api.WorkloadContainer{
				{Indicators: []api.Indicator{{Name: "risk-2", Type: "risk", Severity: 3}}},
			},
		},
	}

	summary := summarizeWorkloads(cluster, workloads)
	if summary.Workloads != 3 {
		t.Fatalf("Workloads = %d, want 3", summary.Workloads)
	}
	if summary.Namespaces != 2 {
		t.Fatalf("Namespaces = %d, want 2", summary.Namespaces)
	}
	if summary.Types != 2 {
		t.Fatalf("Types = %d, want 2", summary.Types)
	}
	if summary.MutedWorkloads != 1 {
		t.Fatalf("MutedWorkloads = %d, want 1", summary.MutedWorkloads)
	}
	if summary.RiskyWorkloads != 2 {
		t.Fatalf("RiskyWorkloads = %d, want 2", summary.RiskyWorkloads)
	}
	if summary.WasteWorkloads != 2 {
		t.Fatalf("WasteWorkloads = %d, want 2", summary.WasteWorkloads)
	}
	if summary.TotalCost != 90 {
		t.Fatalf("TotalCost = %v, want 90", summary.TotalCost)
	}
	if summary.TotalWaste != 35 {
		t.Fatalf("TotalWaste = %v, want 35", summary.TotalWaste)
	}
	if summary.TopNamespace != "backend" || summary.TopNamespaceWaste != 20 {
		t.Fatalf("TopNamespace = %q/%v, want backend/20", summary.TopNamespace, summary.TopNamespaceWaste)
	}
	if summary.TopType != "Deployment" || summary.TopTypeWaste != 20 {
		t.Fatalf("TopType = %q/%v, want Deployment/20", summary.TopType, summary.TopTypeWaste)
	}

	groups := groupWorkloads(cluster, workloads, "namespace")
	sortWorkloadGroups(groups, "waste", "desc")
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].Key != "backend" {
		t.Fatalf("groups[0].Key = %q, want backend", groups[0].Key)
	}
	if groups[0].MutedWorkloads != 1 || groups[0].RiskyWorkloads != 1 {
		t.Fatalf("groups[0] muted/risky = %d/%d, want 1/1", groups[0].MutedWorkloads, groups[0].RiskyWorkloads)
	}

	for i := range workloads {
		switch workloads[i].ID {
		case "a":
			workloads[i].OptimizationPolicy = "balanced"
		case "b":
			workloads[i].OptimizationPolicy = "balanced"
		case "c":
			workloads[i].OptimizationPolicy = "aggressive"
		}
	}

	policyGroups := groupWorkloads(cluster, workloads, "optimization-policy")
	sortWorkloadGroups(policyGroups, "workloads", "desc")
	if len(policyGroups) != 2 {
		t.Fatalf("len(policyGroups) = %d, want 2", len(policyGroups))
	}
	if policyGroups[0].Key != "balanced" || policyGroups[0].Workloads != 2 {
		t.Fatalf("policyGroups[0] = %#v, want balanced with 2 workloads", policyGroups[0])
	}

	riskSeverityGroups := groupWorkloads(cluster, workloads, "risk-severity")
	sortWorkloadGroups(riskSeverityGroups, "key", "asc")
	if len(riskSeverityGroups) != 3 {
		t.Fatalf("len(riskSeverityGroups) = %d, want 3", len(riskSeverityGroups))
	}
	if riskSeverityGroups[0].Key != "0" || riskSeverityGroups[1].Key != "2" || riskSeverityGroups[2].Key != "3" {
		t.Fatalf("riskSeverityGroups keys = [%s %s %s], want [0 2 3]", riskSeverityGroups[0].Key, riskSeverityGroups[1].Key, riskSeverityGroups[2].Key)
	}
}

func TestResolveWorkloadRiskLabelsAndMuted(t *testing.T) {
	expires := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	workloads := []api.Workload{
		{
			ID:        "alpha-1",
			Name:      "api",
			Namespace: "backend",
			Type:      "Deployment",
			Period:    "30d",
			Cost:      42,
			Waste:     18,
			MuteStatus: api.MuteStatus{
				IsMuted: true,
				Expires: &expires,
			},
			WorkloadLabels: map[string]string{"app": "api", "team": "platform"},
			Indicators:     []api.Indicator{{Name: "risk-1", Type: "risk", Severity: 2}},
		},
		{
			ID:             "alpha-2",
			Name:           "api",
			Namespace:      "other",
			Type:           "Deployment",
			Period:         "30d",
			Cost:           10,
			Waste:          1,
			WorkloadLabels: map[string]string{"app": "api"},
		},
		{
			ID:             "beta-1",
			Name:           "worker",
			Namespace:      "backend",
			Type:           "DaemonSet",
			Period:         "30d",
			Cost:           22,
			Waste:          5,
			WorkloadLabels: map[string]string{"team": "platform"},
			Containers: []api.WorkloadContainer{
				{Indicators: []api.Indicator{{Name: "risk-2", Type: "risk", Severity: 3}}},
			},
		},
	}

	if _, err := resolveWorkload(workloads, "", "api", ""); err == nil {
		t.Fatal("resolveWorkload() error = nil for ambiguous name, want non-nil")
	}

	item, err := resolveWorkload(workloads, "", "api", "backend")
	if err != nil {
		t.Fatalf("resolveWorkload() error = %v", err)
	}
	if item.ID != "alpha-1" {
		t.Fatalf("resolved ID = %q, want alpha-1", item.ID)
	}

	risky := filterRiskyWorkloads(workloads, 2)
	sortRiskyWorkloads(risky, "severity", "desc")
	if len(risky) != 2 {
		t.Fatalf("len(risky) = %d, want 2", len(risky))
	}
	if risky[0].ID != "beta-1" {
		t.Fatalf("risky[0].ID = %q, want beta-1", risky[0].ID)
	}

	muted := filterMutedWorkloads(workloads)
	sortMutedWorkloads(muted, "expires", "asc")
	if len(muted) != 1 || muted[0].ID != "alpha-1" {
		t.Fatalf("muted = %#v, want only alpha-1", muted)
	}

	labels := summarizeWorkloadLabels(api.Cluster{UID: "cluster-1", Name: "prod-a"}, workloads, "team", "platform")
	sortWorkloadLabels(labels, "workloads", "desc")
	if len(labels) != 1 {
		t.Fatalf("len(labels) = %d, want 1", len(labels))
	}
	if labels[0].Key != "team" || labels[0].Value != "platform" {
		t.Fatalf("label = %q=%q, want team=platform", labels[0].Key, labels[0].Value)
	}
	if labels[0].Workloads != 2 {
		t.Fatalf("labels[0].Workloads = %d, want 2", labels[0].Workloads)
	}

	labelGroups := groupWorkloadsByLabel(api.Cluster{UID: "cluster-1", Name: "prod-a"}, workloads, "team")
	sortWorkloadGroups(labelGroups, "workloads", "desc")
	if len(labelGroups) != 2 {
		t.Fatalf("len(labelGroups) = %d, want 2", len(labelGroups))
	}
	if labelGroups[0].Key != "platform" || labelGroups[0].Workloads != 2 {
		t.Fatalf("labelGroups[0] = %#v, want platform with 2 workloads", labelGroups[0])
	}
	if labelGroups[1].Key != "<missing>" || labelGroups[1].Workloads != 1 {
		t.Fatalf("labelGroups[1] = %#v, want <missing> with 1 workload", labelGroups[1])
	}
}
