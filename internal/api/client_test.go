package api

import (
	"context"
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
