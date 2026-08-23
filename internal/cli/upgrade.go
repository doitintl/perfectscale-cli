package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	ucli "github.com/urfave/cli/v2"
)

const (
	latestReleaseAPIURL = "https://api.github.com/repos/doitintl/perfectscale-cli/releases/latest"
	releasesPageURL     = "https://github.com/doitintl/perfectscale-cli/releases/latest"
)

func upgradeCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "upgrade",
		Usage: "Check for a newer pscli release and print how to upgrade",
		Description: withCommandName(`Checks the latest pscli release on GitHub and prints the command to
upgrade, based on how pscli was installed. Does not install anything itself.

Examples:
  {{cmd}} upgrade

Output:
  Plain text message. No structured output.`),
		Action: runUpgrade,
	}
}

func runUpgrade(c *ucli.Context) error {
	ver := currentVersion(c)

	tag, err := fetchLatestVersion(latestReleaseAPIURL, ver)
	if err != nil {
		return fmt.Errorf("could not check for updates: %w", err)
	}

	fmt.Fprint(c.App.Writer, upgradeStatus(ver, tag))

	return nil
}

// versionPrinter replaces urfave/cli's default --version/-v output: it
// prints the usual version line, then the same up-to-date/newer-version
// status `pscli upgrade` reports, appended below it. The release lookup is
// silently skipped on failure (no network, rate-limited) so --version never
// fails or blocks on it for longer than necessary.
func versionPrinter(c *ucli.Context) {
	fmt.Fprintf(c.App.Writer, "%s version %s\n", c.App.Name, c.App.Version)

	ver := currentVersion(c)

	tag, err := fetchLatestVersion(latestReleaseAPIURL, ver)
	if err != nil {
		return
	}

	fmt.Fprint(c.App.Writer, "\n"+upgradeStatus(ver, tag))
}

// upgradeStatus formats either the up-to-date message or the newer-version
// notice, depending on how current compares to latest.
func upgradeStatus(current, latest string) string {
	if !isNewerVersion(current, latest) {
		return fmt.Sprintf("pscli %s is up to date (latest release: %s).\n", current, strings.TrimPrefix(latest, "v"))
	}

	return updateNotice(current, latest, upgradeInstruction(executablePath(), runtime.GOOS))
}

// currentVersion reads the raw semver string stashed in app metadata
// (main.version, ldflags-injected), defaulting to "dev" for local builds.
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
		return "", fmt.Errorf("release lookup returned HTTP %d", resp.StatusCode)
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

// upgradeInstruction returns the upgrade command for the install method
// detected from the executable path. deb/rpm installs, go-install,
// from-source builds, and anything unrecognized all fall back to the
// releases page, since deb/rpm here is a direct package install (not a
// hosted apt/yum repo) with no package-manager upgrade command that would
// actually find a newer version.
func upgradeInstruction(exePath, goos string) string {
	p := strings.ToLower(exePath)

	switch {
	case strings.Contains(p, "cellar") || strings.Contains(p, "homebrew"):
		return "brew upgrade pscli"
	case goos == "windows" && strings.Contains(p, "scoop"):
		return "scoop update pscli"
	}

	return "Download the latest release from " + releasesPageURL
}

// updateNotice formats the new-version message. An instruction that is a
// runnable command gets wrapped in "Run `...` to update."; the
// releases-page fallback from upgradeInstruction is already a full sentence
// and is printed verbatim.
func updateNotice(current, latest, instruction string) string {
	msg := fmt.Sprintf("A new version of pscli is available: %s -> %s\n",
		strings.TrimPrefix(current, "v"), strings.TrimPrefix(latest, "v"))
	if strings.HasPrefix(instruction, "Download ") {
		return msg + instruction + "\n"
	}

	return msg + "Run `" + instruction + "` to update.\n"
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
