package cli

import (
	"fmt"

	ucli "github.com/urfave/cli/v2"
)

func namespacesCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "namespaces",
		Usage: "List namespaces in a cluster, derived from workload data",
		Subcommands: []*ucli.Command{
			{
				Name:  "list",
				Usage: "List namespaces seen in workload data for a cluster",
				Description: withCommandName(`Examples:
  {{cmd}} namespaces list -c prod-a
  {{cmd}} namespaces list -c prod-a -s workloads -r desc
  {{cmd}} namespaces list -c prod-a -n kube -T 5
  {{cmd}} namespaces list -c prod-a -w 30d

The namespace list is aggregated from workload results for the selected period.
Only --period 30d is supported because the public workloads API is fixed to 30 days.`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query", Required: true},
					&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "Time window: 30d", Value: "30d"},
					&ucli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Filter namespaces by name substring"},
					&ucli.StringFlag{Name: "sort", Aliases: []string{"s"}, Usage: "Sort by one of: name, workloads, cost, waste", Value: "name"},
					&ucli.StringFlag{Name: "order", Aliases: []string{"r"}, Usage: "Sort order: asc or desc", Value: "asc"},
					&ucli.IntFlag{Name: "top", Aliases: []string{"T"}, Usage: "Return only the first N namespaces after filtering and sorting"},
					&ucli.IntFlag{Name: "bottom", Aliases: []string{"B"}, Usage: "Return only the last N namespaces after filtering and sorting"},
				},
				Action: runNamespacesList,
			},
		},
	}
}

func runNamespacesList(c *ucli.Context) error {
	rt, err := NewRuntime(c)
	if err != nil {
		return err
	}

	data, err := rt.LoadProfile()
	if err != nil {
		return err
	}

	normalizedPeriod, err := normalizePeriod(c.String("period"))
	if err != nil {
		return err
	}

	token, err := rt.ResolveToken(c.Context, data)
	if err != nil {
		return err
	}

	clusters, err := listClustersForProfile(c.Context, rt, data, token)
	if err != nil {
		return err
	}
	cluster, err := resolveClusterByNameOrUID(clusters, c.String("cluster"))
	if err != nil {
		return err
	}

	workloads, err := fetchWorkloadsForProfile(c.Context, rt, data, token, cluster.UID, normalizedPeriod)
	if err != nil {
		return err
	}

	items := summarizeNamespaces(cluster, workloads)
	items = filterNamespaceSummaries(items, NamespaceFilters{Namespace: c.String("namespace")})
	sortNamespaceSummaries(items, c.String("sort"), c.String("order"))
	items, err = limitNamespaceSummaries(items, c.Int("top"), c.Int("bottom"))
	if err != nil {
		return err
	}

	headers := []string{"NAMESPACE", "WORKLOADS", "COST", "WASTE", "PERIOD"}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.Name,
			fmt.Sprintf("%d", item.Workloads),
			fmt.Sprintf("%.2f", item.TotalCost),
			fmt.Sprintf("%.2f", item.TotalWaste),
			item.Period,
		})
	}

	return rt.RenderTableOrJSON(items, headers, rows)
}
