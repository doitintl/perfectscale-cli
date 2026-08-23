# Perfectscale CLI

`pscli` is a small Go CLI for querying Perfectscale's public API with a UI-generated service token.

It is optimized for fast terminal exploration and agent-friendly output, with:

- stored local auth profiles
- sensible production defaults
- table, JSON, and JSONL output
- workload filtering, sorting, aggregation, and CSV export
- GitHub Actions builds for macOS, Windows, and Linux
- Homebrew install via `brew install doitintl/tap/pscli`
- Scoop install on Windows via `scoop bucket add pscli ... && scoop install pscli`
- `.deb`/`.rpm` packages for direct install on Linux

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

## Installation

### Homebrew (macOS and Linux)

```bash
brew install doitintl/tap/pscli
```

To upgrade to the latest release:

```bash
brew upgrade pscli
```

### Scoop (Windows)

```powershell
scoop bucket add pscli https://github.com/doitintl/perfectscale-cli
scoop install pscli
```

To upgrade to the latest release:

```powershell
scoop update pscli
```

### deb/rpm (Linux)

Download the package for your architecture from
[Releases](https://github.com/doitintl/perfectscale-cli/releases/latest) and
install it directly:

```bash
sudo dpkg -i pscli_<version>_linux_amd64.deb   # Debian/Ubuntu
sudo rpm -i pscli_<version>_linux_amd64.rpm    # Fedora/RHEL
```

> This is a direct package install, not a hosted apt/yum repository — there's
> no `apt install pscli` or `add-apt-repository` step.

### From a release archive

Download the archive for your platform from
[Releases](https://github.com/doitintl/perfectscale-cli/releases/latest) and
extract the `pscli` binary onto your `PATH`.

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
- `-g` node group name

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

## Node Group Commands

`nodegroups list` and `nodegroups get` wrap the public InfraFit node-groups
endpoint: node count/pod count, cost and idle breakdown, CPU/memory/GPU
utilization percentiles, and instance-type or Karpenter recommendations per
node group.

Examples:

```bash
pscli nodegroups list -c prod-a
pscli nodegroups list -c prod-a --autoscaler-type karpenter --has-recommendations
pscli nodegroups list -c prod-a --all -o jsonl
pscli nodegroups list -c prod-a -V gpu
pscli nodegroups get -c prod-a -g clickhouse
```

Notes:

- `-V`/`--view`: `default` (cost, CPU/memory, recommendation summary) or
  `gpu` (GPU architecture/utilization; `-` in GPU columns for non-GPU
  groups). `-o json`/`-o jsonl` always include the full payload regardless
  of view.
- `--autoscaler-type`, `--has-recommendations`, `--include-muted` are
  server-side filters.
- `recommendations` is a discriminated union (`standard` or `karpenter`).
  Table output shows a summary only; use `-o json`/`-o jsonl` for the full
  payload, including Karpenter `current_config`/`recommended_config` diffs.
- `-o json` wraps the list as `{"node_groups": [...], "pagination": {...}}`;
  `-o jsonl` emits one node group per line with no cursor.
- `--page-size` is 1–500 (default 50). `--page-token` consumes the cursor
  from `pagination.next` (wire field: `meta.pagination.next`).
- `--all` auto-paginates until no next cursor remains, capped by
  `--page-cap` (default 50), always requesting page size 500 regardless of
  `--page-size` since the backend recomputes the full set each request.

## Unevictable Pod Commands

`unevictable list`, `report`, `show`, and `muted` wrap the public
unevictable-pods endpoints: pods that autoscalers can't evict, why, and what
it's costing. All four are served from the latest pre-computed snapshot for
the cluster — there's no request-time recompute.

Examples:

```bash
pscli unevictable list -c prod-a
pscli unevictable list -c prod-a -n payments --reason pod_disruption_budget
pscli unevictable report -c prod-a -C 5 -s blockedCostHourly -r desc
pscli unevictable show -c prod-a -i a1b2c3d4
pscli unevictable muted -c prod-a
```

Notes:

- `list`/`report` show one row per pod with reason codes concatenated (e.g.
  `pod_disruption_budget,node_selector`); `report` also carries `node` and
  `priority`. `show` returns full single-pod detail — remediation (fix
  summary, risk, confidence, current/recommended spec, unified diff) and
  sibling pod names — not populated by `list`/`report`.
- `-n`, `--reason`, `-g`, `-C`/`--min-blocked-cost` are server-side filters
  (AND-combined). `--reason` only works on `list` — `report`'s filter schema
  doesn't accept it.
- `--mute` controls muted-finding visibility: `exclude` (default), `include`,
  or `only`.
- `-s`/`--sort` only accepts `blockedCostHourly`, the only server-side sort
  key today.
- `muted` is read-only — mute/dismissal rules can only be managed via the
  web app or the user API.
- A 202 (snapshot processing) or 422 (processing failed) response surfaces
  as a distinct error, not a generic HTTP failure.
- Pagination flags (`--page-size`, `--page-token`, `--all`, `--page-cap`)
  match `nodegroups list`. Snapshot metadata (time, algorithm version,
  summary counts) appears in the table footer and top-level `-o json`
  fields, but not in `--all` mode (it spans multiple snapshot reads).

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

CI ([build.yml](./.github/workflows/build.yml)) runs tests plus a skill
sanity build on every `push`/`pull_request` — it never creates releases.

Releases are manual: Actions tab → "Release" → "Run workflow", with a `bump`
input (`patch`/`minor`/`major`, default `patch`).
[release.yml](./.github/workflows/release.yml) cuts the version tag, then
[goreleaser](https://goreleaser.com) ([.goreleaser.yaml](./.goreleaser.yaml))
cross-builds macOS/Windows/Linux (`amd64`/`arm64`) and publishes:

- GitHub Release assets: archives, checksums, `.deb`/`.rpm` packages,
  `perfectscale-skill.zip`
- Homebrew formula → `doitintl/homebrew-tap` (needs the
  `HOMEBREW_TAP_APP_CLIENT_ID`/`HOMEBREW_TAP_APP_PRIVATE_KEY` secrets;
  skipped gracefully until they're set)
- Scoop manifest → this repo's own `bucket/`

Current asset names:

- `pscli-darwin-arm64.tar.gz`
- `pscli-darwin-amd64.tar.gz`
- `pscli-windows-amd64.zip`
- `pscli-windows-arm64.zip`
- `pscli-linux-amd64.tar.gz`
- `pscli-linux-arm64.tar.gz`
- `pscli_<version>_linux_amd64.deb` / `.rpm`
- `pscli_<version>_linux_arm64.deb` / `.rpm`

Each release archive contains a `pscli` binary, or `pscli.exe` on Windows.
`checksums.txt` is published alongside them. The goreleaser config lives in
[.goreleaser.yaml](./.goreleaser.yaml).

`perfectscale-skill.zip` is a portable "skill" bundle for coding agents
(Claude Code, OpenAI Agents SDK, etc.) that teaches them how to drive
`pscli`. Source lives under [plugins/perfectscale](./plugins/perfectscale/SKILL.md).
Build it locally with `make skill`.

## OpenAPI Generation

The local spec is [public-api.yaml](./public-api.yaml); it generates
[internal/publicapi/client.gen.go](./internal/publicapi/client.gen.go) via
`make openapi`. Don't hand-edit the generated client — update the spec and
regenerate. [internal/api/client.go](./internal/api/client.go) is the
handwritten adapter on top: auth headers, response validation, and mapping
into CLI types.

## Known Limits

- only service-token auth is supported
- only the public API is supported
- workloads are fixed to a `30d` period because the public endpoint is fixed-window
- namespace and many workload filters are client-side
- CSV is the only export format in v1

