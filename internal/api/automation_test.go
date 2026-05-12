package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const sampleAutomationLogResponse = `{
  "data": [
    {
      "started_at": "2026-04-15T12:00:00Z",
      "cluster_uid": "cluster-1",
      "cluster_name": "prod-a",
      "workload_id": "wl-1",
      "workload_name": "api",
      "workload_type": "Deployment",
      "namespace": "backend",
      "labels": {"app":"api"},
      "executed": "regular-eviction",
      "container": {
        "name": "api",
        "cpu": {"cpuCoresRequest":2,"recommendCpuCoresRequest":1,"cpuCoresLimits":4,"recommendCpuCoresLimits":2,"cpuRequestImpact":-1,"cpuLimitImpact":-2,"cpuRequestChangePercent":-50,"cpuLimitChangePercent":-50,"cpuRequestChangeAbsolute":-1,"cpuLimitChangeAbsolute":-2},
        "memory": {"memMiBRequest":512,"recommendMemMiBRequest":256,"memMiBLimits":1024,"recommendMemMiBLimits":512,"memMiBRequestImpact":-256,"memMiBLimitImpact":-512,"memRequestChangePercent":-50,"memLimitChangePercent":-50,"memMiBRequestChangeAbsolute":-256,"memMiBLimitChangeAbsolute":-512}
      }
    }
  ],
  "meta": {
    "pagination": {"has_next": true, "next": "CURSOR_NEXT", "has_prev": false, "prev": null, "page_size": 1000}
  }
}`

func TestClientListAutomationAuditLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/public/v1/automation/audit_logs" {
			t.Fatalf("path = %s, want /public/v1/automation/audit_logs", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer service-token" {
			t.Fatalf("authorization = %q, want Bearer service-token", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if parsed["page_size"].(float64) != 250 {
			t.Fatalf("page_size = %v, want 250", parsed["page_size"])
		}
		uids, ok := parsed["cluster_uids"].([]any)
		if !ok || len(uids) != 1 || uids[0] != "cluster-1" {
			t.Fatalf("cluster_uids = %v, want [cluster-1]", parsed["cluster_uids"])
		}
		ns, ok := parsed["namespaces"].([]any)
		if !ok || len(ns) != 2 {
			t.Fatalf("namespaces = %v, want 2 entries", parsed["namespaces"])
		}
		if parsed["from"] == nil {
			t.Fatalf("from = nil, want timestamp")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleAutomationLogResponse))
	}))
	defer server.Close()

	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	pageSize := 250
	client := NewClient()
	page, err := client.ListAutomationAuditLogs(context.Background(), server.URL+"/public/v1", "service-token", AutomationAuditLogsInput{
		From:        &from,
		PageSize:    &pageSize,
		ClusterUIDs: []string{"cluster-1"},
		Namespaces:  []string{"backend", "frontend"},
	})
	if err != nil {
		t.Fatalf("ListAutomationAuditLogs() error = %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(page.Entries))
	}
	got := page.Entries[0]
	if got.ClusterUID != "cluster-1" || got.WorkloadName != "api" || got.Executed != "regular-eviction" {
		t.Fatalf("entry mapping wrong: %+v", got)
	}
	if got.Container.CPU.CPURequestChangePercent != -50 {
		t.Fatalf("cpu pct = %v, want -50", got.Container.CPU.CPURequestChangePercent)
	}
	if got.Container.Memory.MemMiBRequestChangeAbsolute != -256 {
		t.Fatalf("mem abs = %v, want -256", got.Container.Memory.MemMiBRequestChangeAbsolute)
	}
	if !page.Pagination.HasNext || page.Pagination.Next != "CURSOR_NEXT" {
		t.Fatalf("pagination = %+v, want HasNext=true Next=CURSOR_NEXT", page.Pagination)
	}
	if page.Pagination.PageSize != 1000 {
		t.Fatalf("pagination.PageSize = %d, want 1000", page.Pagination.PageSize)
	}
}

func TestClientListAllAutomationAuditLogsFollowsCursors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			if parsed["after"] != nil {
				t.Fatalf("first call should not include after, got %v", parsed["after"])
			}
			_, _ = w.Write([]byte(`{"data":[{"started_at":"2026-04-15T12:00:00Z","cluster_uid":"cluster-1","cluster_name":"prod-a","workload_id":"wl-1","workload_name":"api","workload_type":"Deployment","namespace":"backend","labels":{},"executed":"regular-eviction","container":{"name":"api","cpu":{"cpuCoresRequest":0,"recommendCpuCoresRequest":0,"cpuCoresLimits":0,"recommendCpuCoresLimits":0,"cpuRequestImpact":0,"cpuLimitImpact":0,"cpuRequestChangePercent":0,"cpuLimitChangePercent":0,"cpuRequestChangeAbsolute":0,"cpuLimitChangeAbsolute":0},"memory":{"memMiBRequest":0,"recommendMemMiBRequest":0,"memMiBLimits":0,"recommendMemMiBLimits":0,"memMiBRequestImpact":0,"memMiBLimitImpact":0,"memRequestChangePercent":0,"memLimitChangePercent":0,"memMiBRequestChangeAbsolute":0,"memMiBLimitChangeAbsolute":0}}}],"meta":{"pagination":{"has_next":true,"next":"CURSOR_2","has_prev":false,"prev":null,"page_size":1}}}`))
		case 2:
			if parsed["after"] != "CURSOR_2" {
				t.Fatalf("second call after = %v, want CURSOR_2", parsed["after"])
			}
			_, _ = w.Write([]byte(`{"data":[{"started_at":"2026-04-15T12:01:00Z","cluster_uid":"cluster-1","cluster_name":"prod-a","workload_id":"wl-2","workload_name":"web","workload_type":"Deployment","namespace":"frontend","labels":{},"executed":"inplace-resize","container":{"name":"web","cpu":{"cpuCoresRequest":0,"recommendCpuCoresRequest":0,"cpuCoresLimits":0,"recommendCpuCoresLimits":0,"cpuRequestImpact":0,"cpuLimitImpact":0,"cpuRequestChangePercent":0,"cpuLimitChangePercent":0,"cpuRequestChangeAbsolute":0,"cpuLimitChangeAbsolute":0},"memory":{"memMiBRequest":0,"recommendMemMiBRequest":0,"memMiBLimits":0,"recommendMemMiBLimits":0,"memMiBRequestImpact":0,"memMiBLimitImpact":0,"memRequestChangePercent":0,"memLimitChangePercent":0,"memMiBRequestChangeAbsolute":0,"memMiBLimitChangeAbsolute":0}}}],"meta":{"pagination":{"has_next":false,"next":null,"has_prev":true,"prev":"CURSOR_1","page_size":1}}}`))
		default:
			t.Fatalf("unexpected extra call %d", calls)
		}
	}))
	defer server.Close()

	client := NewClient()
	entries, pagination, err := client.ListAllAutomationAuditLogs(context.Background(), server.URL+"/public/v1", "service-token", AutomationAuditLogsInput{}, 0)
	if err != nil {
		t.Fatalf("ListAllAutomationAuditLogs() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].WorkloadName != "api" || entries[1].WorkloadName != "web" {
		t.Fatalf("entries order wrong: %v / %v", entries[0].WorkloadName, entries[1].WorkloadName)
	}
	if pagination.HasNext {
		t.Fatalf("final pagination HasNext = true, want false")
	}
}
