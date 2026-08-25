package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/perfectscale/poc-cli/internal/config"
	"github.com/perfectscale/poc-cli/internal/output"
	ucli "github.com/urfave/cli/v2"
)

const (
	latestReleaseAPIURL = "https://api.github.com/repos/doitintl/perfectscale-cli/releases/latest"
	releasesPageURL     = "https://github.com/doitintl/perfectscale-cli/releases/latest"
	releaseDownloadBase = "https://github.com/doitintl/perfectscale-cli/releases/download"
)

func updateCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "update",
		Usage: "Check for a newer pscli release and print how to update",
		Description: withCommandName(`Checks the latest pscli release on GitHub and prints the command to
update, based on how pscli was installed. Does not install anything itself.

Examples:
  {{cmd}} update

Output:
  Plain text message by default; -o json/jsonl prints a structured result
  ({"current", "latest", "update_available", "instruction"}).`),
		Action: runUpdate,
	}
}

func runUpdate(c *ucli.Context) error {
	ver := currentVersion(c)

	tag, err := fetchLatestVersion(latestReleaseAPIURL, ver)
	if err != nil {
		return fmt.Errorf("could not check for updates: %w", err)
	}

	outputMode, err := config.NormalizeOutput(stringFlagValue(c, "output"))
	if err != nil {
		return err
	}

	if outputMode == "json" || outputMode == "jsonl" {
		return output.WriteJSON(c.App.Writer, buildUpdateStatusResult(ver, tag))
	}

	fmt.Fprint(c.App.Writer, updateStatus(ver, tag))

	return nil
}

// versionPrinter replaces urfave/cli's default --version/-v output: it
// prints the usual version line, then the same up-to-date/newer-version
// status `pscli update` reports, appended below it. The release lookup is
// silently skipped on failure (no network, rate-limited) so --version never
// fails or blocks on it for longer than necessary. This path always prints
// plain text — it isn't a `-o`-aware command.
func versionPrinter(c *ucli.Context) {
	fmt.Fprintf(c.App.Writer, "%s version %s\n", c.App.Name, c.App.Version)

	ver := currentVersion(c)

	tag, err := fetchLatestVersion(latestReleaseAPIURL, ver)
	if err != nil {
		return
	}

	fmt.Fprint(c.App.Writer, updateStatus(ver, tag))
}

// updateStatusResult is the structured result for -o json/jsonl.
type updateStatusResult struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	Instruction     string `json:"instruction,omitempty"`
}

// buildUpdateStatusResult computes the structured status shared by the
// plain-text (updateStatus) and JSON (-o json/jsonl) render paths, so both
// stay derived from one source of truth. Instruction is included whenever
// there's something actionable to report: a newer release, or an
// unparseable version on either side where staleness can't be determined.
func buildUpdateStatusResult(current, latest string) updateStatusResult {
	_, currentOK := parseVersion(current)
	_, latestOK := parseVersion(latest)
	available := currentOK && latestOK && isNewerVersion(current, latest)

	result := updateStatusResult{
		Current:         strings.TrimPrefix(current, "v"),
		Latest:          strings.TrimPrefix(latest, "v"),
		UpdateAvailable: available,
	}
	if available || !currentOK || !latestOK {
		result.Instruction = updateInstruction(executablePath(), runtime.GOOS, latest, realPackageOwnershipProbe)
	}

	return result
}

// updateStatus formats the up-to-date message, the newer-version notice,
// or — when current or latest isn't a parseable release version (e.g. a
// local "dev" build, or an unexpected non-x.y.z release tag) — a message
// that doesn't claim to be up to date since there's no way to tell. It
// still surfaces the latest release and how to get it.
func updateStatus(current, latest string) string {
	result := buildUpdateStatusResult(current, latest)

	_, currentOK := parseVersion(current)
	_, latestOK := parseVersion(latest)
	if !currentOK || !latestOK {
		return fmt.Sprintf("pscli %s — can't tell if this is current. Latest release: %s.\n",
			current, result.Latest) + instructionLine(result.Instruction)
	}

	if !result.UpdateAvailable {
		return fmt.Sprintf("pscli %s is up to date (latest release: %s).\n", result.Current, result.Latest)
	}

	return updateNotice(current, latest, result.Instruction)
}

// currentVersion reads the raw semver string stashed in app metadata
// (main.version, ldflags-injected), defaulting to "dev" for local builds.
// Shared with NewRuntime's identical lookup in context.go.
func currentVersion(c *ucli.Context) string {
	ver, _ := c.App.Metadata["version"].(string)
	if ver == "" {
		ver = "dev"
	}

	return ver
}

// fetchLatestVersion returns the tag name of the newest GitHub release
// (e.g. "v1.5.0").
func fetchLatestVersion(url, currentVersion string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pscli/"+currentVersion)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("release lookup returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return "", err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return "", fmt.Errorf("release lookup returned an empty tag")
	}

	return release.TagName, nil
}

// executablePath resolves symlinks so a Homebrew `bin/pscli -> ../Cellar/...`
// link (or a Scoop shim) is recognized correctly.
func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}

	return exe
}

// ownershipProbeFunc reports whether the packaging tool (dpkg/rpm) claims
// ownership of a path. Passed as a parameter rather than a package var so
// tests can inject a fake per table-test case without mutating shared
// state — safe under t.Parallel(), unlike a reassign-then-restore package var.
type ownershipProbeFunc func(tool string, args ...string) bool

// realPackageOwnershipProbe is the production probe: dpkg -S / rpm -qf exit
// zero for a path the tool owns. A missing tool simply fails the probe — a
// dpkg-less system cannot hold a dpkg-owned binary.
func realPackageOwnershipProbe(tool string, args ...string) bool {
	cmd := exec.Command(tool, args...)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard

	return cmd.Run() == nil
}

// updateInstruction returns the upgrade command for the install method
// detected from the executable path. Homebrew and Scoop delegate to the
// package manager's own upgrade command; deb/rpm (detected via dpkg/rpm
// ownership probes on Linux) get a copy-pasteable download+install
// one-liner; go-install, from-source builds, and anything unrecognized fall
// back to the releases page, since there's no upgrade command to suggest.
func updateInstruction(exePath, goos, targetVersion string, probe ownershipProbeFunc) string {
	p := strings.ToLower(exePath)

	switch {
	case strings.Contains(p, "cellar") || strings.Contains(p, "homebrew"):
		return "brew upgrade pscli"
	case goos == "windows" && strings.Contains(p, "scoop"):
		return "scoop update pscli"
	}

	if goos == "linux" && exePath != "" {
		if probe("dpkg", "-S", exePath) {
			return linuxPackagePipeline("deb", targetVersion)
		}
		if probe("rpm", "-qf", exePath) {
			return linuxPackagePipeline("rpm", targetVersion)
		}
	}

	return "Download the latest release from " + releasesPageURL
}

// linuxPackageAssetName mirrors .goreleaser.yaml's nfpms.file_name_template.
func linuxPackageAssetName(format, targetVersion string) string {
	return fmt.Sprintf("pscli_%s_linux_%s.%s", strings.TrimPrefix(targetVersion, "v"), runtime.GOARCH, format)
}

// linuxPackagePipeline builds a single copy-pasteable download+install
// command for a deb/rpm install — still check-only, it prints the command
// rather than running it.
func linuxPackagePipeline(format, targetVersion string) string {
	asset := linuxPackageAssetName(format, targetVersion)
	install := "sudo dpkg -i " + asset
	if format == "rpm" {
		install = "sudo rpm -U " + asset
	}

	return fmt.Sprintf("curl -fsSLO %s/%s/%s && %s", releaseDownloadBase, targetVersion, asset, install)
}

// updateNotice formats the new-version message.
func updateNotice(current, latest, instruction string) string {
	msg := fmt.Sprintf("A new version of pscli is available: %s -> %s\n",
		strings.TrimPrefix(current, "v"), strings.TrimPrefix(latest, "v"))

	return msg + instructionLine(instruction)
}

// instructionLine wraps a runnable command as "Run `...` to update."; the
// releases-page fallback from updateInstruction is already a full sentence
// and is printed verbatim.
func instructionLine(instruction string) string {
	if strings.HasPrefix(instruction, "Download ") {
		return instruction + "\n"
	}

	return "Run `" + instruction + "` to update.\n"
}

// parseVersion parses a "v"-optional major.minor.patch triple. Prerelease or
// otherwise decorated tags (1.5.0-rc.1) intentionally fail to parse so they
// never trigger an upgrade notification.
func parseVersion(s string) ([3]int, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}

	var v [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		v[i] = n
	}

	return v, true
}

// isNewerVersion reports whether latest is a strictly newer release than
// current. Unparseable versions on either side (dev builds, prerelease
// tags, garbage) compare as not-newer.
func isNewerVersion(current, latest string) bool {
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	l, ok := parseVersion(latest)
	if !ok {
		return false
	}

	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}

	return false
}
