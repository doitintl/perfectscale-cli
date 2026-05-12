# pscli CLI Reference

Authoritative source: the CLI's own `--help` output and the project README. Use
`pscli <command> --help` at runtime to confirm flags before scripting.

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

## Exit Codes

- `0` success
- non-zero on auth failure, validation error, or API error — stderr carries the
  human-readable message.
