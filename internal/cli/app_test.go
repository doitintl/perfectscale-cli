package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/perfectscale/poc-cli/internal/config"
	"github.com/perfectscale/poc-cli/internal/profile"
)

func TestHelpIncludesDefaultsAndExamples(t *testing.T) {
	output, err := runCLI(t, nil, "--help")
	if err != nil {
		t.Fatalf("runCLI(--help) error = %v", err)
	}

	assertContains(t, output, "Available commands:")
	assertContains(t, output, "workloads list|summary|group-by namespace|group-by type|group-by optimization-policy|group-by risk-severity|group-by label|show|export|risky|labels|muted")
	assertContains(t, output, "Common short options:")
	assertContains(t, output, "-c cluster, -w period window, -n namespace, -t type")
	assertContains(t, output, "-V workload view preset")
	assertContains(t, output, "Workload list views (--view, -V):")
	assertContains(t, output, "Summed container usage percentiles for each workload")
	assertContains(t, output, "The broadest enriched workload view")
	assertContains(t, output, "Output modes (--output, -o):")
	assertContains(t, output, "jsonl")
	assertContains(t, output, "useful for agents and pipelines")
	assertContains(t, output, "pscli auth login -s -i ps_xxx -k ps_yyy")
	assertContains(t, output, "pscli clusters emission -c prod-a -s value -r desc")
	assertContains(t, output, "pscli workloads export -c prod-a -F workloads.csv")
	assertContains(t, output, "pscli namespaces list -c prod-a -s workloads -r desc")
	assertContains(t, output, "pscli workloads list -c prod-a -V usage")
	assertContains(t, output, "pscli workloads group-by optimization-policy -c prod-a -s waste -r desc")
	assertContains(t, output, "pscli workloads group-by risk-severity -c prod-a -s workloads -r desc")
	assertContains(t, output, "pscli workloads group-by label -c prod-a -k team -s waste -r desc")
	assertContains(t, output, "pscli workloads list -c prod-a -V all")
	assertContains(t, output, "pscli -o jsonl workloads list -c prod-a -V all -s waste -r desc -T 10")
	assertContains(t, output, "$PERFECTSCALE_PUBLIC_API_URL")
	assertContains(t, output, config.DefaultPublicAPIURL)
	assertContains(t, output, config.DefaultOutput)
	assertContains(t, output, "--profile value, -p value")
	assertContains(t, output, "--public-api-url value, -u value")
}

func TestWorkloadsHelpIncludesTopAndWasteExample(t *testing.T) {
	output, err := runCLI(t, nil, "workloads", "list", "--help")
	if err != nil {
		t.Fatalf("runCLI(workloads help) error = %v", err)
	}

	assertContains(t, output, "pscli workloads list -c prod-a -w 30d -s waste -r desc -T 10")
	assertContains(t, output, "pscli workloads list -c prod-a -w 30d -s waste -r asc -B 10")
	assertContains(t, output, "pscli workloads list -c prod-a --view usage")
	assertContains(t, output, "pscli workloads list -c prod-a --view all")
	assertContains(t, output, "all")
	assertContains(t, output, "defaults to jsonl")
	assertContains(t, output, "--cluster value, -c value")
	assertContains(t, output, "--min-waste value, -W value")
	assertContains(t, output, "--view value, -V value")
	assertContains(t, output, "--top value")
	assertContains(t, output, "--bottom value")
}

func TestWorkloadsAdvancedHelpIncludesNewCommands(t *testing.T) {
	output, err := runCLI(t, nil, "workloads", "--help")
	if err != nil {
		t.Fatalf("runCLI(workloads --help) error = %v", err)
	}

	assertContains(t, output, "summary")
	assertContains(t, output, "group-by")
	assertContains(t, output, "show")
	assertContains(t, output, "export")
	assertContains(t, output, "risky")
	assertContains(t, output, "labels")
	assertContains(t, output, "muted")
}

func TestWorkloadsGroupByHelpIncludesNewSubcommands(t *testing.T) {
	output, err := runCLI(t, nil, "workloads", "group-by", "--help")
	if err != nil {
		t.Fatalf("runCLI(workloads group-by --help) error = %v", err)
	}

	assertContains(t, output, "Available group-by options:")
	assertContains(t, output, "namespace")
	assertContains(t, output, "type")
	assertContains(t, output, "optimization-policy")
	assertContains(t, output, "risk-severity")
	assertContains(t, output, "label")
	assertContains(t, output, "This mode requires --key or -k")
}

func TestAutomationAuditLogsHelpIncludesFlagsAndExamples(t *testing.T) {
	output, err := runCLI(t, nil, "automation", "audit-logs", "--help")
	if err != nil {
		t.Fatalf("runCLI(automation audit-logs --help) error = %v", err)
	}

	assertContains(t, output, "automation audit-logs")
	assertContains(t, output, "cursor-paginated")
	assertContains(t, output, "--cluster value, -c value")
	assertContains(t, output, "--namespace value, -n value")
	assertContains(t, output, "--from value")
	assertContains(t, output, "--to value")
	assertContains(t, output, "--since value")
	assertContains(t, output, "--page-size value")
	assertContains(t, output, "--after value")
	assertContains(t, output, "--all")
	assertContains(t, output, "pscli automation audit-logs --since 24h")
	assertContains(t, output, "pscli automation audit-logs --all -o jsonl")
}

func TestAppHelpListsAutomation(t *testing.T) {
	output, err := runCLI(t, nil, "--help")
	if err != nil {
		t.Fatalf("runCLI(--help) error = %v", err)
	}
	assertContains(t, output, "automation audit-logs")
	assertContains(t, output, "pscli automation audit-logs -c prod-a --since 24h")
}

func TestWorkloadsGroupByLabelHelpIncludesKeyFlag(t *testing.T) {
	output, err := runCLI(t, nil, "workloads", "group-by", "label", "--help")
	if err != nil {
		t.Fatalf("runCLI(workloads group-by label --help) error = %v", err)
	}

	assertContains(t, output, "pscli workloads group-by label -c prod-a -k team")
	assertContains(t, output, "--key value, -k value")
}

func TestClustersHelpIncludesDetailCommands(t *testing.T) {
	output, err := runCLI(t, nil, "clusters", "--help")
	if err != nil {
		t.Fatalf("runCLI(clusters --help) error = %v", err)
	}

	assertContains(t, output, "get")
	assertContains(t, output, "emission")
}

func TestNamespacesHelpIncludesExamples(t *testing.T) {
	output, err := runCLI(t, nil, "namespaces", "list", "--help")
	if err != nil {
		t.Fatalf("runCLI(namespaces help) error = %v", err)
	}

	assertContains(t, output, "pscli namespaces list -c prod-a -s workloads -r desc")
	assertContains(t, output, "pscli namespaces list -c prod-a -n kube -T 5")
	assertContains(t, output, "--namespace value, -n value")
	assertContains(t, output, "--sort value, -s value")
}

func TestWorkloadsShowHelpIncludesDisambiguationExample(t *testing.T) {
	output, err := runCLI(t, nil, "workloads", "show", "--help")
	if err != nil {
		t.Fatalf("runCLI(workloads show --help) error = %v", err)
	}

	assertContains(t, output, "pscli workloads show -c prod-a -i workload-123")
	assertContains(t, output, "pscli workloads show -c prod-a -m api -n backend")
	assertContains(t, output, "--id value, -i value")
	assertContains(t, output, "--name value, -m value")
}

func TestWorkloadsListRejectsUnsupportedServiceTokenPeriod(t *testing.T) {
	output, err := runCLI(t, &profile.Data{
		SchemaVersion: 1,
		Name:          config.DefaultProfileName,
		AuthMode:      profile.AuthModeServiceToken,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, "workloads", "list", "--cluster", "prod-a", "--period", "7d")
	if err == nil {
		t.Fatalf("runCLI() error = nil, want non-nil; output=%s", output)
	}
	assertContains(t, err.Error(), "only --period 30d is supported")
}

func TestNamespacesListRejectsUnsupportedServiceTokenPeriod(t *testing.T) {
	output, err := runCLI(t, &profile.Data{
		SchemaVersion: 1,
		Name:          config.DefaultProfileName,
		AuthMode:      profile.AuthModeServiceToken,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, "namespaces", "list", "--cluster", "prod-a", "--period", "7d")
	if err == nil {
		t.Fatalf("runCLI() error = nil, want non-nil; output=%s", output)
	}
	assertContains(t, err.Error(), "only --period 30d is supported")
}

func TestAuthStatusJSONReportsAuthMode(t *testing.T) {
	expiresAt := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	output, err := runCLI(t, &profile.Data{
		SchemaVersion: 1,
		Name:          config.DefaultProfileName,
		AuthMode:      profile.AuthModeServiceToken,
		PublicAPIURL:  config.DefaultPublicAPIURL,
		ExpiresAt:     expiresAt,
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, "--output", "json", "auth", "status")
	if err != nil {
		t.Fatalf("runCLI(auth status) error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput=%s", err, output)
	}
	if payload["auth_mode"] != string(profile.AuthModeServiceToken) {
		t.Fatalf("auth_mode = %#v, want %q", payload["auth_mode"], profile.AuthModeServiceToken)
	}
	if payload["public_api_url"] != config.DefaultPublicAPIURL {
		t.Fatalf("public_api_url = %#v, want %q", payload["public_api_url"], config.DefaultPublicAPIURL)
	}
	if payload["expires_at"] != expiresAt.Format(time.RFC3339) {
		t.Fatalf("expires_at = %#v, want %q", payload["expires_at"], expiresAt.Format(time.RFC3339))
	}
	if payload["has_service_token"] != true {
		t.Fatalf("has_service_token = %#v, want true", payload["has_service_token"])
	}
}

func TestAuthWithoutProfileShowsSetupGuide(t *testing.T) {
	output, err := runCLI(t, nil, "auth")
	if err != nil {
		t.Fatalf("runCLI(auth) error = %v", err)
	}

	assertContains(t, output, `No saved service-token credentials found for profile "default".`)
	assertContains(t, output, "user-initials avatar at the bottom-left of the sidebar")
	assertContains(t, output, "Open Organization Settings")
	assertContains(t, output, "Open the API Tokens tab")
	assertContains(t, output, "Click Generate Token")
	assertContains(t, output, "Assign a Read Only role")
	assertContains(t, output, "pscli auth login -i 'YOUR_CLIENT_ID' -k 'YOUR_CLIENT_SECRET'")
	assertContains(t, output, "Or run the interactive prompt:")
}

func TestAuthWithMissingServiceTokenShowsSetupGuide(t *testing.T) {
	output, err := runCLI(t, &profile.Data{
		SchemaVersion: 1,
		Name:          config.DefaultProfileName,
		AuthMode:      profile.AuthModeServiceToken,
		PublicAPIURL:  config.DefaultPublicAPIURL,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, "auth")
	if err != nil {
		t.Fatalf("runCLI(auth) error = %v", err)
	}

	assertContains(t, output, `Profile "default" is missing a saved client_id/client_secret.`)
	assertContains(t, output, "pscli auth login -i 'YOUR_CLIENT_ID' -k 'YOUR_CLIENT_SECRET'")
}

func TestAuthWithSavedServiceTokenShowsStatus(t *testing.T) {
	output, err := runCLI(t, &profile.Data{
		SchemaVersion: 1,
		Name:          config.DefaultProfileName,
		AuthMode:      profile.AuthModeServiceToken,
		PublicAPIURL:  config.DefaultPublicAPIURL,
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, "auth")
	if err != nil {
		t.Fatalf("runCLI(auth) error = %v", err)
	}

	assertContains(t, output, "FIELD")
	assertContains(t, output, "has_service_token")
	assertContains(t, output, "true")
}

func TestAuthHelpIncludesOrgTokenGuidance(t *testing.T) {
	output, err := runCLI(t, nil, "auth", "login", "--help")
	if err != nil {
		t.Fatalf("runCLI(auth login --help) error = %v", err)
	}

	assertContains(t, output, "Open Organization Settings")
	assertContains(t, output, "Open the API Tokens tab")
	assertContains(t, output, "Assign a Read Only role")
	assertContains(t, output, "pscli auth login -s -i ps_xxx -k ps_yyy")
}

func TestAuthStatusDefaultOutputIsTable(t *testing.T) {
	expiresAt := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	output, err := runCLI(t, &profile.Data{
		SchemaVersion: 1,
		Name:          config.DefaultProfileName,
		AuthMode:      profile.AuthModeServiceToken,
		PublicAPIURL:  config.DefaultPublicAPIURL,
		ExpiresAt:     expiresAt,
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, "auth", "status")
	if err != nil {
		t.Fatalf("runCLI(auth status) error = %v", err)
	}
	assertContains(t, output, "FIELD")
	assertContains(t, output, "auth_mode")
	assertContains(t, output, string(profile.AuthModeServiceToken))
}

func TestAuthStatusAcceptsShortOutputFlagAfterSubcommand(t *testing.T) {
	expiresAt := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	output, err := runCLI(t, &profile.Data{
		SchemaVersion: 1,
		Name:          config.DefaultProfileName,
		AuthMode:      profile.AuthModeServiceToken,
		PublicAPIURL:  config.DefaultPublicAPIURL,
		ExpiresAt:     expiresAt,
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, "auth", "status", "-o", "json")
	if err != nil {
		t.Fatalf("runCLI(auth status -o json) error = %v", err)
	}
	assertContains(t, output, `"auth_mode": "service_token"`)
}

func TestAuthStatusAcceptsLongOutputFlagAfterSubcommand(t *testing.T) {
	expiresAt := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	output, err := runCLI(t, &profile.Data{
		SchemaVersion: 1,
		Name:          config.DefaultProfileName,
		AuthMode:      profile.AuthModeServiceToken,
		PublicAPIURL:  config.DefaultPublicAPIURL,
		ExpiresAt:     expiresAt,
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, "auth", "status", "--output", "json")
	if err != nil {
		t.Fatalf("runCLI(auth status --output json) error = %v", err)
	}
	assertContains(t, output, `"public_api_url": "https://api.app.perfectscale.io/public/v1"`)
}

func runCLI(t *testing.T, data *profile.Data, args ...string) (string, error) {
	t.Helper()

	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)
	t.Setenv("AppData", configHome)
	t.Setenv("LocalAppData", configHome)

	if data != nil {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			t.Fatalf("UserConfigDir() error = %v", err)
		}
		store, err := profile.NewStore(filepath.Join(userConfigDir, "perfectscale-cli", "profiles"))
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}
		if err := store.Save(data); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	app := New("test", "deadbeef", "2026-04-27T00:00:00Z")
	var buf bytes.Buffer
	app.Writer = &buf
	app.ErrWriter = &buf

	err := app.RunContext(context.Background(), append([]string{config.BinaryName}, args...))
	return buf.String(), err
}

func assertContains(t *testing.T, text string, substring string) {
	t.Helper()
	if !strings.Contains(text, substring) {
		t.Fatalf("text does not contain substring %q\ntext=%s", substring, text)
	}
}
