package cli

import (
	"testing"
	"time"

	"github.com/perfectscale/poc-cli/internal/api"
)

func TestNormalizeWorkloadView(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "default empty", input: "", want: workloadViewDefault},
		{name: "capacity", input: "capacity", want: workloadViewCapacity},
		{name: "usage uppercase", input: "USAGE", want: workloadViewUsage},
		{name: "policy", input: "policy", want: workloadViewPolicy},
		{name: "risk", input: "risk", want: workloadViewRisk},
		{name: "all", input: "all", want: workloadViewAll},
		{name: "invalid", input: "unknown", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeWorkloadView(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("normalizeWorkloadView() error = nil, want non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeWorkloadView() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeWorkloadView() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImplicitOutputForWorkloadView(t *testing.T) {
	if got := implicitOutputForWorkloadView(workloadViewAll, false, "table"); got != "jsonl" {
		t.Fatalf("implicitOutputForWorkloadView(all,false,table) = %q, want jsonl", got)
	}
	if got := implicitOutputForWorkloadView(workloadViewAll, true, "table"); got != "table" {
		t.Fatalf("implicitOutputForWorkloadView(all,true,table) = %q, want table", got)
	}
	if got := implicitOutputForWorkloadView(workloadViewUsage, false, "table"); got != "table" {
		t.Fatalf("implicitOutputForWorkloadView(usage,false,table) = %q, want table", got)
	}
}

func TestWorkloadListRowsViews(t *testing.T) {
	lastSeen := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	expires := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	workloads := []api.Workload{
		{
			ID:                           "workload-1",
			Name:                         "api",
			Namespace:                    "backend",
			Type:                         "Deployment",
			Period:                       "30d",
			ReplicasCounts:               api.ReplicasCounts{MaxCount: 4, AvgCount: 3},
			OptimizationPolicy:           "balanced",
			OptimizationPolicyTimeWindow: "30d",
			CPUOptimizationPolicy:        "burstable",
			MemoryOptimizationPolicy:     "conservative",
			MemoryRequestEqualsLimit:     false,
			ResilienceLevel:              "high",
			MuteStatus:                   api.MuteStatus{IsMuted: true, Expires: &expires},
			Cost:                         101.2,
			Waste:                        51.7,
			PotentialSavings:             12.3,
			LastSeen:                     &lastSeen,
			MaxIndicator:                 &api.Indicator{Name: "oom-risk", Type: "risk", Severity: 3},
			WorkloadLabels:               map[string]string{"app": "api", "team": "platform"},
			Derived: api.WorkloadDerived{
				ContainerCount:                   2,
				RiskIndicatorsCount:              1,
				WasteIndicatorsCount:             2,
				CurrentCPURequestCoresTotal:      1.25,
				CurrentCPULimitCoresTotal:        2.00,
				CurrentMemoryRequestMiBTotal:     768,
				CurrentMemoryLimitMiBTotal:       1536,
				RecommendedCPURequestCoresTotal:  0.75,
				RecommendedMemoryRequestMiBTotal: 512,
				CPUUsageP90CoresSum:              0.45,
				CPUUsageP95CoresSum:              0.60,
				CPUUsageP100CoresSum:             0.80,
				MemoryUsageP90MiBSum:             350,
				MemoryUsageP95MiBSum:             420,
				MemoryUsageP100MiBSum:            500,
			},
			Indicators: []api.Indicator{
				{Name: "waste-cpu", Type: "waste", Severity: 2},
				{Name: "oom-risk", Type: "risk", Severity: 3},
			},
		},
	}

	headers, rows := workloadListRows(workloads, workloadViewCapacity)
	if headers[3] != "REPL_MAX" || rows[0][3] != "4" {
		t.Fatalf("capacity view mismatch: headers=%v rows=%v", headers, rows)
	}
	if rows[0][6] != "1.25" || rows[0][10] != "0.75" {
		t.Fatalf("capacity metrics mismatch: row=%v", rows[0])
	}

	headers, rows = workloadListRows(workloads, workloadViewUsage)
	if headers[4] != "CPU_P90_SUM" || rows[0][5] != "0.60" || rows[0][8] != "420.00" {
		t.Fatalf("usage view mismatch: headers=%v row=%v", headers, rows[0])
	}

	headers, rows = workloadListRows(workloads, workloadViewPolicy)
	if headers[3] != "POLICY" || rows[0][3] != "balanced" || rows[0][9] != "true" {
		t.Fatalf("policy view mismatch: headers=%v row=%v", headers, rows[0])
	}

	headers, rows = workloadListRows(workloads, workloadViewRisk)
	if headers[3] != "RISK_SEVERITY" || rows[0][3] != "3" || rows[0][4] != "1" || rows[0][5] != "2" {
		t.Fatalf("risk view mismatch: headers=%v row=%v", headers, rows[0])
	}

	headers, rows = workloadListRows(workloads, workloadViewAll)
	if headers[0] != "ID" || rows[0][0] != "workload-1" || rows[0][11] != "2" {
		t.Fatalf("all view mismatch: headers=%v row=%v", headers, rows[0])
	}

	headers, rows = workloadListRows(workloads, workloadViewDefault)
	if headers[0] != "NAME" || rows[0][0] != "api" {
		t.Fatalf("default view mismatch: headers=%v row=%v", headers, rows[0])
	}
}
