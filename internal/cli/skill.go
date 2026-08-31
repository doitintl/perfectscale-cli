package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/perfectscale/poc-cli/internal/clierr"
	"github.com/perfectscale/poc-cli/internal/config"
	"github.com/perfectscale/poc-cli/internal/output"
	"github.com/perfectscale/poc-cli/internal/skill"
	ucli "github.com/urfave/cli/v2"
)

const (
	embeddedSkillRoot = "perfectscale"
	skillFolderName   = "perfectscale"
	skillManifestName = ".pscli-skill-manifest.json"
	skillManifestVer  = 1
)

var userHomeDir = os.UserHomeDir

type skillAgent struct {
	Name      string
	DetectRel string
	DestRel   string
}

var skillAgents = []skillAgent{
	{Name: "claude", DetectRel: ".claude", DestRel: filepath.Join(".claude", "skills", skillFolderName)},
	{Name: "codex", DetectRel: ".codex", DestRel: filepath.Join(".agents", "skills", skillFolderName)},
	{Name: "cursor", DetectRel: ".cursor", DestRel: filepath.Join(".cursor", "skills", skillFolderName)},
	{Name: "gemini", DetectRel: ".gemini", DestRel: filepath.Join(".gemini", "skills", skillFolderName)},
	{Name: "kiro", DetectRel: ".kiro", DestRel: filepath.Join(".kiro", "skills", skillFolderName)},
	{
		Name:      "opencode",
		DetectRel: filepath.Join(".config", "opencode"),
		DestRel:   filepath.Join(".config", "opencode", "skills", skillFolderName),
	},
}

type skillManifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

type skillDiff struct {
	Changed []string
	Missing []string
	Extra   []string
}

type skillFileInfo struct {
	Path            string `json:"path"`
	Bytes           int    `json:"bytes"`
	EstimatedTokens int    `json:"estimated_tokens"`
}

type skillInstallResult struct {
	Agent   string   `json:"agent"`
	Path    string   `json:"path"`
	Backups []string `json:"backups,omitempty"`
	Updated bool     `json:"updated,omitempty"`
}

type skillTarget struct {
	agent skillAgent
	dest  string
}

func skillCommand() *ucli.Command {
	agentCommands := make([]*ucli.Command, 0, len(skillAgents)+2)
	for _, configured := range skillAgents {
		agent := configured
		agentCommands = append(agentCommands, &ucli.Command{
			Name:  agent.Name,
			Usage: "Install the Perfectscale skill for " + agent.Name,
			Description: withCommandName(`Installs the embedded Perfectscale skill into this agent's user-level
skills directory.

Examples:
  {{cmd}} skill ` + agent.Name + `
  {{cmd}} skill ` + agent.Name + ` --dir /tmp/agent-home --force`),
			Flags:  skillTargetFlags(),
			Action: func(c *ucli.Context) error { return runSkillInstallAgent(c, agent) },
		})
	}

	agentCommands = append(agentCommands,
		&ucli.Command{
			Name:  "list",
			Usage: "List embedded skill files and token estimates",
			Description: withCommandName(`Lists the skill files compiled into this {{cmd}} binary.

Examples:
  {{cmd}} skill list
  {{cmd}} skill list -o json`),
			Action: runSkillList,
		},
		&ucli.Command{
			Name:      "update",
			Usage:     "Refresh installed skill files from this CLI version",
			ArgsUsage: "[agent]",
			Description: withCommandName(`Rewrites managed skill files from the copy embedded in this {{cmd}} binary.
With no agent, updates every installation that already exists.

Examples:
  {{cmd}} skill update
  {{cmd}} skill update cursor
  {{cmd}} skill update cursor --force`),
			Flags:  skillTargetFlags(),
			Action: runSkillUpdate,
		},
	)

	return &ucli.Command{
		Name:  "skill",
		Usage: "Install the Perfectscale skill for coding agents",
		Description: withCommandName(`Copies the Perfectscale agent skill (SKILL.md and supporting files) from
this {{cmd}} binary into an agent's user-level skills directory. No network
call; content is compiled in at build time.

Agents: claude, codex, cursor, gemini, kiro, opencode.

Examples:
  {{cmd}} skill cursor
  {{cmd}} skill --all
  {{cmd}} skill list
  {{cmd}} skill update

Output:
  table prints the install path (and backup paths if --force saved local edits).
  -o json/jsonl prints {agent, path, backups}.`),
		Flags: []ucli.Flag{
			&ucli.BoolFlag{
				Name:  "all",
				Usage: "Install into every detected agent directory",
			},
			&ucli.BoolFlag{
				Name:  "force",
				Usage: "Back up and overwrite locally edited skill files",
			},
		},
		Subcommands: agentCommands,
		Action:      runSkill,
	}
}

func skillTargetFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.StringFlag{
			Name:  "dir",
			Usage: "Override the agent configuration directory",
		},
		&ucli.BoolFlag{
			Name:  "force",
			Usage: "Back up and overwrite locally edited skill files",
		},
	}
}

func runSkill(c *ucli.Context) error {
	force := boolFlagValue(c, "force")
	all := boolFlagValue(c, "all")
	if force && !all {
		return clierr.Usage("--force requires --all or a named agent")
	}

	if !all {
		return ucli.ShowSubcommandHelp(c)
	}

	home, err := userHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	targets, err := detectedSkillTargets(home, false)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		return clierr.Usage("no supported agent directories were detected")
	}

	results := make([]skillInstallResult, 0, len(targets))
	for _, target := range targets {
		result, err := installSkillAt(target.agent, target.dest, force, false)
		if err != nil {
			return fmt.Errorf("install %s skill: %w", target.agent.Name, err)
		}

		results = append(results, result)
	}

	return writeSkillInstallOutput(c, results)
}

func runSkillInstallAgent(c *ucli.Context, agent skillAgent) error {
	dest, err := resolveSkillDest(agent, stringFlagValue(c, "dir"))
	if err != nil {
		return err
	}

	result, err := installSkillAt(agent, dest, boolFlagValue(c, "force"), false)
	if err != nil {
		return err
	}

	return writeSkillInstallOutput(c, []skillInstallResult{result})
}

func runSkillUpdate(c *ucli.Context) error {
	args := c.Args().Slice()
	dirOverride := stringFlagValue(c, "dir")
	if len(args) > 1 {
		return clierr.Usage("skill update accepts at most one agent name")
	}

	home, err := userHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	var targets []skillTarget
	if len(args) == 0 {
		if strings.TrimSpace(dirOverride) != "" {
			return clierr.Usage("--dir requires an agent argument")
		}

		targets, err = detectedSkillTargets(home, true)
		if err != nil {
			return err
		}

		if len(targets) == 0 {
			return clierr.Usage("no supported agent directories were detected")
		}
	} else {
		agent, ok := skillAgentByName(args[0])
		if !ok {
			return clierr.Usage("unknown agent %q", args[0])
		}

		dest, destErr := resolveSkillDest(agent, dirOverride)
		if destErr != nil {
			return destErr
		}

		if _, statErr := os.Stat(dest); statErr != nil {
			if os.IsNotExist(statErr) {
				return clierr.NotFound("skill is not installed for %s; run %s skill %s first",
					agent.Name, config.BinaryName, agent.Name)
			}

			return statErr
		}

		targets = []skillTarget{{agent: agent, dest: dest}}
	}

	force := boolFlagValue(c, "force")
	results := make([]skillInstallResult, 0, len(targets))
	for _, target := range targets {
		result, err := installSkillAt(target.agent, target.dest, force, true)
		if err != nil {
			return fmt.Errorf("update %s skill: %w", target.agent.Name, err)
		}

		results = append(results, result)
	}

	return writeSkillInstallOutput(c, results)
}

func runSkillList(c *ucli.Context) error {
	files, err := embeddedSkillFiles()
	if err != nil {
		return err
	}

	outputMode, err := config.NormalizeOutput(stringFlagValue(c, "output"))
	if err != nil {
		return err
	}

	writer := c.App.Writer
	switch outputMode {
	case "json":
		return output.WriteJSON(writer, map[string]any{"files": files}, false)
	case "jsonl":
		values := make([]any, 0, len(files))
		for _, file := range files {
			values = append(values, file)
		}

		return output.WriteJSONL(writer, values)
	default:
		rows := make([][]string, 0, len(files))
		for _, file := range files {
			rows = append(rows, []string{
				file.Path,
				strconv.Itoa(file.Bytes),
				strconv.Itoa(file.EstimatedTokens),
			})
		}

		return output.WriteTable(writer, []string{"PATH", "BYTES", "ESTIMATED TOKENS"}, rows)
	}
}

func skillDestination(agent skillAgent, home, dirOverride string) string {
	if strings.TrimSpace(dirOverride) != "" {
		return filepath.Join(filepath.Clean(dirOverride), "skills", skillFolderName)
	}

	return filepath.Join(home, agent.DestRel)
}

func resolveSkillDest(agent skillAgent, dirOverride string) (string, error) {
	if strings.TrimSpace(dirOverride) != "" {
		return skillDestination(agent, "", dirOverride), nil
	}

	home, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	return skillDestination(agent, home, ""), nil
}

func skillAgentByName(name string) (skillAgent, bool) {
	for _, agent := range skillAgents {
		if agent.Name == name {
			return agent, true
		}
	}

	return skillAgent{}, false
}

func detectedSkillTargets(home string, installedOnly bool) ([]skillTarget, error) {
	result := make([]skillTarget, 0)
	for _, agent := range skillAgents {
		dest := skillDestination(agent, home, "")
		probe := filepath.Join(home, agent.DetectRel)
		if installedOnly {
			probe = dest
		}

		info, err := os.Stat(probe)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, err
		}

		if !info.IsDir() {
			continue
		}

		result = append(result, skillTarget{agent: agent, dest: dest})
	}

	return result, nil
}

func installSkillAt(agent skillAgent, dest string, force, update bool) (skillInstallResult, error) {
	_, err := os.Stat(dest)
	fresh := os.IsNotExist(err)
	if err != nil && !fresh {
		return skillInstallResult{}, err
	}

	var backups []string
	if !fresh {
		diff, inspectErr := inspectInstalledSkill(dest)
		if inspectErr != nil {
			return skillInstallResult{}, inspectErr
		}

		if diff.hasLocalChanges() && !force {
			return skillInstallResult{}, clierr.Usage(
				"installed skill has local changes to %s; inspect them and re-run with --force to overwrite",
				strings.Join(diff.localChangePaths(), ", "))
		}

		if force && len(diff.Changed) > 0 {
			backups, err = backupChangedSkillFiles(dest, diff.Changed)
			if err != nil {
				return skillInstallResult{}, err
			}
		}
	}

	if err := writeEmbeddedSkill(dest); err != nil {
		return skillInstallResult{}, err
	}

	return skillInstallResult{
		Agent:   agent.Name,
		Path:    dest,
		Backups: backups,
		Updated: update && !fresh,
	}, nil
}

func writeEmbeddedSkill(dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	manifest := skillManifest{Version: skillManifestVer, Files: map[string]string{}}
	err := fs.WalkDir(skill.FS, embeddedSkillRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(embeddedSkillRoot, path)
		if err != nil {
			return err
		}

		if relativePath == "." {
			return nil
		}

		destination := filepath.Join(dest, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}

		data, err := skill.FS.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}

		manifest.Files[filepath.ToSlash(relativePath)] = skillFileDigest(data)

		return os.WriteFile(destination, data, 0o644)
	})
	if err != nil {
		return err
	}

	return writeSkillManifest(dest, manifest)
}

func backupChangedSkillFiles(dest string, changed []string) ([]string, error) {
	backupRoot, err := os.MkdirTemp(filepath.Dir(dest), ".pscli-skill-backup-*")
	if err != nil {
		return nil, err
	}

	backups := make([]string, 0, len(changed))
	for _, relativePath := range changed {
		path := filepath.Join(dest, relativePath)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		backupPath := filepath.Join(backupRoot, relativePath)
		if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
			return nil, err
		}

		if err := os.WriteFile(backupPath, data, 0o600); err != nil {
			return nil, err
		}

		backups = append(backups, backupPath)
	}

	return backups, nil
}

func writeSkillManifest(root string, manifest skillManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')

	return os.WriteFile(filepath.Join(root, skillManifestName), data, 0o644)
}

func readSkillManifest(root string) (skillManifest, bool, error) {
	data, err := os.ReadFile(filepath.Join(root, skillManifestName))
	if os.IsNotExist(err) {
		return skillManifest{}, false, nil
	}

	if err != nil {
		return skillManifest{}, false, err
	}

	var manifest skillManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return skillManifest{}, false, err
	}

	if manifest.Version != skillManifestVer || manifest.Files == nil {
		return skillManifest{}, false, fmt.Errorf("unsupported skill manifest version %d", manifest.Version)
	}

	return manifest, true, nil
}

func skillFileDigest(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func embeddedSkillFiles() ([]skillFileInfo, error) {
	files := make([]skillFileInfo, 0)
	err := fs.WalkDir(skill.FS, embeddedSkillRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		data, err := skill.FS.ReadFile(path)
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(embeddedSkillRoot, path)
		if err != nil {
			return err
		}

		files = append(files, skillFileInfo{
			Path:            filepath.ToSlash(relativePath),
			Bytes:           len(data),
			EstimatedTokens: (len(data) + 3) / 4,
		})

		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	return files, err
}

func inspectInstalledSkill(root string) (skillDiff, error) {
	embedded := map[string][]byte{}
	err := fs.WalkDir(skill.FS, embeddedSkillRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(embeddedSkillRoot, path)
		if err != nil {
			return err
		}

		data, err := skill.FS.ReadFile(path)
		if err != nil {
			return err
		}

		embedded[filepath.ToSlash(filepath.Clean(relativePath))] = data

		return nil
	})
	if err != nil {
		return skillDiff{}, err
	}

	manifest, hasManifest, err := readSkillManifest(root)
	if err != nil {
		return skillDiff{}, err
	}

	baseline := manifest.Files
	if !hasManifest {
		baseline = make(map[string]string, len(embedded))
		for relativePath, data := range embedded {
			baseline[relativePath] = skillFileDigest(data)
		}
	}

	result := skillDiff{}
	for relativePath, expectedDigest := range baseline {
		actual, readErr := os.ReadFile(filepath.Join(root, relativePath))
		if os.IsNotExist(readErr) {
			result.Missing = append(result.Missing, filepath.ToSlash(relativePath))
			continue
		}

		if readErr != nil {
			return skillDiff{}, readErr
		}

		if skillFileDigest(actual) != expectedDigest {
			result.Changed = append(result.Changed, filepath.ToSlash(relativePath))
		}
	}

	if _, statErr := os.Stat(root); statErr == nil {
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() {
				return nil
			}

			relativePath, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}

			relativePath = filepath.ToSlash(filepath.Clean(relativePath))
			if relativePath == skillManifestName {
				return nil
			}

			if _, exists := baseline[relativePath]; !exists {
				result.Extra = append(result.Extra, relativePath)
			}

			return nil
		})
		if err != nil {
			return skillDiff{}, err
		}
	}

	sort.Strings(result.Changed)
	sort.Strings(result.Missing)
	sort.Strings(result.Extra)

	return result, nil
}

func (diff skillDiff) hasLocalChanges() bool {
	return len(diff.Changed) > 0 || len(diff.Missing) > 0
}

func (diff skillDiff) localChangePaths() []string {
	paths := append([]string{}, diff.Changed...)
	paths = append(paths, diff.Missing...)
	sort.Strings(paths)

	return paths
}

func writeSkillInstallOutput(c *ucli.Context, results []skillInstallResult) error {
	outputMode, err := config.NormalizeOutput(stringFlagValue(c, "output"))
	if err != nil {
		return err
	}

	writer := c.App.Writer
	switch outputMode {
	case "json":
		if len(results) == 1 {
			return output.WriteJSON(writer, results[0], false)
		}

		return output.WriteJSON(writer, results, false)
	case "jsonl":
		values := make([]any, 0, len(results))
		for _, result := range results {
			values = append(values, result)
		}

		return output.WriteJSONL(writer, values)
	default:
		for _, result := range results {
			for _, backup := range result.Backups {
				fmt.Fprintf(writer, "Local edits backed up to %s\n", backup)
			}

			switch {
			case result.Updated:
				fmt.Fprintf(writer, "Skill updated for %s at %s\n", result.Agent, result.Path)
			case len(results) > 1:
				fmt.Fprintf(writer, "Skill installed for %s at %s\n", result.Agent, result.Path)
			default:
				fmt.Fprintf(writer, "Skill installed to %s\n", result.Path)
			}
		}

		return nil
	}
}
