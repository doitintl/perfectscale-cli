package api

import (
	"context"
	"fmt"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/perfectscale/poc-cli/internal/publicapi"
)

// RealizedSavingsRange is [Start, End) in UTC days, as the API takes it.
type RealizedSavingsRange struct {
	Start time.Time
	End   time.Time
}

// RealizedSavingsListOptions serves all three tables; ClusterUID and Namespace are ignored where a table doesn't take them.
type RealizedSavingsListOptions struct {
	Range      RealizedSavingsRange
	ClusterUID string
	Namespace  string
	SortBy     *string
	SortOrder  *string
	PageSize   *int
	PageToken  *string
}

type RealizedSavingsPage[T any] struct {
	Rows       []T
	Pagination CursorPagination
}

type RealizedSavingsHours struct {
	Absolute float64 `json:"absolute"`
	Pct      int     `json:"pct"`
}

type RealizedSavingsCluster struct {
	ClusterUID         string               `json:"cluster_uid"`
	Name               string               `json:"name"`
	PricingSource      string               `json:"pricing_source"`
	TransmissionStatus string               `json:"transmission_status"`
	UpgradeStatus      string               `json:"upgrade_status"`
	TotalCost          float64              `json:"total_cost"`
	RealizedSavings    float64              `json:"realized_savings"`
	RealizedSavingsPct float64              `json:"realized_savings_pct"`
	CPUCoreHoursSaved  RealizedSavingsHours `json:"cpu_core_hours_saved"`
	MemoryGBHoursSaved RealizedSavingsHours `json:"memory_gb_hours_saved"`
}

type RealizedSavingsNamespace struct {
	ClusterUID         string               `json:"cluster_uid"`
	Namespace          string               `json:"namespace"`
	TotalCost          float64              `json:"total_cost"`
	RealizedSavings    float64              `json:"realized_savings"`
	RealizedSavingsPct float64              `json:"realized_savings_pct"`
	CPUCoreHoursSaved  RealizedSavingsHours `json:"cpu_core_hours_saved"`
	MemoryGBHoursSaved RealizedSavingsHours `json:"memory_gb_hours_saved"`
}

type RealizedSavingsWorkload struct {
	ClusterUID         string               `json:"cluster_uid"`
	Namespace          string               `json:"namespace"`
	ID                 string               `json:"id"`
	Name               string               `json:"name"`
	Type               string               `json:"type"`
	TotalCost          float64              `json:"total_cost"`
	RealizedSavings    float64              `json:"realized_savings"`
	RealizedSavingsPct float64              `json:"realized_savings_pct"`
	CPUCoreHoursSaved  RealizedSavingsHours `json:"cpu_core_hours_saved"`
	MemoryGBHoursSaved RealizedSavingsHours `json:"memory_gb_hours_saved"`
}

type RealizedSavingsSummaryCluster struct {
	ClusterUID    string `json:"cluster_uid"`
	Name          string `json:"name"`
	Cloud         string `json:"cloud,omitempty"`
	Region        string `json:"region,omitempty"`
	PricingSource string `json:"pricing_source"`
}

type RealizedSavingsSummary struct {
	TotalCost                 float64                        `json:"total_cost"`
	BaselineCost              float64                        `json:"baseline_cost"`
	RealizedSavings           float64                        `json:"realized_savings"`
	RealizedSavingsPct        float64                        `json:"realized_savings_pct"`
	AutomatedCost             float64                        `json:"automated_cost"`
	NotAutomatedCost          float64                        `json:"not_automated_cost"`
	ResiliencyIssuesAddressed int64                          `json:"resiliency_issues_addressed"`
	Cluster                   *RealizedSavingsSummaryCluster `json:"cluster,omitempty"`
}

type RealizedSavingsWorkloadSummary struct {
	TotalCost          float64              `json:"total_cost"`
	BaselineCost       float64              `json:"baseline_cost"`
	RealizedSavings    float64              `json:"realized_savings"`
	RealizedSavingsPct float64              `json:"realized_savings_pct"`
	CPUCoreHoursSaved  RealizedSavingsHours `json:"cpu_core_hours_saved"`
	MemoryGBHoursSaved RealizedSavingsHours `json:"memory_gb_hours_saved"`
}

func (c *Client) GetPublicRealizedSavingsTenantSummary(ctx context.Context, publicAPIBaseURL string, token string, dates RealizedSavingsRange) (RealizedSavingsSummary, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return RealizedSavingsSummary{}, err
	}

	res, err := client.GetRealizedSavingsSummaryWithResponse(ctx, publicapi.RealizedSavingsRange{
		StartDate: apiDate(dates.Start), EndDate: apiDate(dates.End),
	})
	if err != nil {
		return RealizedSavingsSummary{}, fmt.Errorf("get realized savings summary: %w", err)
	}
	if res.JSON200 == nil {
		return RealizedSavingsSummary{}, unexpectedPublicAPIResponse("get realized savings summary", res.StatusCode(), res.Body, res.HTTPResponse)
	}
	return toRealizedSavingsSummary(res.JSON200.Data), nil
}

func (c *Client) GetPublicRealizedSavingsClusterSummary(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, dates RealizedSavingsRange) (RealizedSavingsSummary, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return RealizedSavingsSummary{}, err
	}

	res, err := client.GetRealizedSavingsClusterSummaryWithResponse(ctx, publicapi.GetRealizedSavingsClusterSummaryJSONRequestBody{
		ClusterUid: clusterUID, StartDate: apiDate(dates.Start), EndDate: apiDate(dates.End),
	})
	if err != nil {
		return RealizedSavingsSummary{}, fmt.Errorf("get realized savings cluster summary: %w", err)
	}
	if res.JSON200 == nil {
		return RealizedSavingsSummary{}, unexpectedPublicAPIResponse("get realized savings cluster summary", res.StatusCode(), res.Body, res.HTTPResponse)
	}
	return toRealizedSavingsSummary(res.JSON200.Data), nil
}

func (c *Client) GetPublicRealizedSavingsNamespaceSummary(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, namespace string, dates RealizedSavingsRange) (RealizedSavingsSummary, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return RealizedSavingsSummary{}, err
	}

	res, err := client.GetRealizedSavingsNamespaceSummaryWithResponse(ctx, publicapi.GetRealizedSavingsNamespaceSummaryJSONRequestBody{
		ClusterUid: clusterUID, Namespace: namespace, StartDate: apiDate(dates.Start), EndDate: apiDate(dates.End),
	})
	if err != nil {
		return RealizedSavingsSummary{}, fmt.Errorf("get realized savings namespace summary: %w", err)
	}
	if res.JSON200 == nil {
		return RealizedSavingsSummary{}, unexpectedPublicAPIResponse("get realized savings namespace summary", res.StatusCode(), res.Body, res.HTTPResponse)
	}
	return toRealizedSavingsSummary(res.JSON200.Data), nil
}

func (c *Client) GetPublicRealizedSavingsWorkloadSummary(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, namespace string, workloadID string, dates RealizedSavingsRange) (RealizedSavingsWorkloadSummary, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return RealizedSavingsWorkloadSummary{}, err
	}

	res, err := client.GetRealizedSavingsWorkloadSummaryWithResponse(ctx, publicapi.GetRealizedSavingsWorkloadSummaryJSONRequestBody{
		ClusterUid: clusterUID, Namespace: namespace, WorkloadId: workloadID, StartDate: apiDate(dates.Start), EndDate: apiDate(dates.End),
	})
	if err != nil {
		return RealizedSavingsWorkloadSummary{}, fmt.Errorf("get realized savings workload summary: %w", err)
	}
	if res.JSON200 == nil {
		return RealizedSavingsWorkloadSummary{}, unexpectedPublicAPIResponse("get realized savings workload summary", res.StatusCode(), res.Body, res.HTTPResponse)
	}

	item := res.JSON200.Data
	return RealizedSavingsWorkloadSummary{
		TotalCost:          parseMoney(item.TotalCost),
		BaselineCost:       parseMoney(item.BaselineCost),
		RealizedSavings:    parseMoney(item.RealizedSavings),
		RealizedSavingsPct: item.RealizedSavingsPct,
		CPUCoreHoursSaved:  RealizedSavingsHours(item.CpuCoreHoursSaved),
		MemoryGBHoursSaved: RealizedSavingsHours(item.MemoryGbHoursSaved),
	}, nil
}

func (c *Client) ListPublicRealizedSavingsClusters(ctx context.Context, publicAPIBaseURL string, token string, opts RealizedSavingsListOptions) (RealizedSavingsPage[RealizedSavingsCluster], error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return RealizedSavingsPage[RealizedSavingsCluster]{}, err
	}

	res, err := client.ListRealizedSavingsClustersWithResponse(ctx, publicapi.ListRealizedSavingsClustersJSONRequestBody{
		StartDate: apiDate(opts.Range.Start), EndDate: apiDate(opts.Range.End),
		SortBy: opts.SortBy, SortOrder: opts.SortOrder, PageSize: opts.PageSize, PageToken: opts.PageToken,
	})
	if err != nil {
		return RealizedSavingsPage[RealizedSavingsCluster]{}, fmt.Errorf("list realized savings clusters: %w", err)
	}
	if res.JSON200 == nil {
		return RealizedSavingsPage[RealizedSavingsCluster]{}, unexpectedPublicAPIResponse("list realized savings clusters", res.StatusCode(), res.Body, res.HTTPResponse)
	}

	rows := make([]RealizedSavingsCluster, 0, len(res.JSON200.Data))
	for _, item := range res.JSON200.Data {
		rows = append(rows, RealizedSavingsCluster{
			ClusterUID:         item.ClusterUid,
			Name:               item.Name,
			PricingSource:      item.PricingSource,
			TransmissionStatus: item.TransmissionStatus,
			UpgradeStatus:      item.UpgradeStatus,
			TotalCost:          parseMoney(item.TotalCost),
			RealizedSavings:    parseMoney(item.RealizedSavings),
			RealizedSavingsPct: item.RealizedSavingsPct,
			CPUCoreHoursSaved:  RealizedSavingsHours(item.CpuCoreHoursSaved),
			MemoryGBHoursSaved: RealizedSavingsHours(item.MemoryGbHoursSaved),
		})
	}
	return RealizedSavingsPage[RealizedSavingsCluster]{Rows: rows, Pagination: toCursorPagination(res.JSON200.Meta.Pagination)}, nil
}

func (c *Client) ListPublicRealizedSavingsNamespaces(ctx context.Context, publicAPIBaseURL string, token string, opts RealizedSavingsListOptions) (RealizedSavingsPage[RealizedSavingsNamespace], error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return RealizedSavingsPage[RealizedSavingsNamespace]{}, err
	}

	res, err := client.ListRealizedSavingsNamespacesWithResponse(ctx, publicapi.ListRealizedSavingsNamespacesJSONRequestBody{
		ClusterUid: opts.ClusterUID, StartDate: apiDate(opts.Range.Start), EndDate: apiDate(opts.Range.End),
		SortBy: opts.SortBy, SortOrder: opts.SortOrder, PageSize: opts.PageSize, PageToken: opts.PageToken,
	})
	if err != nil {
		return RealizedSavingsPage[RealizedSavingsNamespace]{}, fmt.Errorf("list realized savings namespaces: %w", err)
	}
	if res.JSON200 == nil {
		return RealizedSavingsPage[RealizedSavingsNamespace]{}, unexpectedPublicAPIResponse("list realized savings namespaces", res.StatusCode(), res.Body, res.HTTPResponse)
	}

	rows := make([]RealizedSavingsNamespace, 0, len(res.JSON200.Data))
	for _, item := range res.JSON200.Data {
		rows = append(rows, RealizedSavingsNamespace{
			ClusterUID:         item.ClusterUid,
			Namespace:          item.Namespace,
			TotalCost:          parseMoney(item.TotalCost),
			RealizedSavings:    parseMoney(item.RealizedSavings),
			RealizedSavingsPct: item.RealizedSavingsPct,
			CPUCoreHoursSaved:  RealizedSavingsHours(item.CpuCoreHoursSaved),
			MemoryGBHoursSaved: RealizedSavingsHours(item.MemoryGbHoursSaved),
		})
	}
	return RealizedSavingsPage[RealizedSavingsNamespace]{Rows: rows, Pagination: toCursorPagination(res.JSON200.Meta.Pagination)}, nil
}

func (c *Client) ListPublicRealizedSavingsWorkloads(ctx context.Context, publicAPIBaseURL string, token string, opts RealizedSavingsListOptions) (RealizedSavingsPage[RealizedSavingsWorkload], error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return RealizedSavingsPage[RealizedSavingsWorkload]{}, err
	}

	res, err := client.ListRealizedSavingsWorkloadsWithResponse(ctx, publicapi.ListRealizedSavingsWorkloadsJSONRequestBody{
		ClusterUid: opts.ClusterUID, Namespace: opts.Namespace, StartDate: apiDate(opts.Range.Start), EndDate: apiDate(opts.Range.End),
		SortBy: opts.SortBy, SortOrder: opts.SortOrder, PageSize: opts.PageSize, PageToken: opts.PageToken,
	})
	if err != nil {
		return RealizedSavingsPage[RealizedSavingsWorkload]{}, fmt.Errorf("list realized savings workloads: %w", err)
	}
	if res.JSON200 == nil {
		return RealizedSavingsPage[RealizedSavingsWorkload]{}, unexpectedPublicAPIResponse("list realized savings workloads", res.StatusCode(), res.Body, res.HTTPResponse)
	}

	rows := make([]RealizedSavingsWorkload, 0, len(res.JSON200.Data))
	for _, item := range res.JSON200.Data {
		rows = append(rows, RealizedSavingsWorkload{
			ClusterUID:         item.ClusterUid,
			Namespace:          item.Namespace,
			ID:                 item.Id,
			Name:               item.Name,
			Type:               item.Type,
			TotalCost:          parseMoney(item.TotalCost),
			RealizedSavings:    parseMoney(item.RealizedSavings),
			RealizedSavingsPct: item.RealizedSavingsPct,
			CPUCoreHoursSaved:  RealizedSavingsHours(item.CpuCoreHoursSaved),
			MemoryGBHoursSaved: RealizedSavingsHours(item.MemoryGbHoursSaved),
		})
	}
	return RealizedSavingsPage[RealizedSavingsWorkload]{Rows: rows, Pagination: toCursorPagination(res.JSON200.Meta.Pagination)}, nil
}

// ListAllRealizedSavings walks every page of one table at the maximum page size, starting from opts.PageToken.
func ListAllRealizedSavings[T any](opts RealizedSavingsListOptions, pageCap int, list func(RealizedSavingsListOptions) (RealizedSavingsPage[T], error)) (RealizedSavingsPage[T], error) {
	maxPageSize := 500
	opts.PageSize = &maxPageSize

	var last CursorPagination
	rows, err := fetchAllPages(pageCap, func(pageToken *string) ([]T, *string, error) {
		if pageToken != nil {
			opts.PageToken = pageToken
		}
		page, err := list(opts)
		last = page.Pagination
		return page.Rows, page.Pagination.Next, err
	})
	if err != nil {
		return RealizedSavingsPage[T]{}, err
	}
	return RealizedSavingsPage[T]{Rows: rows, Pagination: last}, nil
}

func apiDate(value time.Time) openapi_types.Date {
	return openapi_types.Date{Time: value}
}

func toRealizedSavingsSummary(item publicapi.RealizedSavingsSummary) RealizedSavingsSummary {
	summary := RealizedSavingsSummary{
		TotalCost:                 parseMoney(item.TotalCost),
		BaselineCost:              parseMoney(item.BaselineCost),
		RealizedSavings:           parseMoney(item.RealizedSavings),
		RealizedSavingsPct:        item.RealizedSavingsPct,
		AutomatedCost:             parseMoney(item.AutomatedCost),
		NotAutomatedCost:          parseMoney(item.NotAutomatedCost),
		ResiliencyIssuesAddressed: item.ResiliencyIssuesAddressed,
	}
	if cluster := item.Cluster; cluster != nil {
		summary.Cluster = &RealizedSavingsSummaryCluster{
			ClusterUID:    cluster.ClusterUid,
			Name:          cluster.Name,
			Cloud:         cluster.Cloud,
			Region:        cluster.Region,
			PricingSource: cluster.PricingSource,
		}
	}
	return summary
}
