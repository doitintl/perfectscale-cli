package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCommandsJSONCatalogIncludesPathsAndShortFlags(t *testing.T) {
	raw, err := runCLI(t, nil, "commands", "-o", "json")
	if err != nil {
		t.Fatalf("runCLI(commands -o json) error = %v", err)
	}

	var catalog commandCatalog
	if err := json.Unmarshal([]byte(raw), &catalog); err != nil {
		t.Fatalf("unmarshal catalog: %v\nraw=%s", err, raw)
	}

	if catalog.Version != commandCatalogSchemaVersion {
		t.Fatalf("version = %q, want %q", catalog.Version, commandCatalogSchemaVersion)
	}

	if catalog.CLIVersion != "test" {
		t.Fatalf("cli_version = %q, want test", catalog.CLIVersion)
	}

	list, ok := findCatalogCommand(catalog.Commands, "workloads", "list")
	if !ok {
		t.Fatal("catalog missing path workloads list")
	}

	if !list.Runnable {
		t.Fatal("workloads list should be runnable")
	}

	cluster, ok := findCatalogFlag(list.Flags, "cluster")
	if !ok {
		t.Fatal("workloads list missing cluster flag")
	}

	if !containsString(cluster.Aliases, "c") {
		t.Fatalf("cluster aliases = %v, want c", cluster.Aliases)
	}

	if cluster.Runtime {
		t.Fatal("cluster should not be a runtime flag")
	}

	if cluster.Type != "string" {
		t.Fatalf("cluster type = %q, want string", cluster.Type)
	}

	profileFlag, ok := findCatalogFlag(list.Flags, "profile")
	if !ok {
		t.Fatal("workloads list missing runtime profile flag")
	}

	if !profileFlag.Runtime {
		t.Fatal("profile should be tagged runtime")
	}

	group, ok := findCatalogCommand(catalog.Commands, "workloads", "group-by")
	if !ok {
		t.Fatal("catalog missing path workloads group-by")
	}

	if group.Runnable {
		t.Fatal("workloads group-by should not be runnable")
	}

	self, ok := findCatalogCommand(catalog.Commands, "commands")
	if !ok {
		t.Fatal("catalog missing commands itself")
	}

	if _, ok := findCatalogFlag(self.Flags, "json"); ok {
		t.Fatal("commands should not have a --json flag; use -o json")
	}

	for _, command := range catalog.Commands {
		for _, part := range command.Path {
			if part == "help" || part == "h" {
				t.Fatalf("catalog includes urfave help command %v", command.Path)
			}
		}
	}
}

func TestCommandsOutputJSONBeforeSubcommand(t *testing.T) {
	raw, err := runCLI(t, nil, "-o", "json", "commands")
	if err != nil {
		t.Fatalf("runCLI(-o json commands) error = %v", err)
	}

	var catalog commandCatalog
	if err := json.Unmarshal([]byte(raw), &catalog); err != nil {
		t.Fatalf("unmarshal catalog: %v\nraw=%s", err, raw)
	}

	if catalog.Version != commandCatalogSchemaVersion {
		t.Fatalf("version = %q, want %q", catalog.Version, commandCatalogSchemaVersion)
	}
}

func TestCommandsJSONLEmitsOneCommandPerLine(t *testing.T) {
	raw, err := runCLI(t, nil, "commands", "-o", "jsonl")
	if err != nil {
		t.Fatalf("runCLI(commands -o jsonl) error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) < 10 {
		t.Fatalf("jsonl line count = %d, want at least 10\nraw=%s", len(lines), raw)
	}

	var first catalogCommand
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal first jsonl line: %v\nline=%s", err, lines[0])
	}

	if len(first.Path) == 0 {
		t.Fatal("jsonl command missing path")
	}
}

func TestCommandsTableListsPaths(t *testing.T) {
	raw, err := runCLI(t, nil, "commands")
	if err != nil {
		t.Fatalf("runCLI(commands) error = %v", err)
	}

	assertContains(t, raw, "PATH")
	assertContains(t, raw, "USAGE")
	assertContains(t, raw, "workloads list")
	assertContains(t, raw, "commands")
}

func findCatalogCommand(commands []catalogCommand, path ...string) (catalogCommand, bool) {
	want := strings.Join(path, "\x00")
	for _, command := range commands {
		if strings.Join(command.Path, "\x00") == want {
			return command, true
		}
	}

	return catalogCommand{}, false
}

func findCatalogFlag(flags []catalogFlag, name string) (catalogFlag, bool) {
	for _, flag := range flags {
		if flag.Name == name {
			return flag, true
		}
	}

	return catalogFlag{}, false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}

	return false
}
