# Optional local Hermes observer

Disabled by default. The general Agent never opens Hermes source files. It reads
only this helper's aggregate `report.json`, writes `control.json` with exactly
one boolean `enabled`, and writes individual UUID files under `acks/` after an
explicit backend `accepted_sample_ids` response.

## Local installation inputs

Linux systemd installer options:

- `--enable-hermes-ai` (or `--enable-hermes-ai=false`)
- `--install-hermes-owner USER`: existing Hermes owner; defaults to the owner of
  an explicitly supplied home's database, otherwise `SUDO_USER`/root.
- `--install-hermes-home /absolute/hermes/home`: defaults to that user's
  `.hermes`. The directory and source files must not be symlinks or writable by
  other users. The installer never changes their permissions.
- `--install-hermes-sha256 DIGEST`: independently pinned digest of
  `hermes_collector.py`. Mandatory for panel/custom artifact sources. Official
  pinned release sources may use their `SHA256SUMS.txt` entry.

Publish `hermes_collector.py` beside the matching Agent artifact in the existing
verified Agent inventory. Helper missing/checksum mismatch fails before replacing
any working service. Existing installer version/source validation remains intact.
The helper is not fetched from a moving branch or a second service.

The helper unit is `${service_name}-hermes.service`, runs as the Hermes owner,
hides home directories, binds **only** source DB/WAL/SHM/auth files read-only into
its private runtime directory, drops capabilities, and allows persistent writes
only to independent `/var/lib/${service_name}-hermes/{private,spool}` paths.
The Agent unit hides the selected Hermes home instead of receiving read grants.
Installation first starts the helper with disabled control and waits for its
fresh disabled snapshot. The Agent then applies the explicit enable flag.
OpenRC enabling and Windows enabling are unsupported; disabled workflows remain.

For manually provisioned helpers, local-only arguments are `--database`,
`--auth-path`, `--state-dir`, `--spool`, `--interval` (default 10 seconds),
`--quota-interval` (minimum 900 seconds), `--retention-days` (1..30), and optional
`--account-alias` (non-secret, explicitly selected account identity fallback).
Run with `python3 -I`, as the Hermes owner, in an equivalent restricted namespace.
The Agent's local path option is `--hermes-ai-spool` / `hermes_ai_spool` /
`AGENT_HERMES_AI_SPOOL`; paths are intentionally absent from remote protocol.

## Accounting and delivery semantics

- Fixed read-only metric queries use only `session_model_usage`, with an optional
  sessions ID/rewind-count JOIN. Session summaries are never added again.
- Canonical noncached input, output, cache read/write, and reasoning counters are
  kept separate. Reasoning is not added to output a second time.
- First observation is a baseline. New rows count only when `first_seen` proves
  creation inside a continuously covered interval. Decrease/rewind rebaselines;
  partial-group resets mark coverage false. Gaps/errors/disable force rebaseline.
- Zero-delta samples preserve known idle coverage. `coverage` is boolean; exact
  floating Unix start/end times support sampled rates, not generation speed.
- UUID sample IDs, collector UUID, baselines and outbox are atomically persisted
  and fsynced before reporting. Retries preserve IDs and original sample times.
- Relay uses an independent 30-second loop, eight-second timeout, existing node
  authentication and Cloudflare headers, and forbids HTTP redirects.
- ACKs are offered-ID-only durable markers. The helper persists outbox removal
  before deleting markers; orphan markers are pruned. Network/ACK failures never
  block heartbeat. Backend deduplication is required for crash/retry races.
- Successful runtime config results explicitly include the actually applied
  `enable_hermes_ai` boolean. True requires a fresh provisioned helper snapshot;
  known-unavailable/stale helpers and unsupported platforms fail the revision.
  An acknowledged control change is not a promise that quota/usage is available.

## Safety bounds and deliberate limitations

Queries have a short busy timeout, execution budget and 4096-row cap; source
cardinality overflow is unavailable rather than truncated accounting. At most
256 aggregate groups per interval, 256 rows per report, and 4096 queued rows are
retained. Backlogs older than configured retention or over the cap are dropped;
missing intervals remain coverage gaps, not interpolated history. JSON reads and
HTTP bodies are bounded, and one helper process holds a nonblocking flock.

Quota reads only the existing singleton `providers.openai-codex.tokens` store.
It requires a usable JWT expiry and non-secret account ID (explicit token field
or ChatGPT account claim), or an operator-selected alias. Pool-only/ambiguous
credentials are unavailable: no resolver, selection/rotation or refresh runs.
Only GET to the fixed official ChatGPT usage endpoint is allowed, with no proxy
or redirect. The attempt throttle is durable before network I/O. Only sanitized
window percentages, durations, reset timestamps, plan and namespaced account-ID
hash are stored; missing values stay null. Same-account failures retain a stale
sanitized snapshot. No reset-bank redemption or authentication writes occur.

Read-only **file** mounts pin inodes. Replaced auth/DB/WAL files may require an
operator helper restart to remount; unavailable WAL/SHM never causes permission
widening or an immutable SQLite fallback. This feature does not read conversation
content, backfill old usage, infer routing tiers, calculate official prices, or
emit optional historical Hermes cost totals whose delta attribution is unknown.

Tests use temporary synthetic metric/OAuth fixtures and local TLS servers only;
never run a live Hermes usage command or enable an actual node for testing.
