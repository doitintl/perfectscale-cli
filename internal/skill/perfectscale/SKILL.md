---
name: perfectscale
description: Query Perfectscale Kubernetes cost, waste, risk, and automation data through the `pscli` public-API CLI. Use this skill whenever the user asks about Perfectscale clusters, namespaces, workloads (cost, waste, recommendations, risk severity, optimization policy, mute state, labels), node groups (InfraFit utilization, cost, recommendations), unevictable pods (autoscaler block reasons, blocked cost, remediation), cluster carbon emission, or automation audit logs. Trigger on phrases like "perfectscale", "pscli", "kubernetes waste", "k8s cost optimization", "rightsizing recommendations", "wasteful workloads", "unevictable", "node groups", "infrafit".
---

# Perfectscale CLI Skill

Teaches you to use `pscli`, a Go CLI wrapping Perfectscale's public API. It's the only supported path through this skill — don't call the public API directly.

## When To Use

- list/inspect Perfectscale-monitored Kubernetes clusters
- find wasteful, costly, or risky workloads; group by namespace, type, optimization policy, risk severity, or label
- export workload data as CSV or JSONL
- inspect InfraFit node groups — utilization, cost, node-type recommendations (standard or Karpenter)
- find unevictable pods blocking autoscaler scale-down, why, and what they cost
- review automation audit logs (eviction, in-place resize, cleanup)
- check cluster carbon emission

## Bootstrap

1. Verify the binary: `pscli --help`. If missing, ask the user to install it (`brew install doitintl/tap/pscli`, or on Windows `scoop bucket add pscli https://github.com/doitintl/perfectscale-cli && scoop install pscli`; `.deb`/`.rpm` and GitHub release archives are documented in the [README](https://github.com/doitintl/perfectscale-cli)). Once `pscli` is on PATH, they can install this skill with `pscli skill <agent>` (or `pscli skill --all`). Check for upgrades with `pscli update`.

2. `pscli` is multi-profile (default name: `default`) — discover the authenticated profile before running commands:

   ```bash
   pscli auth status >/dev/null 2>&1 && echo default
   # If that fails, list profiles and try each until one authenticates:
   # ($XDG_CONFIG_HOME, ~/Library/Application Support on macOS, or %AppData%)/perfectscale-cli/profiles/<name>.json
   ls "${XDG_CONFIG_HOME:-$HOME/Library/Application Support}/perfectscale-cli/profiles/" 2>/dev/null \
     || ls "$HOME/.config/perfectscale-cli/profiles/" 2>/dev/null
   # For each <name>.json: pscli -p <name> auth status
   ```

   Export the working profile for the session: `export PERFECTSCALE_PROFILE=<name>` (or pass `-p <name>` every call).

   If none authenticate, get a **service token** from the user: `https://app.perfectscale.io` → bottom-left avatar → Organization Settings → API Tokens → Generate Token (Read Only role) → copy `client_id`/`client_secret`. Then `pscli auth login --client-id "$CLIENT_ID" --client-secret "$CLIENT_SECRET"`. Never echo/log the secret; prefer env vars when scripting.

3. Default endpoint is production (`https://api.app.perfectscale.io/public/v1`). Override only if asked (`PERFECTSCALE_PUBLIC_API_URL` or `-u`).

## Output Modes

Always pick output for the consumer:

- `-o table` (default) — only when streaming directly to a human terminal.
- `-o json` — single pretty-printed document. Use for `show`/`summary`/`get`/single-record reads.
- `-o jsonl` — one compact JSON object per line. **Use for any list/group-by/export output you need to parse** — far easier with `jq -c`/`jq -s` than table output.

**`-o jsonl` drops the pagination cursor** on cursor-paginated commands (`nodegroups list`, `unevictable list`/`report`, `automation audit-logs`) — it only prints the current page, no signal more pages exist (`-o json` has a `pagination` object with `next`; `-o table` prints a footer hint; `-o jsonl` has neither). Default to pairing `-o jsonl` with `--all` on these commands. Use `-o json` instead only if paging manually via `--page-token`.

`workloads list --view all` auto-promotes to `jsonl` unless `-o` is explicit.

**Don't guess the command surface or output fields.** Run `pscli commands -o json` for the live catalog (paths, flag types, short-flag `aliases`, `runtime` for global flags). Every command's `--help` ends with an `Output schema` block (field names, types, nesting) for `-o json`/`-o jsonl`. Run `pscli <command> --help` before `jq`-ing an unfamiliar response.

## Core Command Cheatsheet

```bash
# Discover commands and flags
pscli commands -o json

# Clusters
pscli clusters list
pscli clusters get -c <cluster>
pscli clusters emission -c <cluster> -s value -r desc

# Namespaces (derived from workloads)
pscli namespaces list -c <cluster> -s workloads -r desc

# Workloads — list & filter (period is fixed to 30d server-side)
pscli workloads list -c <cluster> -V default
pscli workloads list -c <cluster> -V all                 # auto-jsonl, full enriched objects
pscli workloads list -c <cluster> -n kube-system -s waste -r desc -T 10
pscli workloads list -c <cluster> -m api -t Deployment -C 25 -W 10
pscli workloads list -c <cluster> -V capacity            # replicas, current vs recommended
pscli workloads list -c <cluster> -V usage               # p90/p95/p100 sums
pscli workloads list -c <cluster> -V policy              # opt policy, resilience, mute
pscli workloads list -c <cluster> -V risk                # severity & risk counts

# Aggregations
pscli workloads summary -c <cluster>
pscli workloads group-by namespace          -c <cluster> -s waste -r desc -T 10
pscli workloads group-by type               -c <cluster> -s workloads -r desc
pscli workloads group-by optimization-policy -c <cluster> -s waste -r desc
pscli workloads group-by risk-severity      -c <cluster> -s workloads -r desc
pscli workloads group-by label              -c <cluster> -k team -s waste -r desc

# Inspection / export
pscli workloads show   -c <cluster> -i <workload-id>
pscli workloads show   -c <cluster> -m <name> -n <namespace>
pscli workloads export -c <cluster> -F /tmp/workloads.csv
pscli workloads risky  -c <cluster> -S 2 -s severity -r desc -T 10
pscli workloads labels -c <cluster> -k app -s waste -r desc
pscli workloads muted  -c <cluster> -s expires -r asc

# Automation audit logs (cursor-paginated, last 30 days)
pscli automation audit-logs --since 24h --all -o jsonl
pscli automation audit-logs -c prod-a -c prod-b -n kube-system --all -o jsonl
pscli automation audit-logs --execution inplace-resize --all -o jsonl

# Node groups (InfraFit)
pscli nodegroups list -c <cluster>
pscli nodegroups list -c <cluster> --autoscaler-type karpenter --has-recommendations
pscli nodegroups list -c <cluster> -V gpu --all -o jsonl
pscli nodegroups get -c <cluster> -g <node-group>

# Unevictable pods
pscli unevictable list -c <cluster>
pscli unevictable list -c <cluster> -n payments --reason pod_disruption_budget
pscli unevictable list -c <cluster> -C 5 -s blockedCostHourly -r desc
pscli unevictable list -c <cluster> --mute include --all -o jsonl
pscli unevictable report -c <cluster> -C 5 -s blockedCostHourly -r desc
pscli unevictable show -c <cluster> -i <pod-uid>
pscli unevictable muted -c <cluster>
```

## Short-Flag Reference

Stable across commands — memorize instead of typing `--long`:

`-p` profile · `-o` output · `-u` public-api-url · `-d` debug · `-c` cluster ·
`-w` period (30d only) · `-n` namespace · `-m` workload name · `-t` workload type ·
`-s` sort · `-r` order (`asc`/`desc`) · `-T` top N · `-B` bottom N ·
`-C` min-cost / min-blocked-cost · `-W` min-waste · `-V` view · `-i` id/client-id ·
`-k` client-secret / label key · `-v` label value · `-S` min-severity ·
`-g` node group name · `-f` export format · `-F` export file path.

## Hard Limits (Don't Lie To The User)

- Workload period is **30d only** — `-w` accepts nothing else right now.
- `--namespace`/`--name`/`--type`/`--min-cost`/`--min-waste` are **client-side** for workloads (fetches full list, filters locally) — prefer `-T`/`-B` + sort on huge clusters.
- Namespaces are **derived** from workloads; no namespace endpoint.
- `clusters list` can return empty/null `uid` (agent hasn't reported in yet — typically test/demo clusters) — not an error; reference by `name` with `-c` instead.
- **Cluster names aren't guaranteed unique** (seen live: two `karpenter-metrics` clusters). `-c <name>` errors cleanly with both UIDs listed; resolve with the UID.
- `nodegroups list` filters (`--autoscaler-type`, `--has-recommendations`, `--include-muted`) are **server-side**, cursor-paginated; `--all` always uses max page size (backend recomputes the full set every request).
- `unevictable` filters (`-n`, `--reason`, `-g`, `-C`) are **server-side**, AND-combined. `--reason` is `list`-only, not `report`. Data is a pre-computed snapshot — check `snapshot_time` for freshness.
- Audit logs: last 30 days only, cursor-paginated (no offset), `--execution` filtered client-side.
- Only service-token auth — no SSO/JWT.
- CSV is the only `workloads export` format.

If asked for something outside this surface, say so and suggest the closest supported command.

## Parsing & Interpretation Gotchas

- **`unevictable`'s `blocked_cost_hourly` is the pod's *node* cost, not a per-pod share.** Dedupe by node before summing across pods sharing one:
  ```bash
  jq '[.rows[] | {node, blocked_cost_hourly}] | unique_by(.node) | map(.blocked_cost_hourly) | add'
  ```
- **`waste`/`potential_savings` can be stale or legitimately zero.** A right-sized workload can still show leftover nonzero `waste` — check `indicators`/`max_indicator` are non-empty, or current-vs-recommended resources already match, before treating it as an open opportunity. Conversely `potential_savings: 0` alongside nonzero `waste` is a real, confirmed state (verified live) — they're independent metrics, and the field is never dropped for being zero.
- **`nodegroups` recommendations: check `recommendations.type` first.** `karpenter` = NodePool config diffs (no ranked instance list); `standard` = ranked `node_type_recommendations` with `estimated_savings`/`estimated_savings_pct`. `estimated_savings` is a **monthly** dollar figure (confirmed against the web UI's "Forecast $X → $Y · Save $N" panel), and its pct is `estimated_savings / currentMonthlyForecast` — a different, independent basis from this node group's own `cost.timeframe` field. Don't cross-check one against the other.
- **Presence in `automation audit-logs` is a weak "is this automated" signal** — only covers 30 days, so an older-automated, since-converged workload won't appear.
- **Multi-line fields** (unevictable's `current_spec`/`recommended_spec`/`yaml_diff`) contain literal `\n` escapes — use `jq -r`, not default `jq`, to render them.
- **Use `printf '%s\n' "$var"`, not `echo "$var"`**, when piping captured JSON through a shell variable — zsh's `echo` can turn `\n` into a real newline, corrupting embedded-newline strings before `jq` sees them.

## Recipes

**Top-10 waste in production:**
```bash
pscli -o jsonl workloads list -c prod-a -s waste -r desc -T 10 -V all
```

**Environment-wide savings scan (waste + node-type swaps + blocked cost):**
```bash
# 1. Discover clusters that actually have live data (skip empty/null uid).
pscli -o jsonl clusters list | jq -r 'select(.uid != "" and .uid != null) | .name' > clusters.txt

# 2. Waste/potential-savings per cluster. Pace the loop — each cluster-scoped
#    call costs 2 requests (name resolve + the call itself), and the API caps
#    around 120 req/min, so an unthrottled loop over many clusters will start
#    silently failing partway through.
while IFS= read -r c; do
  pscli -o json workloads summary -c "$c"
  sleep 0.7
done < clusters.txt | jq -s 'sort_by(-.total_waste) | .[] | {cluster: .cluster_name, total_cost, total_waste, total_potential_saving}'

# 3. Node-type swap candidates (standard node groups only — see gotcha above).
pscli -o jsonl nodegroups list -c <cluster> --has-recommendations \
  | jq -c 'select(.recommendations.type=="standard") | {id, best: (.recommendations.node_type_recommendations | max_by(.estimated_savings_pct))}'

# 4. Blocked cost from unevictable pods, deduped by node.
pscli -o json unevictable report -c <cluster> \
  | jq '[.rows[] | {node, blocked_cost_hourly}] | unique_by(.node) | map(.blocked_cost_hourly) | add'
```

**Cluster overview for a status report:**
```bash
pscli -o json workloads summary -c prod-a
pscli -o jsonl workloads group-by namespace -c prod-a -s waste -r desc -T 5
```

**Find risky deployments above severity 2:**
```bash
pscli -o jsonl workloads risky -c prod-a -S 2 -s severity -r desc
```

**What did Perfectscale automation do this week?**
```bash
pscli automation audit-logs --since 168h --all -o jsonl
```

## References

- `references/cli-reference.md` — full command/flag reference, kept in sync with the README.
- `agents/openai.yaml` — equivalent skill manifest for OpenAI Agents SDK runtimes.
