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

type RealizedSavingsClusterListOptions struct {
	Range     RealizedSavingsRange
	SortBy    *string
	SortOrder *string
	PageSize  *int
	PageToken *string
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

type RealizedSavingsClusterPage struct {
	Clusters   []RealizedSavingsCluster `json:"clusters"`
	Pagination CursorPagination         `json:"pagination"`
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

func (c *Client) GetPublicRealizedSavingsSummary(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, dates RealizedSavingsRange) (RealizedSavingsSummary, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return RealizedSavingsSummary{}, err
	}

	if clusterUID == "" {
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

func (c *Client) ListPublicRealizedSavingsClusters(ctx context.Context, publicAPIBaseURL string, token string, opts RealizedSavingsClusterListOptions) (RealizedSavingsClusterPage, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return RealizedSavingsClusterPage{}, err
	}

	body := publicapi.ListRealizedSavingsClustersJSONRequestBody{
		StartDate: apiDate(opts.Range.Start),
		EndDate:   apiDate(opts.Range.End),
		PageSize:  opts.PageSize,
		PageToken: opts.PageToken,
	}
	if opts.SortBy != nil {
		sortBy := publicapi.ListRealizedSavingsClustersJSONBodySortBy(*opts.SortBy)
		body.SortBy = &sortBy
	}
	if opts.SortOrder != nil {
		sortOrder := publicapi.ListRealizedSavingsClustersJSONBodySortOrder(*opts.SortOrder)
		body.SortOrder = &sortOrder
	}

	res, err := client.ListRealizedSavingsClustersWithResponse(ctx, body)
	if err != nil {
		return RealizedSavingsClusterPage{}, fmt.Errorf("list realized savings clusters: %w", err)
	}
	if res.JSON200 == nil {
		return RealizedSavingsClusterPage{}, unexpectedPublicAPIResponse("list realized savings clusters", res.StatusCode(), res.Body, res.HTTPResponse)
	}

	clusters := make([]RealizedSavingsCluster, 0, len(res.JSON200.Data))
	for _, item := range res.JSON200.Data {
		clusters = append(clusters, toRealizedSavingsCluster(item))
	}
	return RealizedSavingsClusterPage{
		Clusters:   clusters,
		Pagination: toCursorPagination(res.JSON200.Meta.Pagination),
	}, nil
}

func (c *Client) ListAllPublicRealizedSavingsClusters(ctx context.Context, publicAPIBaseURL string, token string, opts RealizedSavingsClusterListOptions, pageCap int) (RealizedSavingsClusterPage, error) {
	maxPageSize := 500
	opts.PageSize = &maxPageSize

	var last CursorPagination
	clusters, err := fetchAllPages(pageCap, func(pageToken *string) ([]RealizedSavingsCluster, *string, error) {
		if pageToken != nil {
			opts.PageToken = pageToken
		}
		page, err := c.ListPublicRealizedSavingsClusters(ctx, publicAPIBaseURL, token, opts)
		last = page.Pagination
		return page.Clusters, page.Pagination.Next, err
	})
	if err != nil {
		return RealizedSavingsClusterPage{}, err
	}
	return RealizedSavingsClusterPage{Clusters: clusters, Pagination: last}, nil
}

func apiDate(value time.Time) openapi_types.Date {
	return openapi_types.Date{Time: value}
}

func toRealizedSavingsCluster(item publicapi.RealizedSavingsCluster) RealizedSavingsCluster {
	return RealizedSavingsCluster{
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
	}
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
			Cloud:         stringValue(cluster.Cloud),
			Region:        stringValue(cluster.Region),
			PricingSource: cluster.PricingSource,
		}
	}
	return summary
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
