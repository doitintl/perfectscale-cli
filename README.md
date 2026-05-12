# Perfectscale CLI

`pscli` is a small Go CLI for querying Perfectscale's public API with a UI-generated service token.

It is optimized for fast terminal exploration and agent-friendly output, with:

- stored local auth profiles
- sensible production defaults
- table, JSON, and JSONL output
- workload filtering, sorting, aggregation, and CSV export
- GitHub Actions builds for macOS, Windows, and Linux

## What It Supports

This CLI is intentionally public-API only.

Current command groups:

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
- `automation`
  - `audit-logs`

## Authentication

The CLI uses Perfectscale public API service tokens from the Perfectscale UI.

To generate one:

1. Open `app.perfectscale.io`
2. Click the user circle in the bottom left
3. Open `Org Settings`
4. Open `API Tokens`
5. Click `Generate Token`
6. Assign a `Read Only` role
7. Copy the `client_id` and `client_secret`

If you run `pscli auth` without saved credentials, the CLI prints the same setup guide plus a ready-to-copy login example.

Then log in:

```bash
pscli auth login
```

Or pass the credentials directly:

```bash
pscli auth login --client-id 'YOUR_CLIENT_ID' --client-secret 'YOUR_CLIENT_SECRET'
```

The CLI validates the token by calling the public auth endpoint and saves the profile locally.

## Defaults

The CLI is designed so common usage does not require flags.

Default profile:

```text
default
```

Default output:

```text
table
```

Default public API base URL:

```text
https://api.app.perfectscale.io/public/v1
```

If you do not export any environment variables and do not pass any global flags, the CLI will talk to production.

## Environment Variables

Global flags can be set through environment variables:

- `PERFECTSCALE_PROFILE`
- `PERFECTSCALE_OUTPUT`
- `PERFECTSCALE_DEBUG`
- `PERFECTSCALE_PUBLIC_API_URL`

Examples:

```bash
export PERFECTSCALE_PROFILE='dev-public'
export PERFECTSCALE_PUBLIC_API_URL='https://api.dev.perfectscale.click/public/v1'
export PERFECTSCALE_OUTPUT='jsonl'
```

## Local Credential Storage

Profiles are stored under the OS config directory.

On macOS, the default profile path is usually:

```text
~/Library/Application Support/perfectscale-cli/profiles/default.json
```

Storage behavior:

- profile directory permissions: `0700`
- profile file permissions: `0600`
- `auth logout` deletes the selected local profile

## Build And Run

Requirements:

- Go 1.24+

Run directly:

```bash
go run . clusters list
```

Build locally:

```bash
make build
./dist/pscli clusters list
```

Or build the binary directly:

```bash
go build -o pscli .
./pscli clusters list
```

Regenerate the local public API client:

```bash
make openapi
```

Run tests:

```bash
go test ./...
```

## Global Options

These flags work at the top level and on leaf commands.

- `--profile`, `-p`
- `--output`, `-o`
- `--debug`, `-d`
- `--public-api-url`, `-u`

Output modes:

- `table`
  - human-friendly terminal output
- `json`
  - one JSON document
- `jsonl`
  - one JSON object per line for list commands and automation

Example:

```bash
pscli -o jsonl workloads list -c prod-a -s waste -r desc -T 10
```

## Common Short Options

The CLI uses short options consistently across commands:

- `-p` profile
- `-o` output
- `-u` public API URL
- `-d` debug
- `-c` cluster
- `-w` period window
- `-n` namespace
- `-m` workload name
- `-t` workload type
- `-s` sort
- `-r` order
- `-T` top
- `-B` bottom
- `-C` min-cost
- `-W` min-waste
- `-V` workload view
- `-i` id or client-id, depending on command
- `-k` client-secret or label key, depending on command
- `-f` export format
- `-F` export file
- `-S` min-severity
- `-v` label value

## Quick Start

Log in:

```bash
pscli auth login
```

Check auth:

```bash
pscli auth status
```

List clusters:

```bash
pscli clusters list
```

Inspect one cluster:

```bash
pscli clusters get -c prod-a
```

Show top wasteful workloads:

```bash
pscli workloads list -c prod-a -s waste -r desc -T 10
```

Show least wasteful workloads:

```bash
pscli workloads list -c prod-a -s waste -r asc -B 10
```

List namespaces:

```bash
pscli namespaces list -c prod-a -s workloads -r desc
```

## Workload Filtering

`workloads list` supports client-side filtering and ranking with:

- `--cluster`, `-c`
- `--period`, `-w`
- `--namespace`, `-n`
- `--name`, `-m`
- `--type`, `-t`
- `--min-cost`, `-C`
- `--min-waste`, `-W`
- `--sort`, `-s`
- `--order`, `-r`
- `--top`, `-T`
- `--bottom`, `-B`
- `--view`, `-V`

Important:

- the public workloads API is fixed to `30d` today
- the CLI enforces `--period 30d`
- non-cluster filters are applied client-side after the workload list is fetched

Examples:

```bash
pscli workloads list -c prod-a -n kube-system -s waste -r desc
pscli workloads list -c prod-a -m api -t Deployment -C 25 -W 10
pscli workloads list -c prod-a -s cost -r desc -T 20
```

## Workload Views

`workloads list` supports view presets through `--view` or `-V`.

Available views:

- `default`
  - cost, waste, namespace, type, and max-indicator overview
- `capacity`
  - replica counts and current vs recommended request totals
- `usage`
  - summed container usage percentiles
- `policy`
  - optimization policy, resilience, and mute state
- `risk`
  - risk severity, risk counts, and waste counts
- `all`
  - the broadest enriched workload view

Special behavior:

- if `--view all` is used without explicitly setting `--output`, the CLI switches to `jsonl`
- this makes the full enriched workload objects easier to consume in pipelines and by agents

Examples:

```bash
pscli workloads list -c prod-a -V default
pscli workloads list -c prod-a -V capacity
pscli workloads list -c prod-a -V usage
pscli workloads list -c prod-a -V policy
pscli workloads list -c prod-a -V risk
pscli workloads list -c prod-a -V all
pscli -o json workloads list -c prod-a -V all
```

## Workload Aggregations

Cluster summary:

```bash
pscli workloads summary -c prod-a
```

Group by namespace:

```bash
pscli workloads group-by namespace -c prod-a -s waste -r desc -T 10
```

Group by workload type:

```bash
pscli workloads group-by type -c prod-a -s workloads -r desc
```

Group by optimization policy:

```bash
pscli workloads group-by optimization-policy -c prod-a -s waste -r desc
```

Group by risk severity:

```bash
pscli workloads group-by risk-severity -c prod-a -s workloads -r desc
```

Group by label value:

```bash
pscli workloads group-by label -c prod-a -k team -s waste -r desc
```

## Detailed Workload Commands

Show one workload:

```bash
pscli workloads show -c prod-a -i workload-123
pscli workloads show -c prod-a -m api -n backend
```

Export CSV:

```bash
pscli workloads export -c prod-a -F workloads.csv
pscli workloads export -c prod-a -n kube-system -s waste -r desc -T 25
```

List risky workloads:

```bash
pscli workloads risky -c prod-a -S 2 -s severity -r desc -T 10
```

Explore workload labels:

```bash
pscli workloads labels -c prod-a
pscli workloads labels -c prod-a -k app -s waste -r desc -T 20
pscli workloads labels -c prod-a -v production
```

List muted workloads:

```bash
pscli workloads muted -c prod-a -s expires -r asc
```

## Cluster Commands

List clusters:

```bash
pscli clusters list
```

Get cluster details:

```bash
pscli clusters get -c prod-a
```

Show carbon emission metrics:

```bash
pscli clusters emission -c prod-a -s value -r desc
```

## Namespace Commands

Namespaces are derived from workloads.

Examples:

```bash
pscli namespaces list -c prod-a
pscli namespaces list -c prod-a -s workloads -r desc
pscli namespaces list -c prod-a -n kube -T 5
```

## Automation Commands

`automation audit-logs` lists the actions Perfectscale's automation took in your
clusters. The endpoint is cursor-paginated and returns events from the last 30
days.

Examples:

```bash
pscli automation audit-logs
pscli automation audit-logs -c prod-a -c prod-b
pscli automation audit-logs -c prod-a -n kube-system -n default
pscli automation audit-logs --from 2026-04-01T00:00:00Z --to 2026-04-15T00:00:00Z
pscli automation audit-logs --since 24h
pscli automation audit-logs --all -o jsonl
pscli automation audit-logs --page-size 200 --after BASE64CURSOR
```

Notes:

- `--cluster` (`-c`) and `--namespace` (`-n`) are repeatable. Cluster values
  may be UID or name.
- `--from` and `--to` accept RFC3339 (UTC). `--since` accepts a relative
  duration (`24h`, `7d`, `30m`) and is shorthand for `--from now-since`.
- `--page-size` is 1–5000 (default 1000).
- `--after` / `--before` consume cursor tokens from a previous response's
  `pagination.next` / `pagination.prev`.
- `--all` auto-paginates forward until the server reports `has_next=false`,
  capped by `--page-cap` (default 50) as a safety net.
- `--execution` filters client-side to one of `regular-eviction`,
  `inplace-resize`, or `cleanup`.

## Release Workflow

GitHub Actions is configured in [build.yml](./.github/workflows/build.yml).

Behavior:

- runs tests on every `push` and `pull_request`
- cross-builds binaries for:
  - macOS `arm64`
  - Windows `amd64`
  - Linux `amd64`
  - Linux `arm64`
- uploads workflow artifacts for each target
- on pushes to the repository default branch:
  - determines the next version starting at `v1.0.0`
  - increments the patch version on each new commit
  - creates or reuses a GitHub Release
  - uploads all built binaries as release assets

Current asset names:

- `pscli-darwin-arm64.tar.gz`
- `pscli-windows-amd64.zip`
- `pscli-linux-amd64.tar.gz`
- `pscli-linux-arm64.tar.gz`

Each release archive contains a `pscli` binary, or `pscli.exe` on Windows.

In addition, every release publishes `perfectscale-skill.zip` — a portable
"skill" bundle for coding agents (Claude Code, OpenAI Agents SDK, etc.) that
teaches them how to drive `pscli`. Source lives under [skill/perfectscale](./skill/perfectscale).
Build it locally with `make skill`.

## OpenAPI Generation

This repo keeps its own local copy of the public OpenAPI spec at:

- [public-api.yaml](./public-api.yaml)

The generated public API client lives at:

- [internal/publicapi/client.gen.go](./internal/publicapi/client.gen.go)

Important rules:

- do not hand-edit the generated client
- update the local YAML spec first
- regenerate with `make openapi`
- the handwritten adapter in [internal/api/client.go](./internal/api/client.go) stays responsible for:
  - auth headers
  - response validation
  - mapping generated types into CLI types
  - derived workload fields used by views and summaries

## Known Limits

- only service-token auth is supported
- only the public API is supported
- workloads are fixed to a `30d` period because the public endpoint is fixed-window
- namespace and many workload filters are client-side
- there is no first-class public nodegroup command yet
- CSV is the only export format in v1

## Next Steps

These are the next improvements we discussed, ordered by how much they unlock for the CLI with minimal API churn.

### Public API improvements

1. Add `period` support to public workloads.
   This would unlock `1d` and `7d` views for cost and waste instead of the current fixed `30d` behavior.

2. Add a few optional server-side filters to public workloads.
   Best first candidates:
   - `namespace`
   - `name`
   - `type`
   - `node_group`
   - `node_type`
   - `reservation_type`

   This would reduce payload size, speed up the CLI, and make agent queries more precise.

3. Expose nodegroup placement on each workload.
   Best shapes:
   - `primaryNodeGroup`
   - or better, `runningMinutesByNodeGroup`

   This would enable:
   - `nodegroups list`
   - `group-by nodegroup`
   - "top wasteful workloads on nodegroup X"

### CLI follow-ups once the API expands

- add `workloads group-by nodegroup`
- add `nodegroups list --cluster ...`
- allow non-`30d` workload periods
- push more filters server-side when the API supports them

### CLI follow-ups that might still useful even without API changes

- add `--fields` for exact field projection on `workloads list`, `show`, and `export`
- add `--raw` on `workloads show`
- add `workloads containers` for per-container inspection
- add more export formats if needed beyond CSV

## Repo Layout

Key directories and files:

- [main.go](./main.go)
  - CLI entrypoint
- [internal/cli](./internal/cli)
  - command definitions, runtime, rendering, aggregations
- [internal/api](./internal/api)
  - public API adapter and response mapping
- [internal/publicapi](./internal/publicapi)
  - generated public OpenAPI client
- [public-api.yaml](./public-api.yaml)
  - local public OpenAPI spec copy used for generation
- [internal/auth](./internal/auth)
  - service-token exchange and token refresh
- [internal/profile](./internal/profile)
  - local profile storage
- [internal/output](./internal/output)
  - table, JSON, and JSONL output
- [Makefile](./Makefile)
  - OpenAPI regeneration and common developer tasks
- [.github/workflows/build.yml](./.github/workflows/build.yml)
  - CI, cross-builds, and releases
