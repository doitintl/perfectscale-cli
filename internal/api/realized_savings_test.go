package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/perfectscale/poc-cli/internal/clierr"
)

var octoberRange = RealizedSavingsRange{
	Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	End:   time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
}

const realizedSavingsSummaryBody = `{"data":{
	"totalCost":{"amount":"1200.00","currency":"USD"},
	"baselineCost":{"amount":"1500.00","currency":"USD"},
	"realizedSavings":{"amount":"300.00","currency":"USD"},
	"realizedSavingsPct":20,
	"automatedCost":{"amount":"800.00","currency":"USD"},
	"notAutomatedCost":{"amount":"400.00","currency":"USD"},
	"resiliencyIssuesAddressed":7%s}}`

func TestClientGetPublicRealizedSavingsSummaries(t *testing.T) {
	tests := map[string]struct {
		get         func(client *Client, baseURL string) (RealizedSavingsSummary, error)
		wantPath    string
		wantBody    map[string]string
		clusterJSON string
		wantCluster *RealizedSavingsSummaryCluster
	}{
		"tenant": {
			get: func(client *Client, baseURL string) (RealizedSavingsSummary, error) {
				return client.GetPublicRealizedSavingsTenantSummary(context.Background(), baseURL, "service-token", octoberRange)
			},
			wantPath: "/public/v1/realized-savings/summary",
			wantBody: map[string]string{"startDate": "2026-10-01", "endDate": "2026-11-01"},
		},
		"cluster": {
			get: func(client *Client, baseURL string) (RealizedSavingsSummary, error) {
				return client.GetPublicRealizedSavingsClusterSummary(context.Background(), baseURL, "service-token", "cluster-1", octoberRange)
			},
			wantPath:    "/public/v1/realized-savings/clusters/summary",
			wantBody:    map[string]string{"clusterUid": "cluster-1", "startDate": "2026-10-01", "endDate": "2026-11-01"},
			clusterJSON: `,"cluster":{"clusterUid":"cluster-1","name":"prod-a","cloud":"aws","region":"us-east-1","pricingSource":"doit"}`,
			wantCluster: &RealizedSavingsSummaryCluster{ClusterUID: "cluster-1", Name: "prod-a", Cloud: "aws", Region: "us-east-1", PricingSource: "doit"},
		},
		"namespace": {
			get: func(client *Client, baseURL string) (RealizedSavingsSummary, error) {
				return client.GetPublicRealizedSavingsNamespaceSummary(context.Background(), baseURL, "service-token", "cluster-1", "payments", octoberRange)
			},
			wantPath:    "/public/v1/realized-savings/namespaces/summary",
			wantBody:    map[string]string{"clusterUid": "cluster-1", "namespace": "payments", "startDate": "2026-10-01", "endDate": "2026-11-01"},
			clusterJSON: `,"cluster":{"clusterUid":"cluster-1","name":"prod-a","pricingSource":"doit"}`,
			wantCluster: &RealizedSavingsSummaryCluster{ClusterUID: "cluster-1", Name: "prod-a", PricingSource: "doit"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != tt.wantPath {
					t.Fatalf("request = %s %s, want POST %s", r.Method, r.URL.Path, tt.wantPath)
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if len(body) != len(tt.wantBody) {
					t.Fatalf("body = %v, want %v", body, tt.wantBody)
				}
				for key, want := range tt.wantBody {
					if body[key] != want {
						t.Fatalf("body[%s] = %q, want %q", key, body[key], want)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(strings.Replace(realizedSavingsSummaryBody, "%s", tt.clusterJSON, 1)))
			}))
			defer server.Close()

			got, err := tt.get(newTestClient(), server.URL+"/public/v1")
			if err != nil {
				t.Fatalf("get summary error = %v", err)
			}
			if got.RealizedSavings != 300 || got.BaselineCost != 1500 || got.NotAutomatedCost != 400 || got.ResiliencyIssuesAddressed != 7 {
				t.Fatalf("summary = %+v", got)
			}
			if (got.Cluster == nil) != (tt.wantCluster == nil) || (got.Cluster != nil && *got.Cluster != *tt.wantCluster) {
				t.Fatalf("cluster = %+v, want %+v", got.Cluster, tt.wantCluster)
			}
		})
	}
}

func TestClientGetPublicRealizedSavingsSummaryPartialMonthKeepsDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"partial_month_range","detail":"Use startDate 2026-03-01 and endDate 2026-11-19 instead."}`))
	}))
	defer server.Close()

	_, err := newTestClient().GetPublicRealizedSavingsTenantSummary(context.Background(), server.URL+"/public/v1", "service-token", octoberRange)

	var statusErr *clierr.HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("error = %v, want a 422 HTTPStatusError", err)
	}
	if !strings.Contains(statusErr.Body, "Use startDate 2026-03-01") {
		t.Fatalf("body = %q, want the suggested range", statusErr.Body)
	}
}

func TestClientListAllPublicRealizedSavingsClusters(t *testing.T) {
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/public/v1/realized-savings/clusters" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		bodies = append(bodies, body)

		w.Header().Set("Content-Type", "application/json")
		if body["pageToken"] == nil {
			_, _ = w.Write([]byte(`{"data":[{"clusterUid":"cluster-1","name":"prod-a","pricingSource":"doit","transmissionStatus":"in_progress","upgradeStatus":"none",
				"totalCost":{"amount":"100.50","currency":"USD"},"realizedSavings":{"amount":"25.25","currency":"USD"},"realizedSavingsPct":20.08,
				"cpuCoreHoursSaved":{"absolute":12.5,"pct":30},"memoryGbHoursSaved":{"absolute":40,"pct":-5}}],
				"meta":{"pagination":{"next":"page-2","prev":null,"pageSize":500}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"clusterUid":"deleted-uid","name":"deleted-uid","pricingSource":"unknown","transmissionStatus":"deleted","upgradeStatus":"none",
			"totalCost":{"amount":"0","currency":"USD"},"realizedSavings":{"amount":"0","currency":"USD"},"realizedSavingsPct":0,
			"cpuCoreHoursSaved":{"absolute":0,"pct":0},"memoryGbHoursSaved":{"absolute":0,"pct":0}}],
			"meta":{"pagination":{"next":null,"prev":"page-1","pageSize":500}}}`))
	}))
	defer server.Close()

	sortBy, sortOrder := "totalCost", "asc"
	page, err := ListAllRealizedSavings(RealizedSavingsListOptions{Range: octoberRange, SortBy: &sortBy, SortOrder: &sortOrder}, 0,
		func(opts RealizedSavingsListOptions) (RealizedSavingsPage[RealizedSavingsCluster], error) {
			return newTestClient().ListPublicRealizedSavingsClusters(context.Background(), server.URL+"/public/v1", "service-token", opts)
		})
	if err != nil {
		t.Fatalf("ListAllRealizedSavings() error = %v", err)
	}

	if len(bodies) != 2 || bodies[1]["pageToken"] != "page-2" {
		t.Fatalf("bodies = %v, want a second request with pageToken page-2", bodies)
	}
	for _, body := range bodies {
		if body["startDate"] != "2026-10-01" || body["endDate"] != "2026-11-01" || body["sortBy"] != "totalCost" || body["sortOrder"] != "asc" || body["pageSize"] != float64(500) {
			t.Fatalf("body = %v", body)
		}
	}
	want := RealizedSavingsCluster{
		ClusterUID: "cluster-1", Name: "prod-a", PricingSource: "doit", TransmissionStatus: "in_progress", UpgradeStatus: "none",
		TotalCost: 100.5, RealizedSavings: 25.25, RealizedSavingsPct: 20.08,
		CPUCoreHoursSaved: RealizedSavingsHours{Absolute: 12.5, Pct: 30}, MemoryGBHoursSaved: RealizedSavingsHours{Absolute: 40, Pct: -5},
	}
	if len(page.Rows) != 2 || page.Rows[0] != want || page.Rows[1].TransmissionStatus != "deleted" {
		t.Fatalf("clusters = %+v", page.Rows)
	}
	if page.Pagination.Next != nil {
		t.Fatalf("pagination.next = %v, want nil after the last page", *page.Pagination.Next)
	}
}

const realizedSavingsRowMoney = `"totalCost":{"amount":"100.50","currency":"USD"},"realizedSavings":{"amount":"25.25","currency":"USD"},"realizedSavingsPct":20.08,
	"cpuCoreHoursSaved":{"absolute":12.5,"pct":30},"memoryGbHoursSaved":{"absolute":40,"pct":-5}`

func serveRealizedSavings(t *testing.T, wantPath string, response string, body *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != wantPath {
			t.Fatalf("request = %s %s, want POST %s", r.Method, r.URL.Path, wantPath)
		}
		if err := json.NewDecoder(r.Body).Decode(body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
}

func TestClientListPublicRealizedSavingsNamespaces(t *testing.T) {
	var body map[string]any
	server := serveRealizedSavings(t, "/public/v1/realized-savings/namespaces",
		`{"data":[{"clusterUid":"cluster-1","namespace":"payments",`+realizedSavingsRowMoney+`}],"meta":{"pagination":{"next":null,"prev":null,"pageSize":50}}}`, &body)
	defer server.Close()

	page, err := newTestClient().ListPublicRealizedSavingsNamespaces(context.Background(), server.URL+"/public/v1", "service-token",
		RealizedSavingsListOptions{Range: octoberRange, ClusterUID: "cluster-1", Namespace: "ignored"})
	if err != nil {
		t.Fatalf("ListPublicRealizedSavingsNamespaces() error = %v", err)
	}

	if body["clusterUid"] != "cluster-1" || body["namespace"] != nil || body["startDate"] != "2026-10-01" {
		t.Fatalf("body = %v", body)
	}
	want := RealizedSavingsNamespace{
		ClusterUID: "cluster-1", Namespace: "payments", TotalCost: 100.5, RealizedSavings: 25.25, RealizedSavingsPct: 20.08,
		CPUCoreHoursSaved: RealizedSavingsHours{Absolute: 12.5, Pct: 30}, MemoryGBHoursSaved: RealizedSavingsHours{Absolute: 40, Pct: -5},
	}
	if len(page.Rows) != 1 || page.Rows[0] != want {
		t.Fatalf("rows = %+v", page.Rows)
	}
}

func TestClientListPublicRealizedSavingsWorkloads(t *testing.T) {
	var body map[string]any
	server := serveRealizedSavings(t, "/public/v1/realized-savings/workloads",
		`{"data":[{"clusterUid":"cluster-1","namespace":"payments","id":"payments-deployment-api","name":"api","type":"deployment",`+realizedSavingsRowMoney+`}],
		"meta":{"pagination":{"next":"cursor-2","prev":null,"pageSize":50}}}`, &body)
	defer server.Close()

	page, err := newTestClient().ListPublicRealizedSavingsWorkloads(context.Background(), server.URL+"/public/v1", "service-token",
		RealizedSavingsListOptions{Range: octoberRange, ClusterUID: "cluster-1", Namespace: "payments"})
	if err != nil {
		t.Fatalf("ListPublicRealizedSavingsWorkloads() error = %v", err)
	}

	if body["clusterUid"] != "cluster-1" || body["namespace"] != "payments" || body["endDate"] != "2026-11-01" {
		t.Fatalf("body = %v", body)
	}
	want := RealizedSavingsWorkload{
		ClusterUID: "cluster-1", Namespace: "payments", ID: "payments-deployment-api", Name: "api", Type: "deployment",
		TotalCost: 100.5, RealizedSavings: 25.25, RealizedSavingsPct: 20.08,
		CPUCoreHoursSaved: RealizedSavingsHours{Absolute: 12.5, Pct: 30}, MemoryGBHoursSaved: RealizedSavingsHours{Absolute: 40, Pct: -5},
	}
	if len(page.Rows) != 1 || page.Rows[0] != want || page.Pagination.Next == nil || *page.Pagination.Next != "cursor-2" {
		t.Fatalf("page = %+v", page)
	}
}

func TestClientGetPublicRealizedSavingsWorkloadSummary(t *testing.T) {
	var body map[string]any
	server := serveRealizedSavings(t, "/public/v1/realized-savings/workloads/summary",
		`{"data":{"baselineCost":{"amount":"125.75","currency":"USD"},`+realizedSavingsRowMoney+`}}`, &body)
	defer server.Close()

	got, err := newTestClient().GetPublicRealizedSavingsWorkloadSummary(context.Background(), server.URL+"/public/v1", "service-token",
		"cluster-1", "payments", "payments-deployment-api", octoberRange)
	if err != nil {
		t.Fatalf("GetPublicRealizedSavingsWorkloadSummary() error = %v", err)
	}

	if body["clusterUid"] != "cluster-1" || body["namespace"] != "payments" || body["workloadId"] != "payments-deployment-api" {
		t.Fatalf("body = %v", body)
	}
	want := RealizedSavingsWorkloadSummary{
		TotalCost: 100.5, BaselineCost: 125.75, RealizedSavings: 25.25, RealizedSavingsPct: 20.08,
		CPUCoreHoursSaved: RealizedSavingsHours{Absolute: 12.5, Pct: 30}, MemoryGBHoursSaved: RealizedSavingsHours{Absolute: 40, Pct: -5},
	}
	if got != want {
		t.Fatalf("summary = %+v, want %+v", got, want)
	}
}
