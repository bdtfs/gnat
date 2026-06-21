# GNAT stateful engine — build plan (BDT-348..356)

Source: design+recon workflow synthesis (2026-06-21). Stdlib-only, Go 1.26, NO code comments.

## Locked decisions
- New `gnat run` subcommand; `attack`/`serve`/`internal/runner`/`internal/cli` legacy path untouched.
- Per-VU `cookiejar.Jar` + own `*http.Client` over ONE shared Transport (`pkg/clients/http.WithConfig`).
- Leaf packages (parallel-safe, zero internal deps except metrics→models): `extract`, `checks`, `pow`, `metrics`.
- Composition: `vu` (uses extract/checks/pow/metrics), `executor` (uses vu/metrics), `scenario` (config seam → builds vu.Flow + executor.VUConfig), `cmd/gnat run.go+report.go`.
- Dependency arrow `scenario → vu`, never reverse (avoid cycle).
- Executors: `constant-rps`, `constant-vus`, `ramping-vus`. Own-executor scenarios run as independent concurrent plans (weight ignored unless a shared executor).
- Per-step `once: true` → runs only on a VU's first iteration (clearance reuse via persisted jar).
- TTFB via `httptrace.GotFirstResponseByte`. `read_bytes_cap` via `io.LimitReader` + drain + close.
- `checks.DefaultSpec()` = status 200–399 (matches legacy success).
- Report: new `RunReport`; reuse `cli.Evaluate` (thresholds) + `converters.StatsToDTO` (JSON) via `metrics.StepStats.ToModelsStats()`.

## ToonCache run profile
Anonymous-cleared browse path (no login): challenge → solve PoW (sha256(salt+":"+nonce), leading-zero-BITS ≥ difficulty) → verify (tc_clear cookie) → browse /api/shows (extract slug) → show detail → recent/search. Clearance steps `once:true`. Box IP allowlisted on prod so per-VU PoW does not trip the challenge/auth buckets. tc_clear bound to UA(hash)+/24 → keep User-Agent stable.

## Prod allowlist
`RL_ALLOWLIST_IPS` (CSV IPs/CIDRs) in apiserver: short-circuit `rateLimit()` + seed abuse `lists.Allow` so WAF/netguard/risk/PoW-escalation all honor it. Match on `clientIP(r)` (forwarded public IP, trusted-proxy gated). Clearance stays required (PoW realistic). Deploy: tooncache-tv CI → semver image → manual argocd-apps tag bump (broken PAT) → `argocd app sync tooncache-api`. Revert = remove env line + sync (code inert when unset).
