package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/perfectscale/poc-cli/internal/auth"
	"github.com/perfectscale/poc-cli/internal/config"
	"github.com/perfectscale/poc-cli/internal/profile"
	ucli "github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func authCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "auth",
		Usage: "Authenticate with Perfectscale",
		Description: withCommandName(`Perfectscale CLI uses service tokens generated in the Perfectscale UI.

Get a working public API token from:
  1. https://app.perfectscale.io
  2. User-initials avatar at the bottom-left of the sidebar (below the gear icon)
  3. Organization Settings
  4. API Tokens tab
  5. Generate Token
  6. Use the "Read Only" role

Then install it with:
  {{cmd}} auth login -i 'YOUR_CLIENT_ID' -k 'YOUR_CLIENT_SECRET'`),
		Action: runAuthOverview,
		Subcommands: []*ucli.Command{
			{
				Name:  "login",
				Usage: "Install a Perfectscale public API service token",
				Description: withCommandName(`Generate a token in the Perfectscale UI first:
  1. Open https://app.perfectscale.io
  2. Click the user-initials avatar at the bottom-left of the sidebar (below the gear icon)
  3. Open Organization Settings
  4. Open the API Tokens tab
  5. Click Generate Token
  6. Assign a Read Only role
  7. Copy the client_id and client_secret

Examples:
  {{cmd}} auth login
  {{cmd}} auth login -s
  {{cmd}} auth login -s -i ps_xxx -k ps_yyy

Short options:
  -s service-token, -i client-id, -k client-secret

Output:
  Plain text confirmation message. No structured output.`),
				Flags: []ucli.Flag{
					&ucli.BoolFlag{
						Name:    "service-token",
						Aliases: []string{"s"},
						Usage:   "Optional compatibility flag; service-token auth is the only supported auth mode",
					},
					&ucli.StringFlag{
						Name:    "client-id",
						Aliases: []string{"i"},
						Usage:   "Service token client_id from the Perfectscale UI",
					},
					&ucli.StringFlag{
						Name:    "client-secret",
						Aliases: []string{"k"},
						Usage:   "Service token client_secret from the Perfectscale UI",
					},
				},
				Action: runAuthLogin,
			},
			{
				Name:  "status",
				Usage: "Show the current profile and authentication status",
				Description: withCommandName(`Displays the active auth mode, token expiry, and saved endpoint configuration for the selected profile.

Example:
  {{cmd}} auth status

Output schema (--output json):
  {
    "profile":           string,
    "auth_mode":         string,
    "expires_at":        string (RFC3339 or ""),
    "has_refresh_token": bool,
    "has_service_token": bool,
    "public_api_url":    string
  }`),
				Action: runAuthStatus,
			},
			{
				Name:  "logout",
				Usage: "Remove the stored profile and credentials",
				Description: `Deletes the selected profile from the local credential store.

Output:
  Plain text confirmation message. No structured output.`,
				Action: runAuthLogout,
			},
		},
	}
}

func runAuthOverview(c *ucli.Context) error {
	rt, err := NewRuntime(c)
	if err != nil {
		return err
	}

	data, err := rt.LoadProfile()
	if err != nil {
		if errors.Is(err, profile.ErrProfileNotFound) {
			writeAuthSetupHelp(rt.Writer, rt.Config.Profile, false)
			return nil
		}
		return err
	}

	if !hasServiceTokenCredentials(data) {
		writeAuthSetupHelp(rt.Writer, rt.Config.Profile, true)
		return nil
	}

	return renderAuthStatus(rt, data)
}

func runAuthLogin(c *ucli.Context) error {
	rt, err := NewRuntime(c)
	if err != nil {
		return err
	}

	return loginWithServiceToken(c.Context, rt, c.String("client-id"), c.String("client-secret"))
}

func loginWithServiceToken(ctx context.Context, rt *Runtime, clientID string, clientSecret string) error {
	reader := bufio.NewReader(os.Stdin)
	if strings.TrimSpace(clientID) == "" {
		fmt.Fprint(rt.Writer, "Client ID: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read client id: %w", err)
		}
		clientID = strings.TrimSpace(line)
	}
	if strings.TrimSpace(clientSecret) == "" {
		fmt.Fprint(rt.Writer, "Client secret: ")
		secret, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(rt.Writer)
		if err != nil {
			return fmt.Errorf("read client secret: %w", err)
		}
		clientSecret = strings.TrimSpace(string(secret))
	}
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("both client id and client secret are required")
	}

	tokens, err := auth.ExchangeServiceToken(ctx, rt.Config.PublicAPIURL, clientID, clientSecret)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	data := &profile.Data{
		SchemaVersion: 1,
		Name:          rt.Config.Profile,
		AuthMode:      profile.AuthModeServiceToken,
		PublicAPIURL:  rt.Config.PublicAPIURL,
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	auth.ApplyTokenResponse(data, tokens)
	if err := rt.Auth.Save(data); err != nil {
		return err
	}

	fmt.Fprintf(rt.Writer, "Saved service-token profile %q\n", rt.Config.Profile)
	return nil
}

func runAuthStatus(c *ucli.Context) error {
	rt, err := NewRuntime(c)
	if err != nil {
		return err
	}

	data, err := rt.LoadProfile()
	if err != nil {
		return err
	}

	return renderAuthStatus(rt, data)
}

func renderAuthStatus(rt *Runtime, data *profile.Data) error {
	status := map[string]any{
		"profile":           data.Name,
		"auth_mode":         data.AuthMode,
		"expires_at":        formatTimestamp(data.ExpiresAt),
		"has_refresh_token": data.RefreshToken != "",
		"has_service_token": hasServiceTokenCredentials(data),
		"public_api_url":    data.PublicAPIURL,
	}

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"profile", data.Name},
		{"auth_mode", string(data.AuthMode)},
		{"expires_at", formatTimestamp(data.ExpiresAt)},
		{"has_refresh_token", fmt.Sprintf("%t", data.RefreshToken != "")},
		{"has_service_token", fmt.Sprintf("%t", hasServiceTokenCredentials(data))},
		{"public_api_url", data.PublicAPIURL},
	}

	return rt.RenderTableOrJSON(status, headers, rows)
}

func runAuthLogout(c *ucli.Context) error {
	rt, err := NewRuntime(c)
	if err != nil {
		return err
	}
	if err := rt.Auth.Delete(rt.Config.Profile); err != nil {
		return err
	}
	fmt.Fprintf(rt.Writer, "Deleted profile %q\n", rt.Config.Profile)
	return nil
}

func formatTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func hasServiceTokenCredentials(data *profile.Data) bool {
	if data == nil {
		return false
	}
	return strings.TrimSpace(data.ClientID) != "" && strings.TrimSpace(data.ClientSecret) != ""
}

func writeAuthSetupHelp(writer io.Writer, profileName string, profileExists bool) {
	if writer == nil {
		writer = os.Stdout
	}

	if profileExists {
		fmt.Fprintf(writer, "Profile %q is missing a saved client_id/client_secret.\n\n", profileName)
	} else {
		fmt.Fprintf(writer, "No saved service-token credentials found for profile %q.\n\n", profileName)
	}

	fmt.Fprintln(writer, "Create a token in the Perfectscale UI:")
	fmt.Fprintln(writer, "  1. Open https://app.perfectscale.io")
	fmt.Fprintln(writer, "  2. Click the user-initials avatar at the bottom-left of the sidebar (below the gear icon)")
	fmt.Fprintln(writer, "  3. Open Organization Settings")
	fmt.Fprintln(writer, "  4. Open the API Tokens tab")
	fmt.Fprintln(writer, "  5. Click Generate Token")
	fmt.Fprintln(writer, "  6. Assign a Read Only role")
	fmt.Fprintln(writer, "  7. Copy the client_id and client_secret")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Then install it with:")
	fmt.Fprintf(writer, "  %s auth login -i 'YOUR_CLIENT_ID' -k 'YOUR_CLIENT_SECRET'\n", config.BinaryName)
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Or run the interactive prompt:")
	fmt.Fprintf(writer, "  %s auth login\n", config.BinaryName)
}
