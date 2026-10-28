package cli

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/perfectscale/poc-cli/internal/api"
	"github.com/perfectscale/poc-cli/internal/config"
	"github.com/perfectscale/poc-cli/internal/profile"
)

const showClustersListBody = `{"data":[{"id":"123","uid":"prod-a","name":"prod-a","cloud":"aws","region":"us-east-1","createdAt":"2026-04-01T00:00:00Z","lastTransmittedAt":"2026-04-02T00:00:00Z"}]}`

const showWorkloadsListBody = `{"data":[{"id":"workload-1","name":"api","type":"Deployment","namespace":"backend","runningMinutes":1440,"firstSeen":"2026-04-01T00:00:00Z","lastSeen":"2026-04-02T00:00:00Z","replicasCounts":{"maxCount":1,"avgCount":1},"muteStatus":{"isMuted":false},"automationStatus":"Active","costAnalysis":{"past30Days":{"totalCost":1,"wastedCost":0,"costPerHour":0.01},"next30Days":{"potentialSavings":0,"costIncrease":0}}}]}`

func runShowCLI(t *testing.T, baseURL string, args ...string) (string, error) {
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

func newShowServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(showClustersListBody))
		case "/public/v1/clusters/prod-a/workloads":
			_, _ = w.Write([]byte(showWorkloadsListBody))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestWorkloadsShowJSONIncludesAutomationStatus(t *testing.T) {
	server := newShowServer(t)

	output, err := runShowCLI(t, server.URL, "workloads", "show", "-c", "prod-a", "-i", "workload-1", "-o", "json")
	if err != nil {
		t.Fatalf("runShowCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, `"automation_status": "Active"`)
}

func TestWorkloadsShowTableIncludesAutomationStatus(t *testing.T) {
	server := newShowServer(t)

	output, err := runShowCLI(t, server.URL, "workloads", "show", "-c", "prod-a", "-i", "workload-1")
	if err != nil {
		t.Fatalf("runShowCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, "automation_status")
	assertContains(t, output, "Active")
}

func TestWriteWorkloadsCSV(t *testing.T) {
	tests := []struct {
		name          string
		workload      api.Workload
		wantLastValue string
	}{
		{
			name: "happy_path",
			workload: api.Workload{
				ID:               "workload-1",
				Name:             "api",
				Namespace:        "backend",
				Type:             "Deployment",
				Period:           "30d",
				Cost:             101.2,
				Waste:            51.7,
				PotentialSavings: 12.3,
				CostPerHour:      0.42,
				RunningMinutes:   1440,
				MuteStatus:       api.MuteStatus{IsMuted: false, Expires: nil},
				AutomationStatus: "Active",
			},
			wantLastValue: "Active",
		},
		{
			name: "empty_automation_status",
			workload: api.Workload{
				ID:               "workload-2",
				Name:             "worker",
				Namespace:        "backend",
				Type:             "Deployment",
				Period:           "30d",
				Cost:             10,
				Waste:            1,
				PotentialSavings: 0,
				CostPerHour:      0.1,
				RunningMinutes:   60,
				MuteStatus:       api.MuteStatus{IsMuted: false, Expires: nil},
				AutomationStatus: "",
			},
			wantLastValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := writeWorkloadsCSV(&buf, []api.Workload{tt.workload}); err != nil {
				t.Fatalf("writeWorkloadsCSV() error = %v", err)
			}

			records, err := csv.NewReader(&buf).ReadAll()
			if err != nil {
				t.Fatalf("csv.ReadAll() error = %v", err)
			}
			if len(records) != 2 {
				t.Fatalf("len(records) = %d, want 2 (header + 1 row)", len(records))
			}

			header := records[0]
			if len(header) != 14 {
				t.Fatalf("len(header) = %d, want 14", len(header))
			}
			if last := header[len(header)-1]; last != "automation_status" {
				t.Fatalf("last header = %q, want automation_status", last)
			}

			row := records[1]
			if last := row[len(row)-1]; last != tt.wantLastValue {
				t.Fatalf("last value = %q, want %q", last, tt.wantLastValue)
			}
		})
	}
}
