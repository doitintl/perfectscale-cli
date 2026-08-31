package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perfectscale/poc-cli/internal/clierr"
	"github.com/perfectscale/poc-cli/internal/config"
)

func TestSkillDestination(t *testing.T) {
	t.Parallel()

	home := filepath.FromSlash("/home/user")
	override := filepath.FromSlash("/tmp/agent-home")
	cases := []struct {
		agent    string
		wantRel  string
		override string
		want     string
	}{
		{agent: "claude", wantRel: filepath.Join(".claude", "skills", "perfectscale")},
		{agent: "cursor", wantRel: filepath.Join(".cursor", "skills", "perfectscale")},
		{agent: "gemini", wantRel: filepath.Join(".gemini", "skills", "perfectscale")},
		{agent: "kiro", wantRel: filepath.Join(".kiro", "skills", "perfectscale")},
		{agent: "opencode", wantRel: filepath.Join(".config", "opencode", "skills", "perfectscale")},
		{agent: "codex", wantRel: filepath.Join(".agents", "skills", "perfectscale")},
		{agent: "cursor", override: override, want: filepath.Join(override, "skills", "perfectscale")},
		{agent: "codex", override: override, want: filepath.Join(override, "skills", "perfectscale")},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.agent+"_"+tc.override, func(t *testing.T) {
			t.Parallel()

			agent, ok := skillAgentByName(tc.agent)
			if !ok {
				t.Fatalf("unknown agent %q", tc.agent)
			}

			got := skillDestination(agent, home, tc.override)
			want := tc.want
			if want == "" {
				want = filepath.Join(home, tc.wantRel)
			}

			if got != want {
				t.Fatalf("skillDestination(%s) = %q, want %q", tc.agent, got, want)
			}
		})
	}
}

func TestSkillInstallFreshAndJSON(t *testing.T) {
	dir := t.TempDir()
	output, err := runCLI(t, nil, "skill", "cursor", "--dir", dir, "-o", "json")
	if err != nil {
		t.Fatalf("skill cursor: %v\n%s", err, output)
	}

	var result skillInstallResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, output)
	}

	wantPath := filepath.Join(dir, "skills", "perfectscale")
	if result.Path != wantPath {
		t.Fatalf("path = %q, want %q", result.Path, wantPath)
	}

	if result.Agent != "cursor" {
		t.Fatalf("agent = %q, want cursor", result.Agent)
	}

	if _, err := os.Stat(filepath.Join(wantPath, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md missing: %v", err)
	}

	if _, err := os.Stat(filepath.Join(wantPath, skillManifestName)); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
}

func TestSkillLocalEditsBlockWithoutForce(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, nil, "skill", "cursor", "--dir", dir); err != nil {
		t.Fatalf("initial install: %v", err)
	}

	skillPath := filepath.Join(dir, "skills", "perfectscale", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("local edit\n"), 0o644); err != nil {
		t.Fatalf("write local edit: %v", err)
	}

	_, err := runCLI(t, nil, "skill", "cursor", "--dir", dir)
	if err == nil {
		t.Fatal("expected local-edit error")
	}

	if clierr.Classify(err).ExitCode != clierr.ExitUsage {
		t.Fatalf("exit classification = %+v, want usage", clierr.Classify(err))
	}

	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error = %q, want --force hint", err.Error())
	}
}

func TestSkillForceBacksUpChangedFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, nil, "skill", "cursor", "--dir", dir); err != nil {
		t.Fatalf("initial install: %v", err)
	}

	skillPath := filepath.Join(dir, "skills", "perfectscale", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("local edit\n"), 0o644); err != nil {
		t.Fatalf("write local edit: %v", err)
	}

	output, err := runCLI(t, nil, "skill", "cursor", "--dir", dir, "--force")
	if err != nil {
		t.Fatalf("force install: %v\n%s", err, output)
	}

	if !strings.Contains(output, "Local edits backed up to") {
		t.Fatalf("output missing backup line: %s", output)
	}

	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}

	if string(data) == "local edit\n" {
		t.Fatal("SKILL.md was not overwritten")
	}
}

func TestSkillExtraFilesDoNotBlock(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, nil, "skill", "cursor", "--dir", dir); err != nil {
		t.Fatalf("initial install: %v", err)
	}

	extra := filepath.Join(dir, "skills", "perfectscale", "notes.md")
	if err := os.WriteFile(extra, []byte("keep me\n"), 0o644); err != nil {
		t.Fatalf("write extra: %v", err)
	}

	if _, err := runCLI(t, nil, "skill", "cursor", "--dir", dir); err != nil {
		t.Fatalf("reinstall with extra file: %v", err)
	}

	got, err := os.ReadFile(extra)
	if err != nil {
		t.Fatalf("extra file gone: %v", err)
	}

	if string(got) != "keep me\n" {
		t.Fatalf("extra file content = %q", got)
	}
}

func TestSkillAllDetectsAgentDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("AppData", home)
	t.Setenv("LocalAppData", home)
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = os.UserHomeDir })

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir claude: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatalf("mkdir codex: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0o755); err != nil {
		t.Fatalf("mkdir gemini: %v", err)
	}

	app := New("test")
	var buf strings.Builder
	app.Writer = &buf
	app.ErrWriter = &buf
	err := app.Run([]string{config.BinaryName, "skill", "--all"})
	output := buf.String()
	if err != nil {
		t.Fatalf("skill --all: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "perfectscale", "SKILL.md")); err != nil {
		t.Fatalf("claude skill missing: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "perfectscale", "SKILL.md")); err != nil {
		t.Fatalf("codex skill missing at ~/.agents/skills: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".gemini", "skills", "perfectscale", "SKILL.md")); err != nil {
		t.Fatalf("gemini skill missing: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "perfectscale")); err == nil {
		t.Fatal("codex skill written to legacy ~/.codex/skills")
	}

	if _, err := os.Stat(filepath.Join(home, ".cursor", "skills", "perfectscale")); err == nil {
		t.Fatal("cursor skill installed without ~/.cursor detect dir")
	}
}

func TestSkillListJSON(t *testing.T) {
	output, err := runCLI(t, nil, "skill", "list", "-o", "json")
	if err != nil {
		t.Fatalf("skill list: %v\n%s", err, output)
	}

	var payload struct {
		Files []skillFileInfo `json:"files"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, output)
	}

	if len(payload.Files) == 0 {
		t.Fatal("expected embedded files")
	}

	foundSkill := false
	for _, file := range payload.Files {
		if file.Path == "SKILL.md" {
			foundSkill = true
		}

		if strings.HasPrefix(file.Path, "scripts/") {
			t.Fatalf("scripts should not be embedded: %s", file.Path)
		}
	}

	if !foundSkill {
		t.Fatalf("SKILL.md missing from list: %+v", payload.Files)
	}
}

func TestSkillForceRequiresTarget(t *testing.T) {
	_, err := runCLI(t, nil, "skill", "--force")
	if err == nil {
		t.Fatal("expected usage error")
	}

	if clierr.Classify(err).ExitCode != clierr.ExitUsage {
		t.Fatalf("classify = %+v, want usage", clierr.Classify(err))
	}
}

func TestSkillUpdateUnknownAgent(t *testing.T) {
	_, err := runCLI(t, nil, "skill", "update", "not-an-agent")
	if err == nil {
		t.Fatal("expected usage error")
	}

	if clierr.Classify(err).ExitCode != clierr.ExitUsage {
		t.Fatalf("classify = %+v, want usage", clierr.Classify(err))
	}
}

func TestSkillHelpListsAgents(t *testing.T) {
	output, err := runCLI(t, nil, "skill", "--help")
	if err != nil {
		t.Fatalf("skill --help: %v\n%s", err, output)
	}

	for _, name := range []string{"claude", "codex", "cursor", "gemini", "kiro", "opencode", "list", "update"} {
		if !strings.Contains(output, name) {
			t.Fatalf("help missing %q\n%s", name, output)
		}
	}
}
