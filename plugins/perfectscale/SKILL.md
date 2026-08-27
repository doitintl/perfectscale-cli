---
name: perfectscale
description: Query Perfectscale Kubernetes cost, waste, risk, and automation data through the `pscli` public-API CLI. Use this skill whenever the user asks about Perfectscale clusters, namespaces, workloads (cost, waste, recommendations, risk severity, optimization policy, mute state, labels), node groups (InfraFit utilization, cost, recommendations), unevictable pods (autoscaler block reasons, blocked cost, remediation), cluster carbon emission, or automation audit logs. Trigger on phrases like "perfectscale", "pscli", "kubernetes waste", "k8s cost optimization", "rightsizing recommendations", "wasteful workloads", "unevictable", "node groups", "infrafit".
---

# Perfectscale CLI Skill

This skill teaches you to use `pscli`, a Go CLI that wraps Perfectscale's public API. It is the only supported way through this skill — do not call the public API directly.

## When To Use

Use `pscli` when the user wants to:

- list or inspect their Perfectscale-monitored Kubernetes clusters
- find wasteful, costly, or risky workloads
- group workloads by namespace, type, optimization policy, risk severity, or label
- export workload data as CSV or JSONL for analysis
- inspect InfraFit node groups — utilization, cost, and node-type recommendations (standard or Karpenter)
- find unevictable pods that block autoscaler scale-down, why they're blocked, and what that costs
- review Perfectscale automation audit logs (eviction, in-place resize, cleanup)
- check cluster carbon emission

## Bootstrap

1. Verify the binary is on `PATH`:

   ```bash
   pscli --help
   ```

   If it is missing, run the installer from `scripts/`:
   - macOS / Linux / WSL / Git Bash: `bash scripts/install.sh`
   - Native Windows (PowerShell): `powershell -ExecutionPolicy Bypass -File scripts\install.ps1 -AddToPath`

   Both fetch the latest release matching the host OS/arch from `https://github.com/doitintl/perfectscale-cli/releases/latest`.

2. Verify auth. `pscli` is multi-profile (default name: `default`); the user may have authenticated under a different name (e.g. `dev-public`, `prod`). Always discover the right profile before running commands:

   ```bash
   # Try the default profile first.
   pscli auth status >/dev/null 2>&1 && echo default

   # If that fails, list profiles on disk and try each until one authenticates.
   # Profiles live in $XDG_CONFIG_HOME (or ~/Library/Application Support on macOS,
   # %AppData% on Windows) under perfectscale-cli/profiles/<name>.json
   ls "${XDG_CONFIG_HOME:-$HOME/Library/Application Support}/perfectscale-cli/profiles/" 2>/dev/null \
     || ls "$HOME/.config/perfectscale-cli/profiles/" 2>/dev/null
   # For each <name>.json, try: pscli -p <name> auth status
   ```

   Once you find an authenticated profile, export it for the rest of the session so every subsequent command picks it up automatically:

   ```bash
   export PERFECTSCALE_PROFILE=<name>
   ```

   (Or pass `-p <name>` explicitly on every call — the env var is just less error-prone.)

   If no profile authenticates, ask the user for a Perfectscale **service token** (`client_id` and `client_secret`). To generate one:

   1. Open `https://app.perfectscale.io`
   2. Click the user-initials avatar at the **bottom-left** of the sidebar (below the gear/Settings icon)
   3. In the pop-up menu, choose **Organization Settings**
   4. Open the **API Tokens** tab
   5. Click **Generate Token**, assign a **Read Only** role (enough for this skill), and copy both `client_id` and `client_secret`

   Then:

   ```bash
   pscli auth login --client-id "$CLIENT_ID" --client-secret "$CLIENT_SECRET"
   ```

   Never echo or log the secret. Prefer environment variables when scripting.

3. Default endpoint is production (`https://api.app.perfectscale.io/public/v1`). Override only when the user explicitly asks (set `PERFECTSCALE_PUBLIC_API_URL` or pass `-u`).

## Output Modes

Always pick output for the consumer:

- `-o table` (default) — only when streaming directly to a human terminal.
- `-o json` — single JSON document. Use this for `show`, `summary`, `get`, single-record reads.
- `-o jsonl` — one JSON object per line. **Use this for any list/group-by/export-style output you (the agent) need to parse.** It is far easier to slice with `jq -s`/`jq -c` than table output.

**On cursor-paginated commands (`nodegroups list`, `unevictable list`/`report`, `automation audit-logs`), `-o jsonl` drops the pagination cursor entirely** — it only prints the current page's items, with no signal that more exist. (`-o json` includes a `pagination` object with the `next` cursor; `-o table` prints a "More available — pass --page-token ... (or use --all)" footer; `-o jsonl` gives you neither.) Default to pairing `-o jsonl` with `--all` on these commands so the CLI pulls every page before printing and there's nothing left to silently miss. Only skip `--all` if you're deliberately paging yourself via `--page-token` — in that case use `-o json` instead, since it's the only format that exposes the cursor.

`workloads list --view all` auto-promotes to `jsonl` unless `-o` is explicit. Take advantage of that when you need every enriched field.

**Don't guess output fields — read the schema.** Every command's `--help` ends with an `Output schema` block documenting the exact shape returned with `-o json`/`-o jsonl` (field names, types, nesting). When you need to `jq` a response and aren't sure of the keys, run `pscli <command> --help` first instead of inferring them.

## Core Command Cheatsheet

```bash
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

Stable across commands — memorize these instead of typing `--long`:

`-p` profile · `-o` output · `-u` public-api-url · `-d` debug · `-c` cluster ·
`-w` period (30d only) · `-n` namespace · `-m` workload name · `-t` workload type ·
`-s` sort · `-r` order (`asc`/`desc`) · `-T` top N · `-B` bottom N ·
`-C` min-cost / min-blocked-cost · `-W` min-waste · `-V` view · `-i` id/client-id ·
`-k` client-secret / label key · `-v` label value · `-S` min-severity ·
`-g` node group name · `-f` export format · `-F` export file path.

## Hard Limits (Don't Lie To The User)

- Workload period is **30d only** — `-w` accepts `30d` and nothing else right now.
- `--namespace`, `--name`, `--type`, `--min-cost`, `--min-waste` are **client-side** for workloads: the CLI fetches the full cluster list and filters locally. For huge clusters, prefer `-T`/`-B` and a sort to bound the work.
- Namespaces are **derived** from workloads; there is no namespace endpoint.
- `clusters list` can return an empty/null `uid` for some clusters — typically test/demo clusters or ones that are frequently torn down and recreated, where the connected agent hasn't fully reported in yet. Treat a missing `uid` as "no live identifier yet," not an error; you can usually still reference the cluster by `name` with `-c`.
- **Cluster names aren't guaranteed unique** — dev/test environments especially can have two clusters sharing a name (seen live: `karpenter-metrics`). `-c <name>` correctly errors with an ambiguous-match message listing both UIDs in that case; resolve by passing the UID instead of the name.
- `nodegroups list` filters (`--autoscaler-type`, `--has-recommendations`, `--include-muted`) are **server-side**. Pagination uses opaque cursors; the backend recomputes the full set on every page so `--all` always requests the maximum page size.
- `unevictable` filters (`-n`, `--reason`, `-g`, `-C`) are **server-side** (AND-combined). `--reason` is only accepted by `unevictable list`, not `unevictable report`. Data comes from a pre-computed snapshot — check `snapshot_time` in the output for freshness.
- Audit logs are limited to the last 30 days, are cursor-paginated (no offset), and `--execution` is filtered client-side.
- Only service-token auth — there is no SSO/JWT flow.
- CSV is the only `workloads export` format.

If a user asks for something outside this surface, say so plainly and suggest the closest supported command.

## Parsing & Interpretation Gotchas

- **`unevictable`'s `blocked_cost_hourly` is the pod's *node* cost, not a per-pod share.** If several unevictable pods share a node, summing `blocked_cost_hourly` across those pods overcounts that node's cost once per pod. Group by `node` first, then sum:
  ```bash
  jq '[.rows[] | {node, blocked_cost_hourly}] | unique_by(.node) | map(.blocked_cost_hourly) | add'
  ```
- **`waste`/`potential_savings` on a workload can be stale.** A workload that's already been right-sized can still show a leftover nonzero `waste` value from before the fix. Before treating it as an open opportunity, check that `indicators`/`max_indicator` are non-empty, or compare a container's `resources.current` to `resources.recommended` — if they already match, there's nothing left to act on.
- **`potential_savings: 0` alongside nonzero `waste` is a real, confirmed state, not a bug or missing data** (verified against a live backend trace) — `waste` and `potential_savings` are computed independently, and the recommendation engine can simply have no actionable recommendation for a workload that's still flagged as wasteful. The field is always present in `-o json`/`-o jsonl` output (never silently dropped for being zero) — treat `0` at face value.
- **`nodegroups` recommendations come in two shapes** — check `recommendations.type` before parsing: `karpenter` is a list of NodePool config diffs (consolidation policy, broadened instance selectors — no ranked instance list), while `standard` has a ranked `node_type_recommendations` array with `estimated_savings`/`estimated_savings_pct` per candidate instance type. **`estimated_savings` is a *monthly* dollar figure** (confirmed against the web UI's own "Forecast $X → $Y · Save $estimated_savings (estimated_savings_pct%)" panel — not hourly, not a 30-day total). `estimated_savings_pct` reconciles correctly as `estimated_savings / currentMonthlyForecast` — but that current-monthly-forecast basis is **not** the same as this node group's own `cost.timeframe` field from `nodegroups list`/`get` (that's a realized/trailing cost over its own window, a different basis) — don't try to cross-check the recommendation's percentage against `cost.timeframe`; they're computed independently.
- **Presence in `automation audit-logs` is a weak "is this automated" signal.** The log only covers the last 30 days, so a workload automated longer ago (and since converged, with nothing left to fix) won't appear — absence doesn't mean automation is off for it.
- **Multi-line fields (e.g. unevictable's `current_spec`/`recommended_spec`/`yaml_diff`) contain literal `\n` escapes** — correct JSON, but unreadable as raw text. Pipe through `jq -r` (not the default `jq` mode) to render them.
- **When piping captured JSON through a shell variable, use `printf '%s\n' "$var"`, not `echo "$var"`.** Some shells (e.g. zsh) interpret `\n` inside `echo` as an actual newline by default, silently corrupting any JSON string that contains an escaped newline before it reaches `jq`.

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

## References & Scripts

- `references/cli-reference.md` — full command/flag reference, kept in sync with the README.
- `scripts/install.sh` — fetch the latest `pscli` release on macOS / Linux / WSL / Git Bash.
- `scripts/install.ps1` — fetch the latest `pscli` release on native Windows (PowerShell).
- `agents/openai.yaml` — equivalent skill manifest for OpenAI Agents SDK runtimes.

