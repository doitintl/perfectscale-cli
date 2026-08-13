package cli

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/perfectscale/poc-cli/internal/config"
	"github.com/perfectscale/poc-cli/internal/profile"
)

const clustersListBody = `{"data":[{"uid":"cluster-1","name":"prod-a","cloud":"aws","region":"us-east-1","createdAt":"2026-04-01T00:00:00Z","lastTransmittedAt":"2026-04-02T00:00:00Z"}]}`

func TestNodegroupsListWiresServerSideFilters(t *testing.T) {
	var nodeGroupsQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/node-groups":
			nodeGroupsQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"next":null,"prev":null,"pageSize":25},"timeframe":"P30D"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL,
		"nodegroups", "list",
		"-c", "prod-a",
		"--autoscaler-type", "karpenter",
		"--has-recommendations",
		"--include-muted",
		"--recommendation-limit", "7",
		"--page-size", "25",
		"--page-token", "opaque-cursor",
	)
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, output)
	}

	query := mustParseQuery(t, nodeGroupsQuery)
	if got := query.Get("autoscalerType"); got != "karpenter" {
		t.Fatalf("autoscalerType = %q, want karpenter", got)
	}
	if got := query.Get("hasRecommendations"); got != "true" {
		t.Fatalf("hasRecommendations = %q, want true", got)
	}
	if got := query.Get("includeMuted"); got != "true" {
		t.Fatalf("includeMuted = %q, want true", got)
	}
	if got := query.Get("recommendationLimit"); got != "7" {
		t.Fatalf("recommendationLimit = %q, want 7", got)
	}
	if got := query.Get("pageSize"); got != "25" {
		t.Fatalf("pageSize = %q, want 25", got)
	}
	if got := query.Get("pageToken"); got != "opaque-cursor" {
		t.Fatalf("pageToken = %q, want opaque-cursor", got)
	}
}

func TestNodegroupsListRejectsOversizedPageSize(t *testing.T) {
	output, err := runNodegroupsCLI(t, "https://example.invalid",
		"nodegroups", "list", "-c", "prod-a", "--page-size", "501",
	)
	if err == nil {
		t.Fatalf("runNodegroupsCLI() error = nil, want error; output=%s", output)
	}
	assertContains(t, err.Error(), "--page-size must be <= 500")
}

func TestNodegroupsGetWiresFlags(t *testing.T) {
	var (
		nodeGroupPath  string
		nodeGroupQuery string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/node-groups/clickhouse":
			nodeGroupPath = r.URL.Path
			nodeGroupQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"data":{"id":"clickhouse","autoscalerType":"karpenter","nodes":{"min":1,"max":1,"avg":1},"pods":{"capacity":1,"allocatable":1,"avgCount":1},"cpu":{"requested":{"avgCores":0,"minCores":0,"maxCores":0,"p80Cores":0,"p95Cores":0,"p99Cores":0,"p999Cores":0},"used":{"avgCores":0,"minCores":0,"maxCores":0,"p80Cores":0,"p95Cores":0,"p99Cores":0,"p999Cores":0}},"mem":{"requested":{"avgMiB":0,"minMiB":0,"maxMiB":0,"p80MiB":0,"p95MiB":0,"p99MiB":0,"p999MiB":0},"used":{"avgMiB":0,"minMiB":0,"maxMiB":0,"p80MiB":0,"p95MiB":0,"p99MiB":0,"p999MiB":0}},"cost":{"hourly":{"amount":"1.00","currency":"USD"},"timeframe":{"amount":"1.00","currency":"USD"},"idle":{"cpu":{"amount":"0","currency":"USD"},"gpu":null,"mem":{"amount":"0","currency":"USD"},"total":{"amount":"0","currency":"USD"}}},"nodeTypes":[],"recommendations":{"type":"standard","hasChanges":false,"nodeTypes":[]}}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL,
		"nodegroups", "get",
		"-c", "prod-a",
		"-g", "clickhouse",
		"--recommendation-limit", "9",
	)
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, output)
	}

	if nodeGroupPath != "/public/v1/clusters/cluster-1/node-groups/clickhouse" {
		t.Fatalf("path = %q, want node-groups/clickhouse", nodeGroupPath)
	}
	query := mustParseQuery(t, nodeGroupQuery)
	if got := query.Get("recommendationLimit"); got != "9" {
		t.Fatalf("recommendationLimit = %q, want 9", got)
	}
	assertContains(t, output, "recommendations.type")
	assertContains(t, output, "standard")
}

func TestNodegroupsGetRequiresNodeGroupFlag(t *testing.T) {
	_, err := runNodegroupsCLI(t, "https://example.invalid", "nodegroups", "get", "-c", "prod-a")
	if err == nil {
		t.Fatal("runNodegroupsCLI() error = nil, want error for missing --node-group")
	}
}

const nodeGroupListBodyWithGPU = `{
  "data": [
    {
      "id": "cron",
      "autoscalerType": "karpenter",
      "nodes": {"min": 1, "max": 2, "avg": 1.5},
      "pods": {"capacity": 10, "allocatable": 10, "avgCount": 5},
      "cpu": {"requested": {"avgCores": 0, "minCores": 0, "maxCores": 0, "p80Cores": 0, "p95Cores": 0, "p99Cores": 0, "p999Cores": 0}, "used": {"avgCores": 0, "minCores": 0, "maxCores": 0, "p80Cores": 0, "p95Cores": 0, "p99Cores": 0, "p999Cores": 0}},
      "mem": {"requested": {"avgMiB": 0, "minMiB": 0, "maxMiB": 0, "p80MiB": 0, "p95MiB": 0, "p99MiB": 0, "p999MiB": 0}, "used": {"avgMiB": 0, "minMiB": 0, "maxMiB": 0, "p80MiB": 0, "p95MiB": 0, "p99MiB": 0, "p999MiB": 0}},
      "cost": {"hourly": {"amount": "0.19", "currency": "USD"}, "timeframe": {"amount": "1", "currency": "USD"}, "idle": {"cpu": {"amount": "0", "currency": "USD"}, "gpu": null, "mem": {"amount": "0", "currency": "USD"}, "total": {"amount": "0", "currency": "USD"}}},
      "nodeTypes": [],
      "recommendations": {"type": "karpenter", "hasChanges": false, "currentConfig": null, "recommendedConfig": null, "changes": []}
    },
    {
      "id": "default-spot-gpu",
      "autoscalerType": "karpenter",
      "nodes": {"min": 1, "max": 1, "avg": 1},
      "pods": {"capacity": 29, "allocatable": 29, "avgCount": 8},
      "cpu": {"requested": {"avgCores": 0.07, "minCores": 0.07, "maxCores": 0.07, "p80Cores": 0.07, "p95Cores": 0.07, "p99Cores": 0.07, "p999Cores": 0.07}, "used": {"avgCores": 0.02, "minCores": 0, "maxCores": 0.07, "p80Cores": 0.02, "p95Cores": 0.03, "p99Cores": 0.05, "p999Cores": 0.06}},
      "mem": {"requested": {"avgMiB": 0, "minMiB": 0, "maxMiB": 0, "p80MiB": 0, "p95MiB": 0, "p99MiB": 0, "p999MiB": 0}, "used": {"avgMiB": 0, "minMiB": 0, "maxMiB": 0, "p80MiB": 0, "p95MiB": 0, "p99MiB": 0, "p999MiB": 0}},
      "gpu": {
        "architectures": ["Tesla T4"],
        "sharingType": ["time_slicing"],
        "idle": {"memoryMiB": 2607.61, "units": 0.7},
        "requested": {"avgMemoryMiB": 0, "minMemoryMiB": 0, "maxMemoryMiB": 0, "p80MemoryMiB": 0, "p95MemoryMiB": 0, "p99MemoryMiB": 0, "p999MemoryMiB": 0, "avgUnits": 0.18, "minUnits": 0.18, "maxUnits": 0.18, "p80Units": 0.18, "p95Units": 0.18, "p99Units": 0.18, "p999Units": 0.18},
        "used": {"avgMemoryMiB": 269.93, "minMemoryMiB": 5.62, "maxMemoryMiB": 655.68, "p80MemoryMiB": 655.68, "p95MemoryMiB": 655.68, "p99MemoryMiB": 655.68, "p999MemoryMiB": 655.68, "avgUnits": 0.03, "minUnits": 0, "maxUnits": 0.18, "p80Units": 0.18, "p95Units": 0.18, "p99Units": 0.18, "p999Units": 0.18}
      },
      "cost": {"hourly": {"amount": "0.35", "currency": "USD"}, "timeframe": {"amount": "1", "currency": "USD"}, "idle": {"cpu": {"amount": "0", "currency": "USD"}, "gpu": null, "mem": {"amount": "0", "currency": "USD"}, "total": {"amount": "0", "currency": "USD"}}},
      "nodeTypes": [],
      "recommendations": {"type": "karpenter", "hasChanges": false, "currentConfig": null, "recommendedConfig": null, "changes": []}
    }
  ],
  "meta": {"pagination": {"next": null, "prev": null, "pageSize": 50}, "timeframe": "P30D"}
}`

func TestNodegroupsListGPUViewShowsColumnsAndDashForNonGPU(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/node-groups":
			_, _ = w.Write([]byte(nodeGroupListBodyWithGPU))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL, "nodegroups", "list", "-c", "prod-a", "-V", "gpu")
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, output)
	}

	assertContains(t, output, "GPU_ARCH")
	assertContains(t, output, "GPU_REQ_UNITS_AVG")
	assertContains(t, output, "Tesla T4")
	assertContains(t, output, "0.180")
	assertContains(t, output, "0.030")
	assertContains(t, output, "269.930")

	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "cron ") {
			if !strings.Contains(line, "-") {
				t.Fatalf("non-GPU row should show dashes for GPU columns: %q", line)
			}
			if strings.Contains(line, "Tesla") {
				t.Fatalf("non-GPU row should not show GPU data: %q", line)
			}
		}
	}
}

func TestNodegroupsListRejectsInvalidView(t *testing.T) {
	_, err := runNodegroupsCLI(t, "https://example.invalid", "nodegroups", "list", "-c", "prod-a", "-V", "bogus")
	if err == nil {
		t.Fatal("runNodegroupsCLI() error = nil, want error for invalid --view")
	}
	assertContains(t, err.Error(), `unsupported --view "bogus"`)
}

func TestNodegroupsHelpIncludesPaginationFlags(t *testing.T) {
	output, err := runCLI(t, nil, "nodegroups", "list", "--help")
	if err != nil {
		t.Fatalf("runCLI(nodegroups list --help) error = %v", err)
	}
	assertContains(t, output, "--page-size value")
	assertContains(t, output, "--page-token value")
	assertContains(t, output, "--all")
	assertContains(t, output, "--autoscaler-type value")
	assertContains(t, output, "--has-recommendations")
}

// runNodegroupsCLI runs the CLI with a profile pointed at the given base URL,
// using a pre-set access token so no token-exchange HTTP call is made.
func runNodegroupsCLI(t *testing.T, baseURL string, args ...string) (string, error) {
	t.Helper()
	return runCLI(t, &profile.Data{
		SchemaVersion: 1,
		Name:          config.DefaultProfileName,
		AuthMode:      profile.AuthModeServiceToken,
		PublicAPIURL:  baseURL + "/public/v1",
		AccessToken:   "service-token",
		ExpiresAt:     time.Now().UTC().Add(time.Hour),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, args...)
}

func mustParseQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("parse query %q: %v", raw, err)
	}
	return values
}
