package cli

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/perfectscale/poc-cli/internal/clierr"
	"github.com/perfectscale/poc-cli/internal/output"
	ucli "github.com/urfave/cli/v2"
)

// appBaseURL is the Perfectscale frontend host. Unlike the public API URL
// (-u), this isn't overridable: pscli only ever deep-links to the prod UI.
const appBaseURL = "https://app.perfectscale.io"

func openCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "open",
		Usage: "Open a Perfectscale resource page in your default browser",
		Subcommands: []*ucli.Command{
			{
				Name:  "cluster",
				Usage: "Open a cluster's PodFit overview page",
				Description: withCommandName(`Examples:
  {{cmd}} open cluster -c prod-a
  {{cmd}} open cluster -c prod-a -w 7d

Resolves --cluster (-c) by name or UID via the public API, then opens the
matching PodFit page on app.perfectscale.io in your default browser.

-o json/jsonl prints {"url": "..."} instead of opening a browser.`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to open"},
					&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "UI time window shown on the opened page", Value: "30d"},
				},
				Action: runOpenCluster,
			},
			{
				Name:  "workload",
				Usage: "Open a workload's PodFit zoom-in page",
				Description: withCommandName(`Examples:
  {{cmd}} open workload -c prod-a -i workload-123
  {{cmd}} open workload -c prod-a -m my-deployment -n my-namespace

Resolves --cluster (-c) and the workload (by --id/-i or --name/-m, optionally
disambiguated with --namespace/-n — same rules as "workloads show") via the
public API, then opens the matching PodFit zoom-in page in your default
browser. --period (-w) only sets the UI time window on the opened page; the
workload lookup itself always uses the API's fixed 30-day window.

-o json/jsonl prints {"url": "..."} instead of opening a browser.`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query"},
					&ucli.StringFlag{Name: "id", Aliases: []string{"i"}, Usage: "Exact workload ID"},
					&ucli.StringFlag{Name: "name", Aliases: []string{"m"}, Usage: "Exact workload name"},
					&ucli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Optional namespace to disambiguate --name"},
					&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "UI time window shown on the opened page", Value: "30d"},
				},
				Action: runOpenWorkload,
			},
			{
				Name:  "nodegroup",
				Usage: "Open a node group's InfraFit detail page",
				Description: withCommandName(`Examples:
  {{cmd}} open nodegroup -c prod-a -g clickhouse

Resolves --cluster (-c) by name or UID via the public API, then opens the
matching InfraFit node group page in your default browser. The node group
name (-g) itself is not validated against the API.

-o json/jsonl prints {"url": "..."} instead of opening a browser.`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to query"},
					&ucli.StringFlag{Name: "node-group", Aliases: []string{"g"}, Usage: "Node group name"},
					&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "UI time window shown on the opened page", Value: "30d"},
				},
				Action: runOpenNodegroup,
			},
			{
				Name:  "alerts",
				Usage: "Open the Alerts/Resilience page for a cluster",
				Description: withCommandName(`Examples:
  {{cmd}} open alerts -c prod-a

Resolves --cluster (-c) by name or UID via the public API, then opens the
matching Alerts page (Resilience tab) in your default browser.

-o json/jsonl prints {"url": "..."} instead of opening a browser.`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name or UID to open"},
				},
				Action: runOpenAlerts,
			},
			{
				Name:  "automation",
				Usage: "Open the automation audit log, optionally pre-filtered",
				Description: withCommandName(`Examples:
  {{cmd}} open automation
  {{cmd}} open automation -c prod-a -n my-namespace -m my-deployment -t Deployment --container exporter

All filters are optional and are passed straight through as the audit log
page's own filter query params — unlike open cluster/workload/nodegroup,
--cluster (-c) here is not resolved via the public API, so it must be typed
exactly as it appears in Perfectscale (name or UID, whichever the UI shows).

-o json/jsonl prints {"url": "..."} instead of opening a browser.`),
				Flags: []ucli.Flag{
					&ucli.StringFlag{Name: "cluster", Aliases: []string{"c"}, Usage: "Cluster name filter (not resolved via the API)"},
					&ucli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Namespace filter"},
					&ucli.StringFlag{Name: "name", Aliases: []string{"m"}, Usage: "Workload name filter"},
					&ucli.StringFlag{Name: "type", Aliases: []string{"t"}, Usage: "Workload type filter"},
					&ucli.StringFlag{Name: "container", Usage: "Container name filter"},
					&ucli.StringFlag{Name: "period", Aliases: []string{"w"}, Usage: "UI time window shown on the opened page", Value: "30d"},
				},
				Action: runOpenAutomation,
			},
		},
	}
}

func runOpenCluster(c *ucli.Context) error {
	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	cluster, err := resources.resolveCluster(c.Context, c.String("cluster"))
	if err != nil {
		return err
	}

	target := buildAppURL(fmt.Sprintf("/pod-fit/%s", url.PathEscape(cluster.UID)), url.Values{
		"period": {periodValue(c)},
	})

	return finishOpen(resources.Runtime, target)
}

func runOpenAlerts(c *ucli.Context) error {
	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	cluster, err := resources.resolveCluster(c.Context, c.String("cluster"))
	if err != nil {
		return err
	}

	target := buildAppURL("/alerts", url.Values{"cluster": {cluster.UID}})

	return finishOpen(resources.Runtime, target)
}

func runOpenWorkload(c *ucli.Context) error {
	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	// The fetch period is fixed at "30d" (the only window the public
	// workloads API supports) independently of --period, which only
	// controls the UI window on the opened page.
	cluster, workloads, err := resources.loadWorkloads(c.Context, c.String("cluster"), "30d")
	if err != nil {
		return err
	}

	workload, err := resolveWorkload(workloads, c.String("id"), c.String("name"), c.String("namespace"))
	if err != nil {
		return err
	}

	target := buildAppURL(fmt.Sprintf("/pod-fit/%s/zoom_in/%s", url.PathEscape(cluster.UID), url.PathEscape(workload.ID)), url.Values{
		"period": {periodValue(c)},
	})

	return finishOpen(resources.Runtime, target)
}

func runOpenNodegroup(c *ucli.Context) error {
	nodeGroupName := strings.TrimSpace(c.String("node-group"))
	if nodeGroupName == "" {
		return clierr.Usage("--node-group (-g) is required")
	}

	resources, err := loadCommandResources(c)
	if err != nil {
		return err
	}

	cluster, err := resources.resolveCluster(c.Context, c.String("cluster"))
	if err != nil {
		return err
	}

	target := buildAppURL(fmt.Sprintf("/infra-fit/%s/node-groups/%s", url.PathEscape(cluster.UID), url.PathEscape(nodeGroupName)), url.Values{
		"nodeMode":               {"node_detailed"},
		"period":                 {periodValue(c)},
		"selectedGroupName":      {nodeGroupName},
		"utilization":            {"Avg"},
		"workloadsChartInterval": {"1d"},
	})

	return finishOpen(resources.Runtime, target)
}

func runOpenAutomation(c *ucli.Context) error {
	rt, err := NewRuntime(c)
	if err != nil {
		return err
	}

	query := url.Values{"period": {periodValue(c)}}
	setIfNotEmpty(query, "cluster_name", c.String("cluster"))
	setIfNotEmpty(query, "namespace", c.String("namespace"))
	setIfNotEmpty(query, "workload_name", c.String("name"))
	setIfNotEmpty(query, "workload_type", c.String("type"))
	setIfNotEmpty(query, "container_name", c.String("container"))

	target := buildAppURL("/automation", query)

	return finishOpen(rt, target)
}

func setIfNotEmpty(query url.Values, key string, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		query.Set(key, trimmed)
	}
}

func periodValue(c *ucli.Context) string {
	if trimmed := strings.TrimSpace(c.String("period")); trimmed != "" {
		return trimmed
	}
	return "30d"
}

func buildAppURL(path string, query url.Values) string {
	u := url.URL{Scheme: "https", Host: strings.TrimPrefix(appBaseURL, "https://"), Path: path}
	u.RawQuery = query.Encode()
	return u.String()
}

// finishOpen prints the target URL and, in table mode, opens it in the
// default browser. -o json/jsonl only print {"url": ...} — an agent or
// script has no browser to open, so opening one would be an unwanted side
// effect of asking for machine-readable output.
func finishOpen(rt *Runtime, target string) error {
	writer := rt.Writer
	if writer == nil {
		writer = os.Stdout
	}

	if rt.Config.Output == "json" || rt.Config.Output == "jsonl" {
		return output.WriteJSON(writer, map[string]string{"url": target}, rt.Config.Output == "jsonl")
	}

	fmt.Fprintf(writer, "Opening %s\n", target)
	if err := openInBrowser(target); err != nil {
		return fmt.Errorf("could not open browser: %w", err)
	}
	return nil
}

func openInBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
