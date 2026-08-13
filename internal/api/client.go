package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/perfectscale/poc-cli/internal/publicapi"
)

// ErrUnevictableSnapshotProcessing and ErrUnevictableSnapshotFailed signal the
// two non-data states the unevictable-pods snapshot can be in server-side,
// detected from the HTTP status alone (202 / 422) rather than by parsing a
// response body, since 422 carries a generic error body with no fixed shape.
var (
	ErrUnevictableSnapshotProcessing = errors.New("unevictable snapshot is still processing; try again shortly")
	ErrUnevictableSnapshotFailed     = errors.New("unevictable snapshot processing failed")
)

func checkUnevictableSnapshotStatus(statusCode int) error {
	switch statusCode {
	case http.StatusAccepted:
		return ErrUnevictableSnapshotProcessing
	case http.StatusUnprocessableEntity:
		return ErrUnevictableSnapshotFailed
	default:
		return nil
	}
}

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

// buildUnevictableFilter composes the server's composite filter DSL:
// key[:op]:value clauses joined by "|" (AND). Only single-value clauses are
// needed here since no flag in this ticket's scope accepts multiple values.
func buildUnevictableFilter(opts UnevictableListOptions) *string {
	var clauses []string

	if opts.Namespace != nil {
		if value := strings.TrimSpace(*opts.Namespace); value != "" {
			clauses = append(clauses, "namespace:"+value)
		}
	}
	if opts.Reason != nil {
		if value := strings.TrimSpace(*opts.Reason); value != "" {
			clauses = append(clauses, "reasonCode:"+value)
		}
	}
	if opts.NodeGroup != nil {
		if value := strings.TrimSpace(*opts.NodeGroup); value != "" {
			clauses = append(clauses, "nodeGroup:"+value)
		}
	}
	if opts.MinBlockedCost != nil {
		clauses = append(clauses, "blockedCostHourly:gte:"+strconv.FormatFloat(*opts.MinBlockedCost, 'f', -1, 64))
	}

	if len(clauses) == 0 {
		return nil
	}

	filter := strings.Join(clauses, "|")
	return &filter
}

// ListPublicUnevictablePods fetches a single page of unevictable pods.
//
// Use Pagination.Next as PageToken on the next call to traverse forward.
// ListAllPublicUnevictablePods is the auto-paginating convenience wrapper.
func (c *Client) ListPublicUnevictablePods(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, opts UnevictableListOptions) (UnevictablePodPage, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return UnevictablePodPage{}, err
	}

	params := &publicapi.ListUnevictablePodsParams{
		Filter:    buildUnevictableFilter(opts),
		PageSize:  opts.PageSize,
		PageToken: opts.PageToken,
	}
	if opts.Mute != nil {
		mute := publicapi.ListUnevictablePodsParamsMute(*opts.Mute)
		params.Mute = &mute
	}
	if opts.SortBy != nil {
		sortBy := publicapi.ListUnevictablePodsParamsSortBy(*opts.SortBy)
		params.SortBy = &sortBy
	}
	if opts.SortOrder != nil {
		sortOrder := publicapi.ListUnevictablePodsParamsSortOrder(*opts.SortOrder)
		params.SortOrder = &sortOrder
	}

	res, err := client.ListUnevictablePodsWithResponse(ctx, clusterUID, params)
	if err != nil {
		return UnevictablePodPage{}, fmt.Errorf("list unevictable pods: %w", err)
	}
	if res.JSON200 == nil {
		if statusErr := checkUnevictableSnapshotStatus(res.StatusCode()); statusErr != nil {
			return UnevictablePodPage{}, statusErr
		}
		return UnevictablePodPage{}, unexpectedPublicAPIResponse("list unevictable pods", res.StatusCode(), res.Body)
	}

	pods := make([]UnevictablePod, 0, len(res.JSON200.Data))
	for _, item := range res.JSON200.Data {
		pods = append(pods, toUnevictablePod(item))
	}

	return UnevictablePodPage{
		Pods:             pods,
		Pagination:       toCursorPagination(res.JSON200.Meta.Pagination),
		SnapshotTime:     res.JSON200.Meta.SnapshotTime,
		AlgorithmVersion: res.JSON200.Meta.AlgorithmVersion,
		Summary:          toUnevictableSummary(res.JSON200.Meta.Summary),
	}, nil
}

// ListAllPublicUnevictablePods auto-paginates ListPublicUnevictablePods.
// pageCap bounds pages fetched as a safety net (set <=0 for default).
func (c *Client) ListAllPublicUnevictablePods(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, opts UnevictableListOptions, pageCap int) ([]UnevictablePod, error) {
	maxPageSize := 500
	opts.PageSize = &maxPageSize

	return fetchAllPages(pageCap, func(pageToken *string) ([]UnevictablePod, *string, error) {
		opts.PageToken = pageToken

		page, err := c.ListPublicUnevictablePods(ctx, publicAPIBaseURL, token, clusterUID, opts)
		if err != nil {
			return nil, nil, err
		}

		return page.Pods, page.Pagination.Next, nil
	})
}

// GetPublicUnevictableReport fetches a single page of the per-pod unevictable
// report. opts.Reason is ignored here — the report endpoint's filter schema
// doesn't support reasonCode; callers must reject it before calling this.
func (c *Client) GetPublicUnevictableReport(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, opts UnevictableListOptions) (UnevictableReportPage, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return UnevictableReportPage{}, err
	}

	params := &publicapi.GetUnevictableReportParams{
		Filter:    buildUnevictableFilter(opts),
		PageSize:  opts.PageSize,
		PageToken: opts.PageToken,
	}
	if opts.Mute != nil {
		mute := publicapi.GetUnevictableReportParamsMute(*opts.Mute)
		params.Mute = &mute
	}
	if opts.SortBy != nil {
		sortBy := publicapi.GetUnevictableReportParamsSortBy(*opts.SortBy)
		params.SortBy = &sortBy
	}
	if opts.SortOrder != nil {
		sortOrder := publicapi.GetUnevictableReportParamsSortOrder(*opts.SortOrder)
		params.SortOrder = &sortOrder
	}

	res, err := client.GetUnevictableReportWithResponse(ctx, clusterUID, params)
	if err != nil {
		return UnevictableReportPage{}, fmt.Errorf("get unevictable report: %w", err)
	}
	if res.JSON200 == nil {
		if statusErr := checkUnevictableSnapshotStatus(res.StatusCode()); statusErr != nil {
			return UnevictableReportPage{}, statusErr
		}
		return UnevictableReportPage{}, unexpectedPublicAPIResponse("get unevictable report", res.StatusCode(), res.Body)
	}

	rows := make([]UnevictableReportRow, 0, len(res.JSON200.Data))
	for _, item := range res.JSON200.Data {
		rows = append(rows, toUnevictableReportRow(item))
	}

	return UnevictableReportPage{
		Rows:             rows,
		Pagination:       toCursorPagination(res.JSON200.Meta.Pagination),
		SnapshotTime:     res.JSON200.Meta.SnapshotTime,
		AlgorithmVersion: res.JSON200.Meta.AlgorithmVersion,
		Summary:          toUnevictableSummary(res.JSON200.Meta.Summary),
	}, nil
}

// ListAllPublicUnevictableReport auto-paginates GetPublicUnevictableReport.
func (c *Client) ListAllPublicUnevictableReport(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, opts UnevictableListOptions, pageCap int) ([]UnevictableReportRow, error) {
	maxPageSize := 500
	opts.PageSize = &maxPageSize

	return fetchAllPages(pageCap, func(pageToken *string) ([]UnevictableReportRow, *string, error) {
		opts.PageToken = pageToken

		page, err := c.GetPublicUnevictableReport(ctx, publicAPIBaseURL, token, clusterUID, opts)
		if err != nil {
			return nil, nil, err
		}

		return page.Rows, page.Pagination.Next, nil
	})
}

// GetPublicUnevictablePod fetches full detail for a single pod. Unlike every
// other endpoint in this API, the response has no {data} envelope — that
// asymmetry is handled here so it never leaks into CLI command code.
func (c *Client) GetPublicUnevictablePod(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, podUID string) (*UnevictablePod, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return nil, err
	}

	res, err := client.GetUnevictablePodWithResponse(ctx, clusterUID, podUID)
	if err != nil {
		return nil, fmt.Errorf("get unevictable pod: %w", err)
	}
	if res.JSON200 == nil {
		if statusErr := checkUnevictableSnapshotStatus(res.StatusCode()); statusErr != nil {
			return nil, statusErr
		}
		return nil, unexpectedPublicAPIResponse("get unevictable pod", res.StatusCode(), res.Body)
	}

	pod := toUnevictablePod(*res.JSON200)
	return &pod, nil
}

// ListPublicUnevictableMutedWorkloads fetches a single page of workloads that
// have an active mute/dismissal rule.
//
// Read-only: mute rules can only be created or removed via the web app or
// user API — this CLI has no corresponding write command.
func (c *Client) ListPublicUnevictableMutedWorkloads(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, pageSize *int, pageToken *string) (UnevictableMutedWorkloadPage, error) {
	client, err := c.newPublicClient(publicAPIBaseURL, token)
	if err != nil {
		return UnevictableMutedWorkloadPage{}, err
	}

	params := &publicapi.ListUnevictableMutedWorkloadsParams{PageSize: pageSize, PageToken: pageToken}

	res, err := client.ListUnevictableMutedWorkloadsWithResponse(ctx, clusterUID, params)
	if err != nil {
		return UnevictableMutedWorkloadPage{}, fmt.Errorf("list unevictable muted workloads: %w", err)
	}
	if res.JSON200 == nil {
		return UnevictableMutedWorkloadPage{}, unexpectedPublicAPIResponse("list unevictable muted workloads", res.StatusCode(), res.Body)
	}

	workloads := make([]UnevictableMutedWorkload, 0, len(res.JSON200.Data))
	for _, item := range res.JSON200.Data {
		workloads = append(workloads, toUnevictableMutedWorkload(item))
	}

	return UnevictableMutedWorkloadPage{
		Workloads:  workloads,
		Pagination: toCursorPagination(res.JSON200.Meta.Pagination),
	}, nil
}

// ListAllPublicUnevictableMutedWorkloads auto-paginates ListPublicUnevictableMutedWorkloads.
func (c *Client) ListAllPublicUnevictableMutedWorkloads(ctx context.Context, publicAPIBaseURL string, token string, clusterUID string, pageCap int) ([]UnevictableMutedWorkload, error) {
	maxPageSize := 500

	return fetchAllPages(pageCap, func(pageToken *string) ([]UnevictableMutedWorkload, *string, error) {
		page, err := c.ListPublicUnevictableMutedWorkloads(ctx, publicAPIBaseURL, token, clusterUID, &maxPageSize, pageToken)
		if err != nil {
			return nil, nil, err
		}

		return page.Workloads, page.Pagination.Next, nil
	})
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

func toUnevictableWorkloadRef(item publicapi.UnevictableWorkloadRef) UnevictableWorkloadRef {
	ref := UnevictableWorkloadRef{ID: item.Id, Type: item.Type}
	if item.Name != nil {
		ref.Name = *item.Name
	}

	return ref
}

func toUnevictableRemediation(item *publicapi.UnevictableRemediation) UnevictableRemediation {
	if item == nil {
		return UnevictableRemediation{}
	}

	return UnevictableRemediation{
		FixSummary:      item.FixSummary,
		Risk:            string(item.Risk),
		Confidence:      string(item.Confidence),
		CurrentSpec:     item.CurrentSpec,
		RecommendedSpec: item.RecommendedSpec,
		YAMLDiff:        item.YamlDiff,
	}
}

func toUnevictableMutedByRule(item *publicapi.UnevictableMutedByRule) *UnevictableMutedByRule {
	if item == nil {
		return nil
	}

	rule := &UnevictableMutedByRule{CreatedBy: item.CreatedBy, CreateTime: item.CreateTime}
	if item.Note != nil {
		rule.Note = *item.Note
	}

	return rule
}

func toUnevictableReasons(items []publicapi.UnevictableReason) []UnevictableReason {
	reasons := make([]UnevictableReason, 0, len(items))
	for _, item := range items {
		reason := UnevictableReason{
			Reason:      item.Reason,
			Details:     item.Details,
			Remediation: toUnevictableRemediation(item.Remediation),
			MutedByRule: toUnevictableMutedByRule(item.MutedByRule),
		}
		if item.ReasonCode != nil {
			reason.ReasonCode = string(*item.ReasonCode)
		}
		if item.Mute != nil {
			reason.Mute = *item.Mute
		}
		reasons = append(reasons, reason)
	}

	return reasons
}

func toUnevictablePodAffinity(item *publicapi.UnevictablePodAffinity) *UnevictablePodAffinity {
	if item == nil {
		return nil
	}

	affinity := &UnevictablePodAffinity{}
	if item.NodeAffinity != nil {
		affinity.NodeAffinity = *item.NodeAffinity
	}
	if item.PodAffinity != nil {
		affinity.PodAffinity = *item.PodAffinity
	}
	if item.PodAntiAffinity != nil {
		affinity.PodAntiAffinity = *item.PodAntiAffinity
	}

	return affinity
}

func toUnevictablePodSpec(item *publicapi.UnevictablePodSpec) UnevictablePodSpec {
	if item == nil {
		return UnevictablePodSpec{}
	}

	spec := UnevictablePodSpec{Affinity: toUnevictablePodAffinity(item.Affinity)}
	if item.Node != nil {
		spec.Node = *item.Node
	}
	if item.NodeGroup != nil {
		spec.NodeGroup = *item.NodeGroup
	}
	if item.Priority != nil {
		spec.Priority = *item.Priority
	}
	if item.NodeSelector != nil {
		spec.NodeSelector = *item.NodeSelector
	}
	if item.Tolerations != nil {
		for _, toleration := range *item.Tolerations {
			mapped := UnevictablePodToleration{Key: toleration.Key, Operator: toleration.Operator, Effect: toleration.Effect}
			if toleration.Value != nil {
				mapped.Value = *toleration.Value
			}
			spec.Tolerations = append(spec.Tolerations, mapped)
		}
	}
	if item.Containers != nil {
		for _, container := range *item.Containers {
			spec.Containers = append(spec.Containers, UnevictablePodContainer{
				Name:             container.Name,
				Image:            container.Image,
				CPURequestCores:  container.CpuRequestCores,
				CPULimitCores:    container.CpuLimitCores,
				MemoryRequestMiB: container.MemoryRequestMiB,
				MemoryLimitMiB:   container.MemoryLimitMiB,
				GPURequest:       container.GpuRequest,
				GPULimit:         container.GpuLimit,
			})
		}
	}
	if item.Volumes != nil {
		for _, volume := range *item.Volumes {
			mapped := UnevictablePodVolume{Name: volume.Name}
			if volume.HostPath != nil {
				mapped.HostPath = *volume.HostPath
			}
			if volume.EmptyDir != nil {
				mapped.EmptyDir = *volume.EmptyDir
			}
			if volume.PvcClaimName != nil {
				mapped.PVCClaimName = *volume.PvcClaimName
			}
			spec.Volumes = append(spec.Volumes, mapped)
		}
	}
	if item.TopologySpreadConstraints != nil {
		for _, constraint := range *item.TopologySpreadConstraints {
			mapped := UnevictablePodTopologySpreadConstraint{
				MaxSkew:           constraint.MaxSkew,
				TopologyKey:       constraint.TopologyKey,
				WhenUnsatisfiable: constraint.WhenUnsatisfiable,
			}
			if constraint.LabelSelector != nil {
				mapped.LabelSelector = *constraint.LabelSelector
			}
			spec.TopologySpreadConstraints = append(spec.TopologySpreadConstraints, mapped)
		}
	}
	if item.OwnerReferences != nil {
		for _, ownerRef := range *item.OwnerReferences {
			spec.OwnerReferences = append(spec.OwnerReferences, UnevictablePodOwnerReference{
				APIVersion: ownerRef.ApiVersion,
				Kind:       ownerRef.Kind,
				Name:       ownerRef.Name,
				Controller: ownerRef.Controller,
			})
		}
	}

	return spec
}

func toUnevictablePod(item publicapi.UnevictablePod) UnevictablePod {
	pod := UnevictablePod{
		Name:      item.Name,
		Namespace: item.Namespace,
		ID:        item.Id,
		Workload:  toUnevictableWorkloadRef(item.Workload),
		Reasons:   toUnevictableReasons(item.Reasons),
		Phase:     item.Phase,
		StartTime: item.StartTime,
		Spec:      toUnevictablePodSpec(item.Spec),
	}
	if item.Labels != nil {
		pod.Labels = *item.Labels
	}
	if item.Annotations != nil {
		pod.Annotations = *item.Annotations
	}
	if item.BlockedNodeCount != nil {
		pod.BlockedNodeCount = *item.BlockedNodeCount
	}
	if item.BlockedNodes != nil {
		pod.BlockedNodes = *item.BlockedNodes
	}
	if item.BlockedCostHourly != nil {
		pod.BlockedCostHourly = parseMoney(*item.BlockedCostHourly)
	}
	if item.ClusterUid != nil {
		pod.ClusterUID = *item.ClusterUid
	}
	if item.Mute != nil {
		pod.Mute = *item.Mute
	}
	if item.SiblingPodNames != nil {
		pod.SiblingPodNames = *item.SiblingPodNames
	}

	return pod
}

func toUnevictableReportRow(item publicapi.UnevictableReportRow) UnevictableReportRow {
	row := UnevictableReportRow{
		Name:      item.Name,
		ID:        item.Id,
		Workload:  toUnevictableWorkloadRef(item.Workload),
		Namespace: item.Namespace,
		Reasons:   toUnevictableReasons(item.Reasons),
		Mute:      item.Mute,
	}
	if item.Labels != nil {
		row.Labels = *item.Labels
	}
	if item.Node != nil {
		row.Node = *item.Node
	}
	if item.NodeGroup != nil {
		row.NodeGroup = *item.NodeGroup
	}
	if item.Priority != nil {
		row.Priority = *item.Priority
	}
	if item.BlockedCostHourly != nil {
		row.BlockedCostHourly = parseMoney(*item.BlockedCostHourly)
	}

	return row
}

func toUnevictableSummary(item publicapi.UnevictableSummary) UnevictableSummary {
	summary := UnevictableSummary{
		TotalPods:       item.TotalPods,
		UnevictablePods: item.UnevictablePods,
		Mute:            item.Mute,
		TotalNodes:      item.TotalNodes,
	}
	if item.AutoscalerType != nil {
		summary.AutoscalerType = *item.AutoscalerType
	}

	return summary
}

func toUnevictableMutedWorkload(item publicapi.UnevictableMutedWorkload) UnevictableMutedWorkload {
	workload := UnevictableMutedWorkload{
		ClusterUID: item.ClusterUid,
		ID:         item.Id,
		CreatedBy:  item.CreatedBy,
		CreateTime: item.CreateTime,
		UpdateTime: item.UpdateTime,
	}
	if item.Namespace != nil {
		workload.Namespace = *item.Namespace
	}
	if item.WorkloadName != nil {
		workload.WorkloadName = *item.WorkloadName
	}
	if item.Note != nil {
		workload.Note = *item.Note
	}

	return workload
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
