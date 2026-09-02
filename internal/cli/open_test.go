package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/perfectscale/poc-cli/internal/config"
	"github.com/perfectscale/poc-cli/internal/profile"
)

const openClustersListBody = `{"data":[{"id":"123","uid":"prod-a","name":"prod-a","cloud":"aws","region":"us-east-1","createdAt":"2026-04-01T00:00:00Z","lastTransmittedAt":"2026-04-02T00:00:00Z"}]}`

const openWorkloadsListBody = `{"data":[{"id":"workload-1","name":"api","type":"Deployment","namespace":"backend","runningMinutes":1440,"firstSeen":"2026-04-01T00:00:00Z","lastSeen":"2026-04-02T00:00:00Z","replicasCounts":{"maxCount":1,"avgCount":1},"muteStatus":{"isMuted":false},"costAnalysis":{"past30Days":{"totalCost":1,"wastedCost":0,"costPerHour":0.01},"next30Days":{"potentialSavings":0,"costIncrease":0}}}]}`

func TestOpenClusterJSONPrintsPodFitURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/public/v1/clusters" {
			_, _ = w.Write([]byte(openClustersListBody))
			return
		}
		t.Fatalf("unexpected path %s", r.URL.Path)
	}))
	defer server.Close()

	output, err := runOpenCLI(t, server.URL, "open", "cluster", "-c", "prod-a", "-o", "json")
	if err != nil {
		t.Fatalf("runOpenCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, `"url": "https://app.perfectscale.io/pod-fit/prod-a?period=30d"`)
}

func TestOpenClusterCustomPeriod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(openClustersListBody))
	}))
	defer server.Close()

	output, err := runOpenCLI(t, server.URL, "open", "cluster", "-c", "prod-a", "-w", "7d", "-o", "json")
	if err != nil {
		t.Fatalf("runOpenCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, `"url": "https://app.perfectscale.io/pod-fit/prod-a?period=7d"`)
}

func TestOpenWorkloadByIDPrintsZoomInURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(openClustersListBody))
		case "/public/v1/clusters/prod-a/workloads":
			_, _ = w.Write([]byte(openWorkloadsListBody))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runOpenCLI(t, server.URL, "open", "workload", "-c", "prod-a", "-i", "workload-1", "-o", "json")
	if err != nil {
		t.Fatalf("runOpenCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, `"url": "https://app.perfectscale.io/pod-fit/prod-a/zoom_in/workload-1?period=30d"`)
}

func TestOpenWorkloadByNameAndNamespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(openClustersListBody))
		case "/public/v1/clusters/prod-a/workloads":
			_, _ = w.Write([]byte(openWorkloadsListBody))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runOpenCLI(t, server.URL, "open", "workload", "-c", "prod-a", "-m", "api", "-n", "backend", "-o", "jsonl")
	if err != nil {
		t.Fatalf("runOpenCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, `"url":"https://app.perfectscale.io/pod-fit/prod-a/zoom_in/workload-1?period=30d"`)
}

func TestOpenWorkloadRequiresIDOrName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(openClustersListBody))
		case "/public/v1/clusters/prod-a/workloads":
			_, _ = w.Write([]byte(openWorkloadsListBody))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runOpenCLI(t, server.URL, "open", "workload", "-c", "prod-a", "-o", "json")
	if err == nil {
		t.Fatalf("runOpenCLI() error = nil, want non-nil; output=%s", output)
	}
	assertContains(t, err.Error(), "either --id (-i) or --name (-m) is required")
}

func TestOpenNodegroupPrintsInfraFitURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/public/v1/clusters" {
			_, _ = w.Write([]byte(openClustersListBody))
			return
		}
		t.Fatalf("unexpected path %s", r.URL.Path)
	}))
	defer server.Close()

	output, err := runOpenCLI(t, server.URL, "open", "nodegroup", "-c", "prod-a", "-g", "clickhouse", "-o", "json")
	if err != nil {
		t.Fatalf("runOpenCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, `"url": "https://app.perfectscale.io/infra-fit/prod-a/node-groups/clickhouse?nodeMode=node_detailed&period=30d&selectedGroupName=clickhouse&utilization=Avg&workloadsChartInterval=1d"`)
}

func TestOpenNodegroupRequiresNodeGroupFlag(t *testing.T) {
	output, err := runOpenCLI(t, "", "open", "nodegroup", "-c", "prod-a")
	if err == nil {
		t.Fatalf("runOpenCLI() error = nil, want non-nil; output=%s", output)
	}
	assertContains(t, err.Error(), "--node-group (-g) is required")
}

func TestOpenAlertsPrintsAlertsURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/public/v1/clusters" {
			_, _ = w.Write([]byte(openClustersListBody))
			return
		}
		t.Fatalf("unexpected path %s", r.URL.Path)
	}))
	defer server.Close()

	output, err := runOpenCLI(t, server.URL, "open", "alerts", "-c", "prod-a", "-o", "json")
	if err != nil {
		t.Fatalf("runOpenCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, `"url": "https://app.perfectscale.io/alerts?cluster=prod-a"`)
}

func TestOpenAutomationWithNoFlagsOnlySetsPeriod(t *testing.T) {
	output, err := runCLI(t, nil, "open", "automation", "-o", "json")
	if err != nil {
		t.Fatalf("runCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, `"url": "https://app.perfectscale.io/automation?period=30d"`)
}

func TestOpenAutomationWithFiltersOrdersQueryAlphabetically(t *testing.T) {
	output, err := runCLI(t, nil, "open", "automation",
		"-c", "apps-namespace-15s",
		"-n", "ab-java-svc",
		"-m", "java-svc",
		"-t", "Deployment",
		"--container", "java-svc",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("runCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, `"url": "https://app.perfectscale.io/automation?cluster_name=apps-namespace-15s&container_name=java-svc&namespace=ab-java-svc&period=30d&workload_name=java-svc&workload_type=Deployment"`)
}

func TestOpenHelpListsSubcommands(t *testing.T) {
	output, err := runCLI(t, nil, "open", "--help")
	if err != nil {
		t.Fatalf("runCLI(open --help) error = %v", err)
	}
	assertContains(t, output, "cluster")
	assertContains(t, output, "workload")
	assertContains(t, output, "nodegroup")
	assertContains(t, output, "alerts")
	assertContains(t, output, "automation")
}

func runOpenCLI(t *testing.T, baseURL string, args ...string) (string, error) {
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
