package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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

// NodeGroupListOptions captures every filter/pagination input ListPublicNodeGroups accepts.
type NodeGroupListOptions struct {
	Period              *string
	PageSize            *int
	PageToken           *string
	RecommendationLimit *int
	HasRecommendations  *bool
	IncludeMuted        *bool
	AutoscalerType      *string
}

// ListPublicNodeGroups fetches a single page of node groups.
//
// Use Pagination.Next as PageToken on the next call to traverse forward.
// ListAllPublicNodeGroups is the auto-paginating convenience wrapper.
func (c *Client) ListPublicNodeGroups(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, opts NodeGroupListOptions) (NodeGroupPage, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return NodeGroupPage{}, err
	}

	params := &publicapi.ListInfraFitNodeGroupsParams{
		Period:              opts.Period,
		PageSize:            opts.PageSize,
		PageToken:           opts.PageToken,
		RecommendationLimit: opts.RecommendationLimit,
		HasRecommendations:  opts.HasRecommendations,
		IncludeMuted:        opts.IncludeMuted,
		AutoscalerType:      opts.AutoscalerType,
	}

	res, err := client.ListInfraFitNodeGroupsWithResponse(ctx, clusterUID, params)
	if err != nil {
		return NodeGroupPage{}, fmt.Errorf("list public node groups: %w", err)
	}
	if res.JSON200 == nil {
		return NodeGroupPage{}, unexpectedPublicAPIResponse("list public node groups", res.StatusCode(), res.Body)
	}

	groups := make([]NodeGroup, 0, len(res.JSON200.Data))
	for _, item := range res.JSON200.Data {
		groups = append(groups, toNodeGroup(item))
	}

	return NodeGroupPage{
		NodeGroups: groups,
		Pagination: toCursorPagination(res.JSON200.Meta.Pagination),
		Timeframe:  res.JSON200.Meta.Timeframe,
	}, nil
}

// ListAllPublicNodeGroups auto-paginates ListPublicNodeGroups.
//
// It forces pageSize to the server maximum (500) on every page regardless of
// opts.PageSize: the backend recomputes and re-sorts the entire node-group set
// on every single request (no incremental DB-level cursor), so a small page
// size only multiplies redundant server-side work without any benefit to the
// caller. pageCap bounds pages fetched as a safety net (set <=0 for default).
func (c *Client) ListAllPublicNodeGroups(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, opts NodeGroupListOptions, pageCap int) ([]NodeGroup, error) {
	maxPageSize := 500
	opts.PageSize = &maxPageSize

	return fetchAllPages(pageCap, func(pageToken *string) ([]NodeGroup, *string, error) {
		opts.PageToken = pageToken

		page, err := c.ListPublicNodeGroups(ctx, publicAPIBaseURL, token, clusterUID, opts)
		if err != nil {
			return nil, nil, err
		}

		return page.NodeGroups, page.Pagination.Next, nil
	})
}

// GetPublicNodeGroup fetches a single node group by name.
func (c *Client) GetPublicNodeGroup(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, nodeGroupName string, period string, recommendationLimit int) (*NodeGroup, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return nil, err
	}

	params := &publicapi.GetInfraFitNodeGroupParams{}
	if strings.TrimSpace(period) != "" {
		params.Period = &period
	}
	if recommendationLimit > 0 {
		params.RecommendationLimit = &recommendationLimit
	}

	res, err := client.GetInfraFitNodeGroupWithResponse(ctx, clusterUID, nodeGroupName, params)
	if err != nil {
		return nil, fmt.Errorf("get public node group: %w", err)
	}
	if res.JSON200 == nil {
		return nil, unexpectedPublicAPIResponse("get public node group", res.StatusCode(), res.Body)
	}

	group := toNodeGroup(res.JSON200.Data)

	return &group, nil
}

func toCursorPagination(item publicapi.CursorPagination) CursorPagination {
	return CursorPagination{
		Next:     item.Next,
		Prev:     item.Prev,
		PageSize: item.PageSize,
	}
}

func toNodeGroup(item publicapi.InfraFitNodeGroup) NodeGroup {
	group := NodeGroup{
		ID:              item.Id,
		AutoscalerType:  item.AutoscalerType,
		Nodes:           toNodesCount(item.Nodes),
		Pods:            toPodsCount(item.Pods),
		CPU:             toCPUUtilization(item.Cpu),
		Mem:             toMemUtilization(item.Mem),
		GPU:             toGPUUtilization(item.Gpu),
		Cost:            toNodeGroupCost(item.Cost),
		Recommendations: toNodeGroupRecommendations(item.Recommendations),
	}

	if item.Architectures != nil {
		group.Architectures = *item.Architectures
	}
	if item.Reservations != nil {
		group.Reservations = *item.Reservations
	}
	if item.RunningMinutes != nil {
		group.RunningMinutes = *item.RunningMinutes
	}
	if item.Labels != nil {
		group.Labels = *item.Labels
	}
	if item.Seen != nil {
		group.Seen = toSeenWindow(*item.Seen)
	}
	for _, nodeType := range item.NodeTypes {
		group.NodeTypes = append(group.NodeTypes, toNodeType(nodeType))
	}

	return group
}

func toNodeType(item publicapi.InfraFitNodeType) NodeType {
	nodeType := NodeType{
		ID:           item.Id,
		InstanceType: item.InstanceType,
		Nodes:        toNodesCount(item.Nodes),
		Pods:         toPodsCount(item.Pods),
		CPU:          toCPUUtilization(item.Cpu),
		Mem:          toMemUtilization(item.Mem),
		GPU:          toGPUUtilization(item.Gpu),
		Cost:         toNodeGroupCost(item.Cost),
		IsSpot:       item.IsSpot,
	}

	if item.InstanceFamily != nil {
		nodeType.InstanceFamily = *item.InstanceFamily
	}
	if item.RunningMinutes != nil {
		nodeType.RunningMinutes = *item.RunningMinutes
	}
	if item.Seen != nil {
		nodeType.Seen = toSeenWindow(*item.Seen)
	}

	return nodeType
}

func toNodesCount(item publicapi.NodesCount) NodesCount {
	return NodesCount{Min: item.Min, Max: item.Max, Avg: item.Avg}
}

func toPodsCount(item publicapi.PodsCount) PodsCount {
	return PodsCount{Capacity: item.Capacity, Allocatable: item.Allocatable, AvgCount: item.AvgCount}
}

func toCPUPercentiles(item publicapi.CPUPercentiles) UtilizationPercentiles {
	return UtilizationPercentiles{
		Avg:  item.AvgCores,
		Min:  item.MinCores,
		Max:  item.MaxCores,
		P80:  item.P80Cores,
		P95:  item.P95Cores,
		P99:  item.P99Cores,
		P999: item.P999Cores,
	}
}

func toCPUUtilization(item publicapi.CPUUtilization) ResourceUtilization {
	utilization := ResourceUtilization{
		Requested: toCPUPercentiles(item.Requested),
		Used:      toCPUPercentiles(item.Used),
	}
	if item.IdleCores != nil {
		utilization.IdleCores = *item.IdleCores
	}

	return utilization
}

func toMemPercentiles(item publicapi.MemPercentiles) UtilizationPercentiles {
	return UtilizationPercentiles{
		Avg:  item.AvgMiB,
		Min:  item.MinMiB,
		Max:  item.MaxMiB,
		P80:  item.P80MiB,
		P95:  item.P95MiB,
		P99:  item.P99MiB,
		P999: item.P999MiB,
	}
}

func toMemUtilization(item publicapi.MemUtilization) ResourceUtilization {
	utilization := ResourceUtilization{
		Requested: toMemPercentiles(item.Requested),
		Used:      toMemPercentiles(item.Used),
	}
	if item.IdleMiB != nil {
		utilization.IdleCores = *item.IdleMiB
	}

	return utilization
}

func toGPUPercentiles(item publicapi.GPUPercentiles) GPUPercentiles {
	return GPUPercentiles{
		AvgMemoryMiB:  item.AvgMemoryMiB,
		MinMemoryMiB:  item.MinMemoryMiB,
		MaxMemoryMiB:  item.MaxMemoryMiB,
		P80MemoryMiB:  item.P80MemoryMiB,
		P95MemoryMiB:  item.P95MemoryMiB,
		P99MemoryMiB:  item.P99MemoryMiB,
		P999MemoryMiB: item.P999MemoryMiB,
		AvgUnits:      item.AvgUnits,
		MinUnits:      item.MinUnits,
		MaxUnits:      item.MaxUnits,
		P80Units:      item.P80Units,
		P95Units:      item.P95Units,
		P99Units:      item.P99Units,
		P999Units:     item.P999Units,
	}
}

// toGPUUtilization returns nil when the node group/type has no GPU.
func toGPUUtilization(item *publicapi.GPUUtilization) *GPUUtilization {
	if item == nil {
		return nil
	}

	gpu := &GPUUtilization{
		Requested: toGPUPercentiles(item.Requested),
		Used:      toGPUPercentiles(item.Used),
	}
	if item.Architectures != nil {
		gpu.Architectures = *item.Architectures
	}
	if item.SharingType != nil {
		gpu.SharingType = *item.SharingType
	}
	if item.Idle != nil {
		gpu.IdleMemoryMiB = item.Idle.MemoryMiB
		gpu.IdleUnits = item.Idle.Units
	}

	return gpu
}

func toNodeGroupCost(item publicapi.NodeGroupCost) NodeGroupCost {
	cost := NodeGroupCost{
		Hourly:    parseMoney(item.Hourly),
		Timeframe: parseMoney(item.Timeframe),
		IdleCPU:   parseMoney(item.Idle.Cpu),
		IdleMem:   parseMoney(item.Idle.Mem),
		IdleTotal: parseMoney(item.Idle.Total),
	}
	if item.Idle.Gpu != nil {
		cost.IdleGPU = parseMoney(*item.Idle.Gpu)
	}

	return cost
}

func parseMoney(item publicapi.Money) float64 {
	value, err := strconv.ParseFloat(item.Amount, 64)
	if err != nil {
		return 0
	}

	return value
}

func toSeenWindow(item publicapi.SeenWindow) SeenWindow {
	firstTime := item.FirstTime
	lastTime := item.LastTime

	return SeenWindow{FirstTime: &firstTime, LastTime: &lastTime}
}

func toNodeGroupRecommendations(item publicapi.InfraFitNodeGroupRecommendations) NodeGroupRecommendations {
	kind, err := item.Discriminator()
	if err != nil {
		return NodeGroupRecommendations{}
	}

	switch kind {
	case string(publicapi.Karpenter):
		return toKarpenterRecommendations(item)
	case string(publicapi.Standard):
		return toStandardRecommendations(item)
	default:
		return NodeGroupRecommendations{Type: kind}
	}
}

func toKarpenterRecommendations(item publicapi.InfraFitNodeGroupRecommendations) NodeGroupRecommendations {
	karpenter, err := item.AsInfraFitKarpenterRecommendations()
	if err != nil {
		return NodeGroupRecommendations{Type: NodeGroupRecommendationsTypeKarpenter}
	}

	changes := make([]KarpenterChange, 0, len(karpenter.Changes))
	for _, change := range karpenter.Changes {
		changes = append(changes, KarpenterChange{
			ID:               change.Id,
			Title:            change.Title,
			Path:             change.Path,
			Operation:        change.Operation,
			CurrentValue:     change.CurrentValue,
			RecommendedValue: change.RecommendedValue,
			Rationale:        change.Rationale,
		})
	}

	return NodeGroupRecommendations{
		Type:              NodeGroupRecommendationsTypeKarpenter,
		HasChanges:        karpenter.HasChanges,
		Changes:           changes,
		CurrentConfig:     karpenter.CurrentConfig,
		RecommendedConfig: karpenter.RecommendedConfig,
	}
}

func toStandardRecommendations(item publicapi.InfraFitNodeGroupRecommendations) NodeGroupRecommendations {
	standard, err := item.AsInfraFitStandardRecommendations()
	if err != nil {
		return NodeGroupRecommendations{Type: NodeGroupRecommendationsTypeStandard}
	}

	recs := make([]NodeTypeRecommendation, 0, len(standard.NodeTypes))
	for _, nodeType := range standard.NodeTypes {
		rec := NodeTypeRecommendation{
			ID:                  nodeType.Id,
			InstanceType:        nodeType.InstanceType,
			HourlyCost:          nodeType.HourlyCost,
			EstimatedSavings:    nodeType.EstimatedSavings,
			EstimatedSavingsPct: nodeType.EstimatedSavingsPct,
			NodeCount:           nodeType.NodeCount,
		}
		if nodeType.InstanceFamily != nil {
			rec.InstanceFamily = *nodeType.InstanceFamily
		}
		recs = append(recs, rec)
	}

	return NodeGroupRecommendations{
		Type:         NodeGroupRecommendationsTypeStandard,
		HasChanges:   standard.HasChanges,
		NodeTypeRecs: recs,
	}
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
