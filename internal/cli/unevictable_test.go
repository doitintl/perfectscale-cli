package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/perfectscale/poc-cli/internal/api"
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

func TestUnevictableMutedResolvesNameFromWorkloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/unevictable-muted-workloads":
			_, _ = w.Write([]byte(`{"data":[{"clusterUid":"cluster-1","id":"backend-deployment-api","createdBy":"a@example.com","createTime":"2026-04-01T00:00:00Z","updateTime":"2026-04-01T00:00:00Z"},{"clusterUid":"cluster-1","id":"gone-deployment-ghost","createdBy":"a@example.com","createTime":"2026-04-01T00:00:00Z","updateTime":"2026-04-01T00:00:00Z"}],"meta":{"pagination":{"next":null,"prev":null,"pageSize":50}}}`))
		case "/public/v1/clusters/cluster-1/workloads":
			_, _ = w.Write([]byte(showWorkloadsListBody))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL, "unevictable", "muted", "-c", "prod-a")
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, "api")
	assertContains(t, output, "backend")
	assertContains(t, output, "gone-deployment-ghost")
	assertContains(t, output, "(workload deleted)")
}

func TestUnevictableMutedJSONResolvesAndLeavesUnresolvedBlank(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/unevictable-muted-workloads":
			_, _ = w.Write([]byte(`{"data":[{"clusterUid":"cluster-1","id":"backend-deployment-api","createdBy":"a@example.com","createTime":"2026-04-01T00:00:00Z","updateTime":"2026-04-01T00:00:00Z"},{"clusterUid":"cluster-1","id":"gone-deployment-ghost","createdBy":"a@example.com","createTime":"2026-04-01T00:00:00Z","updateTime":"2026-04-01T00:00:00Z"}],"meta":{"pagination":{"next":null,"prev":null,"pageSize":50}}}`))
		case "/public/v1/clusters/cluster-1/workloads":
			_, _ = w.Write([]byte(showWorkloadsListBody))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL, "unevictable", "muted", "-c", "prod-a", "-o", "json")
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, `"workload_name": "api"`)
	assertContains(t, output, `"namespace": "backend"`)
	assertContains(t, output, `"id": "gone-deployment-ghost"`)
}

func TestUnevictableMutedFillsPartialFieldsIndependently(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/unevictable-muted-workloads":
			_, _ = w.Write([]byte(`{"data":[{"clusterUid":"cluster-1","id":"backend-deployment-api","workloadName":"api","createdBy":"a@example.com","createTime":"2026-04-01T00:00:00Z","updateTime":"2026-04-01T00:00:00Z"}],"meta":{"pagination":{"next":null,"prev":null,"pageSize":50}}}`))
		case "/public/v1/clusters/cluster-1/workloads":
			_, _ = w.Write([]byte(showWorkloadsListBody))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL, "unevictable", "muted", "-c", "prod-a", "-o", "json")
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, `"workload_name": "api"`)
	assertContains(t, output, `"namespace": "backend"`)
}

func TestUnevictableMutedKeepsKnownNamespaceWhenNameUnresolved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/unevictable-muted-workloads":
			_, _ = w.Write([]byte(`{"data":[{"clusterUid":"cluster-1","id":"gone-deployment-ghost","namespace":"gone","createdBy":"a@example.com","createTime":"2026-04-01T00:00:00Z","updateTime":"2026-04-01T00:00:00Z"}],"meta":{"pagination":{"next":null,"prev":null,"pageSize":50}}}`))
		case "/public/v1/clusters/cluster-1/workloads":
			_, _ = w.Write([]byte(showWorkloadsListBody))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL, "unevictable", "muted", "-c", "prod-a")
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, "gone")
	if strings.Contains(output, "(workload deleted)") {
		t.Fatalf("output discarded a known namespace in favor of the deleted marker: %s", output)
	}
}

func TestUnevictableMutedAmbiguousIDFallsBackToRawID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/unevictable-muted-workloads":
			_, _ = w.Write([]byte(`{"data":[{"clusterUid":"cluster-1","id":"a-b-c-d","createdBy":"a@example.com","createTime":"2026-04-01T00:00:00Z","updateTime":"2026-04-01T00:00:00Z"}],"meta":{"pagination":{"next":null,"prev":null,"pageSize":50}}}`))
		case "/public/v1/clusters/cluster-1/workloads":
			_, _ = w.Write([]byte(`{"data":[
				{"id":"w1","name":"c-d","type":"b","namespace":"a","runningMinutes":1440,"firstSeen":"2026-04-01T00:00:00Z","lastSeen":"2026-04-02T00:00:00Z","replicasCounts":{"maxCount":1,"avgCount":1},"muteStatus":{"isMuted":false},"automationStatus":"Active","costAnalysis":{"past30Days":{"totalCost":1,"wastedCost":0,"costPerHour":0.01},"next30Days":{"potentialSavings":0,"costIncrease":0}}},
				{"id":"w2","name":"d","type":"c","namespace":"a-b","runningMinutes":1440,"firstSeen":"2026-04-01T00:00:00Z","lastSeen":"2026-04-02T00:00:00Z","replicasCounts":{"maxCount":1,"avgCount":1},"muteStatus":{"isMuted":false},"automationStatus":"Active","costAnalysis":{"past30Days":{"totalCost":1,"wastedCost":0,"costPerHour":0.01},"next30Days":{"potentialSavings":0,"costIncrease":0}}}
			]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL, "unevictable", "muted", "-c", "prod-a")
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, "a-b-c-d")
	assertContains(t, output, "(ambiguous match)")
}

func TestUnevictableMutedLookupFailureDoesNotClaimDeletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/unevictable-muted-workloads":
			_, _ = w.Write([]byte(`{"data":[{"clusterUid":"cluster-1","id":"backend-deployment-api","createdBy":"a@example.com","createTime":"2026-04-01T00:00:00Z","updateTime":"2026-04-01T00:00:00Z"}],"meta":{"pagination":{"next":null,"prev":null,"pageSize":50}}}`))
		case "/public/v1/clusters/cluster-1/workloads":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runNodegroupsCLI(t, server.URL, "unevictable", "muted", "-c", "prod-a")
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, output)
	}
	assertContains(t, output, "backend-deployment-api")
	if strings.Contains(output, "(workload deleted)") {
		t.Fatalf("output falsely claims deletion when the workloads lookup itself failed: %s", output)
	}
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

func TestFilterUnevictablePods(t *testing.T) {
	t.Parallel()

	pod := func(name, ns, ng string, codes ...string) api.UnevictablePod {
		p := api.UnevictablePod{Name: name, Namespace: ns, Spec: api.UnevictablePodSpec{NodeGroup: ng}}
		for _, c := range codes {
			p.Reasons = append(p.Reasons, api.UnevictableReason{ReasonCode: c})
		}
		return p
	}
	pods := []api.UnevictablePod{
		pod("a", "kube-system", "ng1", "pod_disruption_budget"),
		pod("b", "payments", "ng2", "pod_disruption_budget", "strict_affinity"),
		pod("c", "web", "ng3", "topology_locked"),
		{Name: "d", Namespace: "web", Reasons: []api.UnevictableReason{{Reason: "Custom"}}},
	}

	tests := []struct {
		name     string
		excludes unevictableExcludes
		want     []string
	}{
		{"no_excludes_passthrough", unevictableExcludes{}, []string{"a", "b", "c", "d"}},
		{"exclude_namespace", unevictableExcludes{namespaces: map[string]bool{"kube-system": true}}, []string{"b", "c", "d"}},
		{"exclude_node_group_uses_spec_node_group", unevictableExcludes{nodeGroups: map[string]bool{"ng3": true}}, []string{"a", "b", "d"}},
		{"exclude_reason_any_match_drops_pod", unevictableExcludes{reasons: map[string]bool{"strict_affinity": true}}, []string{"a", "c", "d"}},
		{"exclude_reason_ignores_empty_reason_code", unevictableExcludes{reasons: map[string]bool{"": true, "Custom": true}}, []string{"a", "b", "c", "d"}},
		{"multiple_values_in_one_flag", unevictableExcludes{namespaces: map[string]bool{"kube-system": true, "web": true}}, []string{"b"}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, p := range filterUnevictablePods(pods, tc.excludes) {
				got = append(got, p.Name)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFilterUnevictableReportRows(t *testing.T) {
	t.Parallel()

	rows := []api.UnevictableReportRow{
		{Name: "a", NodeGroup: "spot-a", Reasons: []api.UnevictableReason{{ReasonCode: "strict_affinity"}}},
		{Name: "b", NodeGroup: "od", Reasons: []api.UnevictableReason{{ReasonCode: "topology_locked"}}},
		{Name: "c", NodeGroup: "od", Reasons: []api.UnevictableReason{{ReasonCode: "strict_affinity"}}},
	}
	tests := []struct {
		name     string
		excludes unevictableExcludes
		want     string
	}{
		{"exclude_node_group", unevictableExcludes{nodeGroups: map[string]bool{"spot-a": true}}, "b,c"},
		{"exclude_reason", unevictableExcludes{reasons: map[string]bool{"strict_affinity": true}}, "b"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, r := range filterUnevictableReportRows(rows, tc.excludes) {
				got = append(got, r.Name)
			}
			if strings.Join(got, ",") != tc.want {
				t.Fatalf("got %v, want %s", got, tc.want)
			}
		})
	}
}

func TestUnevictableListRejectsInvalidExReason(t *testing.T) {
	_, err := runNodegroupsCLI(t, "https://example.invalid", "unevictable", "list", "-c", "prod-a", "--ex-reason", "bogus")
	if err == nil {
		t.Fatal("runNodegroupsCLI() error = nil, want error for invalid --ex-reason")
	}
	assertContains(t, err.Error(), `unsupported --ex-reason "bogus"`)
}

func TestUnevictableReportRejectsInvalidExReason(t *testing.T) {
	_, err := runNodegroupsCLI(t, "https://example.invalid", "unevictable", "report", "-c", "prod-a", "--ex-reason", "bogus")
	if err == nil {
		t.Fatal("runNodegroupsCLI() error = nil, want error for invalid --ex-reason")
	}
	assertContains(t, err.Error(), `unsupported --ex-reason "bogus"`)
}

func TestUnevictableListRejectsInvalidReason(t *testing.T) {
	_, err := runNodegroupsCLI(t, "https://example.invalid", "unevictable", "list", "-c", "prod-a", "--reason", "bogus")
	if err == nil {
		t.Fatal("runNodegroupsCLI() error = nil, want error for invalid --reason")
	}
	assertContains(t, err.Error(), `unsupported --reason "bogus"`)
}

func TestUnevictableListReasonCommaSeparated(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/unevictable-pods":
			query = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"data":[],` + unevictableSnapshotMeta + `}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := runNodegroupsCLI(t, server.URL, "unevictable", "list", "-c", "prod-a", "--reason", "pod_disruption_budget,strict_affinity")
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v", err)
	}
	assertContains(t, query, "reasonCode%3Apod_disruption_budget%2Cstrict_affinity")

	_, err = runNodegroupsCLI(t, server.URL, "unevictable", "list", "-c", "prod-a", "--reason", "strict_affinity,bogus")
	if err == nil {
		t.Fatal("runNodegroupsCLI() error = nil, want error for invalid part")
	}
	assertContains(t, err.Error(), `unsupported --reason "bogus"`)
}

const unevictableSnapshotMeta = `"meta":{"pagination":{"next":null,"prev":null,"pageSize":50},"snapshotTime":"2026-04-01T00:00:00Z","algorithmVersion":"v1","summary":{"totalPods":3,"unevictablePods":3,"mute":0,"totalNodes":1}}`

func TestUnevictableListExcludeFiltersClientSide(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/unevictable-pods":
			query = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"data":[
{"name":"pod-kube","namespace":"kube-system","reasons":[{"reason":"PDB","reasonCode":"strict_affinity"}],"spec":{"nodeGroup":"ng"}},
{"name":"pod-pdb","namespace":"payments","reasons":[{"reason":"PDB","reasonCode":"pod_disruption_budget"}],"spec":{"nodeGroup":"ng"}},
{"name":"pod-keep","namespace":"payments","reasons":[{"reason":"Affinity","reasonCode":"strict_affinity"}],"spec":{"nodeGroup":"ng"}}
],` + unevictableSnapshotMeta + `}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	out, err := runNodegroupsCLI(t, server.URL, "unevictable", "list", "-c", "prod-a", "-o", "jsonl",
		"--ex-namespace", "kube-system", "--ex-reason", "pod_disruption_budget")
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, out)
	}
	if got := mustParseQuery(t, query).Get("filter"); got != "" {
		t.Fatalf("filter query = %q, want none", got)
	}
	assertContains(t, out, "pod-keep")
	if strings.Contains(out, "pod-kube") || strings.Contains(out, "pod-pdb") {
		t.Fatalf("excluded pods present in output: %s", out)
	}
}

func TestUnevictableReportExcludeFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/public/v1/clusters":
			_, _ = w.Write([]byte(clustersListBody))
		case "/public/v1/clusters/cluster-1/unevictable-report":
			_, _ = w.Write([]byte(`{"data":[
{"name":"row-a","namespace":"n","nodeGroup":"spot-a","reasons":[{"reason":"x","reasonCode":"strict_affinity"}]},
{"name":"row-b","namespace":"n","nodeGroup":"spot-b","reasons":[{"reason":"x","reasonCode":"strict_affinity"}]},
{"name":"row-c","namespace":"n","nodeGroup":"od","reasons":[{"reason":"x","reasonCode":"topology_locked"}]},
{"name":"row-keep","namespace":"n","nodeGroup":"od","reasons":[{"reason":"x","reasonCode":"strict_affinity"}]}
],` + unevictableSnapshotMeta + `}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	out, err := runNodegroupsCLI(t, server.URL, "unevictable", "report", "-c", "prod-a",
		"--ex-node-group", "spot-a,spot-b", "--ex-reason", "topology_locked")
	if err != nil {
		t.Fatalf("runNodegroupsCLI() error = %v; output=%s", err, out)
	}
	assertContains(t, out, "row-keep")
	assertContains(t, out, "excluded client-side")
	for _, gone := range []string{"row-a", "row-b", "row-c"} {
		if strings.Contains(out, gone) {
			t.Fatalf("%s should be excluded: %s", gone, out)
		}
	}
}
