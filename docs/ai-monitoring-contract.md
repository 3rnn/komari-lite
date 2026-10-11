# Optional Hermes AI Monitoring integration contract

This is a development integration contract, not a record of deployed functionality.
All monitoring is opt-in and disabled by default. No real node is enabled as part of testing.

## Ownership

Backend work owns backend/** except embedded public/systemUI/dist and bundled themes. Agent work owns agent/** and backend/web/api/public/agent_installers/{install.sh,install.ps1,test_installer_security.py}. Frontend work owns frontend/src/** and frontend/tests/** only; do not build/synchronize shared embedded artifacts until integration. Coordinator owns release/version metadata, docs, TASK_PROGRESS, generated artifacts and deployment. Existing local worktree is authoritative; preserve unrelated files and do not push, tag, publish, restart services or change Hermes from child tasks.

## Optional configuration

DeploymentProfile.EnableHermesAI bool -> JSON enable_hermes_ai, default false. Both protocol/v2 ConfigParams add EnableHermesAI *bool -> JSON enable_hermes_ai,omitempty. CLI Linux installer flag and Agent flag --enable-hermes-ai; Windows installers explicitly reject enabling with a clear unsupported-platform error before mutation, retaining existing disabled Windows installation behavior. Runtime changes use existing revision/configResult delivery. An old Agent whose config_state omits enable_hermes_ai must NOT acknowledge that the feature was applied merely because it acknowledges other fields. New Agent rejects true if its local helper is unprovisioned/unusable or platform unsupported and reports failure without stopping ordinary heartbeats. Keep all helper database/auth paths operator-local, never remotely writable.

## Collector and reporting

Linux Python stdlib helper: agent/helpers/hermes_collector.py; tests colocated or agent/helpers/tests. It runs as the Hermes owner in a restricted local systemd service, with read-only source database/auth mount access and writes only separate collector state/spool. General Agent never opens Hermes DB/auth. Helper config supplies local Hermes database/home, owner, spool/state directory, interval (10s default), quota refresh (900s minimum default), retention (30 days configurable). Control file from Agent permits only boolean enabled; default disabled and disabling performs NO Hermes reads or quota sampling. Durable outbox IDs/counter baselines live outside Hermes. Snapshot/output files contain aggregate metrics only; never session IDs, chat IDs, base URLs, prompts, messages, credentials, arbitrary SQL or raw quota response. Use SQLite URI mode=ro, bounded short read transactions, never change Hermes schema/journal mode or auth, never run live Hermes resolver/usage CLI that may refresh credentials. WAL read failures are unavailable, not permission-widening or immutable=1 fallbacks.

Agent relay runs independently of heartbeat, roughly every 30s, with bounded timeout/batch and optional custom local spool path. It posts to /api/clients/ai/report using existing requestheaders token authentication. Failures retry identical sample IDs; acknowledge/delete only explicitly accepted IDs. A bounded unacknowledged outbox applies collection backpressure and reports stale rather than silently discarding data or advancing source baselines. Never require model inference calls. Health/status reporting is isolated from ordinary metrics and backward-compatible.

Request shape (JSON, flat strict allowlist; only declared properties):
- version: 1
- collector_id: UUID, stable per local collector state (not session/account identity)
- enabled: boolean
- status: collecting | disabled | hermes_not_detected | database_unavailable | no_usage | stale
- observed_at: Unix seconds, finite
- last_success_at: optional Unix seconds
- sampling_interval_seconds: numeric, default 10
- current_model: optional bounded model identifier
- samples: at most 256 rows
- quota: optional quota shape below

Sample: sample_id (globally unique UUID persisted before reporting), started_at and ended_at (finite Unix seconds); model, provider, billing_mode, task (bounded strings); calls, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, reasoning_tokens (nonnegative safe integer); optional hermes_estimated_cost_usd, recorded_actual_cost_usd, cost_status, cost_source. Optional coverage flag/reset_count. No session identifier leaves helper. Aggregate samples by model/provider/billing_mode/task and interval. Do not sum both sessions totals and session_model_usage totals. Hermes canonical input_tokens EXCLUDES cache read/write; total_tokens = input_tokens + cache_read_tokens + cache_write_tokens + output_tokens. Reasoning is an output subset and is NOT added again. Initial counters are a baseline, not synthetic historical usage. New rows after collector baseline may be counted if first_seen proves they were created during covered collection. Counter decrease/rewind rebaselines that row, never yields negatives or replayed full totals. Conservative coverage gaps and unknown durations remain N/A, not interpolated. Timestamp precision supports sampled throughput only, never true generation speed.

Quota: account_key (lowercase SHA-256 namespace + non-secret account ID, NEVER token hash or raw alias), provider, plan, status (available/unavailable/stale), observed_at, last_success_at, windows [{label: Session|Weekly|unknown label, used_percent: finite 0..100, resets_at: optional ISO8601, window_seconds: optional number}], optional reset_bank_count numeric informational only. account_key absent -> status unavailable/unidentifiable; do not mismerge distinct accounts. Deduplicate backend latest quota per provider/account_key, never sum percentages. Only read existing OAuth store and issue GET to allowlisted official quota endpoint with existing valid access token; no refresh/login/reset, no credential file writes, no arbitrary URL redirects. Expired/unavailable credential is unavailable/stale. Missing quota values are null, never zero. Do not persist raw OAuth/API material or raw quota payload.

Backend /api/clients/ai/report: authenticate ONLY Agent node credential; derive node identity from authentication, never payload. Strict bounded parsing, finite counters/time, future/backdated limits, node rate limiting, transactionally deduplicate (node,sample_id), immutable per-sample price snapshot, ACK accepted_sample_ids. Reject unknown sensitive fields and excessive payloads. Existing agents omit this endpoint entirely.

## Admin API

All paths below require authenticated admin with explicit server-side auth and no public/RPC bypass. Use established REST-to-RPC conventions when practical.
- GET /api/admin/ai/overview?from=<Unix seconds>&to=<Unix seconds>&timezone=Asia%2FShanghai&node=<UUID or empty>&model=<id>&task=<id>
- GET /api/admin/ai/settings
- POST /api/admin/ai/settings (timezone, retention_days, detailed_retention_hours default24 etc bounded)

Overview returns object: range {from,to,timezone}, definitions, live {tps,input_tps,output_tps,cache_read_tps,rps,rpm,tpm,rolling_calls,rolling_tokens,calls_24h,today_api_equivalent_cost_usd,unpriced_calls,status}, accounts[], trends[], daily[], models[], tasks[], nodes[], pricing_coverage. Rates are nullable when coverage/age is insufficient. trends rows {timestamp,calls,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,reasoning_tokens,total_tokens,api_equivalent_cost_usd,unpriced_calls}; daily rows {date,calls,total_tokens,api_equivalent_cost_usd,unpriced_calls,average_tps:null-or-supported-sampled-throughput,models:[]}; models/tasks use same counter fields plus model/provider/billing_mode/task where applicable, average_tps, cost_share. accounts preserve quota shape plus node_sources and safe display label, last successful update. nodes {uuid,name,enabled,status,current_model,last_success_at,last_report_at,sampling_interval_seconds,delivery_status}; derive enabled from saved profile; an older Agent with no AI reports is disabled/unavailable, never collecting.

Pricing is automatic and limited to observed `openai-codex` / `gpt-5.6-terra` samples in `subscription_included` or `unknown` billing mode. Rates are fixed at standard/short-context input $2.00, cache read $0.20, cache write $2.50, and output $12.00 per 1M tokens. It is an API-equivalent estimate, not an actual ChatGPT subscription charge. Other models and tiers remain Unpriced. Frozen rule snapshots/costs are stored at ingestion; changes never reprice previous samples.

Persistence is additive versioned migration in Komari storage, separate from Hermes; retain detailed samples 24h, older minute/hour aggregates without losing price snapshots, configured max30d default. Dedup ledger/outbox and quota bounded. Historical totals only collected deltas, never backfill evenly from cumulative totals. Bound query work/response rows and provider/model/task cardinality; backend failures never block ordinary reporting.

Frontend route /admin/ai-monitoring, MONITORING submenu immediately after LATENCY. Checkbox immediately after detailed GPU under Live collection settings, also saved deployment-profile/runtime interface; reuse existing delivery state messaging and explicitly show pending/failed vs acknowledged applied. Use Radix/Recharts/theme primitives. Desktop/tablet/mobile no page overflow. Account, live metrics, charts, period cards+table, sortable model table, task usage, node states and append-only pricing/settings forms; 1H/24H/7D/30D + custom where practical, node/model/task/timezone filtering. No invented data, unavailable cost/throughput display N/A and Unpriced counts. Real fixtures may be used only in tests/isolated preview, never live node silent enablement.
