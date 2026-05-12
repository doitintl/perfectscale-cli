package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/perfectscale/poc-cli/internal/publicapi"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) GetPublicCluster(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, period string) (*ClusterDetail, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return nil, err
	}

	var params *publicapi.GetClusterParams
	if strings.TrimSpace(period) != "" {
		params = &publicapi.GetClusterParams{Period: &period}
	}

	res, err := client.GetClusterWithResponse(ctx, clusterUID, params)
	if err != nil {
		return nil, fmt.Errorf("get public cluster: %w", err)
	}
	if res.JSON200 == nil {
		return nil, unexpectedPublicAPIResponse("get public cluster", res.StatusCode(), res.Body)
	}

	detail := toClusterDetail(res.JSON200.Data)
	return &detail, nil
}

func (c *Client) ListPublicClusters(ctx context.Context, publicAPIBaseURL string, token string) ([]Cluster, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return nil, err
	}

	res, err := client.GetClustersWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("list public clusters: %w", err)
	}
	if res.JSON200 == nil {
		return nil, unexpectedPublicAPIResponse("list public clusters", res.StatusCode(), res.Body)
	}

	clusters := make([]Cluster, 0, len(res.JSON200.Data))
	for _, item := range res.JSON200.Data {
		clusters = append(clusters, toCluster(item))
	}

	return clusters, nil
}

func (c *Client) ListPublicWorkloads(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string) ([]Workload, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return nil, err
	}

	res, err := client.GetClustersClusterUidWorkloadsWithResponse(ctx, clusterUID)
	if err != nil {
		return nil, fmt.Errorf("list public workloads: %w", err)
	}
	if res.JSON200 == nil {
		return nil, unexpectedPublicAPIResponse("list public workloads", res.StatusCode(), res.Body)
	}

	items := make([]Workload, 0, len(res.JSON200.Data))
	for _, item := range res.JSON200.Data {
		firstSeen := item.FirstSeen
		lastSeen := item.LastSeen
		combinedIndicators := make([]publicapi.Indicator, 0, len(item.Indicators))
		combinedIndicators = append(combinedIndicators, item.Indicators...)
		for _, container := range item.Containers {
			combinedIndicators = append(combinedIndicators, container.Indicators...)
		}
		containers := toPublicContainers(item.Containers)
		indicators := toPublicIndicators(item.Indicators)
		items = append(items, Workload{
			ID:                           item.Id,
			Name:                         item.Name,
			Namespace:                    item.Namespace,
			Type:                         item.Type,
			Period:                       "30d",
			ReplicasCounts:               ReplicasCounts{MaxCount: item.ReplicasCounts.MaxCount, AvgCount: item.ReplicasCounts.AvgCount},
			ResilienceLevel:              string(item.ResilienceLevel),
			OptimizationPolicy:           string(item.OptimizationPolicy),
			OptimizationPolicyTimeWindow: string(item.OptimizationPolicyTimeWindow),
			CPUOptimizationPolicy:        string(item.CpuOptimizationPolicy),
			MemoryOptimizationPolicy:     string(item.MemoryOptimizationPolicy),
			MemoryRequestEqualsLimit:     item.MemoryRequestEqualsLimit,
			MuteStatus:                   toMuteStatus(item.MuteStatus),
			Cost:                         item.CostAnalysis.Past30Days.TotalCost,
			Waste:                        item.CostAnalysis.Past30Days.WastedCost,
			HistoricalWaste:              item.CostAnalysis.Past30Days.WastedCost,
			CostPerHour:                  item.CostAnalysis.Past30Days.CostPerHour,
			PotentialSavings:             item.CostAnalysis.Next30Days.PotentialSavings,
			CostIncrease:                 item.CostAnalysis.Next30Days.CostIncrease,
			RunningMinutes:               item.RunningMinutes,
			FirstSeen:                    &firstSeen,
			LastSeen:                     &lastSeen,
			MaxIndicator:                 maxPublicIndicator(combinedIndicators),
			Indicators:                   indicators,
			WorkloadLabels:               item.WorkloadLabels,
			Containers:                   containers,
			Derived:                      deriveWorkload(containers, indicators),
		})
	}

	return items, nil
}

func (c *Client) newPublicClient(publicAPIBaseURL string, token string) (*publicapi.ClientWithResponses, error) {
	client, err := publicapi.NewClientWithResponses(
		strings.TrimRight(publicAPIBaseURL, "/"),
		publicapi.WithHTTPClient(c.httpClient),
		publicapi.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Accept", "application/json")
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create generated public client: %w", err)
	}
	return client, nil
}

func unexpectedPublicAPIResponse(operation string, statusCode int, body []byte) error {
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("%s failed with status %d", operation, statusCode)
	}
	return fmt.Errorf("%s failed with status %d: %s", operation, statusCode, message)
}

func toCluster(item publicapi.Cluster) Cluster {
	uid := ""
	if item.Uid != nil {
		uid = *item.Uid
	}
	cloud := ""
	if item.Cloud != nil {
		cloud = string(*item.Cloud)
	}
	region := ""
	if item.Region != nil {
		region = *item.Region
	}
	updatedAt := item.LastTransmittedAt

	return Cluster{
		UID:       uid,
		Name:      item.Name,
		Cloud:     cloud,
		Region:    region,
		CreatedAt: item.CreatedAt,
		UpdatedAt: &updatedAt,
	}
}

func toClusterDetail(item publicapi.ClusterDetail) ClusterDetail {
	uid := ""
	if item.Uid != nil {
		uid = *item.Uid
	}
	cloud := ""
	if item.Cloud != nil {
		cloud = string(*item.Cloud)
	}
	region := ""
	if item.Region != nil {
		region = *item.Region
	}
	updatedAt := item.LastTransmittedAt

	return ClusterDetail{
		UID:       uid,
		Name:      item.Name,
		Cloud:     cloud,
		Region:    region,
		CreatedAt: item.CreatedAt,
		UpdatedAt: &updatedAt,
		Emission:  item.Emission,
	}
}

func toPublicIndicators(items []publicapi.Indicator) []Indicator {
	out := make([]Indicator, 0, len(items))
	for _, item := range items {
		out = append(out, Indicator{
			Name:     string(item.Name),
			Type:     string(item.Type),
			Severity: int(item.SeverityLevel),
		})
	}
	return out
}

func maxPublicIndicator(items []publicapi.Indicator) *Indicator {
	converted := toPublicIndicators(items)
	if len(converted) == 0 {
		return nil
	}

	best := converted[0]
	for _, item := range converted[1:] {
		if item.Severity > best.Severity {
			best = item
			continue
		}
		if item.Severity == best.Severity && publicIndicatorTypePriority(item.Type) > publicIndicatorTypePriority(best.Type) {
			best = item
		}
	}

	copy := best
	return &copy
}

func publicIndicatorTypePriority(value string) int {
	switch value {
	case "risk":
		return 2
	case "waste":
		return 1
	default:
		return 0
	}
}

func toMuteStatus(item publicapi.MuteStatus) MuteStatus {
	return MuteStatus{
		IsMuted: item.IsMuted,
		Expires: item.Expires,
	}
}

func toResourceValues(item publicapi.Resources) ResourceValues {
	return ResourceValues{
		MemoryRequestMiB: item.MemoryRequestMiB,
		MemoryLimitMiB:   item.MemoryLimitMiB,
		CPURequestCores:  item.CpuRequestCores,
		CPULimitCores:    item.CpuLimitCores,
	}
}

func toPercentiles(item publicapi.Percentiles) Percentiles {
	return Percentiles{
		P90:  item.P90,
		P95:  item.P95,
		P100: item.P100,
	}
}

func toPublicContainers(items []publicapi.Container) []WorkloadContainer {
	out := make([]WorkloadContainer, 0, len(items))
	for _, item := range items {
		out = append(out, WorkloadContainer{
			Name:           item.Name,
			RunningMinutes: item.RunningMinutes,
			Indicators:     toPublicIndicators(item.Indicators),
			Resources: ContainerResources{
				Current:     toResourceValues(item.Resources.Current),
				Recommended: toResourceValues(item.Resources.Recommended),
			},
			Usage: ContainerUsage{
				CPUCores:  toPercentiles(item.Usage.CpuCores),
				MemoryMiB: toPercentiles(item.Usage.MemoryMiB),
			},
		})
	}
	return out
}

func deriveWorkload(containers []WorkloadContainer, indicators []Indicator) WorkloadDerived {
	derived := WorkloadDerived{
		ContainerCount: len(containers),
	}

	for _, indicator := range indicators {
		derived.IndicatorsCount++
		switch indicator.Type {
		case "risk":
			derived.RiskIndicatorsCount++
		case "waste":
			derived.WasteIndicatorsCount++
		}
	}

	for _, container := range containers {
		for _, indicator := range container.Indicators {
			derived.IndicatorsCount++
			switch indicator.Type {
			case "risk":
				derived.RiskIndicatorsCount++
			case "waste":
				derived.WasteIndicatorsCount++
			}
		}

		derived.CurrentCPURequestCoresTotal += container.Resources.Current.CPURequestCores
		derived.CurrentCPULimitCoresTotal += container.Resources.Current.CPULimitCores
		derived.CurrentMemoryRequestMiBTotal += container.Resources.Current.MemoryRequestMiB
		derived.CurrentMemoryLimitMiBTotal += container.Resources.Current.MemoryLimitMiB
		derived.RecommendedCPURequestCoresTotal += container.Resources.Recommended.CPURequestCores
		derived.RecommendedCPULimitCoresTotal += container.Resources.Recommended.CPULimitCores
		derived.RecommendedMemoryRequestMiBTotal += container.Resources.Recommended.MemoryRequestMiB
		derived.RecommendedMemoryLimitMiBTotal += container.Resources.Recommended.MemoryLimitMiB
		derived.CPUUsageP90CoresSum += container.Usage.CPUCores.P90
		derived.CPUUsageP95CoresSum += container.Usage.CPUCores.P95
		derived.CPUUsageP100CoresSum += container.Usage.CPUCores.P100
		derived.MemoryUsageP90MiBSum += container.Usage.MemoryMiB.P90
		derived.MemoryUsageP95MiBSum += container.Usage.MemoryMiB.P95
		derived.MemoryUsageP100MiBSum += container.Usage.MemoryMiB.P100
	}

	return derived
}
