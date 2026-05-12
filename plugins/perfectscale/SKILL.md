---
name: perfectscale
description: Query Perfectscale Kubernetes cost, waste, risk, and automation data through the `pscli` public-API CLI. Use this skill whenever the user asks about Perfectscale clusters, namespaces, workloads (cost, waste, recommendations, risk severity, optimization policy, mute state, labels), cluster carbon emission, or automation audit logs. Trigger on phrases like "perfectscale", "pscli", "kubernetes waste", "k8s cost optimization", "rightsizing recommendations", "wasteful workloads".
---

# Perfectscale CLI Skill

This skill teaches you to use `pscli`, a Go CLI that wraps Perfectscale's public API. It is the only supported way through this skill — do not call the public API directly.

## When To Use

Use `pscli` when the user wants to:

- list or inspect their Perfectscale-monitored Kubernetes clusters
- find wasteful, costly, or risky workloads
- group workloads by namespace, type, optimization policy, risk severity, or label
- export workload data as CSV or JSONL for analysis
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

2. Verify auth:

   ```bash
   pscli auth status
   ```

   If it reports no profile, ask the user for a Perfectscale **service token** (`client_id` and `client_secret`) generated at `app.perfectscale.io → user circle → Org Settings → API Tokens → Generate Token` (a Read Only role is enough). Then:

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
```

## Short-Flag Reference

Stable across commands — memorize these instead of typing `--long`:

`-p` profile · `-o` output · `-u` public-api-url · `-d` debug · `-c` cluster ·
`-w` period (30d only) · `-n` namespace · `-m` workload name · `-t` workload type ·
`-s` sort · `-r` order (`asc`/`desc`) · `-T` top N · `-B` bottom N ·
`-C` min-cost · `-W` min-waste · `-V` view · `-i` id/client-id ·
`-k` client-secret / label key · `-v` label value · `-S` min-severity ·
`-f` export format · `-F` export file path.

## Hard Limits (Don't Lie To The User)

- Workload period is **30d only** — `-w` accepts `30d` and nothing else right now.
- `--namespace`, `--name`, `--type`, `--min-cost`, `--min-waste` are **client-side**: the CLI fetches the full cluster workload list and filters locally. For huge clusters, prefer `-T`/`-B` and a sort to bound the work.
- Namespaces are **derived** from workloads; there is no namespace endpoint.
- There is no nodegroup command yet.
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
