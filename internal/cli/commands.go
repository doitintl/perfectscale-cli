package cli

import (
	"strconv"
	"strings"

	"github.com/perfectscale/poc-cli/internal/config"
	"github.com/perfectscale/poc-cli/internal/output"
	ucli "github.com/urfave/cli/v2"
)

const commandCatalogSchemaVersion = "1"

type commandCatalog struct {
	Version    string           `json:"version"`
	CLIVersion string           `json:"cli_version"`
	Commands   []catalogCommand `json:"commands"`
}

type catalogCommand struct {
	Path        []string      `json:"path"`
	Usage       string        `json:"usage"`
	Description string        `json:"description,omitempty"`
	Runnable    bool          `json:"runnable"`
	Flags       []catalogFlag `json:"flags,omitempty"`
}

type catalogFlag struct {
	Name     string   `json:"name"`
	Aliases  []string `json:"aliases,omitempty"`
	Type     string   `json:"type"`
	Usage    string   `json:"usage,omitempty"`
	Default  string   `json:"default"`
	EnvVars  []string `json:"env_vars,omitempty"`
	Required bool     `json:"required"`
	Runtime  bool     `json:"runtime"`
}

func commandsCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "commands",
		Usage: "Print a machine-readable catalog of commands and flags",
		Description: withCommandName(`Walks the CLI command tree and emits command paths, descriptions, flags,
and short-flag aliases. Intended for agent tooling and for checking that
docs stay aligned with the real command surface. Does not call the API.

Examples:
  {{cmd}} commands -o json
  {{cmd}} commands -o jsonl
  {{cmd}} commands

Output:
  table prints PATH and USAGE.
  -o json prints one document with version, cli_version, and a commands array.
  Each command has path, usage, description, runnable, and flags.
  Each flag has name, aliases (short flags), type, usage, default, env_vars,
  required, and runtime (true for profile/output/debug/public-api-url).
  -o jsonl prints one command object per line.`),
		Action: runCommands,
	}
}

func runCommands(c *ucli.Context) error {
	outputMode, err := config.NormalizeOutput(stringFlagValue(c, "output"))
	if err != nil {
		return err
	}

	catalog := buildCommandCatalog(c.App)
	writer := c.App.Writer

	switch outputMode {
	case "json":
		return output.WriteJSON(writer, catalog, false)
	case "jsonl":
		values := make([]any, 0, len(catalog.Commands))
		for _, command := range catalog.Commands {
			values = append(values, command)
		}

		return output.WriteJSONL(writer, values)
	default:
		return output.WriteTable(writer, []string{"PATH", "USAGE"}, catalogTableRows(catalog.Commands))
	}
}

func buildCommandCatalog(app *ucli.App) commandCatalog {
	return commandCatalog{
		Version:    commandCatalogSchemaVersion,
		CLIVersion: catalogCLIVersion(app),
		Commands:   collectCatalogCommands(app.Commands, nil, runtimeFlagNameSet()),
	}
}

func catalogCLIVersion(app *ucli.App) string {
	if app != nil && app.Metadata != nil {
		if ver, ok := app.Metadata["version"].(string); ok && ver != "" {
			return ver
		}
	}

	return "dev"
}

func runtimeFlagNameSet() map[string]struct{} {
	names := make(map[string]struct{}, 4)
	for _, flag := range runtimeFlags() {
		flagNames := flag.Names()
		if len(flagNames) == 0 {
			continue
		}

		names[flagNames[0]] = struct{}{}
	}

	return names
}

func collectCatalogCommands(
	commands []*ucli.Command,
	prefix []string,
	runtimeNames map[string]struct{},
) []catalogCommand {
	out := make([]catalogCommand, 0)

	for _, command := range visibleCatalogCommands(commands) {
		path := append(append([]string{}, prefix...), command.Name)
		entry := catalogCommand{
			Path:        path,
			Usage:       command.Usage,
			Description: strings.TrimSpace(command.Description),
			Runnable:    command.Action != nil,
			Flags:       catalogFlags(command.VisibleFlags(), runtimeNames),
		}
		out = append(out, entry)
		out = append(out, collectCatalogCommands(command.VisibleCommands(), path, runtimeNames)...)
	}

	return out
}

func visibleCatalogCommands(commands []*ucli.Command) []*ucli.Command {
	out := make([]*ucli.Command, 0, len(commands))
	for _, command := range commands {
		if command == nil || command.Hidden {
			continue
		}

		if command.Name == "help" || command.Name == "h" {
			continue
		}

		out = append(out, command)
	}

	return out
}

func catalogFlags(flags []ucli.Flag, runtimeNames map[string]struct{}) []catalogFlag {
	out := make([]catalogFlag, 0, len(flags))
	for _, flag := range flags {
		if skipCatalogFlag(flag) {
			continue
		}

		out = append(out, catalogFlagFrom(flag, runtimeNames))
	}

	return out
}

func skipCatalogFlag(flag ucli.Flag) bool {
	if flag == nil {
		return true
	}

	names := flag.Names()
	if len(names) == 0 {
		return true
	}

	switch names[0] {
	case "help", "version":
		return true
	}

	return isHiddenFlag(flag)
}

func isHiddenFlag(flag ucli.Flag) bool {
	switch f := flag.(type) {
	case *ucli.StringFlag:
		return f.Hidden
	case *ucli.BoolFlag:
		return f.Hidden
	case *ucli.IntFlag:
		return f.Hidden
	case *ucli.Float64Flag:
		return f.Hidden
	case *ucli.StringSliceFlag:
		return f.Hidden
	default:
		return false
	}
}

func catalogFlagFrom(flag ucli.Flag, runtimeNames map[string]struct{}) catalogFlag {
	names := flag.Names()
	cf := catalogFlag{
		Name: names[0],
	}
	if len(names) > 1 {
		cf.Aliases = names[1:]
	}

	_, cf.Runtime = runtimeNames[cf.Name]

	switch f := flag.(type) {
	case *ucli.StringFlag:
		cf.Type = "string"
		cf.Usage = f.Usage
		cf.Default = f.Value
		cf.EnvVars = f.EnvVars
		cf.Required = f.Required
	case *ucli.BoolFlag:
		cf.Type = "bool"
		cf.Usage = f.Usage
		cf.Default = strconv.FormatBool(f.Value)
		cf.EnvVars = f.EnvVars
		cf.Required = f.Required
	case *ucli.IntFlag:
		cf.Type = "int"
		cf.Usage = f.Usage
		cf.Default = strconv.Itoa(f.Value)
		cf.EnvVars = f.EnvVars
		cf.Required = f.Required
	case *ucli.Float64Flag:
		cf.Type = "float64"
		cf.Usage = f.Usage
		cf.Default = strconv.FormatFloat(f.Value, 'f', -1, 64)
		cf.EnvVars = f.EnvVars
		cf.Required = f.Required
	case *ucli.StringSliceFlag:
		cf.Type = "string_slice"
		cf.Usage = f.Usage
		cf.EnvVars = f.EnvVars
		cf.Required = f.Required
		if f.Value != nil {
			cf.Default = strings.Join(f.Value.Value(), ",")
		}
	default:
		cf.Type = "flag"
		if doc, ok := flag.(ucli.DocGenerationFlag); ok {
			cf.Usage = doc.GetUsage()
			cf.Default = doc.GetValue()
			cf.EnvVars = doc.GetEnvVars()
		}
	}

	return cf
}

func catalogTableRows(commands []catalogCommand) [][]string {
	rows := make([][]string, 0, len(commands))
	for _, command := range commands {
		rows = append(rows, []string{
			strings.Join(command.Path, " "),
			command.Usage,
		})
	}

	return rows
}
