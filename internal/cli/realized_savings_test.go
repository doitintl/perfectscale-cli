package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const realizedSavingsSummaryResponse = `{"data":{
	"totalCost":{"amount":"1200.00","currency":"USD"},"baselineCost":{"amount":"1500.00","currency":"USD"},
	"realizedSavings":{"amount":"300.00","currency":"USD"},"realizedSavingsPct":20,
	"automatedCost":{"amount":"800.00","currency":"USD"},"notAutomatedCost":{"amount":"400.00","currency":"USD"},
	"resiliencyIssuesAddressed":7,
	"cluster":{"clusterUid":"cluster-1","name":"prod-a","cloud":"aws","region":"us-east-1","pricingSource":"doit"}}}`

const realizedSavingsClustersResponse = `{"data":[{"clusterUid":"cluster-1","name":"prod-a","pricingSource":"doit","transmissionStatus":"in_progress","upgradeStatus":"none",
	"totalCost":{"amount":"100.50","currency":"USD"},"realizedSavings":{"amount":"25.25","currency":"USD"},"realizedSavingsPct":20.08,
	"cpuCoreHoursSaved":{"absolute":12.5,"pct":30},"memoryGbHoursSaved":{"absolute":40,"pct":-5}}],
	"meta":{"pagination":{"next":"opaque-next","prev":null,"pageSize":25}}}`

func TestRealizedSavingsSummaryResolvesClusterName(t *testing.T) {
	var body map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/realized-savings/clusters/summary":
			_ = json.NewDecoder(r.Body).Decode(&body)
			_, _ = w.Write([]byte(realizedSavingsSummaryResponse))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL, "realized-savings", "summary", "-c", "prod-a", "--from", "2026-10-01", "--to", "2026-11-01")
	if err != nil {
		t.Fatalf("realized-savings summary error = %v; output=%s", err, output)
	}

	if body["clusterUid"] != "cluster-1" || body["startDate"] != "2026-10-01" || body["endDate"] != "2026-11-01" {
		t.Fatalf("body = %v", body)
	}
	for _, want := range []string{"cluster_uid", "cluster-1", "realized_savings_usd", "300.00", "not_automated_cost_usd", "400.00"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestRealizedSavingsSummaryDefaultsToLast30DaysForTheTenant(t *testing.T) {
	var body map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public/v1/realized-savings/summary" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realizedSavingsSummaryResponse))
	}))
	defer server.Close()

	if output, err := runNodegroupsCLI(t, server.URL, "realized-savings", "summary", "-o", "json"); err != nil {
		t.Fatalf("realized-savings summary error = %v; output=%s", err, output)
	}

	start, startErr := time.Parse(time.DateOnly, body["startDate"])
	end, endErr := time.Parse(time.DateOnly, body["endDate"])
	if startErr != nil || endErr != nil {
		t.Fatalf("body = %v", body)
	}
	if !end.After(time.Now().UTC()) || end.Sub(start) != 30*24*time.Hour {
		t.Fatalf("range = %s..%s, want 30 days ending tomorrow", body["startDate"], body["endDate"])
	}
}

func TestRealizedSavingsClustersWiresFlagsAndKeepsCursor(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public/v1/realized-savings/clusters" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realizedSavingsClustersResponse))
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL, "realized-savings", "clusters",
		"--from", "2026-10-01", "--to", "2026-11-01",
		"-s", "totalCost", "-r", "ASC",
		"--page-size", "25", "--page-token", "opaque-cursor",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("realized-savings clusters error = %v; output=%s", err, output)
	}

	want := map[string]any{"startDate": "2026-10-01", "endDate": "2026-11-01", "sortBy": "totalCost", "sortOrder": "asc", "pageSize": float64(25), "pageToken": "opaque-cursor"}
	for key, value := range want {
		if body[key] != value {
			t.Fatalf("body[%s] = %v, want %v (body=%v)", key, body[key], value, body)
		}
	}
	for _, fragment := range []string{`"from": "2026-10-01"`, `"to": "2026-11-01"`, `"next": "opaque-next"`, `"realized_savings": 25.25`, `"cpu_core_hours_saved"`} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("output missing %s:\n%s", fragment, output)
		}
	}
}

func TestRealizedSavingsRejectsBadFlags(t *testing.T) {
	tests := map[string][]string{
		"unknown_sort":      {"realized-savings", "clusters", "-s", "waste"},
		"bad_order":         {"realized-savings", "clusters", "-r", "up"},
		"oversized_page":    {"realized-savings", "clusters", "--page-size", "501"},
		"bad_date":          {"realized-savings", "summary", "--from", "2026-10-01T00:00:00Z"},
		"to_not_after_from": {"realized-savings", "summary", "--from", "2026-10-01", "--to", "2026-10-01"},
	}

	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				t.Fatalf("unexpected request %s", r.URL.Path)
			}))
			defer server.Close()

			if _, err := runNodegroupsCLI(t, server.URL, args...); err == nil {
				t.Fatalf("expected a usage error for %v", args)
			}
		})
	}
}

func TestRealizedSavingsSummaryJSONEchoesDefaultRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(realizedSavingsSummaryResponse))
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL, "realized-savings", "summary", "-o", "json")
	if err != nil {
		t.Fatalf("realized-savings summary error = %v; output=%s", err, output)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, output)
	}
	if got["to"] != time.Now().UTC().AddDate(0, 0, 1).Format(time.DateOnly) || got["realized_savings"] != float64(300) {
		t.Fatalf("output = %v, want the default range next to the totals", got)
	}
}

func TestRealizedSavingsHelpListsRangeAndPaginationFlags(t *testing.T) {
	output, err := runCLI(t, nil, "realized-savings", "clusters", "--help")
	if err != nil {
		t.Fatalf("runCLI(realized-savings clusters --help) error = %v", err)
	}
	for _, flag := range []string{"--from value", "--to value", "--sort value, -s value", "--order value, -r value", "--page-token value", "--all"} {
		assertContains(t, output, flag)
	}
}
