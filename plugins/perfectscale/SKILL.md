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

   Both fetch the latest release matching the host OS/arch from `https://github.com/<org>/poc-cli/releases/latest`.

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
pscli automation audit-logs --since 24h -o jsonl
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
- `nodegroups list` filters (`--autoscaler-type`, `--has-recommendations`, `--include-muted`) are **server-side**. Pagination uses opaque cursors; the backend recomputes the full set on every page so `--all` always requests the maximum page size.
- `unevictable` filters (`-n`, `--reason`, `-g`, `-C`) are **server-side** (AND-combined). `--reason` is only accepted by `unevictable list`, not `unevictable report`. Data comes from a pre-computed snapshot — check `snapshot_time` in the output for freshness.
- Audit logs are limited to the last 30 days, are cursor-paginated (no offset), and `--execution` is filtered client-side.
- Only service-token auth — there is no SSO/JWT flow.
- CSV is the only `workloads export` format.

If a user asks for something outside this surface, say so plainly and suggest the closest supported command.

## Recipes

**Top-10 waste in production:**
```bash
pscli -o jsonl workloads list -c prod-a -s waste -r desc -T 10 -V all
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
