package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  [3]int
		ok    bool
	}{
		{"v_prefixed", "v1.2.3", [3]int{1, 2, 3}, true},
		{"no_prefix", "1.2.3", [3]int{1, 2, 3}, true},
		{"dev_build", "dev", [3]int{}, false},
		{"prerelease_tag", "v1.2.3-rc.1", [3]int{}, false},
		{"missing_patch", "v1.2", [3]int{}, false},
		{"non_numeric", "v1.two.3", [3]int{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseVersion(tt.input)
			if ok != tt.ok {
				t.Fatalf("parseVersion(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Fatalf("parseVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"equal_versions", "v1.2.3", "v1.2.3", false},
		{"patch_bump", "v1.2.3", "v1.2.4", true},
		{"minor_bump", "v1.2.3", "v1.3.0", true},
		{"major_bump", "v1.2.3", "v2.0.0", true},
		{"downgrade", "v1.2.3", "v1.2.2", false},
		{"unparseable_current", "dev", "v1.2.3", false},
		{"unparseable_latest", "v1.2.3", "not-a-version", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isNewerVersion(tt.current, tt.latest); got != tt.want {
				t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestUpgradeInstruction(t *testing.T) {
	tests := []struct {
		name    string
		exePath string
		goos    string
		want    string
	}{
		{"homebrew_cellar", "/opt/homebrew/Cellar/pscli/1.0.0/bin/pscli", "darwin", "brew upgrade pscli"},
		{"homebrew_linuxbrew", "/home/linuxbrew/.linuxbrew/Cellar/pscli/1.0.0/bin/pscli", "linux", "brew upgrade pscli"},
		{"scoop_windows", `C:\Users\bogdan\scoop\apps\pscli\current\pscli.exe`, "windows", "scoop update pscli"},
		{"plain_path", "/usr/local/bin/pscli", "linux", "Download the latest release from " + releasesPageURL},
		{"empty_path", "", "darwin", "Download the latest release from " + releasesPageURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := upgradeInstruction(tt.exePath, tt.goos); got != tt.want {
				t.Fatalf("upgradeInstruction(%q, %q) = %q, want %q", tt.exePath, tt.goos, got, tt.want)
			}
		})
	}
}

func TestUpgradeStatus(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    string
	}{
		{
			name:    "up_to_date",
			current: "v1.0.11",
			latest:  "v1.0.11",
			want:    "pscli 1.0.11 is up to date (latest release: 1.0.11).\n",
		},
		{
			name:    "newer_available",
			current: "v1.0.0",
			latest:  "v1.0.11",
			want: "A new version of pscli is available: 1.0.0 -> 1.0.11\n" +
				"Download the latest release from " + releasesPageURL + "\n",
		},
		{
			name:    "unparseable_current_does_not_claim_up_to_date",
			current: "dev",
			latest:  "v1.0.11",
			want: "pscli dev — can't tell if this is current. Latest release: 1.0.11.\n" +
				"Download the latest release from " + releasesPageURL + "\n",
		},
		{
			name:    "unparseable_latest_does_not_claim_up_to_date",
			current: "v1.0.11",
			latest:  "v1.0.11-rc.1",
			want: "pscli v1.0.11 — can't tell if this is current. Latest release: 1.0.11-rc.1.\n" +
				"Download the latest release from " + releasesPageURL + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := upgradeStatus(tt.current, tt.latest); got != tt.want {
				t.Fatalf("upgradeStatus(%q, %q) = %q, want %q", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestUpdateNotice(t *testing.T) {
	t.Run("runnable_command", func(t *testing.T) {
		t.Parallel()

		got := updateNotice("v1.0.0", "v1.1.0", "brew upgrade pscli")
		want := "A new version of pscli is available: 1.0.0 -> 1.1.0\nRun `brew upgrade pscli` to update.\n"
		if got != want {
			t.Fatalf("updateNotice() = %q, want %q", got, want)
		}
	})

	t.Run("releases_page_fallback", func(t *testing.T) {
		t.Parallel()

		instruction := "Download the latest release from " + releasesPageURL
		got := updateNotice("v1.0.0", "v1.1.0", instruction)
		want := "A new version of pscli is available: 1.0.0 -> 1.1.0\n" + instruction + "\n"
		if got != want {
			t.Fatalf("updateNotice() = %q, want %q", got, want)
		}
	})
}

func TestFetchLatestVersion(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("User-Agent"); got != "pscli/v1.0.0" {
				t.Fatalf("User-Agent = %q, want pscli/v1.0.0", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1.5.0"}`))
		}))
		defer server.Close()

		tag, err := fetchLatestVersion(server.URL, "v1.0.0")
		if err != nil {
			t.Fatalf("fetchLatestVersion() error = %v", err)
		}
		if tag != "v1.5.0" {
			t.Fatalf("fetchLatestVersion() = %q, want v1.5.0", tag)
		}
	})

	t.Run("non_200_status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		if _, err := fetchLatestVersion(server.URL, "v1.0.0"); err == nil {
			t.Fatal("fetchLatestVersion() error = nil, want non-nil")
		}
	})

	t.Run("empty_tag", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":""}`))
		}))
		defer server.Close()

		if _, err := fetchLatestVersion(server.URL, "v1.0.0"); err == nil {
			t.Fatal("fetchLatestVersion() error = nil, want non-nil")
		}
	})
}
