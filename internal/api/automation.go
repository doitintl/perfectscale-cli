package api

import (
	"context"
	"fmt"
	"time"

	"github.com/perfectscale/poc-cli/internal/publicapi"
)

// AutomationAuditLogsInput captures every input the public AutomationAuditLogs endpoint accepts.
//
// All fields are optional; the server applies defaults when fields are not set:
//   - From defaults to 00:00 UTC of 30 days ago.
//   - To defaults to "now".
//   - PageSize defaults to 1000 (max 5000).
type AutomationAuditLogsInput struct {
	From        *time.Time
	To          *time.Time
	PageSize    *int
	After       *string
	Before      *string
	ClusterUIDs []string
	Namespaces  []string
}

// AutomationLogPagination is the CLI-side mirror of publicapi.Pagination, with
// pointer-string cursors flattened to plain strings for easier consumption.
type AutomationLogPagination struct {
	HasNext  bool   `json:"has_next"`
	Next     string `json:"next,omitempty"`
	HasPrev  bool   `json:"has_prev"`
	Prev     string `json:"prev,omitempty"`
	PageSize int    `json:"page_size"`
}

// AutomationLogContainerCPU holds CPU-related deltas for a single container.
type AutomationLogContainerCPU struct {
	CPUCoresRequest          int64   `json:"cpu_cores_request"`
	RecommendCPUCoresRequest int64   `json:"recommend_cpu_cores_request"`
	CPUCoresLimits           int64   `json:"cpu_cores_limits"`
	RecommendCPUCoresLimits  int64   `json:"recommend_cpu_cores_limits"`
	CPURequestImpact         int64   `json:"cpu_request_impact"`
	CPULimitImpact           int64   `json:"cpu_limit_impact"`
	CPURequestChangePercent  float64 `json:"cpu_request_change_percent"`
	CPULimitChangePercent    float64 `json:"cpu_limit_change_percent"`
	CPURequestChangeAbsolute int64   `json:"cpu_request_change_absolute"`
	CPULimitChangeAbsolute   int64   `json:"cpu_limit_change_absolute"`
}

// AutomationLogContainerMemory holds memory-related deltas for a single container.
type AutomationLogContainerMemory struct {
	MemMiBRequest               int64   `json:"mem_mib_request"`
	RecommendMemMiBRequest      int64   `json:"recommend_mem_mib_request"`
	MemMiBLimits                int64   `json:"mem_mib_limits"`
	RecommendMemMiBLimits       int64   `json:"recommend_mem_mib_limits"`
	MemMiBRequestImpact         int64   `json:"mem_mib_request_impact"`
	MemMiBLimitImpact           int64   `json:"mem_mib_limit_impact"`
	MemRequestChangePercent     float64 `json:"mem_request_change_percent"`
	MemLimitChangePercent       float64 `json:"mem_limit_change_percent"`
	MemMiBRequestChangeAbsolute int64   `json:"mem_mib_request_change_absolute"`
	MemMiBLimitChangeAbsolute   int64   `json:"mem_mib_limit_change_absolute"`
}

// AutomationLogContainer captures the container-level slice of an audit log entry.
type AutomationLogContainer struct {
	Name   string                       `json:"name"`
	CPU    AutomationLogContainerCPU    `json:"cpu"`
	Memory AutomationLogContainerMemory `json:"memory"`
}

// AutomationLogEntry is the CLI-friendly mapping of a single audit log entry.
type AutomationLogEntry struct {
	StartedAt    time.Time              `json:"started_at"`
	ClusterUID   string                 `json:"cluster_uid"`
	ClusterName  string                 `json:"cluster_name"`
	Namespace    string                 `json:"namespace"`
	WorkloadID   string                 `json:"workload_id"`
	WorkloadName string                 `json:"workload_name"`
	WorkloadType string                 `json:"workload_type"`
	Executed     string                 `json:"executed"`
	Labels       map[string]string      `json:"labels,omitempty"`
	Container    AutomationLogContainer `json:"container"`
}

// AutomationLogPage is a single page of audit log entries plus pagination metadata.
type AutomationLogPage struct {
	Entries    []AutomationLogEntry    `json:"entries"`
	Pagination AutomationLogPagination `json:"pagination"`
}

// ListAutomationAuditLogs fetches a single page of automation audit log entries.
//
// Use Pagination.Next as input.After on the next call to traverse forward, or
// Pagination.Prev as input.Before to go back. ListAllAutomationAuditLogs is the
// auto-paginating convenience wrapper.
func (c *Client) ListAutomationAuditLogs(ctx context.Context, publicAPIBaseURL string, token string, input AutomationAuditLogsInput) (AutomationLogPage, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return AutomationLogPage{}, err
	}

	body := publicapi.AutomationAuditLogsJSONRequestBody{
		From:     input.From,
		To:       input.To,
		PageSize: input.PageSize,
		After:    input.After,
		Before:   input.Before,
	}
	if len(input.ClusterUIDs) > 0 {
		uids := append([]string(nil), input.ClusterUIDs...)
		body.ClusterUids = &uids
	}
	if len(input.Namespaces) > 0 {
		ns := append([]string(nil), input.Namespaces...)
		body.Namespaces = &ns
	}

	res, err := client.AutomationAuditLogsWithResponse(ctx, body)
	if err != nil {
		return AutomationLogPage{}, fmt.Errorf("list automation audit logs: %w", err)
	}
	if res.JSON200 == nil {
		return AutomationLogPage{}, unexpectedPublicAPIResponse("list automation audit logs", res.StatusCode(), res.Body)
	}

	entries := make([]AutomationLogEntry, 0, len(res.JSON200.Data))
	for _, item := range res.JSON200.Data {
		entries = append(entries, toAutomationLogEntry(item))
	}

	return AutomationLogPage{
		Entries:    entries,
		Pagination: toAutomationLogPagination(res.JSON200.Meta.Pagination),
	}, nil
}

// ListAllAutomationAuditLogs walks `next` cursors until the server reports
// has_next=false. The pageCap bounds the maximum number of pages fetched as a
// safety net (set <=0 to use the default of 50).
func (c *Client) ListAllAutomationAuditLogs(ctx context.Context, publicAPIBaseURL string, token string, input AutomationAuditLogsInput, pageCap int) ([]AutomationLogEntry, AutomationLogPagination, error) {
	if pageCap <= 0 {
		pageCap = 50
	}

	var (
		all      []AutomationLogEntry
		lastPage AutomationLogPagination
	)

	cursor := input.After
	for page := 0; page < pageCap; page++ {
		next := input
		next.Before = nil
		next.After = cursor

		current, err := c.ListAutomationAuditLogs(ctx, publicAPIBaseURL, token, next)
		if err != nil {
			return nil, lastPage, err
		}

		all = append(all, current.Entries...)
		lastPage = current.Pagination
		if !current.Pagination.HasNext || current.Pagination.Next == "" {
			return all, lastPage, nil
		}
		token := current.Pagination.Next
		cursor = &token
	}

	return all, lastPage, fmt.Errorf("automation audit logs page cap %d reached; refine --from/--to or pass --page-cap to lift the limit", pageCap)
}

func toAutomationLogPagination(item publicapi.Pagination) AutomationLogPagination {
	out := AutomationLogPagination{
		HasNext:  item.HasNext,
		HasPrev:  item.HasPrev,
		PageSize: item.PageSize,
	}
	if item.Next != nil {
		out.Next = *item.Next
	}
	if item.Prev != nil {
		out.Prev = *item.Prev
	}
	return out
}

func toAutomationLogEntry(item publicapi.AutomationLogEntry) AutomationLogEntry {
	return AutomationLogEntry{
		StartedAt:    item.StartedAt,
		ClusterUID:   item.ClusterUid,
		ClusterName:  item.ClusterName,
		Namespace:    item.Namespace,
		WorkloadID:   item.WorkloadId,
		WorkloadName: item.WorkloadName,
		WorkloadType: item.WorkloadType,
		Executed:     string(item.Executed),
		Labels:       item.Labels,
		Container: AutomationLogContainer{
			Name:   item.Container.Name,
			CPU:    toAutomationLogContainerCPU(item.Container.Cpu),
			Memory: toAutomationLogContainerMemory(item.Container.Memory),
		},
	}
}

func toAutomationLogContainerCPU(item publicapi.AutomatedLogsContainerCpu) AutomationLogContainerCPU {
	return AutomationLogContainerCPU{
		CPUCoresRequest:          item.CpuCoresRequest,
		RecommendCPUCoresRequest: item.RecommendCpuCoresRequest,
		CPUCoresLimits:           item.CpuCoresLimits,
		RecommendCPUCoresLimits:  item.RecommendCpuCoresLimits,
		CPURequestImpact:         item.CpuRequestImpact,
		CPULimitImpact:           item.CpuLimitImpact,
		CPURequestChangePercent:  item.CpuRequestChangePercent,
		CPULimitChangePercent:    item.CpuLimitChangePercent,
		CPURequestChangeAbsolute: item.CpuRequestChangeAbsolute,
		CPULimitChangeAbsolute:   item.CpuLimitChangeAbsolute,
	}
}

func toAutomationLogContainerMemory(item publicapi.AutomatedLogsContainerMemory) AutomationLogContainerMemory {
	return AutomationLogContainerMemory{
		MemMiBRequest:               item.MemMiBRequest,
		RecommendMemMiBRequest:      item.RecommendMemMiBRequest,
		MemMiBLimits:                item.MemMiBLimits,
		RecommendMemMiBLimits:       item.RecommendMemMiBLimits,
		MemMiBRequestImpact:         item.MemMiBRequestImpact,
		MemMiBLimitImpact:           item.MemMiBLimitImpact,
		MemRequestChangePercent:     item.MemRequestChangePercent,
		MemLimitChangePercent:       item.MemLimitChangePercent,
		MemMiBRequestChangeAbsolute: item.MemMiBRequestChangeAbsolute,
		MemMiBLimitChangeAbsolute:   item.MemMiBLimitChangeAbsolute,
	}
}
