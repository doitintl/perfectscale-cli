package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientListPublicClusters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/public/v1/clusters" {
			t.Fatalf("path = %s, want /public/v1/clusters", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer service-token" {
			t.Fatalf("authorization = %q, want Bearer service-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"uid":"cluster-1","name":"prod-a","cloud":"aws","region":"us-east-1","createdAt":"2026-04-01T00:00:00Z","lastTransmittedAt":"2026-04-02T00:00:00Z"}]}`))
	}))
	defer server.Close()

	client := NewClient()
	clusters, err := client.ListPublicClusters(context.Background(), server.URL+"/public/v1", "service-token")
	if err != nil {
		t.Fatalf("ListPublicClusters() error = %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("len(clusters) = %d, want 1", len(clusters))
	}
	if clusters[0].UID != "cluster-1" {
		t.Fatalf("UID = %q, want cluster-1", clusters[0].UID)
	}
	if clusters[0].Name != "prod-a" {
		t.Fatalf("Name = %q, want prod-a", clusters[0].Name)
	}
}

func TestClientGetPublicCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/public/v1/clusters/cluster-1" {
			t.Fatalf("path = %s, want /public/v1/clusters/cluster-1", r.URL.Path)
		}
		if got := r.URL.Query().Get("period"); got != "30d" {
			t.Fatalf("period query = %q, want 30d", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer service-token" {
			t.Fatalf("authorization = %q, want Bearer service-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"uid":"cluster-1","name":"prod-a","cloud":"aws","region":"us-east-1","createdAt":"2026-04-01T00:00:00Z","lastTransmittedAt":"2026-04-02T00:00:00Z","emission":{"co2e":12.5,"kwh":41.2}}}`))
	}))
	defer server.Close()

	client := NewClient()
	cluster, err := client.GetPublicCluster(context.Background(), server.URL+"/public/v1", "service-token", "cluster-1", "30d")
	if err != nil {
		t.Fatalf("GetPublicCluster() error = %v", err)
	}
	if cluster.UID != "cluster-1" {
		t.Fatalf("UID = %q, want cluster-1", cluster.UID)
	}
	if cluster.Emission["co2e"] != 12.5 {
		t.Fatalf("emission[co2e] = %v, want 12.5", cluster.Emission["co2e"])
	}
}

func TestClientListPublicWorkloadsRichMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/public/v1/clusters/cluster-1/workloads" {
			t.Fatalf("path = %s, want /public/v1/clusters/cluster-1/workloads", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer service-token" {
			t.Fatalf("authorization = %q, want Bearer service-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"workload-1","name":"api","type":"Deployment","namespace":"backend","runningMinutes":1440,"firstSeen":"2026-04-01T00:00:00Z","lastSeen":"2026-04-02T00:00:00Z","replicasCounts":{"maxCount":4,"avgCount":3},"resilienceLevel":"high","optimizationPolicy":"balanced","optimizationPolicyTimeWindow":"30d","cpuOptimizationPolicy":"burstable","memoryOptimizationPolicy":"conservative","memoryRequestEqualsLimit":false,"muteStatus":{"isMuted":true,"expires":"2026-05-01T00:00:00Z"},"costAnalysis":{"past30Days":{"totalCost":101.2,"wastedCost":51.7,"costPerHour":0.42},"next30Days":{"potentialSavings":12.3,"costIncrease":1.1}},"workloadLabels":{"app":"api","team":"platform"},"indicators":[{"name":"waste-cpu","type":"waste","severityLevel":2}],"containers":[{"name":"api","runningMinutes":1440,"indicators":[{"name":"oom-risk","type":"risk","severityLevel":3}],"resources":{"current":{"memoryRequestMiB":512,"memoryLimitMiB":1024,"cpuRequestCores":0.5,"cpuLimitCores":1},"recommended":{"memoryRequestMiB":256,"memoryLimitMiB":512,"cpuRequestCores":0.25,"cpuLimitCores":0.5}},"usage":{"cpuCores":{"p90":0.2,"p95":0.3,"p100":0.4},"memoryMiB":{"p90":200,"p95":220,"p100":240}}}]}]}`))
	}))
	defer server.Close()

	client := NewClient()
	workloads, err := client.ListPublicWorkloads(context.Background(), server.URL+"/public/v1", "service-token", "cluster-1")
	if err != nil {
		t.Fatalf("ListPublicWorkloads() error = %v", err)
	}
	if len(workloads) != 1 {
		t.Fatalf("len(workloads) = %d, want 1", len(workloads))
	}

	item := workloads[0]
	if item.ReplicasCounts.MaxCount != 4 {
		t.Fatalf("MaxCount = %d, want 4", item.ReplicasCounts.MaxCount)
	}
	if !item.MuteStatus.IsMuted {
		t.Fatal("MuteStatus.IsMuted = false, want true")
	}
	if item.MaxIndicator == nil {
		t.Fatal("MaxIndicator = nil, want non-nil")
	}
	if item.MaxIndicator.Type != "risk" || item.MaxIndicator.Severity != 3 {
		t.Fatalf("MaxIndicator = %#v, want risk severity 3", item.MaxIndicator)
	}
	if item.WorkloadLabels["team"] != "platform" {
		t.Fatalf("WorkloadLabels[team] = %q, want platform", item.WorkloadLabels["team"])
	}
	if len(item.Containers) != 1 {
		t.Fatalf("len(Containers) = %d, want 1", len(item.Containers))
	}
	if item.Derived.ContainerCount != 1 {
		t.Fatalf("Derived.ContainerCount = %d, want 1", item.Derived.ContainerCount)
	}
	if item.Derived.RiskIndicatorsCount != 1 {
		t.Fatalf("Derived.RiskIndicatorsCount = %d, want 1", item.Derived.RiskIndicatorsCount)
	}
	if item.Derived.WasteIndicatorsCount != 1 {
		t.Fatalf("Derived.WasteIndicatorsCount = %d, want 1", item.Derived.WasteIndicatorsCount)
	}
	if got := item.Derived.CurrentCPURequestCoresTotal; got != 0.5 {
		t.Fatalf("current cpu request total = %v, want 0.5", got)
	}
	if got := item.Derived.RecommendedMemoryRequestMiBTotal; got != 256 {
		t.Fatalf("recommended memory request total = %v, want 256", got)
	}
	if got := item.Derived.CPUUsageP95CoresSum; got != 0.3 {
		t.Fatalf("cpu p95 sum = %v, want 0.3", got)
	}
	if got := item.Derived.MemoryUsageP95MiBSum; got != 220 {
		t.Fatalf("memory p95 sum = %v, want 220", got)
	}
	if got := item.Containers[0].Resources.Recommended.MemoryRequestMiB; got != 256 {
		t.Fatalf("recommended memory request = %v, want 256", got)
	}
	if got := item.Containers[0].Usage.MemoryMiB.P95; got != 220 {
		t.Fatalf("usage memory p95 = %v, want 220", got)
	}
	wantExpires := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if item.MuteStatus.Expires == nil || !item.MuteStatus.Expires.Equal(wantExpires) {
		t.Fatalf("mute expiry = %v, want %v", item.MuteStatus.Expires, wantExpires)
	}
}

const nodeGroupListBody = `{
  "data": [
    {
      "id": "ondemand-a",
      "autoscalerType": "cluster_autoscaler",
      "architectures": ["amd64"],
      "reservations": ["on_demand"],
      "nodes": {"min": 2, "max": 4, "avg": 3},
      "pods": {"capacity": 58, "allocatable": 58, "avgCount": 9.19},
      "runningMinutes": 129449,
      "cpu": {
        "idleCores": 3.24,
        "requested": {"avgCores": 1.44, "minCores": 0.07, "maxCores": 1.46, "p80Cores": 1.44, "p95Cores": 1.44, "p99Cores": 1.44, "p999Cores": 1.46},
        "used": {"avgCores": 0.05, "minCores": 0, "maxCores": 0.88, "p80Cores": 0.06, "p95Cores": 0.08, "p99Cores": 0.44, "p999Cores": 0.51}
      },
      "mem": {
        "idleMiB": 8455.72,
        "requested": {"avgMiB": 6518.44, "minMiB": 14.59, "maxMiB": 6559.3, "p80MiB": 6518.44, "p95MiB": 6518.51, "p99MiB": 6521.06, "p999MiB": 6559.3},
        "used": {"avgMiB": 4110.81, "minMiB": 17.84, "maxMiB": 5776.82, "p80MiB": 4658.4, "p95MiB": 4904.28, "p99MiB": 4935.93, "p999MiB": 5026.56}
      },
      "gpu": {
        "architectures": ["Tesla T4"],
        "sharingType": ["time_slicing"],
        "idle": {"memoryMiB": 2607.61, "units": 0.7},
        "requested": {"avgMemoryMiB": 0, "minMemoryMiB": 0, "maxMemoryMiB": 0, "p80MemoryMiB": 0, "p95MemoryMiB": 0, "p99MemoryMiB": 0, "p999MemoryMiB": 0, "avgUnits": 0.18, "minUnits": 0.18, "maxUnits": 0.18, "p80Units": 0.18, "p95Units": 0.18, "p99Units": 0.18, "p999Units": 0.18},
        "used": {"avgMemoryMiB": 269.93, "minMemoryMiB": 5.62, "maxMemoryMiB": 655.68, "p80MemoryMiB": 655.68, "p95MemoryMiB": 655.68, "p99MemoryMiB": 655.68, "p999MemoryMiB": 655.68, "avgUnits": 0.03, "minUnits": 0, "maxUnits": 0.18, "p80Units": 0.18, "p95Units": 0.18, "p99Units": 0.18, "p999Units": 0.18}
      },
      "cost": {
        "hourly": {"amount": "0.23", "currency": "USD"},
        "timeframe": {"amount": "72.12", "currency": "USD"},
        "idle": {
          "cpu": {"amount": "24.50", "currency": "USD"},
          "gpu": null,
          "mem": {"amount": "11.21", "currency": "USD"},
          "total": {"amount": "35.71", "currency": "USD"}
        }
      },
      "labels": {"instance_type": "m6a.2xlarge"},
      "nodeTypes": [],
      "recommendations": {
        "type": "standard",
        "hasChanges": true,
        "nodeTypes": [
          {"id": "m6a.2xlarge", "instanceType": "m6a.2xlarge", "instanceFamily": "m6a", "hourlyCost": 0.23, "estimatedSavings": 12.5, "estimatedSavingsPct": 30.1, "nodeCount": 3}
        ]
      }
    },
    {
      "id": "clickhouse",
      "autoscalerType": "karpenter",
      "nodes": {"min": 2, "max": 6, "avg": 3.14},
      "pods": {"capacity": 526, "allocatable": 526, "avgCount": 12.09},
      "cpu": {
        "requested": {"avgCores": 33.61, "minCores": 2.04, "maxCores": 39.63, "p80Cores": 39.5, "p95Cores": 39.61, "p99Cores": 39.61, "p999Cores": 39.63},
        "used": {"avgCores": 3.2, "minCores": 0, "maxCores": 43.54, "p80Cores": 7.83, "p95Cores": 18.63, "p99Cores": 30.19, "p999Cores": 36.52}
      },
      "mem": {
        "requested": {"avgMiB": 136127.85, "minMiB": 79650.35, "maxMiB": 162018.71, "p80MiB": 161955.37, "p95MiB": 161977.91, "p99MiB": 161980.47, "p999MiB": 162018.71},
        "used": {"avgMiB": 55705.69, "minMiB": 5460.91, "maxMiB": 132448.23, "p80MiB": 78750.06, "p95MiB": 90906.32, "p99MiB": 102400.28, "p999MiB": 114167.23}
      },
      "cost": {
        "hourly": {"amount": "1.97", "currency": "USD"},
        "timeframe": {"amount": "1376.37", "currency": "USD"},
        "idle": {
          "cpu": {"amount": "378.27", "currency": "USD"},
          "gpu": null,
          "mem": {"amount": "173.87", "currency": "USD"},
          "total": {"amount": "552.14", "currency": "USD"}
        }
      },
      "nodeTypes": [],
      "recommendations": {
        "type": "karpenter",
        "hasChanges": true,
        "currentConfig": {"apiVersion": "karpenter.sh/v1", "kind": "NodePool", "metadata": {"name": "clickhouse"}, "spec": {"disruption": {"consolidationPolicy": "WhenEmpty"}}},
        "recommendedConfig": {"apiVersion": "karpenter.sh/v1", "kind": "NodePool", "metadata": {"name": "clickhouse"}, "spec": {"disruption": {"consolidationPolicy": "WhenEmptyOrUnderutilized"}}},
        "changes": [
          {"id": "consolidation-policy-optimization", "title": "Consolidation Policy", "path": ".spec.disruption.consolidationPolicy", "operation": "replace", "currentValue": "WhenEmpty", "recommendedValue": "WhenEmptyOrUnderutilized", "rationale": "Consolidates underutilized nodes too."}
        ]
      }
    }
  ],
  "meta": {
    "pagination": {"next": "eyJkaXIiOiJmb3J3YXJkIn0", "prev": null, "pageSize": 50},
    "timeframe": "P30D"
  }
}`

func TestClientListPublicNodeGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/public/v1/clusters/cluster-1/node-groups" {
			t.Fatalf("path = %s, want /public/v1/clusters/cluster-1/node-groups", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("period"); got != "P30D" {
			t.Fatalf("period query = %q, want P30D", got)
		}
		if got := query.Get("pageSize"); got != "25" {
			t.Fatalf("pageSize query = %q, want 25", got)
		}
		if got := query.Get("autoscalerType"); got != "karpenter" {
			t.Fatalf("autoscalerType query = %q, want karpenter", got)
		}
		if got := query.Get("hasRecommendations"); got != "true" {
			t.Fatalf("hasRecommendations query = %q, want true", got)
		}
		if got := query.Get("includeMuted"); got != "false" {
			t.Fatalf("includeMuted query = %q, want false", got)
		}
		if got := query.Get("recommendationLimit"); got != "5" {
			t.Fatalf("recommendationLimit query = %q, want 5", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(nodeGroupListBody))
	}))
	defer server.Close()

	period := "P30D"
	pageSize := 25
	autoscalerType := "karpenter"
	hasRecommendations := true
	includeMuted := false
	recommendationLimit := 5

	client := NewClient()
	page, err := client.ListPublicNodeGroups(context.Background(), server.URL+"/public/v1", "service-token", "cluster-1", NodeGroupListOptions{
		Period:              &period,
		PageSize:            &pageSize,
		AutoscalerType:      &autoscalerType,
		HasRecommendations:  &hasRecommendations,
		IncludeMuted:        &includeMuted,
		RecommendationLimit: &recommendationLimit,
	})
	if err != nil {
		t.Fatalf("ListPublicNodeGroups() error = %v", err)
	}
	if len(page.NodeGroups) != 2 {
		t.Fatalf("len(NodeGroups) = %d, want 2", len(page.NodeGroups))
	}
	if page.Timeframe != "P30D" {
		t.Fatalf("Timeframe = %q, want P30D", page.Timeframe)
	}
	if page.Pagination.Next == nil || *page.Pagination.Next == "" {
		t.Fatal("Pagination.Next = nil, want a cursor")
	}

	standard := page.NodeGroups[0]
	if got := standard.CPU.Requested.Avg; got != 1.44 {
		t.Fatalf("standard CPU.Requested.Avg = %v, want 1.44 (avgCores field must map correctly)", got)
	}
	if got := standard.Mem.Requested.Avg; got != 6518.44 {
		t.Fatalf("standard Mem.Requested.Avg = %v, want 6518.44 (avgMiB field must map correctly)", got)
	}
	if got := standard.Cost.Hourly; got != 0.23 {
		t.Fatalf("standard Cost.Hourly = %v, want 0.23 (Money.amount must parse as float)", got)
	}
	if got := standard.Cost.IdleTotal; got != 35.71 {
		t.Fatalf("standard Cost.IdleTotal = %v, want 35.71 (cost.idle.total must map correctly)", got)
	}
	if standard.Recommendations.Type != NodeGroupRecommendationsTypeStandard {
		t.Fatalf("standard Recommendations.Type = %q, want standard", standard.Recommendations.Type)
	}
	if len(standard.Recommendations.NodeTypeRecs) != 1 {
		t.Fatalf("len(standard.Recommendations.NodeTypeRecs) = %d, want 1", len(standard.Recommendations.NodeTypeRecs))
	}
	if got := standard.Recommendations.NodeTypeRecs[0].InstanceType; got != "m6a.2xlarge" {
		t.Fatalf("NodeTypeRecs[0].InstanceType = %q, want m6a.2xlarge", got)
	}
	if standard.GPU == nil {
		t.Fatal("standard.GPU = nil, want populated GPU utilization")
	}
	if got := standard.GPU.Architectures; len(got) != 1 || got[0] != "Tesla T4" {
		t.Fatalf("GPU.Architectures = %v, want [Tesla T4]", got)
	}
	if got := standard.GPU.Used.AvgMemoryMiB; got != 269.93 {
		t.Fatalf("GPU.Used.AvgMemoryMiB = %v, want 269.93", got)
	}
	if got := standard.GPU.IdleUnits; got != 0.7 {
		t.Fatalf("GPU.IdleUnits = %v, want 0.7", got)
	}

	karpenter := page.NodeGroups[1]
	if karpenter.GPU != nil {
		t.Fatalf("karpenter.GPU = %+v, want nil for a group with no gpu field", karpenter.GPU)
	}
	if karpenter.Recommendations.Type != NodeGroupRecommendationsTypeKarpenter {
		t.Fatalf("karpenter Recommendations.Type = %q, want karpenter", karpenter.Recommendations.Type)
	}
	if len(karpenter.Recommendations.Changes) != 1 {
		t.Fatalf("len(karpenter.Recommendations.Changes) = %d, want 1", len(karpenter.Recommendations.Changes))
	}
	if got := karpenter.Recommendations.Changes[0].RecommendedValue; got != "WhenEmptyOrUnderutilized" {
		t.Fatalf("Changes[0].RecommendedValue = %q, want WhenEmptyOrUnderutilized", got)
	}
	if karpenter.Recommendations.CurrentConfig == nil {
		t.Fatal("karpenter Recommendations.CurrentConfig = nil, want full raw config")
	}
	if got := karpenter.Recommendations.CurrentConfig["kind"]; got != "NodePool" {
		t.Fatalf("CurrentConfig[kind] = %v, want NodePool", got)
	}
	if karpenter.Recommendations.RecommendedConfig == nil {
		t.Fatal("karpenter Recommendations.RecommendedConfig = nil, want full raw config")
	}
}

func TestClientGetPublicNodeGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/public/v1/clusters/cluster-1/node-groups/clickhouse" {
			t.Fatalf("path = %s, want /public/v1/clusters/cluster-1/node-groups/clickhouse", r.URL.Path)
		}
		if got := r.URL.Query().Get("recommendationLimit"); got != "3" {
			t.Fatalf("recommendationLimit query = %q, want 3", got)
		}
		w.Header().Set("Content-Type", "application/json")
		body := `{"data": ` + nodeGroupListDataItem(t, 1) + `}`
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient()
	group, err := client.GetPublicNodeGroup(context.Background(), server.URL+"/public/v1", "service-token", "cluster-1", "clickhouse", "P30D", 3)
	if err != nil {
		t.Fatalf("GetPublicNodeGroup() error = %v", err)
	}
	if group.ID != "clickhouse" {
		t.Fatalf("ID = %q, want clickhouse", group.ID)
	}
	if group.Recommendations.Type != NodeGroupRecommendationsTypeKarpenter {
		t.Fatalf("Recommendations.Type = %q, want karpenter", group.Recommendations.Type)
	}
	if group.Cost.Hourly != 1.97 {
		t.Fatalf("Cost.Hourly = %v, want 1.97", group.Cost.Hourly)
	}
}

// nodeGroupListDataItem extracts data[index] from nodeGroupListBody as a raw JSON object.
func nodeGroupListDataItem(t *testing.T, index int) string {
	t.Helper()

	var parsed struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(nodeGroupListBody), &parsed); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if index >= len(parsed.Data) {
		t.Fatalf("fixture has %d items, want index %d", len(parsed.Data), index)
	}
	return string(parsed.Data[index])
}
