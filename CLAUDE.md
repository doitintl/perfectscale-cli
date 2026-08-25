# CLAUDE.md

This file gives future agents and maintainers quick project context for the Perfectscale CLI repository.

## Project Summary

This repo contains a Go CLI named `pscli` for querying Perfectscale's public API using a UI-generated service token.

It is deliberately scoped to the public API and does not currently support:

- user JWT auth
- browser SSO
- tenant-manager user APIs
- data-provider user-only APIs

The repo currently targets a compact, agent-friendly CLI with good defaults and strong help output.

## Current Product Shape

Supported command groups:

- `auth`
  - `login`
  - `status`
  - `logout`
- `clusters`
  - `list`
  - `get`
  - `emission`
- `namespaces`
  - `list`
- `workloads`
  - `list`
  - `summary`
  - `group-by namespace`
  - `group-by type`
  - `group-by optimization-policy`
  - `group-by risk-severity`
  - `group-by label`
  - `show`
  - `export`
  - `risky`
  - `labels`
  - `muted`
- `nodegroups`
  - `list`
  - `get`
- `unevictable`
  - `list`
  - `report`
  - `show`
  - `muted`
- `automation`
  - `audit-logs`
- `update`

Design goals:

- strong terminal UX
- clear short flags
- JSON and JSONL support for automation
- minimal config required for the happy path
- public-prod defaults out of the box

## Defaults And Runtime Behavior

Global defaults are defined in [internal/config/config.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/config/config.go):

- profile: `default`
- output: `table`
- public API URL: `https://api.app.perfectscale.io/public/v1`

Runtime flag behavior is implemented in [internal/cli/app.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/cli/app.go) and [internal/cli/context.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/cli/context.go):

- runtime flags are attached to top-level commands and leaf subcommands
- values are resolved through command lineage, so flags like `-o json` work before or after nested commands

## Authentication Model

Only service-token auth is supported now.

Auth flow:

1. User runs `pscli auth login`
2. CLI exchanges `client_id` and `client_secret` via `POST /auth/public_auth`
3. CLI stores the profile locally
4. Later commands auto-refresh the access token by repeating the service-token exchange

Relevant files:

- [internal/cli/auth.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/cli/auth.go)
- [internal/auth/service_token.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/auth/service_token.go)
- [internal/auth/manager.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/auth/manager.go)
- [internal/profile/store.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/profile/store.go)

Local profile storage:

- base dir defaults to `os.UserConfigDir()/perfectscale-cli/profiles`
- files are written with `0600`
- directories are created with `0700`

## Public API Scope

The CLI is built around the public API and should stay self-contained.

Important current limitations:

- workload period is effectively `30d` only
- namespace, name, type, min-cost, and min-waste filters are client-side
- namespaces are derived from workloads
- there is no authoritative public nodegroup endpoint wired into the CLI
- automation audit logs are limited to the last 30 days, with cursor pagination
  (no offset-based access); `cluster_uids` and `namespaces` are server-side
  filters, while `--execution` is client-side

When proposing new features, prefer:

1. exposing small additions to the existing public workload endpoint
2. reusing data the backend already has
3. keeping the CLI standalone, without importing sibling services as libraries

## OpenAPI Client Generation

This repo keeps its own local copy of the public OpenAPI spec and generates its own public client from that file.

Relevant files:

- [public-api.yaml](/Users/amit.bezalel/workspace/perfectscale/poc-cli/public-api.yaml)
- [internal/publicapi/client.gen.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/publicapi/client.gen.go)
- [Makefile](/Users/amit.bezalel/workspace/perfectscale/poc-cli/Makefile)

Rules:

- do not hand-edit `internal/publicapi/client.gen.go`
- edit the local YAML spec first
- regenerate with `make openapi`

The generated client is intentionally low-level. The handwritten adapter in [internal/api/client.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/api/client.go) remains responsible for:

- auth headers
- response validation
- mapping generated types into CLI types
- deriving enriched workload fields

## Workload Data Model

Workload mapping lives in:

- [internal/api/types.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/api/types.go)
- [internal/api/client.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/api/client.go)

The CLI already enriches workloads with derived values such as:

- container count
- summed current request totals
- summed recommended request totals
- summed container p90/p95/p100 usage
- max indicator / risk / waste counts

That enrichment is used by:

- workload views
- summaries
- group-by commands
- CSV export

If adding new workload features, check whether they should be:

- part of the raw mapped API model
- part of the derived model
- exposed as a new `--view`
- exposed in CSV export

## Output Model

Output rendering is centralized in:

- [internal/output/output.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/output/output.go)
- [internal/output/table.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/output/table.go)
- [internal/cli/context.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/cli/context.go)

Modes:

- `table`
- `json`
- `jsonl`

Important behavior:

- `jsonl` is supported only for list-like payloads
- scalar commands fall back to regular JSON when needed
- `workloads list --view all` implicitly switches to `jsonl` unless output was explicitly set

## Workload Views

`workloads list` supports:

- `default`
- `capacity`
- `usage`
- `policy`
- `risk`
- `all`

Implementation lives in:

- [internal/cli/workload_views.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/cli/workload_views.go)
- [internal/cli/workloads.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/cli/workloads.go)

When adding a field that should be visible in normal list output, consider whether it belongs in:

- one of the existing named views
- a new view
- only `view=all`

## Short Flag Conventions

Try to preserve the current flag scheme unless there is a very strong reason to change it.

Current conventions:

- `-p` profile
- `-o` output
- `-u` public API URL
- `-d` debug
- `-c` cluster
- `-w` period
- `-n` namespace
- `-m` workload name
- `-t` workload type
- `-s` sort
- `-r` order
- `-T` top
- `-B` bottom
- `-C` min-cost
- `-W` min-waste
- `-V` view
- `-i` id or client-id
- `-k` client-secret or label key
- `-S` min-severity
- `-f` format
- `-F` file
- `-v` label value
- `-g` node group name

Before adding a new short flag, make sure it does not overlap in a confusing way with nearby commands.

## Testing

Main test areas already in the repo:

- API parsing
- service-token auth and refresh behavior
- profile storage
- CLI help and flag parsing
- workload filtering, sorting, limiting, summaries, and views

Run the full suite with:

```bash
go test ./...
```

When making command-surface changes, update:

- command help text
- app-level description/examples
- relevant CLI tests

## CI And Releases

CI is in [build.yml](./.github/workflows/build.yml):
tests + a `make skill` sanity build on every push/PR. It does not create
releases — merging to the default branch does not cut a new version.

Releases are triggered manually via
[release.yml](./.github/workflows/release.yml)
(`workflow_dispatch`, with a `bump: patch|minor|major` input, default
`patch`). It computes the next version (or reuses an existing tag on the
selected commit), creates the GitHub Release, then runs
[goreleaser](https://goreleaser.com) (`.goreleaser.yaml`) to cross-build
darwin/arm64+amd64, windows/amd64+arm64, linux/amd64+arm64, archive/checksum
them, upload them as release assets, publish the Homebrew formula to the
shared tap `doitintl/homebrew-tap` (skipped gracefully if the
`HOMEBREW_TAP_APP_CLIENT_ID`/`HOMEBREW_TAP_APP_PRIVATE_KEY` secrets aren't
set yet), publish the Scoop manifest to this repo's own `bucket/` directory
(no separate repo/secret — pushed with `GITHUB_TOKEN`), and build `.deb`/
`.rpm` packages for direct install (not a hosted apt/yum repository). It
also uploads `perfectscale-skill.zip` as a release asset.

Pinned actions are required in this repo. Do not switch back to floating `@vN` refs.

## Documentation Expectations

If you change command behavior, also update:

- [README.md](/Users/amit.bezalel/workspace/perfectscale/poc-cli/README.md)
- [internal/cli/app.go](/Users/amit.bezalel/workspace/perfectscale/poc-cli/internal/cli/app.go) top-level description
- command-specific `Description` blocks in the relevant CLI file
- this file's own "Current Product Shape" command list above

The docs should stay aligned with the actual flags and defaults. Avoid aspirational docs.

## Good Next Public API Enhancements

If the product team asks what minimal API changes unlock more CLI value, the strongest current answers are:

1. add `period` support to public workloads
2. add public server-side filters such as `namespace`, `type`, `name`, `node_group`, `node_type`, `reservation_type`
3. expose workload nodegroup placement, ideally as `runningMinutesByNodeGroup` or a similar authoritative field

Those changes would unlock a lot more CLI surface with limited backend churn.
