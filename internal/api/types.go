package api

import "time"

type Indicator struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Severity int    `json:"severity"`
}

type Cluster struct {
	ID        int64      `json:"id,omitempty"`
	UID       string     `json:"uid"`
	Name      string     `json:"name"`
	Cloud     string     `json:"cloud,omitempty"`
	Region    string     `json:"region,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type ClusterDetail struct {
	UID       string             `json:"uid"`
	Name      string             `json:"name"`
	Cloud     string             `json:"cloud,omitempty"`
	Region    string             `json:"region,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt *time.Time         `json:"updated_at,omitempty"`
	Emission  map[string]float64 `json:"emission,omitempty"`
}

type ClusterEmissionEntry struct {
	ClusterUID  string  `json:"cluster_uid"`
	ClusterName string  `json:"cluster_name"`
	Metric      string  `json:"metric"`
	Value       float64 `json:"value"`
}

type ReplicasCounts struct {
	MaxCount int `json:"max_count"`
	AvgCount int `json:"avg_count"`
}

type MuteStatus struct {
	IsMuted bool       `json:"is_muted"`
	Expires *time.Time `json:"expires,omitempty"`
}

type ResourceValues struct {
	MemoryRequestMiB float64 `json:"memory_request_mib"`
	MemoryLimitMiB   float64 `json:"memory_limit_mib"`
	CPURequestCores  float64 `json:"cpu_request_cores"`
	CPULimitCores    float64 `json:"cpu_limit_cores"`
}

type ContainerResources struct {
	Current     ResourceValues `json:"current"`
	Recommended ResourceValues `json:"recommended"`
}

type Percentiles struct {
	P90  float64 `json:"p90"`
	P95  float64 `json:"p95"`
	P100 float64 `json:"p100"`
}

type ContainerUsage struct {
	CPUCores  Percentiles `json:"cpu_cores"`
	MemoryMiB Percentiles `json:"memory_mib"`
}

type WorkloadContainer struct {
	Name           string             `json:"name"`
	RunningMinutes int                `json:"running_minutes"`
	Indicators     []Indicator        `json:"indicators,omitempty"`
	Resources      ContainerResources `json:"resources"`
	Usage          ContainerUsage     `json:"usage"`
}

type WorkloadDerived struct {
	ContainerCount                   int     `json:"container_count"`
	IndicatorsCount                  int     `json:"indicators_count"`
	RiskIndicatorsCount              int     `json:"risk_indicators_count"`
	WasteIndicatorsCount             int     `json:"waste_indicators_count"`
	CurrentCPURequestCoresTotal      float64 `json:"current_cpu_request_cores_total"`
	CurrentCPULimitCoresTotal        float64 `json:"current_cpu_limit_cores_total"`
	CurrentMemoryRequestMiBTotal     float64 `json:"current_memory_request_mib_total"`
	CurrentMemoryLimitMiBTotal       float64 `json:"current_memory_limit_mib_total"`
	RecommendedCPURequestCoresTotal  float64 `json:"recommended_cpu_request_cores_total"`
	RecommendedCPULimitCoresTotal    float64 `json:"recommended_cpu_limit_cores_total"`
	RecommendedMemoryRequestMiBTotal float64 `json:"recommended_memory_request_mib_total"`
	RecommendedMemoryLimitMiBTotal   float64 `json:"recommended_memory_limit_mib_total"`
	CPUUsageP90CoresSum              float64 `json:"cpu_usage_p90_cores_sum"`
	CPUUsageP95CoresSum              float64 `json:"cpu_usage_p95_cores_sum"`
	CPUUsageP100CoresSum             float64 `json:"cpu_usage_p100_cores_sum"`
	MemoryUsageP90MiBSum             float64 `json:"memory_usage_p90_mib_sum"`
	MemoryUsageP95MiBSum             float64 `json:"memory_usage_p95_mib_sum"`
	MemoryUsageP100MiBSum            float64 `json:"memory_usage_p100_mib_sum"`
}

type Workload struct {
	ID                           string              `json:"id"`
	Name                         string              `json:"name"`
	Namespace                    string              `json:"namespace"`
	Type                         string              `json:"type"`
	Period                       string              `json:"period"`
	ReplicasCounts               ReplicasCounts      `json:"replicas_counts"`
	ResilienceLevel              string              `json:"resilience_level,omitempty"`
	OptimizationPolicy           string              `json:"optimization_policy,omitempty"`
	OptimizationPolicyTimeWindow string              `json:"optimization_policy_time_window,omitempty"`
	CPUOptimizationPolicy        string              `json:"cpu_optimization_policy,omitempty"`
	MemoryOptimizationPolicy     string              `json:"memory_optimization_policy,omitempty"`
	MemoryRequestEqualsLimit     bool                `json:"memory_request_equals_limit,omitempty"`
	MuteStatus                   MuteStatus          `json:"mute_status"`
	Cost                         float64             `json:"cost"`
	Waste                        float64             `json:"waste"`
	HistoricalWaste              float64             `json:"historical_waste,omitempty"`
	CostPerHour                  float64             `json:"cost_per_hour,omitempty"`
	PotentialSavings             float64             `json:"potential_savings,omitempty"`
	CostIncrease                 float64             `json:"cost_increase,omitempty"`
	RunningMinutes               int                 `json:"running_minutes,omitempty"`
	FirstSeen                    *time.Time          `json:"first_seen,omitempty"`
	LastSeen                     *time.Time          `json:"last_seen,omitempty"`
	MaxIndicator                 *Indicator          `json:"max_indicator,omitempty"`
	Indicators                   []Indicator         `json:"indicators,omitempty"`
	WorkloadLabels               map[string]string   `json:"workload_labels,omitempty"`
	Containers                   []WorkloadContainer `json:"containers,omitempty"`
	Derived                      WorkloadDerived     `json:"derived"`
}

type NamespaceSummary struct {
	ClusterUID  string  `json:"cluster_uid"`
	ClusterName string  `json:"cluster_name"`
	Name        string  `json:"name"`
	Workloads   int     `json:"workloads"`
	TotalCost   float64 `json:"total_cost"`
	TotalWaste  float64 `json:"total_waste"`
	Period      string  `json:"period"`
}

type WorkloadSummary struct {
	ClusterUID           string  `json:"cluster_uid"`
	ClusterName          string  `json:"cluster_name"`
	Period               string  `json:"period"`
	Workloads            int     `json:"workloads"`
	Namespaces           int     `json:"namespaces"`
	Types                int     `json:"types"`
	MutedWorkloads       int     `json:"muted_workloads"`
	RiskyWorkloads       int     `json:"risky_workloads"`
	WasteWorkloads       int     `json:"waste_workloads"`
	TotalCost            float64 `json:"total_cost"`
	TotalWaste           float64 `json:"total_waste"`
	TotalPotentialSaving float64 `json:"total_potential_saving"`
	TopNamespace         string  `json:"top_namespace,omitempty"`
	TopNamespaceWaste    float64 `json:"top_namespace_waste,omitempty"`
	TopType              string  `json:"top_type,omitempty"`
	TopTypeWaste         float64 `json:"top_type_waste,omitempty"`
}

type WorkloadGroupSummary struct {
	ClusterUID     string  `json:"cluster_uid"`
	ClusterName    string  `json:"cluster_name"`
	Field          string  `json:"field"`
	Key            string  `json:"key"`
	Period         string  `json:"period"`
	Workloads      int     `json:"workloads"`
	MutedWorkloads int     `json:"muted_workloads"`
	RiskyWorkloads int     `json:"risky_workloads"`
	WasteWorkloads int     `json:"waste_workloads"`
	TotalCost      float64 `json:"total_cost"`
	TotalWaste     float64 `json:"total_waste"`
}

type WorkloadLabelSummary struct {
	ClusterUID  string  `json:"cluster_uid"`
	ClusterName string  `json:"cluster_name"`
	Period      string  `json:"period"`
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	Workloads   int     `json:"workloads"`
	TotalCost   float64 `json:"total_cost"`
	TotalWaste  float64 `json:"total_waste"`
}

type NodesCount struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
}

type PodsCount struct {
	Capacity    int     `json:"capacity"`
	Allocatable int     `json:"allocatable"`
	AvgCount    float64 `json:"avg_count"`
}

type UtilizationPercentiles struct {
	Avg  float64 `json:"avg"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	P80  float64 `json:"p80"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
	P999 float64 `json:"p999"`
}

type ResourceUtilization struct {
	IdleCores float64                `json:"idle,omitempty"`
	Requested UtilizationPercentiles `json:"requested"`
	Used      UtilizationPercentiles `json:"used"`
}

type GPUPercentiles struct {
	AvgMemoryMiB  float64 `json:"avg_memory_mib"`
	MinMemoryMiB  float64 `json:"min_memory_mib"`
	MaxMemoryMiB  float64 `json:"max_memory_mib"`
	P80MemoryMiB  float64 `json:"p80_memory_mib"`
	P95MemoryMiB  float64 `json:"p95_memory_mib"`
	P99MemoryMiB  float64 `json:"p99_memory_mib"`
	P999MemoryMiB float64 `json:"p999_memory_mib"`
	AvgUnits      float64 `json:"avg_units"`
	MinUnits      float64 `json:"min_units"`
	MaxUnits      float64 `json:"max_units"`
	P80Units      float64 `json:"p80_units"`
	P95Units      float64 `json:"p95_units"`
	P99Units      float64 `json:"p99_units"`
	P999Units     float64 `json:"p999_units"`
}

// GPUUtilization is nil for node groups/types with no GPU.
type GPUUtilization struct {
	Architectures []string       `json:"architectures,omitempty"`
	SharingType   []string       `json:"sharing_type,omitempty"`
	IdleMemoryMiB float64        `json:"idle_memory_mib,omitempty"`
	IdleUnits     float64        `json:"idle_units,omitempty"`
	Requested     GPUPercentiles `json:"requested"`
	Used          GPUPercentiles `json:"used"`
}

type NodeGroupCost struct {
	Hourly    float64 `json:"hourly"`
	Timeframe float64 `json:"timeframe"`
	IdleCPU   float64 `json:"idle_cpu"`
	IdleMem   float64 `json:"idle_mem"`
	IdleGPU   float64 `json:"idle_gpu,omitempty"`
	IdleTotal float64 `json:"idle_total"`
}

type SeenWindow struct {
	FirstTime *time.Time `json:"first_time,omitempty"`
	LastTime  *time.Time `json:"last_time,omitempty"`
}

type NodeType struct {
	ID             string              `json:"id"`
	InstanceType   string              `json:"instance_type"`
	InstanceFamily string              `json:"instance_family,omitempty"`
	IsSpot         bool                `json:"is_spot"`
	Nodes          NodesCount          `json:"nodes"`
	Pods           PodsCount           `json:"pods"`
	CPU            ResourceUtilization `json:"cpu"`
	Mem            ResourceUtilization `json:"mem"`
	GPU            *GPUUtilization     `json:"gpu,omitempty"`
	Cost           NodeGroupCost       `json:"cost"`
	RunningMinutes int                 `json:"running_minutes,omitempty"`
	Seen           SeenWindow          `json:"seen"`
}

type NodeTypeRecommendation struct {
	ID                  string  `json:"id"`
	InstanceType        string  `json:"instance_type"`
	InstanceFamily      string  `json:"instance_family,omitempty"`
	HourlyCost          float64 `json:"hourly_cost"`
	EstimatedSavings    float64 `json:"estimated_savings"`
	EstimatedSavingsPct float64 `json:"estimated_savings_pct"`
	NodeCount           int     `json:"node_count"`
}

type KarpenterChange struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Path             string `json:"path"`
	Operation        string `json:"operation"`
	CurrentValue     string `json:"current_value"`
	RecommendedValue string `json:"recommended_value"`
	Rationale        string `json:"rationale"`
}

const (
	NodeGroupRecommendationsTypeStandard  = "standard"
	NodeGroupRecommendationsTypeKarpenter = "karpenter"
)

// NodeGroupRecommendations flattens the public API's discriminated union
// (standard vs karpenter) into one struct; Type selects which fields are populated.
type NodeGroupRecommendations struct {
	Type              string                   `json:"type"`
	HasChanges        bool                     `json:"has_changes"`
	NodeTypeRecs      []NodeTypeRecommendation `json:"node_type_recommendations,omitempty"`
	Changes           []KarpenterChange        `json:"changes,omitempty"`
	CurrentConfig     map[string]any           `json:"current_config,omitempty"`
	RecommendedConfig map[string]any           `json:"recommended_config,omitempty"`
}

type NodeGroup struct {
	ID              string                   `json:"id"`
	AutoscalerType  string                   `json:"autoscaler_type"`
	Architectures   []string                 `json:"architectures,omitempty"`
	Reservations    []string                 `json:"reservations,omitempty"`
	Nodes           NodesCount               `json:"nodes"`
	Pods            PodsCount                `json:"pods"`
	RunningMinutes  int                      `json:"running_minutes,omitempty"`
	CPU             ResourceUtilization      `json:"cpu"`
	Mem             ResourceUtilization      `json:"mem"`
	GPU             *GPUUtilization          `json:"gpu,omitempty"`
	Cost            NodeGroupCost            `json:"cost"`
	Labels          map[string]string        `json:"labels,omitempty"`
	Seen            SeenWindow               `json:"seen"`
	NodeTypes       []NodeType               `json:"node_types"`
	Recommendations NodeGroupRecommendations `json:"recommendations"`
}

type CursorPagination struct {
	Next     *string `json:"next"`
	Prev     *string `json:"prev"`
	PageSize int     `json:"page_size"`
}

type NodeGroupPage struct {
	NodeGroups []NodeGroup      `json:"node_groups"`
	Pagination CursorPagination `json:"pagination"`
	Timeframe  string           `json:"timeframe"`
}
