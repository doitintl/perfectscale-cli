package cli

import (
	"fmt"

	ucli "github.com/urfave/cli/v2"
)

func clustersCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "clusters",
		Usage: "List and inspect clusters visible to the selected profile",
		Subcommands: []*ucli.Command{
			{
				Name:  "list",
				Usage: "List clusters available through the public API",
				Description: withCommandName(`This command uses the public API and the active service-token profile.

Example:
  {{cmd}} clusters list

Output schema (--output json):
  Array of:
    { "uid": string, "name": string, "cloud": string, "region": string,
      "created_at": string (RFC3339), "updated_at": string (RFC3339) }`),
				Action: runClustersList,
			},
			{
				Name:  "get",
				Usage: "Show a single cluster with carbon emission details",
				Description: withCommandName(`Examples:
  {{cmd}} clusters get -c prod-a
  {{cmd}} clusters get -c prod-a -w 30d

This command resolves the cluster by name or UID, then fetches the public cluster detail payload.

Output schema (--output json):
  { "uid": string, "name": string, "cloud": string, "region": string,
    "created_at": string (RFC3339), "updated_at": string (RFC3339),
    "period": string, "emission": map[string]float64 }`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query", Required: true},
					&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "Time window: 30d", Value: "30d"},
				},
				Action: runClustersGet,
			},
			{
				Name:  "emission",
				Usage: "List carbon emission metrics for a cluster",
				Description: withCommandName(`Examples:
  {{cmd}} clusters emission -c prod-a
  {{cmd}} clusters emission -c prod-a -s value -r desc

The public cluster detail endpoint returns a map of emission metrics that this command flattens into rows.

Output schema (--output json):
  Array of:
    { "cluster_uid": string, "cluster_name": string, "metric": string, "value": float64 }`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query", Required: true},
					&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "Time window: 30d", Value: "30d"},
					&ucli.StringFlag{Name: "sort", Aliases: []string{"s"}, Usage: "Sort by one of: metric, value", Value: "metric"},
					&ucli.StringFlag{Name: "order", Aliases: []string{"r"}, Usage: "Sort order: asc or desc", Value: "asc"},
				},
				Action: runClustersEmission,
			},
		},
	}
}

func runClustersList(c *ucli.Context) error {
	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	clusters, err := listClustersForProfile(c.Context, resources.Runtime, resources.Profile, resources.Token)
	if err != nil {
		return err
	}

	headers := []string{"NAME", "UID", "CLOUD", "REGION", "CREATED_AT", "UPDATED_AT"}
	rows := make([][]string, 0, len(clusters))
	for _, item := range clusters {
		rows = append(rows, []string{
			item.Name,
			item.UID,
			item.Cloud,
			item.Region,
			item.CreatedAt.UTC().Format(timeLayout(item.CreatedAt)),
			formatTime(item.UpdatedAt),
		})
	}

	return resources.Runtime.RenderTableOrJSON(clusters, headers, rows)
}

func runClustersGet(c *ucli.Context) error {
	normalizedPeriod, err := normalizePeriod(c.String("period"))
	if err != nil {
		return err
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	cluster, detail, err := resources.loadClusterDetail(c.Context, c.String("cluster"), normalizedPeriod)
	if err != nil {
		return err
	}

	status := map[string]any{
		"uid":        detail.UID,
		"name":       detail.Name,
		"cloud":      detail.Cloud,
		"region":     detail.Region,
		"created_at": detail.CreatedAt.UTC().Format(timeLayout(detail.CreatedAt)),
		"updated_at": formatTime(detail.UpdatedAt),
		"period":     normalizedPeriod,
		"emission":   detail.Emission,
	}

	rows := [][]string{
		{"uid", cluster.UID},
		{"name", detail.Name},
		{"cloud", detail.Cloud},
		{"region", detail.Region},
		{"created_at", detail.CreatedAt.UTC().Format(timeLayout(detail.CreatedAt))},
		{"updated_at", formatTime(detail.UpdatedAt)},
		{"period", normalizedPeriod},
	}

	emissions := clusterEmissionEntries(cluster, detail)
	sortEmissionEntries(emissions, "metric", "asc")
	for _, item := range emissions {
		rows = append(rows, []string{
			fmt.Sprintf("emission.%s", item.Metric),
			fmt.Sprintf("%.4f", item.Value),
		})
	}

	return resources.Runtime.RenderTableOrJSON(status, []string{"FIELD", "VALUE"}, rows)
}

func runClustersEmission(c *ucli.Context) error {
	normalizedPeriod, err := normalizePeriod(c.String("period"))
	if err != nil {
		return err
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	cluster, detail, err := resources.loadClusterDetail(c.Context, c.String("cluster"), normalizedPeriod)
	if err != nil {
		return err
	}

	items := clusterEmissionEntries(cluster, detail)
	sortEmissionEntries(items, c.String("sort"), c.String("order"))

	headers := []string{"CLUSTER", "METRIC", "VALUE"}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.ClusterName,
			item.Metric,
			fmt.Sprintf("%.4f", item.Value),
		})
	}

	return resources.Runtime.RenderTableOrJSON(items, headers, rows)
}

func timeLayout(value any) string {
	return "2006-01-02T15:04:05Z07:00"
}
