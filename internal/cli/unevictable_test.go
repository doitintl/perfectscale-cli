package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnevictableListWiresServerSideFilters(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/unevictable-pods":
			query = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"next":null,"prev":null,"pageSize":50},"snapshotTime":"2026-04-01T00:00:00Z","algorithmVersion":"v1","summary":{"totalPods":0,"unevictablePods":0,"mute":0,"totalNodes":0}}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL,
		"unevictable", "list",
		"-c", "prod-a",
		"-n", "payments",
		"--reason", "pod_disruption_budget",
		"-g", "spot-a",
		"-C", "5",
		"--mute", "include",
		"-s", "blockedCostHourly",
		"-r", "asc",
	)
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, output)
	}

	got := mustParseQuery(t, query)
	if got := got.Get("filter"); got != "namespace:payments|reasonCode:pod_disruption_budget|nodeGroup:spot-a|blockedCostHourly:gte:5" {
		t.Fatalf("filter query = %q, want combined DSL clauses", got)
	}
	if v := got.Get("mute"); v != "include" {
		t.Fatalf("mute query = %q, want include", v)
	}
	if v := got.Get("sortBy"); v != "blockedCostHourly" {
		t.Fatalf("sortBy query = %q, want blockedCostHourly", v)
	}
	if v := got.Get("sortOrder"); v != "asc" {
		t.Fatalf("sortOrder query = %q, want asc", v)
	}
}

func TestUnevictableListRejectsInvalidSort(t *testing.T) {
	_, err := runNodegroupsCLI(t, "https://example.invalid", "unevictable", "list", "-c", "prod-a", "-s", "cost")
	if err == nil {
		t.Fatal("runNodegroupsCLI() error = nil, want error for invalid --sort")
	}
	assertContains(t, err.Error(), "--sort must be")
}

func TestUnevictableListRejectsInvalidMute(t *testing.T) {
	_, err := runNodegroupsCLI(t, "https://example.invalid", "unevictable", "list", "-c", "prod-a", "--mute", "bogus")
	if err == nil {
		t.Fatal("runNodegroupsCLI() error = nil, want error for invalid --mute")
	}
	assertContains(t, err.Error(), "--mute must be one of")
}

func TestUnevictableReportHasNoReasonFlag(t *testing.T) {
	// "report"'s filter schema doesn't support reasonCode, so the flag isn't
	// registered at all — urfave/cli rejects it at parse time.
	_, err := runNodegroupsCLI(t, "https://example.invalid", "unevictable", "report", "-c", "prod-a", "--reason", "pod_disruption_budget")
	if err == nil {
		t.Fatal("runNodegroupsCLI() error = nil, want error since report doesn't support --reason")
	}
	assertContains(t, err.Error(), "flag provided but not defined")
}

func TestUnevictableListSnapshotProcessingSurfacesClearMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/unevictable-pods":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"data":{"status":"processing"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := runNodegroupsCLI(t, server.URL, "unevictable", "list", "-c", "prod-a")
	if err == nil {
		t.Fatal("runNodegroupsCLI() error = nil, want a snapshot-processing error")
	}
	assertContains(t, err.Error(), "still processing")
}

func TestUnevictableShowRequiresID(t *testing.T) {
	_, err := runNodegroupsCLI(t, "https://example.invalid", "unevictable", "show", "-c", "prod-a")
	if err == nil {
		t.Fatal("runNodegroupsCLI() error = nil, want error for missing --id")
	}
}

func TestUnevictableMutedIsReadOnlyHelp(t *testing.T) {
	output, err := runCLI(t, nil, "unevictable", "muted", "--help")
	if err != nil {
		t.Fatalf("runCLI(unevictable muted --help) error = %v", err)
	}
	assertContains(t, output, "Read-only")
	assertContains(t, output, "--page-size value")
	assertContains(t, output, "--all")
}

func TestUnevictableHelpListsAllSubcommands(t *testing.T) {
	output, err := runCLI(t, nil, "unevictable", "--help")
	if err != nil {
		t.Fatalf("runCLI(unevictable --help) error = %v", err)
	}
	assertContains(t, output, "list")
	assertContains(t, output, "report")
	assertContains(t, output, "show")
	assertContains(t, output, "muted")
}
