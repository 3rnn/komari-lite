# TASK_PROGRESS.md

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
