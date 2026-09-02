# pscli CLI Reference

Authoritative source: `pscli commands -o json` (live command/flag catalog),
the CLI's own `--help` output, and the project README. Use
`pscli <command> --help` at runtime to confirm flag *values* before scripting.

## Global Flags

Available on the top-level command and on every leaf subcommand.

| Long              | Short | Env                          | Default                                          |
| ----------------- | ----- | ---------------------------- | ------------------------------------------------ |
| `--profile`       | `-p`  | `PERFECTSCALE_PROFILE`       | `default`                                        |
| `--output`        | `-o`  | `PERFECTSCALE_OUTPUT`        | `table`                                          |
| `--debug`         | `-d`  | `PERFECTSCALE_DEBUG`         | `false`                                          |
| `--public-api-url`| `-u`  | `PERFECTSCALE_PUBLIC_API_URL`| `https://api.app.perfectscale.io/public/v1`      |

Output modes: `table`, `json`, `jsonl`. `jsonl` only applies to list-shaped
payloads; scalar commands fall back to `json`.

## auth

| Command                         | Notes                                                                     |
| ------------------------------- | ------------------------------------------------------------------------- |
| `pscli auth login`             | Interactive prompt for `client_id`/`client_secret`.                       |
| `pscli auth login -i ID -k SEC`| Non-interactive. `-i` = client-id, `-k` = client-secret.                  |
| `pscli auth status`            | Prints the active profile and validates the stored token.                 |
| `pscli auth logout`            | Deletes the local profile file.                                           |

Profiles live under `os.UserConfigDir()/perfectscale-cli/profiles/<profile>.json`
with `0600` permissions.

## clusters

| Command                                  | Key flags                  |
| ---------------------------------------- | -------------------------- |
| `pscli clusters list`                   | `-s`, `-r`, `-T`, `-B`     |
| `pscli clusters get -c <cluster>`       | `-c` accepts UID or name   |
| `pscli clusters emission -c <cluster>`  | `-s value -r desc`         |

## namespaces

`pscli namespaces list -c <cluster>` — derived from workloads. Supports
`-n` substring filter, `-s workloads|cost|waste`, `-r asc|desc`, `-T`, `-B`.

## workloads

Period is locked to `30d`. Only `-c` is server-side; the rest filter client-side.

### `workloads list`

Flags: `-c -w -n -m -t -C -W -s -r -T -B -V`.

Views (`-V`):

- `default` — cost, waste, namespace, type, max indicator
- `capacity` — replicas + current vs recommended request totals
- `usage` — summed container p90/p95/p100
- `policy` — optimization policy, resilience, mute
- `risk` — severity + risk/waste counts
- `all` — full enriched object (auto-switches to `jsonl` unless `-o` is set)

### `workloads summary`

`pscli workloads summary -c <cluster>` — single object, prefer `-o json`.

### `workloads group-by`

| Subcommand                 | Notes                                                |
| -------------------------- | ---------------------------------------------------- |
| `namespace`                | `-s waste\|cost\|workloads`                          |
| `type`                     | groups by Deployment/StatefulSet/...                 |
| `optimization-policy`      | groups by configured policy                          |
| `risk-severity`            | groups by max severity                               |
| `label -k <key> [-v <val>]`| `-k` required; `-v` filters to a label value         |

All accept `-s`, `-r`, `-T`, `-B`.

### Detail commands

| Command                                                    | Notes                                              |
| ---------------------------------------------------------- | -------------------------------------------------- |
| `pscli workloads show -c X -i <id>`                       | Workload UID lookup.                               |
| `pscli workloads show -c X -m <name> -n <ns>`             | Name+namespace lookup.                             |
| `pscli workloads export -c X -F out.csv`                  | CSV only. Inherits `-n -m -t -C -W -s -r -T -B`.   |
| `pscli workloads risky -c X -S 2 -s severity -r desc -T N`| `-S` is min severity (0–4).                        |
| `pscli workloads labels -c X [-k key] [-v value]`         | Explore label values.                              |
| `pscli workloads muted -c X -s expires -r asc`            | Currently muted workloads.                         |

## nodegroups

InfraFit node group data. Server-side cursor pagination; backend recomputes the full set on every request so `--all` ignores `--page-size` and always uses maximum page size.

### `nodegroups list`

Flags: `-c` (required), `-w` (ISO-8601, default `P30D`), `-V` (`default`|`gpu`), `--autoscaler-type`, `--has-recommendations`, `--include-muted`, `--recommendation-limit` (1-20, default 3), `--page-size` (1-500, default 50), `--page-token`, `--all`, `--page-cap`.

All filters are **server-side**.

Views (`-V`):
- `default` — nodes, pods, cost, CPU/memory request averages, recommendation summary
- `gpu` — GPU architecture and utilization averages; non-GPU node groups show `-`

Output (`-o json`): `{ "node_groups": [...], "pagination": {"next","prev","page_size"} }`
Output (`-o jsonl`): one node group per line (no pagination cursor).

### `nodegroups get`

`pscli nodegroups get -c <cluster> -g <node-group>`

Flags: `-c` (required), `-g` (required, node group name), `-w`, `--recommendation-limit`.

Output: same object shape as one entry from `nodegroups list`, prefer `-o json` for full recommendation payload.

## unevictable

Pods that block autoscaler scale-down. Data comes from a pre-computed snapshot — check `snapshot_time` in output for freshness.

### `unevictable list`

Flags: `-c` (required), `-n`, `--reason`, `-g`, `-C` (min-blocked-cost), `--mute` (`exclude`|`include`|`only`, default `exclude`), `-s` (`blockedCostHourly` only), `-r` (`asc`|`desc`, default `desc`), `--page-size` (1-500, default 50), `--page-token`, `--all`, `--page-cap`.

All filters are **server-side** (AND-combined). `--reason` is not available on `unevictable report`.

Output (`-o json`): `{ "pods": [...], "pagination": {...}, "snapshot_time", "algorithm_version", "summary" }`
Output (`-o jsonl`): one pod per line (no snapshot metadata; use `-o json` for that).

### `unevictable report`

`pscli unevictable report -c <cluster>`

Same flags as `unevictable list` except `--reason`. One row per pod combining all reasons.

Output (`-o json`): `{ "rows": [...], "pagination": {...}, "snapshot_time", "algorithm_version", "summary" }`

### `unevictable show`

`pscli unevictable show -c <cluster> -i <pod-uid>`

Full pod detail including remediation (fix summary, risk, confidence, current/recommended spec, unified diff) and sibling pod names.

### `unevictable muted`

`pscli unevictable muted -c <cluster>`

Lists workloads with active mute/dismissal rules. Flags: `--page-size`, `--page-token`, `--all`, `--page-cap`. Read-only — rules can only be created/removed via the web app.

## automation

`pscli automation audit-logs` — last 30 days, cursor pagination.

| Flag              | Meaning                                                             |
| ----------------- | ------------------------------------------------------------------- |
| `-c <cluster>`    | Repeatable. UID or name. **Server-side filter.**                    |
| `-n <namespace>`  | Repeatable. **Server-side filter.**                                 |
| `--from RFC3339`  | UTC. Pair with `--to`.                                              |
| `--to RFC3339`    | UTC.                                                                |
| `--since 24h`     | Shorthand for `--from now-since`. Accepts `30m`, `24h`, `7d`, ...   |
| `--page-size N`   | 1–5000 (default 1000).                                              |
| `--after CURSOR`  | Cursor token from a previous `pagination.next`.                     |
| `--before CURSOR` | Cursor token from a previous `pagination.prev`.                     |
| `--all`           | Auto-paginate forward until `has_next=false`.                       |
| `--page-cap N`    | Safety cap when `--all` is set (default 50).                        |
| `--execution V`   | Client-side filter: `regular-eviction`, `inplace-resize`, `cleanup`.|

Recommended for agents: `--since 24h --all -o jsonl`.

## open

Opens the matching Perfectscale web UI page in the default browser —
`-o table` (the default) launches a real browser process on the machine
`pscli` runs on. **`-o json`/`-o jsonl` print `{"url": "..."}` instead and do
not open anything** — always use one of these in an agent/scripting context.

| Command                                                              | Notes                                                                 |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `pscli open cluster -c <cluster> [-w <period>]`                      | Resolves `-c` via the API. `-w` (default `30d`) is the UI window only. |
| `pscli open workload -c <cluster> -i <id> [-w <period>]`             | Same `-i`/`-m`/`-n` resolution rules as `workloads show`.              |
| `pscli open workload -c <cluster> -m <name> -n <namespace>`          |                                                                        |
| `pscli open nodegroup -c <cluster> -g <node-group> [-w <period>]`    | `-g` is not validated against the API.                                 |
| `pscli open alerts -c <cluster>`                                     | Opens the Alerts/Resilience view filtered to this cluster.            |
| `pscli open automation [-c][-n][-m][-t][--container][-w]`            | All filters optional, **unresolved** passthrough query params — unlike the other `open` subcommands, `-c` here is not looked up via the API. |

For `open workload`, `-w`/`--period` only sets the UI's display window; the
workload lookup itself always uses the public API's fixed 30-day window.

## commands

`pscli commands -o json` — walks the live command tree. No API call, no auth.

`-o json` emits one document `{version, cli_version, commands}`. Each command
has `path`, `usage`, `description`, `runnable`, and `flags`. Each flag has
`name`, `aliases`, `type`, `usage`, `default`, `env_vars`, `required`, and
`runtime` (`true` for profile/output/debug/public-api-url). `-o jsonl` emits
one command object per line. Table (default) prints `PATH` and `USAGE`.

Does not list allowed flag values or illegal flag combinations — use
`pscli <command> --help` for those.

## skill

`pscli skill <agent>` — copies the skill compiled into this binary into the
agent's user-level skills directory. Agents: `claude`, `codex`, `cursor`,
`gemini`, `kiro`, `opencode`. `--all` installs wherever the agent config dir
exists. `--dir` is only on a named agent (`pscli skill cursor --dir …` or
`pscli skill update cursor --dir …`); it is not valid with `--all`.
`--force` overwrites local edits after backing them up. `pscli skill list`
lists embedded files. `pscli skill update` refreshes existing installs.
No API call.

## update

`pscli update` — checks GitHub for a newer release and prints how to upgrade.
Does not install anything. `-o json`/`-o jsonl` print
`{current, latest, update_available, instruction}`.

## Exit Codes

- `0` success
- non-zero on auth failure, validation error, or API error — stderr carries the
  human-readable message.

## Short-Flag Quick Reference

`-p` profile · `-o` output · `-u` public-api-url · `-d` debug · `-c` cluster ·
`-w` period · `-n` namespace · `-m` workload name · `-t` workload type ·
`-s` sort · `-r` order · `-T` top N · `-B` bottom N ·
`-C` min-cost / min-blocked-cost · `-W` min-waste · `-V` view · `-i` id/client-id ·
`-k` client-secret / label key · `-v` label value · `-S` min-severity ·
`-g` node group name · `-f` export format · `-F` export file path.

`open automation`'s `--container` filter has no short alias.
