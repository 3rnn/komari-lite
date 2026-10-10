# TASK_PROGRESS.md

## Telemetry repair — October 11, 2026 (source-only; no deployment)

STATUS: VERIFIED EXCEPT FOR HOST-INCOMPATIBLE RUNTIME BLACK-BOX

- Revalidated the previously reported `backend/database/aimonitor/store_test.go` mutation: the stale-quota regression was present and passes; no blind reapplication was made.
- Confirmed the live 4,100-sample backlog is blocked by the deployed Panel's 100,000-per-node dedup limit: it has 99,975 markers, while relay batches contain up to 256 immutable IDs, so the transaction rolls back with no ACK. The queued IDs are absent from Panel dedup; no queued sample, service, credential, mount, or database was changed.
- Raised the source dedup marker limit to 1,000,000 and added a regression proving a complete new relay batch remains accepted after the legacy 100,000 count. This is additive source behavior; no schema migration or historical deletion is required. Dedup expiry remains retention-based; no historical samples or ACK markers are silently deleted.
- Added capability-gating for additive relay delivery fields: old Agents retain the strict v1 report shape; only an Agent-written local protocol marker version 2 enables delivery/backlog fields. This avoids new-helper → old-Panel strict decode failure.
- Added `today_coverage` source-to-UI semantics. Any collected reset/uncovered Today interval renders Today Tokens as `Unknown`, with the partial collected value retained only in the API—not presented as a verified total.
- Tightened the planned helper sandbox with `InaccessiblePaths=$hermes_home`, preserved path-unit active-state rollback, and retained file-only read-only binds. Static unit parsing/isolation and installer rollback tests pass.
- Added a synthetic namespace-only systemd black-box fixture for repeated atomic `auth.json` replacements. This host cannot start the nested manager (and its user manager cannot create mount namespaces), so the test is explicitly skipped here after cleanup. It must run, unskipped, on a systemd-capable CI/VM before any runtime deployment; the required assertions are helper restart, rebinding to each new inode, account-fingerprint changes, and inaccessible source-private files.
- Full backend, Agent, collector, frontend test/lint/build validation passed locally. The installer security suite's existing binary-download harness was narrowed to its validation/download section after installer lifecycle setup moved later in the file; it now passes again.
- No deployment, push, tag, publication, runtime service/mount/database/credential/Agent change, or queued-sample mutation was performed.

## Optional Hermes AI Monitoring — current implementation

STATUS: DEPLOYED AND VERIFIED (development panel/Agent/helper only; production Agent excluded)

- Inspected the local worktree and current Hermes counter/usage implementation; canonical noncached input and cache categories are disjoint, reasoning is an output subset.
- Defined `docs/ai-monitoring-contract.md` for the backend, isolated collector/Agent and admin UI.
- Verified the official OpenAI reference pricing source and recorded an informational, source-versioned catalog in `docs/ai-pricing-reference.json`; tiers/modes are not guessed from cumulative counters.
- Backend, Agent/collector/installers and frontend are implemented with opt-in collection disabled by default.
- Corrected pricing effective-time selection, opaque quota-key validation, sampling-interval validation, release helper packaging, durable collector outbox backpressure, timezone initialization, and exact filter sentinel handling; targeted/full component validation is recorded in the current task session.
- AI monitoring remains disabled on all existing real nodes; no Hermes database/auth configuration, task, production endpoint, or production service has been changed.
- A local feature commit contains the verified implementation. `/opt/komari` is the intentional development Agent runtime and must not be removed or overwritten; the development panel deployment target is the separate protected `/opt/komari-lite-panel` versioned layout.
- No tag, GitHub Release or production rollout is authorized by this work-in-progress status.

### Isolated test deployment verification — October 9, 2026

- Re-read project instructions and this progress record. The committed feature remains at `07b2e1d6760a914e240ef1912bc4338b96f80aac` (`feat: add optional Hermes AI monitoring`). The source worktree has no tracked diff; two pre-existing unrelated untracked backend restore-test files were preserved. A clean detached verification worktree was created from the exact commit, excluding those untracked files.
- Integrity and validation: `git fsck --full` completed (only pre-existing dangling unreachable objects); commit diff whitespace check passed. Frontend `npm ci`, tests (**421/421**), lint, TypeScript and production System UI build passed. Backend `CGO_ENABLED=1 go test ./...` and `go vet ./...` passed. Agent `go test ./...` and `go vet ./...` passed. The Python Hermes collector suite passed **15/15**.
- The panel was built from the isolated commit with embedded frontend assets and installed only at `/opt/komari-panel-test/komari` (SHA-256 `8086ef7b865574f929534e626428bbe1b72eca2aef4a4da9504a61a4a3fc8aab`). It runs directly, not as a systemd service, on `127.0.0.1:28774` with separate `data/komari-test.db` and `data/metrics-test.db`. Its `version --json` reports version `1.0.26` and hash `07b2e1d6760a914e240ef1912bc4338b96f80aac`.
- Isolated runtime checks passed: first-run state transitioned `ready` to `completed`; test administrator password login established a cookie-authenticated session; `/api/me` identified the test administrator; authenticated AI overview, pricing and settings APIs returned successfully; unauthenticated AI overview returned HTTP 401. The AI settings defaulted without any enabled node profile, and the isolated database contains zero AI deployment profiles.
- The test admin route serves the exact built entry asset and the AI Monitoring chunk; their served SHA-256 values match the embedded build bytes. Collector integration checks passed through the committed Agent test suite, the 15 Python collector tests, and a freshly built test Agent exposing `--enable-hermes-ai`. No AI profile was enabled, no Agent was enrolled to the test panel, and no existing/live node was changed.
- Deployment-path conflict confirmed: `komari-agent.service` remains active with executable and working directory both under `/opt/komari` (`/opt/komari/agent` and `/opt/komari`). The active local panel remains separately active from `/opt/komari-lite-panel`; the test listener is separate. No service was restarted, stopped, reconfigured, or deleted.
- No `/opt/komari` migration is required. It remains the intentional development Agent runtime; the development panel must be upgraded through its protected `/opt/komari-lite-panel` versioned layout.

### Development v1.0.27 release/cutover attempt — October 9, 2026

- Created local commit `543047a770d2e3e54c68ec64dbab950ff4194bb1` (`chore: align development release version to 1.0.27`) on top of the Hermes AI Monitoring feature commit. It changes only panel, Agent and frontend version metadata plus version-alignment tests; it does not tag, push or publish a release.
- Exact-commit validation passed in a clean detached worktree: frontend tests **421/421**, lint (0 errors; 25 pre-existing warnings), TypeScript and embedded System UI build; backend `CGO_ENABLED=1 go test ./...` and `go vet ./...`; Agent `go test ./...`, `go vet ./...`; Hermes collector tests **15/15**.
- All development and production services were healthy immediately before the release build. Development panel remained `1.0.26` / `59b6475e0b999819a547f4c27c344e41f736397c`; the isolated test panel remained `1.0.26` / `07b2e1d6760a914e240ef1912bc4338b96f80aac` on loopback. No live service or configuration was modified.
- The initial complete-release build correctly stopped because the host lacked the arm64 CGO sysroot headers. Installed only the missing build prerequisites `libc6-dev-arm64-cross` and `linux-libc-dev-arm64-cross`; no Komari runtime or production service was changed by that remediation.
- Rebuilt from clean detached commit `9102b70e956be6ea4af6b3c53ca8f7ca7efee27d` after committing the canonical generated System UI asset synchronization. The complete `v1.0.27` inventory passed checksum validation (26 manifest assets, including both panel architectures, 14 Agent binaries, helper, theme bundle and controllers). The final panel binary reports `vcs.modified=false` with revision `9102b70…`.

### Development-only v1.0.27 deployment and Hermes activation — October 9, 2026

- The installed development safe-upgrade controller passed its explicit gate with the complete panel binary and Glass transition bundle. Before cutover, both development SQLite databases passed read-only `PRAGMA quick_check`, no interrupted-upgrade journal was present, and `current` targeted `releases/1.0.26`.
- The controller upgraded **only** `komari-lite-panel.service` from `releases/1.0.26` to `releases/1.0.27`, committed its journal and created recovery backup `20261009T184658-f662bf79`. Post-upgrade: service active; `current` targets `releases/1.0.27`; installed binary checksum matches the verified release artifact; command and loopback API both report `1.0.27` / `9102b70e956be6ea4af6b3c53ca8f7ca7efee27d`; both live databases still pass `quick_check`.
- Authenticated browser acceptance passed at the development panel origin: existing administrator login succeeded, the existing node/history/dashboard remained visible, the AI Monitoring route rendered its release asset and authenticated AI overview/settings/pricing APIs returned HTTP 200. The unauthenticated AI API behavior was previously verified in the isolated test panel. `/opt/komari-panel-test` remains untouched.
- Development Agent only: replaced `/opt/komari/agent` with the verified `1.0.27` binary from the same release revision after retaining root-only rollback `agent.rollback-v1.0.26-20261009T185013`. The Agent retains its original root-private configuration and working directory. It started successfully, uploaded ordinary basic info, and reconnected through v2 WebSocket. Its running executable/cwd remain `/opt/komari/agent` and `/opt/komari`.
- Installed and enabled **only** `komari-agent-hermes.service`. It runs as the Hermes owner with `ProtectHome=tmpfs`, `ProtectSystem=strict`, no capabilities/new privileges, and bind-read-only access only to Hermes `state.db`, optional WAL/SHM, and `auth.json`; private/buffer state is isolated at `/var/lib/komari-agent-hermes/{private,spool}`. The general Agent receives only the aggregate spool path through `AGENT_HERMES_AI_SPOOL`, not access to the Hermes home.
- The helper was first started disabled and produced a fresh disabled report with no samples. The exact development-node UUID `8d7fff8c-958a-44ed-8511-3f9b80830988` was confirmed from the authenticated development panel and matched the upgraded local Agent (`berohost`, `1.0.27`). Only that node's profile was changed from `enable_hermes_ai=false` to true (revision 2). Its delivery state became `applied` with no error; the panel reports `enabled=true`, the Agent wrote helper `control.json` true, and collection is active.
- Live collection verification: helper reports 75 covered samples, one positive-call interval, six local model identifiers, five task identifiers and sanitized quota presence. Panel ingestion shows 14 model rows and five task rows. Current TPS/RPM/TPM remain `N/A`/null because no eligible recent rolling covered-usage window exists; this is correct unavailable semantics, not zero throughput. One sanitized quota account is present but no plan/window values are available.
- Accounting/pricing verification on ingested data: every model row satisfies `total=input+cache_read+cache_write+output`; every reasoning count is an output subset. No automatic price was invented: all 14 observed model rows remain unpriced, `unpriced_calls=2`, and API-equivalent cost remains null until an official exact mapping is added. Cached input and reasoning are therefore not double-counted.
- `komari-agent-production.service` was never stopped, restarted, reconfigured or replaced. It remained active with PID `75770` and the same recorded start time throughout this development-only deployment. No production node profile was changed or enabled.

### Post-deployment re-verification — October 9, 2026 (14:33 UTC)

- Current source integrity: `git fsck --no-reflogs --full` and `git diff --check HEAD^ HEAD` passed. The only remaining worktree entries are the two preserved unrelated untracked restore-test files; no tracked implementation changes are pending.
- Current deployment: development panel and development Agent binaries both report immutable Go build metadata `vcs.revision=9102b70e956be6ea4af6b3c53ca8f7ca7efee27d` with `vcs.modified=false`. Panel `current` resolves to `releases/1.0.27`, and command/API both report `1.0.27` with that same revision. The retained panel and Agent rollback artifacts are present.
- Runtime and data: `komari-lite-panel.service`, `komari-agent.service`, `komari-agent-hermes.service`, and `komari-agent-production.service` are active/enabled. Development `komari.db` and `metrics.db` both return `PRAGMA quick_check=ok`. The panel recovery timer is active/enabled and the committed-upgrade journal/backup `20261009T184658-f662bf79` remains available.
- Development-only identity/acknowledgement: node `8d7fff8c-958a-44ed-8511-3f9b80830988` (`berohost`, Agent `1.0.27`) has only its own deployment profile set to `enable_hermes_ai=true`, revision 2, delivery status `applied`; panel state confirms `enabled=true` and fresh successful/report timestamps. No production profile was changed.
- Live authenticated admin recheck: the development admin dashboard and AI Monitoring route rendered successfully. The authenticated overview returned HTTP 200, the unauthenticated AI overview remains HTTP 401, and the page shows the existing node/history plus current AI totals of 6 collected calls / 1,324,146 tokens. The current helper spool has `enabled=true`, a 10-second cadence and an available sanitized quota status; no Hermes source database or auth file was modified.
- Accounting/pricing recheck: all 32,275 stored covered samples retain nonnegative deltas. The live nonzero usage totals are 226,465 noncached input + 1,092,096 cache-read + 0 cache-write + 5,585 output = 1,324,146; 1,994 reasoning tokens are within output, so neither cached input nor reasoning is double-counted. The observed model remains Unpriced and API-equivalent cost remains `null`; no price rule or alias was invented.
- Isolation: `/opt/komari-panel-test` artifacts and separate test databases remain intact at the earlier `1.0.26`/`07b2e1d` verification build. Its temporary loopback listener is currently stopped and it has no systemd unit; this recheck did not modify it.
- Production isolation: `komari-agent-production.service` remains active with PID `75770`, executable `/opt/komari-agent-production/agent`, and unchanged start timestamp `2026-10-09 07:24:24 CST`.


# komari-lite Optimization Progress

## Current Goal

Continue the maintenance and UI/UX improvement work for **komari-lite** according to `AGENTS.md`, while preserving existing monitoring behavior, APIs, configuration compatibility, deployment behavior, and live-service safety.

The current work should prioritize:

- English-only first-party repository and UI content
- Hermes-inspired Komari dashboard styling
- Shared design tokens and reusable frontend components
- Frontend cleanup and consistency
- Safe backend cleanup only where needed
- Dead and redundant code cleanup within the active task scope
- Low-risk performance and stability improvements
- Compatibility preservation
- No live deployment or service restart unless explicitly requested

The Hermes Agent dashboard is the primary visual reference for Komari UI work. It should guide the visual language without copying Hermes branding, logos, text, or exact page layouts.

Target UI direction:

- Dark deep-green / teal dashboard palette
- Thin borders and subtle separators
- Small or minimal border radius
- Compact, information-dense monitoring layouts
- Strong typography hierarchy
- Technical / monospace styling for metrics, IDs, versions, logs, timestamps, and system information
- Consistent sidebar, cards, tables, buttons, forms, badges, and status indicators
- Minimal shadows, gradients, blur, glassmorphism, and decorative effects
- Responsive desktop, tablet, and mobile behavior
- Preserve Komari identity, workflows, and monitoring purpose

---

## Completed

- Billing Remaining Value Calculator now uses theme-aware Radix Select controls, automatic ExchangeRate-API CNY rates with published-timestamp caching, timeout/rate-limit/cached fallback handling, CNY fixed at 1, manual override and refresh controls, plus concise plain-text resale listings; VPS-JSQ calculation parity remains regression-tested.

- Remaining Value Calculator now follows the public VPS-JSQ reference implementation: direct billing-day denominators, expiration-to-transaction day calculation, fixed premium, fixed discount, and target-price modes; source-backed parity tests cover standard, custom, leap-year, month-boundary, and expired cases.

- Repository structure reviewed
- Existing frontend/backend architecture inspected
- Initial Chinese text scan completed
- Initial English translation batch completed
- First flat-design CSS cleanup completed
- Theme path-validation edge case fixed
- Targeted tests for the first UI batch passed
- Existing Komari functionality and compatibility constraints identified
- `AGENTS.md` design direction updated to use the Hermes Agent dashboard as the primary UI/UX visual reference
- `AGENTS.md` workflow rules updated to avoid unnecessary repository-wide changes outside the active task
- `AGENTS.md` Git/release/deployment rules clarified:
  - Local repository can be treated as the source of truth when explicitly requested
  - Published releases should use a concrete new tag
  - Agent version should match the deployed Server version
  - `latest` should not be used for version-sensitive Agent installation commands
  - Clean redeployment requires an explicit request and an exact runtime target; the source checkout is not the installation directory

---

## In Progress

- Embedded theme English review
- Frontend cleanup
- Hermes-inspired UI design-system consolidation
- Shared colors, typography, spacing, border, radius, control, card, table, badge, and chart styling review
- Frontend component consistency review
- Backend cleanup within current task scope
- Agent-related code cleanup

---

## Remaining

### UI / Frontend

- Complete repository-wide Chinese character scan for first-party project content
- Finish Hermes-inspired dashboard styling
- Consolidate shared theme/design tokens before large page-specific styling
- Review and normalize:
  - Sidebar/navigation
  - Cards and monitoring panels
  - Buttons and form controls
  - Tables
  - Status badges
  - Dialogs and modals
  - Loading, empty, and error states
  - Charts and metric displays
- Review duplicate CSS and frontend assets
- Verify responsive behavior on desktop, tablet, and mobile
- Perform final UI regression review

### Code Quality

- Remove confirmed dead and redundant code within task scope
- Review unused dependencies
- Review low-risk performance bottlenecks
- Review backend error handling
- Avoid unrelated architectural rewrites

### Validation

- Run full frontend build
- Run backend validation
- Run lint/type checks if available
- Run repository tests
- Review final `git diff`
- Confirm only intended files changed
- Perform final compatibility check
- Confirm no secrets or generated artifacts were accidentally added
- Produce final implementation summary

---

## Validation Status

### Passed

- First flat-design CSS targeted tests
- Theme path-validation targeted test

### Pending

- Full frontend build
- Full backend validation
- Full repository test suite
- Lint
- Type check
- Final Chinese character scan
- Final dependency review
- Final Hermes-style UI regression review
- Responsive layout review
- Final compatibility review
- Final `git diff` review

---

## Important Constraints

- Follow `AGENTS.md`.
- The user's current explicit request takes priority over optional cleanup opportunities.
- Inspect the repository before making changes.
- Work in logical, reviewable batches.
- Validate after meaningful batches.
- Do not overwrite, discard, or reset unrelated user changes.
- Do not perform unnecessary rewrites.
- Do not replace frameworks or major libraries without a clear requirement.
- Do not expand a focused task into unrelated repository-wide cleanup.
- Preserve monitoring semantics, APIs, configuration formats, database compatibility, and existing workflows unless explicitly requested otherwise.
- Do not modify or restart live Komari services unless explicitly requested.
- Do not alter production data unless explicitly requested.
- Do not assume old deployment permissions, systemd arguments, or file layouts remain correct after a version change.
- When the user explicitly states that the local repository is the source of truth, do not overwrite local code with remote content merely because the remote branch or tag differs.
- Do not force-push, rewrite published history, or reuse an existing published tag unless explicitly requested.

---

## Release / Version Rules

When a new Komari version is explicitly requested:

1. Treat the validated local repository state as the source of truth when instructed.
2. Inspect the current branch, commit, tags, and remote configuration.
3. Use a new concrete version/tag.
4. Build and validate the exact commit that will be tagged.
5. Create the tag on that exact validated commit.
6. Push the intended branch/commit and the new tag to the remote repository.
7. Verify the remote tag points to the intended commit.

For normal releases, keep version-sensitive artifacts aligned:

```text
Source Tag == Server Version == Agent Version
```

Agent installation commands must use the same concrete version as the deployed Server unless the upstream project explicitly uses a different versioning scheme.

Do not silently use `latest`.

---

## Deployment Rules

Deployment is **not** part of the current cleanup task unless the user explicitly requests it.

If a clean local rebuild and redeployment is explicitly requested later:

1. Use the requested local Git commit/tag as the source of truth.
2. Inspect the current build instructions.
3. Build from the exact intended source version.
4. Verify the build artifacts and version.
5. Stop only the affected Komari services.
6. Verify the exact authorized runtime path is not a symlink or mount point; remove only that path when the user explicitly authorizes data deletion.
7. Recreate the authorized runtime path, preserving the separate local source checkout.
8. Do not infer permission to erase data from a no-backup preference; follow the user's explicit data-lifecycle choice.
9. Deploy the newly built artifacts.
10. Recreate only the configuration, ownership, permissions, and systemd settings required by the current version.
11. Start the service and verify status, logs, runtime behavior, and reported version.
12. Generate Agent installation commands using the same concrete release version.

This section defines the deployment procedure only; it does not authorize deployment by itself.

---

## Live Service Status (historical, before the explicit September 30, 2026 deployment request)

Live Komari services had **NOT** been modified or restarted during the initial optimization work. The user subsequently explicitly authorized a clean deployment to `/opt/komari`; the current state is recorded in the final update below.

---

## Session / Resume Status

The current Hermes session was paused because the model usage limit requires waiting approximately 35 minutes before continuing.

No additional work should be assumed completed during the pause.

When Hermes becomes available again, resume from the repository state on disk rather than relying only on conversation memory.

Before continuing:

1. Read `AGENTS.md`.
2. Read this `TASK_PROGRESS.md`.
3. Run `git status`.
4. Review `git diff`.
5. Confirm the current branch and existing uncommitted changes.
6. Verify that previously completed work is still present and valid.
7. Do not repeat already completed batches unnecessarily.
8. Continue only the unfinished work.
9. Validate each new meaningful batch before proceeding.

---

## Next Action (historical resume guidance, superseded by the later explicit deployment request)

Resume the unfinished frontend/UI work using the Hermes-inspired design direction defined in `AGENTS.md`.

Recommended next sequence:

1. Inspect the current Git working tree and existing frontend changes.
2. Confirm the shared theme/design-token structure.
3. Consolidate common visual primitives before additional page-specific styling.
4. Continue frontend cleanup incrementally.
5. Preserve current Komari functionality and API behavior.
6. Make backend changes only when required by the frontend or current task.
7. Run targeted validation after each meaningful batch.
8. Finish with the full build, tests, UI regression review, compatibility review, and final `git diff`.

Do not deploy or restart live services as part of this sequence.

---

## Last Update

Current state:

- Repository optimization is still in progress.
- Translation and code review are being handled in separate logical batches.
- Theme path validation fix has been tested.
- Hermes-inspired Komari UI direction is now the intended design target.
- The work should proceed through shared design-system cleanup before broad page-by-page restyling.
- Local Git source-of-truth, release-tag, Server/Agent version consistency, and clean-redeployment rules are documented for future explicit release/deployment tasks.
- Live Komari services remain untouched.
- Hermes work is currently paused due to the temporary model usage limit and should resume from the existing local repository state.

---

## Resume Update — September 30, 2026

- Rechecked local `main` and the `v1.0.6` tag: both point to `90b1fe5f55d5bca1a074c3db79b4616611cc852f` before this new UI batch. Existing untracked `AGENTS.md` and backup files were left untouched; only this local progress file received the resume note.
- Published and read back the stable GitHub `v1.0.6` Release. All 21 remote asset names and SHA-256 digests match the already-built local release artifacts; GitHub's latest stable Release now resolves to `v1.0.6`. This does **not** mean the live panel or Agent has been upgraded.
- Added a small local UI batch: a shared technical metric font token/style applied to dashboard summary values, rankings, and alert/latency counts, without changing metric text or behavior. Added a regression test using test-first development.
- Frontend: 238/238 tests passed, TypeScript/build passed, lint passed with 0 errors and 25 warnings. Backend: 65 package test results and `go vet ./...` passed. Agent: 16 package test results and `go vet ./...` passed. `git diff --check` passed. The isolated built-CSS preview applied the metric style at 320, 390, 768, and 1280 px. A follow-up narrow-width check exposed overlapping long values; the shared style now wraps them, and the ranking's first grid row can grow. A 320 px fixture with a long value and larger text then showed no cell or page overflow or overlap with the progress bar. These were synthetic fixtures, **not** an authenticated admin-page visual regression test.
- The new UI source and test remain local and uncommitted. Rebuilding the frontend changes many fingerprinted chunks because the build embeds a timestamp; the backend embedded asset tree was **not** synchronized for this intermediate visual batch. Do not claim this working-tree UI change is part of the published tag or the live service.
- At this historical checkpoint, broader Hermes-inspired component/layout review, authenticated desktop/mobile UI regression testing, and dependency/dead-code review were still under consideration. A later explicit request authorized the deployment described below.

---

## Clean Local-Source Deployment — September 30, 2026

- The later explicit user request superseded the earlier no-deployment rule. Reinspected `main`, HEAD, the `v1.0.6` tag, upstream, dirty files, and untracked work before acting. HEAD and the tag both remain `90b1fe5f55d5bca1a074c3db79b4616611cc852f`. No pull, source reset, new tag, new Release, or Git push was performed. The uncommitted UI changes, this file, `AGENTS.md`, and unrelated backup files remain in `/opt/komari-lite`.
- Added and tested a 320 px first-run installer footer/action fix; the built installer fits the viewport and its controls respond to pointer input. Final frontend validation: 239/239 tests passed, TypeScript/build passed, lint 0 errors/25 warnings. Backend and Agent Go tests and vet passed. A tracked-text Han scan returned zero files. Independent read-only source review passed with no security or logic findings. Existing optional broader UI/dependency cleanup is not a claim of completed redesign.
- Rebuilt the panel from the **dirty local tree** with `VersionHash=90b1fe5f55d5bca1a074c3db79b4616611cc852f-local`; synchronized 364 frontend/embed files byte-for-byte, built and checksummed 14 Agent release artifacts, and verified the fixed `v1.0.6` Linux AMD64 Agent download SHA-256. An isolated scratch installer check passed before downtime.
- Stopped only the affected Komari units, verified `/opt/komari` was a real non-mount directory distinct from the source, removed **all** of the old `/opt/komari` without backup, then recreated it. Installed root-owned panel and pinned Agent binaries and a dedicated `komari`-owned data directory. Replaced Komari systemd units, started the panel, and retained Caddy and unrelated `lite-agent.service` without restarting them. The new administrator completed first-run setup over HTTPS; `/api/install/status` reports `state=completed`, `required=false`.
- Both HTTPS hostnames return valid TLS and HTTP 200 for the public dashboard, admin entry, `/api/version`, and `/api/nodes`; HTTP redirects to HTTPS. The live admin JS/CSS and bundled mobile Glass CSS match the local source's built/embedded bytes. A real 320 px public-page browser check found no horizontal overflow and displayed 1 of 1 nodes online. Authenticated admin-page visual regression testing remains unverified because the new password was not shared, reused, or exposed to this session.
- Installed and running panel executable SHA-256 equals the prepared local build (`59f5d0c784bda37531e41090507cca5c079fcec1eeb103688f7a90f545d65150`). The running API reports version `1.0.6`, hash `90b1fe5f55d5bca1a074c3db79b4616611cc852f-local`; Go build metadata reports the same VCS revision and `vcs.modified=true`. The Agent executable matches the pinned `v1.0.6` Linux AMD64 release digest. The user created one fresh node and installed/started the matching Agent; the public API shows recent live reports and persisted CPU records, and Agent logs confirm a WebSocket connection after restart.
- The installer-generated Agent unit initially carried a fresh token in process arguments. Moved it without printing into `/etc/komari-agent-current.json` (root-only `0600`), replaced the unit with `--config` and no inline credential, then restarted just that Agent. Verified the token is absent from the new unit, systemd `ExecStart`, and process command line; the panel and unrelated services stayed running. Do not reuse the stale old `/etc/komari-agent.json` configuration.
- Final readback passed: at 2026-09-30 04:17 UTC the newest Agent report was 0.5 seconds old, with 20 recent live reports and 16 persisted CPU records. Both Komari services, Caddy, and the unrelated `lite-agent.service` were active/enabled; the panel listened only on `127.0.0.1:25774`; recent Komari logs had no panic, fatal, unauthorized, database-lock, or permission-denied signals. Caddy configuration validated (with a pre-existing formatting warning). `git diff --check` passed and the source HEAD/tag remained unchanged. The current local UI changes are deployed but remain uncommitted and are **not** part of the already published `v1.0.6` Release.

---

## Ping Monitoring Follow-up — September 30, 2026

- Rechecked `/opt/komari-lite` Git state and preserved its existing uncommitted changes; `/opt/komari` still contains the previously installed panel, Agent and data. Both Komari units and Caddy are active; first-run installation is completed, the public APIs respond, and dmit has three assigned TCP ping tasks named CT/CU/CM. Four-hour ping history remains empty while the running old Agent logs repeated `unknown v2 event method agent.ping` messages. **No live service, production data, or production theme has been modified in this follow-up.**
- Added local Agent `agent.ping` handling for TCP, HTTP and ICMP, with task-specific results through WebSocket or HTTP. Regression tests exposed and fixed both loss of HTTP results after probe timeout and loss of results when an established WebSocket write fails; both targeted tests and the Agent suite, vet, build and targeted race test pass. The candidate binary is distinct from the still-running pinned Agent.
- Added a testable Glass node-card model keyed by task ID. Its existing admin switch now selects one task or shows every assigned task separately; a new selector chooses a preferred task ID for single mode and falls back to the first task in backend weight/id order. A review found public RPC omits task weight; the model now preserves that already-sorted order rather than wrongly re-sorting by ID, covered by a red/green regression test. Statistics and bars use only that task's last-hour records; a task with no samples displays `-`, while all-failed probes display `100%` loss and no invented latency. Updated the generated Glass chunk through a guarded patch script and a new SHA-256-named asset, plus the manifest and admin validation. The prior three hard-coded carrier-name selectors are no longer displayed in the new manifest.
- Since `EnsureBundledThemes()` deliberately preserves an already-installed local theme, added `frontend/script/upgrade-glass-theme.mjs`: dry-run by default, rejects changes to the pinned original manifest/index/chunk or theme symlinks, and requires explicit `--apply` to stage a copy, overlay only three new theme files, and keep the original directory as an intact backup. Its dry-run against the real installed Glass succeeded without writing; tests exercised dry-run, explicit apply in a scratch fixture, and rejecting custom HTML. The new deployment path is prepared **but was not applied** to production.
- Final local validation: frontend 254/254 tests, lint 0 errors/25 existing warnings, TypeScript/build passed. Backend full tests and vet passed with proxy environment variables unset after an initial network-dependent GeoIP test failed through Privoxy; a later Go test failed only because its old chunk-scanning assertion expected a single bundled asset, and passed after checking the one *referenced* content-hashed asset. All 364 embedded frontend files match byte-for-byte, and the freshly rebuilt panel contains the new Glass asset `3859ru-ed32134c.js`. Isolated browser preview at 320 px showed CT/CU/CM as separate rows, selected CU in single mode, and five rows without page overflow; the preview deliberately has no Agent WebSocket, so its node appears offline, and no actual ping measurements were claimed.
- **Not deployed as of the follow-up above:** The live panel, Agent, and already installed local Glass copy still ran the older assets at that point. Updating only the panel binary would not replace the installed Glass theme. A non-destructive production rollout and live measurements required a separate explicit authorization and post-rollout verification; do not repeat the earlier clean deletion of `/opt/komari`.

---

## Authorized Non-Destructive Local Redeployment — September 30, 2026

- The user explicitly requested redeployment of the new local source and will reinstall the Agent on the monitoring VPS independently. First approval timed out **without running the cutover**; the user renewed approval, and the guarded cutover was then approved and completed. No Git pull/reset, tag/release creation, database deletion, or remote VPS action occurred.
- Corrected an important distribution mismatch before cutover: the admin one-click commands had pointed to the older public GitHub `v1.0.6` Agent, which lacks the new ping handler. After red/green tests, both per-node and auto-discovery commands use this panel's own `/agent/install.*` installer and `/agent/download` artifact catalog. Built 14 Agent platforms from the dirty local `90b1fe5f55d5bca1a074c3db79b4616611cc852f` checkout, checked every manifest SHA-256, and published the catalog under the existing runtime's `data/agent-release/`; the served Linux AMD64 artifact SHA-256 is `b1d5abd85103bf2e046d4e91213d80d3dbd5997765ba6b4cf68fdf7df132a91e`, version header `1.0.6`. Do not direct this installation to the same-numbered, older GitHub Release.
- Frontend full test suite **254/254** passed after the ownership regression fix, lint 0 errors/25 existing warnings, and frontend build passed; all 364 frontend files were synchronized to the backend embedded systemUI byte-for-byte. Backend/Agent full tests and vet passed, Agent targeted race tests passed, panel rebuilt from this asset tree. A new red/green test detected that the Glass upgrade's staged files lost the `komari` owner; the migration now preserves directory/file ownership and modes, verified on a non-root-owned scratch fixture before live use.
- Stopped only `komari.service` and `komari-agent.service` for the cutover. Preserved the prior binaries as `/opt/komari/.komari-before-ping-upgrade` and `.agent-before-ping-upgrade` and preserved the prior theme as `data/theme/Glass.backup-2026-09-30T09-49-13.341Z-636191eb`; no databases, nodes, settings, credentials, or unrelated `lite-agent.service` were removed. Installed the new Glass files into the existing theme without wiping it, and restarted the two Komari units. The first panel build had left the API version hash as `unknown`; immediately rebuilt with the truthful dirty-checkout `-local` hash and restarted only the panel, preserving the intervening panel binary as `.komari-ping-without-version`.
- Final installed panel SHA-256 `affef87da877d74132a59461588a095333a709e3b9d82ff724765d34b37299a0`; running API reports `1.0.6` and `90b1fe5f55d5bca1a074c3db79b4616611cc852f-local`. Installed local Agent SHA-256 equals the new served Linux AMD64 artifact above. Both Komari services, Caddy and unrelated `lite-agent.service` remain active. Installation is completed, existing node count remains one, and both HTTPS hostnames serve status 200 with valid TLS for version/dashboard/new Glass chunk and Agent download. The public theme manifest, new chunk, and admin distribution JS match the new local build, and the download is byte-identical to the manifest; no fresh GitHub tag or release was published.
- After local Agent restart, dmit's 1-hour ping API returned **21 actual records: 7 each for CT/CU/CM**, with no new `agent.ping` unknown-method events observed. The live Glass page loaded the new `3859ru-ed32134c.js` and rendered CT 168 ms, CU 170 ms, CM 172 ms separately with 0.0% loss each at the browser check; a real 320 px browser check showed zero page-level horizontal overflow. These measurements are a point-in-time verification of the locally running Agent, not proof that the user has updated a separate monitoring VPS. The latter remains user-owned follow-up; fresh actual measurements after that installation should be checked separately.

---

## Adaptive Glass Ping Card Layout — September 30, 2026 (Local Source Only)

- Inspected the user's screenshot and the live Glass DOM. The fixed `7.75rem` ping-panel minimum, `flex-1 justify-between` task list, and outer card's `336px` minimum/`justify-between` spread two rows far apart and leave single-row cards too tall. In the generated Glass patch script, removed those fixed per-task heights, made metric rows a compact content-driven flex column, and limited the card min-height/spacing override to the separate-per-task mode. Single-task legacy mode, metric values/colors, polling, APIs and Agent code are unchanged.
- Added red/green bundle assertions covering adaptive panel/card CSS and long-label truncation. Extended the guarded, backup-preserving theme migration to accept either the untouched vendor Glass files or the exact previously deployed ping-version manifest/index/chunk; tests cover dry-run and explicit migration of both versions, plus rejection of customized files. Dry-run on the **actual installed** theme returned eligible without writing.
- The new source theme references `3859ru-1c261694.js` with its SHA-256-derived fingerprint. In a read-only local preview (allowlisted read-only RPC calls to the running panel), rendered mixed 1-, 2-, and 5-task VPS card DOM without modifying monitoring data. At desktop width, panels measured 55/91/199 px and cards 357/393/501 px, respectively; rows held an 8 px gap. At 280 and 320 px, cards stacked at their own heights, no page-level horizontal overflow or long-label/metric overlap was measured. Previewed sample cards were DOM clones, **not** actual additional VPS nodes; the backend reported the one existing dmit node (offline at preview time).
- Full frontend tests **255/255** passed, lint 0 errors/25 existing warnings, TypeScript/production UI build passed; backend web/public tests and migration dry-run passed. A combined build/check shell command returned nonzero only because its final optional node invocation used `frontend/script` while already inside `frontend`; that command was rerun from the repository root and passed. `git diff --check` passed. No new tag/release/push or production theme/service changes were authorized or performed in that local-source batch; the new layout was **not yet deployed at that point**. The subsequently authorized production rollout is recorded below.

---

## Authorized Adaptive Layout Deployment — September 30, 2026

- The user explicitly confirmed deployment of the local code. Rechecked Git HEAD/status (preserved all dirty/untracked work), production panel/service/theme ownership and digests, free disk space, installed data and node API, candidate theme fingerprint, and the installed theme's eligibility for the guarded migration. No reset, pull, clean install, database deletion, Agent rebuild/replacement, new tag, or Git push occurred.
- Full frontend tests **255/255**, lint 0 errors/25 pre-existing warnings, backend full tests and vet passed. An initial backend full-test run failed only at the network-dependent GeoIP test through Privoxy; rerun with proxy environment variables unset passed. The pre-built frontend's 364 generated files were synchronized and byte-checked against backend embedded systemUI (only 96 stale generated JS/CSS fingerprints removed), then backend web/public tests passed and the panel was rebuilt with the true local `-local` Git hash. The first sync attempt used a wrong working-directory-relative frontend path, failed before writing, and was corrected with absolute paths before rebuilding.
- Staged and hash-checked new panel SHA-256 `f4262ea60be01f22b5c4e916362af5fb8854686645c21116e4b51f0c696c2bc0`. Stopped **only** `komari.service`, applied the owner/mode-preserving Glass migration to the pinned installed theme, atomically installed the panel, and restarted that service with rollback handling. Previous binary retained at `/opt/komari/.komari-before-adaptive-layout`; intact previous theme backup is `/opt/komari/data/theme/Glass.backup-2026-09-30T11-23-49.743Z-2a98ad04`. Existing database, settings, node data and Agent distribution were preserved; `komari-agent.service`, Caddy and unrelated `lite-agent.service` remained running.
- Post-deploy readback: installed binary matches the staged SHA-256 and embeds the new theme asset; installed theme HTML and `3859ru-1c261694.js` exactly match source; new asset is owned by the service account and served as HTTP 200 with the expected digest. Local admin entry matches the embedded frontend build, install state is completed, and Agent download digest is unchanged. Both HTTPS hostnames return HTTP 200 with valid TLS and serve HTML referencing the new chunk. Browser loaded this **deployed** chunk and rendered real dmit CT/CU/CM separately at 8 px row gaps; at 320 px the three-row panels each measured 127 px and the document width equaled the viewport (no horizontal overflow). The only existing node remained visible with fresh ping records (120 in a one-hour readback); actual simultaneous 1/2/5-node comparison remains a synthetic preview because only one VPS is installed. Both Komari units, Caddy and the other lite agent were active, and recent panel logs had zero panic/fatal/database-lock matches.

---

## Reference-image Visual Redesign — September 30, 2026 (Local Source Only)

- Confirmed `/root/.hermes/cache/images/img_7b0eb3d9d72f.jpg` as the user's primary reference: deep teal canvas, narrow left navigation, compact rectangular panels, thin borders and technical metrics. The public Glass theme and React/Radix admin UI are separate renderers.
- Added shared light/dark Komari color tokens, flat panel/kicker styles, dark-jade default for new React users, compact admin navigation/cards/controls and a matching standalone login panel. Existing appearance choices remain available. No monitoring semantics or API was changed for this visual batch.
- Glass public theme now uses a SHA-256-fingerprinted visual stylesheet layered after pinned vendor CSS; its default appearance is dark while light/system remain selectable. The ping-task chunk is unchanged. The migration dry run recognizes the *actual installed* adaptive-layout Glass by exact hashes and does not write to production; fixture tests exercise backup-preserving apply and reject customized files.
- Frontend 263/263 tests, TypeScript/build, backend full tests/vet and Agent full tests/vet passed. Lint had 0 errors and 25 existing warnings. Browser fixture measured Glass card background `rgb(11,41,42)`, 4px radius, no shadow/blur, and no page overflow at 375px; local login preview at 320px showed no horizontal overflow and shared panel computed colors. Authenticated admin pages and live public data have **not** been visually validated with this new source.
- The redesign remains **local, uncommitted and not deployed**. `/opt/komari` services, theme, database and Agent were not modified or restarted for it. A new production rollout needs separate authorization; a new binary by itself would not replace the installed Glass theme. Read-only status at the final check: `komari.service` active; `komari-agent.service` inactive after a graceful stop at 20:52:41 local time, before this design work. Its inactive state was not changed as part of this task and should not be mistaken for a successful live-Agent check.
- Independent read-only review found two adjacent rollout/setting issues. A deleted preferred ping task now normalizes to automatic selection only after the task list loads successfully; settings saving waits for the list, exposes fetch errors with retry, and no longer logs the entire settings payload. The two-rename Glass migration is still not an atomic exchange; `frontend/script/GLASS_RECOVERY.md` documents pinned-hash verification and manual restoration if interruption removes the live theme path. No recovery or migration apply was run on production.
- After these review fixes, frontend **265/265** tests passed, lint 0 errors/25 warnings, TypeScript/build passed, and the real installed Glass again passed a read-only migration dry run (`eligible: true`).

---

## Authorized Local-Source Visual Deployment — September 30, 2026

- The user explicitly requested deployment from the current local Git source. Preserved the dirty/untracked `/opt/komari-lite` tree (HEAD `90b1fe5f55d5bca1a074c3db79b4616611cc852f`), installed accounts/nodes/database, Agent catalog and services; no pull, reset, tag, release, push, reinstall or data deletion occurred.
- Re-ran frontend tests **265/265**, lint (0 errors/25 warnings), TypeScript/build, backend and Agent full tests/vet. Synchronized and byte-verified all 364 generated frontend files into the Go embed tree, rechecked the pinned installed Glass theme, and built panel version `1.0.6` with the truthful dirty-local hash. Candidate panel SHA-256: `550ca665216048d4afabe54f8c067deaac6bedb9cdf9ac57f085b7b157d2f038`.
- Staged and checked both candidate and rollback binaries before stopping only `komari.service`. Applied the guarded, owner-preserving Glass migration with backup `/opt/komari/data/theme/Glass.backup-2026-09-30T14-28-26.547Z-75206abb`, replaced the panel binary, then restarted that service. Previous panel binary is `/opt/komari/.komari-before-visual-550ca665`. No Agent or Caddy service was restarted.
- Post-deployment readback: running panel binary matches the candidate digest, API reports `1.0.6` and `90b1fe5f55d5bca1a074c3db79b4616611cc852f-local`, and installation remains completed with one node. Installed Glass manifest/index/chunk/visual stylesheet match the built source; live admin JS/CSS and public CSS match built bytes. Both configured HTTPS hostnames returned valid TLS and HTTP 200 for dashboard, admin, API and new public assets; HTTP redirects to HTTPS. Authenticated desktop admin dashboard and public/mobile pages loaded the new styles; at 320 px both sampled dashboards had no document overflow. The real node is offline because `komari-agent.service` was already inactive before deployment; that service remains inactive, while `lite-agent.service` and Caddy remain active. Panel logs after restart had no panic/fatal/database-lock/permission-denied matches.
- Glass migration's two directory renames remain non-atomic; keep the exact backup and follow `frontend/script/GLASS_RECOVERY.md` if an interrupted future migration loses the live theme path. No new tag or Release was created, and the visual changes remain uncommitted in the local tree.

---

## Explicit Destructive Local Rebuild — October 1, 2026 (Local Time)

- User explicitly requested recompiling the latest **local dirty working tree**, deleting all of `/opt/komari`, and not making a backup. Rechecked source HEAD `90b1fe5f55d5bca1a074c3db79b4616611cc852f`, local edits, installed runtime, service state, mounts, build instructions and Caddy routes. Did not pull/reset/commit/tag/push or alter `/opt/komari-lite`. The prior accounts, one node, databases, monitoring history, settings, theme copies, rollback files and Agent catalog under `/opt/komari` were deliberately removed without backup.
- Before downtime, ran `npm ci`, frontend **265/265** tests, lint (0 errors/25 warnings), TypeScript/build, backend full tests/vet and Agent full tests/vet. Synchronized and byte-verified all 364 admin UI embedded files. Rebuilt panel SHA-256 `550ca665216048d4afabe54f8c067deaac6bedb9cdf9ac57f085b7b157d2f038` with `1.0.6` and `90b1fe5f55d5bca1a074c3db79b4616611cc852f-local` metadata; built and checksum-verified 14 same-source Agent artifacts, including Linux AMD64 SHA-256 `b1d5abd85103bf2e046d4e91213d80d3dbd5997765ba6b4cf68fdf7df132a91e`.
- Stopped only `komari.service`, removed the verified real non-mounted `/opt/komari` runtime, recreated least-privilege root-owned binary/komari-owned data paths, installed the rebuilt panel and local Agent distribution catalog, then started the panel. New Glass index and visual CSS digests match bundled source. `/api/install/status` reports `ready, required:true`; the new database passes integrity check and has zero users and zero clients. First-run administrator setup and Agent re-enrollment remain pending. An old root-only `/etc/komari-agent-current.json` outside the authorized deleted path remains **stale** and must not be reused; `komari-agent.service` was inactive before and remains inactive. Unrelated `lite-agent.service` was not touched.
- The initial first-run proxy guard blocked public `/api/install` mutations while leaving the local installer available. This conflicted with the user's established HTTPS-domain installation workflow and caused their final submit to return 403. On their report, confirmed public POST 403 versus loopback validation 400, removed the guard from Caddy, validated and reloaded it. Readback on **both** HTTPS hostnames: installer GET 200 with valid TLS; an empty POST to `/api/install/complete` and `/api/install/upload/init` now reaches backend validation (400), not proxy 403. `/api/install/status` remains `ready, required:true` until the user completes setup. Warn that the publicly claimable installer should be completed promptly. Before setup, `/api/version`, Agent download and dashboard are intentionally gated; after setup verify completed status, public/admin pages, panel-served Agent catalog and Agent enrollment separately.

---

## Release v1.0.21 Preparation — October 8, 2026

- The existing `v1.0.20` tag and public stable Release already point to the earlier calculator commit, so the verified exchange-rate/UI work is being released as the new `v1.0.21` version rather than overwriting a published tag.
- Aligned server, Agent, frontend package, admin Agent installer source, release-version tests, and English Agent documentation to `1.0.21`/`v1.0.21`.
- Added root-level `update-komari.sh`: strict Bash updater that detects `amd64`/`arm64`, resolves one stable official GitHub Release, verifies `SHA256SUMS.txt`, verifies binary/tag/version/schema metadata, preserves the existing systemd unit/arguments and persistent data, creates an executable backup, and health-checks rollback behavior. It refuses schema downgrades and backs up SQLite via the SQLite backup API before a potential migration; it does not blindly restart an old binary if the schema changed.
- Added `scripts/build-release.sh` for reproducible release assembly: frontend test/lint/build/embed-sync, Linux `amd64` and `arm64` CGO panel builds, 14 platform Agent artifacts, transition Glass bundle, controller/updater assets, full checksum inventory, and release manifest.
- Added English fresh-install, version-pinned Agent installation, explicit/dry-run update, backup, rollback-limit, and running-version documentation in `README.md`.
- Validation before tag/release: frontend tests/build/lint/embed synchronization, backend tests/vet, Agent tests/vet, 111 deployment/controller tests, Bash syntax checks, ShellCheck, and `git diff --check` passed. No production service or data was modified during release preparation.

---

## Versioned Production Updater Compatibility — October 8, 2026

- Reworked `update-komari.sh` to detect both a protected flat executable layout and the versioned `komari -> current/komari`, `current -> releases/<version>` layout. It rejects broken, escaping, unexpected, writable, or untrusted symlink/directory chains instead of replacing a symlink target.
- For the known v1.0.19 compatibility controller SHA-256, a versioned update now downloads and verifies the panel binary and `Glass.zip`, then delegates the complete transaction to the already-installed `/var/lib/komari-upgrade/safe_upgrade.py`. The wrapper never replaces the controller, its `ExecStartPre` gate, systemd unit, or historical releases.
- Added isolated black-box updater fixtures covering flat and versioned v1.0.19-to-v1.0.21 updates, clean dry runs, checksum rejection, escaping symlinks, unsafe upgrade state, controller/gate rejection, interruption fail-closed behavior, idempotent re-execution, flat startup rollback, and preservation of both SQLite databases. Existing safe-upgrade controller tests continue to cover journaled snapshots, migration/start failures, rollback, and automatic recovery. No production host was accessed or modified.
- The first updater-only `v1.0.23` prerelease allowlisted the production controller fingerprint without changing `v1.0.21`. A subsequent read-only controller preflight exposed a stale installed recovery service unit, even though the simpler gate passed. `v1.0.24` added the controller's complete read-only `preflight()` check to versioned dry runs and update preparation; `v1.0.25` preserves its diagnostic output on rejection. Both abort before any service stop if recovery, theme, database, schema, disk-space, or controller checks fail. The production controller SHA-256 is `1fe3c0525b3358fff7367ef5ef0b1174bcfa3e23f6f03a9d8fb9362301659cd1`, matching tracked compatibility source commit `0216e11e2b120abde41c98b7887ee9c5fd209db3`. No controller, systemd unit, service, symlink, or database was modified.

---

## Country Icons and Trusted Node GeoIP — October 8, 2026

- Replaced the incomplete country picker path with a comprehensive ISO 3166-1 alpha-2 selection set. The UI exposes searchable entries, includes United Kingdom as `GB` / 🇬🇧, accepts `UK` as an input alias, and retains the canonical `GB` value. Server-side overrides now enforce the same canonical model and reject invalid two-letter regional-indicator values.
- Automatic country enrichment now selects only globally routable Agent-reported public addresses, deterministically preferring IPv4 before IPv6 and structured inventory. IPv4-mapped IPv6 is normalized to IPv4; loopback, private, link-local, documentation, multicast, and other non-public addresses are rejected. Direct peer metadata is only a last-resort fallback and request forwarding headers remain untrusted.
- The Agent validates public-address provider responses with `netip`, preserves valid custom/NIC values, and falls back by address family without turning private or malformed values into node identities.
- GeoIP provider access now swaps atomically, coalesces simultaneous lookups, clears cached values on provider replacement/database refresh, and caches transient failures briefly to avoid repeated external traffic. Existing manual overrides are preserved and provider failures do not clear a previously stored automatic region.
- Validation passed: frontend tests/lint/build with byte-for-byte frontend-to-embed synchronization; backend full tests/vet plus focused race tests; Agent full tests/vet. A local runtime node exists for final live validation, but no production deployment has been performed.

### Development Validation Completion — October 8, 2026

- Added an isolated end-to-end backend integration regression that submits the stable v2 Agent basic-info contract using an older Agent version (`1.0.19`), a mapped public IPv4 scalar, a public IPv6 scalar, and structured public addresses. The test verifies mapped-IPv4 normalization and IPv4 preference, persistence of both validated public families, GeoIP resolution to canonical `GB` / 🇬🇧, and preservation of an existing manual 🇺🇸 override. Invalid/private-address rejection and GeoIP failure preservation remain covered by focused existing regressions.
- Frontend regression coverage confirms `UK` searches/normalizes to canonical `GB`, the displayed English name is `United Kingdom`, and the supported flag set includes 🇬🇧. Authenticated admin UI validation on the local development panel also confirmed that searching `UK` exposes `GB — United Kingdom`; no node record was saved or modified.
- Read-only local development-node observation: `aether2` submitted successful v2 RPC basic-info requests after the local development runtime was updated, at `2026-10-08 13:23:16 UTC`. Its reported Agent version is `1.0.21`, validated IPv4/IPv6 values are present, its automatic region is 🇺🇸, and no manual override is set. This is a real-node compatibility/readback observation only; it does not claim that a newly changed country was produced by that specific report.
- Development validation is complete without waiting for `aether2`. The isolated integration result is controlled and deterministic; the read-only node observation is live but non-mutating. Nuyek production remains unchanged, and no Git tag, GitHub Release, production node-record mutation, deployment, service restart, or Agent reinstall was performed for this validation.

---

## AI Monitoring Final UI Redesign — October 9, 2026 (Local Source Only)

- Redesigned the default AI Monitoring route around exactly four compact sections: Account Quotas, Live Overview, one selectable Usage Trend chart, and Daily Usage Summary cards. Kept Komari navigation/theme and moved filters, full model/task tables, accounting, collector diagnostics, pricing, retention, and timezone administration behind a collapsed Advanced disclosure.
- Quota cards now show remaining quota as the primary value, explicit remaining/used copy, a progress bar labeled as remaining quota, reset time, freshness, and provider/plan only; opaque account keys are not rendered. Zero-delta covered periods render explicit healthy-no-usage states and suppress zero-only model rows. Daily cards are newest-first, token-share ordered, show four active models by default, and expand additional active models on demand. Unknown prices remain Unpriced and sampled throughput remains N/A where not reliable.
- Fixed the confirmed AI read-rate-limit defect: limits are now per authenticated principal **and resource**, rate-limit responses use JSON-RPC `RateLimited` mapped to HTTP 429 with `Retry-After: 10`, and the frontend performs one bounded retry for idempotent read requests only. Added a focused backend regression for independent overview/settings/pricing windows.
- Validation: frontend tests **418/418** passed; ESLint completed with **0 errors** and 25 pre-existing warnings; TypeScript and production embedded-system-UI build passed; backend `go test ./...` and `go vet ./...` passed; `git diff --check` passed. Source preview was rendered in a real browser with isolated fixture responses at desktop 1280px and mobile 390px. The compact dashboard, explicit quota labels, four overview cards, daily cards, collapsed Advanced control, and zero page-level horizontal overflow were observed. The preview fixture was isolated under `/root/.hermes/cache/scratch` and did not affect source/runtime data.
- The generated embedded frontend assets were refreshed by the verified production build. Independent pre-commit review caught and corrected the API-key rate-limit partition: the request principal now carries only a SHA-256 fingerprint of an accepted Authorization value, never the credential itself. Today Tokens is returned as a timezone-aware calendar-day aggregate rather than a rolling-minute counter. No live deployment, service restart, database/configuration change, GitHub push/tag/release, or modification to `komari-agent-production.service` occurred. The development v1.0.27 deployment remains untouched until separate authorization.

### Development v1.0.28 AI Monitoring deployment — October 10, 2026

- Deployment source was the clean, locally committed `0c8ec87d520b07d16110cdbef390f987fbe618d7` (`build: synchronize v1.0.28 system UI assets`), on top of the UI redesign and `1.0.28` version-alignment commit. The detached release build passed frontend tests **418/418**, lint, TypeScript/build, backend tests/vet, Agent tests/vet, a full 26-artifact SHA-256 inventory, and clean Go VCS metadata (`vcs.modified=false`). No tag, GitHub push, or Release was created.
- The initial controller attempt refused the recovery-unit check because it was invoked with the controller default `systemctl` path while the verified installed recovery service was intentionally pinned to `/usr/bin/systemctl`. Read-only comparison against the installed controller's exact generated unit/drop-in text showed the recovery service, timer, and pre-start gate were all intact; their controller hash matches local source. `systemd-analyze verify` passed and the recovery timer remained enabled/active. No unit repair was necessary. Re-running the controller with its canonical installed `--systemctl /usr/bin/systemctl` argument passed its integrity checks.
- The safe controller upgraded **only** `komari-lite-panel.service` from `1.0.27` to `1.0.28`. `current` now resolves to `releases/1.0.28`; loopback and public version APIs report `1.0.28` / `0c8ec87d520b07d16110cdbef390f987fbe618d7`; installed panel bytes equal the verified release artifact; both panel SQLite databases pass `PRAGMA quick_check`; and committed rollback snapshot `20261010T054503-9c5e281b` exists with `old_version=1.0.27` and `target=1.0.28`. The recovery timer is enabled/active and the post-cutover controller gate passed.
- Isolation readback: `komari-agent.service`, `komari-agent-hermes.service`, and `komari-agent-production.service` stayed active with their pre-cutover PIDs and start times; no Agent service, profile, credential, database, node, user, history, theme, or configuration was changed directly by this rollout.
- Authenticated browser readback loaded the new deployed AI Monitoring assets, the exact four default sections, explicit quota labels, Today Tokens card, Advanced content, and a 390px viewport with no document horizontal overflow. However, the deployed 24H overview request returns HTTP 400 because the existing raw-detail query reaches its `MaxQueryRows=50000` limit: `ai_samples` contains 97,050 rows in the last 24 hours and the handler fetches `50001` rows before returning `ErrQueryLimit`. This leaves actual quota/live/daily values unavailable in the browser despite the page structure loading. The rate-limit behavior itself did not return HTTP 429 during this acceptance attempt. This is a confirmed post-deployment acceptance defect; no follow-up source/runtime change has been applied pending separate authorization.

### Development v1.0.29 AI aggregation deployment — October 10, 2026

- Prepared the locally verified database-side overview aggregation fix as version `1.0.29`. Version metadata, frontend package/installer metadata, release assertions, and generated embedded System UI assets were aligned and committed as `ade8d30ca295f63a414d042356f07e25c21d1022` (`chore: align development release version to 1.0.29`), on top of aggregation/API fix `fca06cadcb5b654a32eb40ba9d87b0347e497e41`. No tag, GitHub push, or public release was created.
- A fresh detached build from exactly `ade8d30…` passed frontend tests **418/418**, lint with 0 errors (25 existing warnings), TypeScript/System UI production build, backend and Agent test suites, 14 cross-platform Agent artifacts, complete release checksums, embedded Glass bundle checks, and release-manifest checks. The panel artifact reports version `1.0.29`, revision `ade8d30…`, and `vcs.modified=false`.
- Before cutover, read-only SQLite `PRAGMA quick_check` passed for both development panel databases; the prior committed recovery backup was present; `systemd-analyze verify` passed for recovery service/timer; the timer was enabled/active; and the installed gate passed when invoked with its required explicit `/usr/bin/systemctl` path. Candidate artifact hashes were verified before controller use. No Agent or Hermes helper service was changed.
- The safe controller upgraded **only** `komari-lite-panel.service` from `1.0.28` to `1.0.29`. The post-cutover journal is committed with rollback snapshot `20261010T065115-41f16502`, `old_version=1.0.28`, and `target=1.0.29`. `current` resolves to `releases/1.0.29`; loopback and public version APIs report `1.0.29` / `ade8d30…`; installed panel bytes match the release artifact; both panel databases still pass `quick_check`; and the recovery timer remains enabled/active. `komari-agent.service`, `komari-agent-hermes.service`, and `komari-agent-production.service` retained their pre-cutover PIDs and start times.
- Authenticated live browser/API verification confirms the 24-hour overview now returns HTTP 200 in about 361–423 ms against the real ~100k-sample dataset, with real 1,324,146-token / 6-call daily data, Unpriced model state, explicit N/A live rate state, quota cards, task/model aggregates, and Advanced disclosure functioning. Desktop width 1280px and emulated 390px mobile width have no document horizontal overflow. The 7-day request also returns HTTP 200 in about 359 ms.
- Acceptance limitation discovered after cutover: the live UI’s exact 30-day request returns HTTP 400 with `AI range exceeds configured retention`, even though settings report `retention_days=30`. The range-validation boundary is time-sensitive: a request generated as an exact trailing 30 days is rejected by the server after even a small processing delay. This is distinct from the repaired raw-row aggregation defect. No retention, database, service, or source change was made to bypass it; further repair requires separate authorization.

### v1.0.29 30-day retention-boundary repair — October 10, 2026 (Local Source Only)

- Root cause: the frontend sent client-clock `from`/`to` timestamps for 24H/7D/30D. The server compared the 30-day `from` value to its later current timestamp, so even a few seconds of transport/render delay made an exact 30-day request appear older than configured retention.
- Added a server-authoritative `preset` overview parameter. `24H`, `7D`, and `30D` now derive trailing fixed-duration boundaries from the server clock; `Today` derives local midnight through the selected IANA timezone. Presets ignore stale/skewed client bounds. Explicit custom `from`/`to` ranges remain strictly retention- and future-skew-bounded, so this does not expand readable history.
- The dashboard now requests its 24H/7D/30D choices using `preset`, while the JSON route allows that parameter through to the existing authenticated handler. Raw/rollup aggregation, detail limits, daily timezone/DST behavior, pricing, token accounting, and coverage logic remain unchanged.
- Regression coverage includes delayed/stale preset request values, exact 24H/7D/30D server boundaries, strict custom retention/future bounds, UTC and Asia/Shanghai Today boundaries, US DST transition and exact-midnight Today behavior. The high-volume 100,001-row database test now exercises 24H/7D/30D presets, with 30D using Asia/Shanghai.
- No service, panel database, Agent, helper, credential, rollback snapshot, tag, push, publication, or deployment was changed for this repair. Full backend/frontend validation, lint, TypeScript build, embedded-asset synchronization, `go vet`, and `git diff --check` passed before the local commit.

### Development v1.0.30 preset-range deployment — October 10, 2026

- Prepared and committed the exact deployment source as `7be80fb7914c6950016e34b77f89899aef19e53d` (`chore: align development release version to 1.0.30`), on top of preset-range repair `b520f32c817b82254c904fcc8e4104aaf827c9d7`. Server, Agent, frontend package/lockfile, admin installer metadata, release tests, and generated embedded System UI assets were aligned to `1.0.30`. No tag, GitHub push, or public release was created.
- A detached clean worktree build from exactly `7be80fb…` passed frontend tests **418/418**, TypeScript/System UI production build, lint with 0 errors and 25 existing warnings, backend `go test ./...`/`go vet ./...`, Agent `go test ./...`/`go vet ./...`, cross-platform Agent packaging, theme transition verification, SHA-256 inventory verification, and release-manifest verification. The release contains 26 artifacts; its AMD64 panel reports `1.0.30` / `7be80fb…` with Go metadata `vcs.modified=false`.
- Read-only preflight passed: both `/opt/komari-lite-panel/data/komari.db` and `metrics.db` returned `PRAGMA quick_check=ok`; the current v1.0.29 release and committed rollback backup were present; recovery service/timer unit verification passed, and the timer was enabled/active. The installed controller gate passed using the required explicit `--systemctl /usr/bin/systemctl` plus explicit panel root/state/unit/service paths. Candidate binary and Glass bundle SHA-256 values were verified before use.
- The journaled safe controller upgraded **only** `komari-lite-panel.service` from `1.0.29` to `1.0.30`. Post-cutover, `current` resolves to `releases/1.0.30`; local and public version APIs report `1.0.30` / `7be80fb…`; installed binary SHA-256 matches the release inventory; both databases continue to pass `quick_check`; the recovery timer/gate remain healthy; and committed rollback snapshot `20261010T074020-d9d20247` records `previous=1.0.29` and `target=1.0.30` with data/manifest/metadata present.
- Isolation readback confirms `komari-agent.service` (PID 262870), `komari-agent-hermes.service` (PID 262831), and `komari-agent-production.service` (PID 75770) retained their pre-cutover PIDs/start identities. No Agent service, development profile, production identity, credential, node, user, history, theme, or persistent configuration was modified outside the panel safe-upgrade transaction.
- Authenticated browser/API acceptance at `https://262526.xyz/admin/ai-monitoring` succeeded. Real 24H, 7D, and 30D preset API calls each returned HTTP 200 with the existing historical data (6 calls / 1,324,146 tokens; 14 model and 5 task aggregates; daily data present). Measured direct browser-session request times were approximately 436 ms (24H), 371 ms (7D), and 381 ms (30D); the former retention-boundary 30D failure did not recur. Today returned HTTP 200 for both UTC and Asia/Shanghai. The page rendered quota cards, selected-timezone status, Today Tokens, daily summary, Unpriced state, Advanced controls, and the 30D selector. At an emulated 390×844 viewport, Advanced remained usable and the document had no horizontal overflow.

### v1.0.31 RC blocker remediation — October 10, 2026 (Local Source Only)

- Aligned Panel, Agent, frontend package/lockfile, Admin-generated Agent commands, public installation/update examples, and release notes to `v1.0.31`. Corrected the release builder’s previous-tag default to `v1.0.26`.
- Flat-layout updates now persist a fsynced transaction journal before stopping the panel. The updater installs a root-only durable recovery helper, boot-enabled recovery unit, and service ordering drop-in before interruption; an interrupted stop is automatically rolled back before the original service starts at reboot. Black-box tests cover abrupt parent termination, recovery-unit enable-before-stop ordering, and reboot-triggered restoration of the executable and both SQLite databases.
- The direct safe-upgrade controller independently compares the installed SQLite schema with the candidate schema before service stop or journal mutation. First-run administrator and configuration persistence is now transactional; a fault-injection test verifies settings failure leaves neither account nor configuration records.
- SQLite test connections now close explicitly under Python 3.13. Source-only verification passed: frontend tests 419/419 and lint with 0 errors/25 existing warnings; backend and Agent `go test ./...` and `go vet ./...`; updater/controller/compatibility/installer security tests 132/132 with `ResourceWarning` promoted to errors; shell syntax checks and generated System UI synchronization passed. No runtime service, database, Agent, credential, deployment, tag, push, or publication was changed.

### Development v1.0.31 RC deployment — October 10, 2026

- Deployed only `komari-lite-panel.service` through the installed journaled controller, using explicit `/usr/bin/systemctl`, from the verified 26-artifact release directory built cleanly from `c457211d33c3f75751b3b5cf4d256239b8416662`. The controller preflight/gate, candidate checksums and provenance, panel/metrics SQLite `quick_check`, existing rollback inventory, recovery unit/timer verification, and disk/layout gates passed before service cutover.
- The controller committed snapshot `20261010T151522-320e33e3`, recording `previous=1.0.30` and `target=1.0.31`; its data, prior-release executable, manifest, and metadata are present. Current panel bytes SHA-256 `ac04d03a2c352ead248484e6b15dcaa80cc6e00eab4e56ce4133c22edf3da403` exactly match the release artifact. Local and public version APIs report `1.0.31` / `c457211d33c3f75751b3b5cf4d256239b8416662`; both live SQLite databases remain healthy at schemas 2 and 9. The controller recovery timer remains enabled/active and the post-cutover gate passed.
- Read-only authenticated browser validation confirmed the existing administrator session, one online existing monitored node, dashboard traffic/storage data, and deployed AI Monitoring quota cards, daily summary, 24H/7D/30D range views, and expanded Advanced UI. Today-token content is visible in Live Overview. Isolated updater/controller regressions passed 55/55, including flat-layout reboot recovery and direct schema-downgrade protection; no destructive recovery scenario was run against the live panel.
- Protected `komari-agent.service`, `komari-agent-hermes.service`, and `komari-agent-production.service` retained their pre-cutover PIDs and start timestamps. No Agent, user, node, credential, or persistent setting was intentionally changed. A post-validation direct authenticated `preset=Today&timezone=UTC` request returned HTTP 429 without `Retry-After` after repeated AI overview reads, including after a 65-second wait; this is recorded as an acceptance limitation requiring separate investigation, with no source/runtime remediation applied. No tag, push, or public Release was created.
