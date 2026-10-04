# AGENTS.md

# komari-lite Project Instructions

This repository is **komari-lite**.

These instructions apply to all AI coding agents working in this repository, including Hermes, Codex, and similar automated assistants.

The goal is to improve komari-lite while preserving its monitoring behavior, compatibility, deployment model, and maintainability.

---

## 1. Instruction Priority and Scope

Follow this order of priority:

1. The user's current explicit request.
2. Safety, compatibility, and data-preservation requirements in this file.
3. Project-specific conventions discovered in the repository.
4. General cleanup or optimization opportunities.

Do not expand the scope of a task merely because unrelated cleanup is possible.

For every task:

- Inspect before editing.
- Understand the existing implementation and repository state.
- Change only what is necessary for the requested outcome.
- Preserve unrelated user changes.
- Prefer focused, reviewable changes over broad rewrites.
- Validate meaningful changes before continuing.
- Fix regressions introduced by your work before considering the task complete.

For normal Komari development requests, the default delivery workflow includes local build, clean local deployment, live service restart, post-deployment verification, Git commit, and Git push as defined in **Default Change Delivery Workflow** below.

Do not alter production data outside the established `/opt/komari` clean-redeployment workflow unless the user explicitly requests it.

---

## 2. Default Change Delivery Workflow

Unless the user explicitly asks for a different scope, every requested Komari code or UI change should be treated as a complete delivery task.

The default workflow is:

1. Re-read `AGENTS.md` and `TASK_PROGRESS.md` when present.
2. Inspect the current local Git state: current branch, `git status`, `git diff`, existing uncommitted changes, and relevant recent commits when useful.
3. Treat the current local repository as the source of truth.
4. Understand the requested change and the affected frontend/backend/Agent code.
5. Implement the requested change.
6. Review the diff and remove accidental or unrelated edits.
7. Run all relevant available validation: targeted tests, frontend tests, backend tests, Agent tests, lint, type checks, build, and visual/responsive checks when UI is affected.
8. Fix regressions introduced by the change and rerun failed checks.
9. If validation succeeds, create a focused Git commit for the completed change.
10. Build the exact committed source locally.
11. Perform a clean local deployment to `/opt/komari`: stop the existing Komari service, remove `/opt/komari`, do not back up the previous `/opt/komari` contents/configuration/data/ownership/permissions, recreate `/opt/komari`, install the newly built artifacts, recreate only the configuration/ownership/permissions/systemd settings required by the current source, and restart the Komari service.
12. Verify the deployed system: service status, startup/runtime logs, frontend availability, administrator UI when applicable, API availability, monitoring data, Agent communication, and actual running version/build.
13. If deployment verification succeeds, push the current branch and the new commit to the configured GitHub remote.
14. Report the final commit, validation results, deployment result, push result, and final execution status.

Do not stop for confirmation between these normal stages.

Pause only when one of the following genuinely requires user action or a decision:

- credentials, administrator setup, MFA, or another interactive authentication step
- a destructive database/schema operation outside the established clean `/opt/komari` replacement workflow
- unresolved unrelated local changes that could be overwritten
- an ambiguous product decision that cannot be safely inferred
- an external rate limit or unavailable dependency
- a failed validation or deployment problem that cannot be safely repaired automatically

If a step fails, do not blindly continue to later stages.

In particular:

- Do not deploy code that has failed required validation.
- Do not push a change that has not been successfully committed.
- Do not claim deployment or push success unless verified.
- Do not create a Git tag or GitHub Release for every change. Tags and Releases are created only when explicitly requested.
- Do not use `git reset --hard`, force-push, or overwrite unrelated local changes.
- Do not pull remote changes over the validated local source unless explicitly requested or clearly necessary and safe.

The intended normal lifecycle is:

```text
User request
  -> inspect
  -> modify
  -> validate
  -> commit
  -> build committed source
  -> clean deploy to /opt/komari
  -> restart
  -> post-deployment verification
  -> push to GitHub
  -> final status
```

---

## 3. Core Engineering Principles

Always prefer:

- Existing project architecture over unnecessary replacement.
- Backward compatibility over breaking changes.
- Maintainability, stability, and clarity over clever abstractions.
- Reusable components and shared primitives over duplicated implementations.
- Small, logical batches over one large rewrite.
- Evidence from the current repository over assumptions from memory.

Do not:

- Rewrite working code without a clear reason.
- Replace frameworks, major libraries, build systems, or storage mechanisms unless required.
- Mix unrelated refactors into the requested task.
- Introduce speculative abstractions.
- Change behavior merely to make code look cleaner.

When a larger redesign is explicitly requested, architectural changes are allowed when they materially improve the result, but they must still preserve required functionality and compatibility.

---

## 4. English-Only First-Party Repository Content

First-party project content should use English only.

This includes, where applicable:

- UI text
- Buttons
- Menus
- Labels
- Forms
- Dialogs and modals
- Notifications
- Tooltips
- Error and status messages
- Logs
- Comments
- Inline documentation
- Configuration descriptions
- Example text
- README and project documentation
- Developer-facing messages
- CLI output
- Test descriptions

Do not leave accidental mixed Chinese and English text in first-party project content.

Do not blindly modify:

- Vendored dependencies
- Generated files
- Build artifacts
- Third-party source code
- External protocol values
- Third-party API identifiers
- User-provided runtime data
- Compatibility-sensitive persisted data

If Chinese text exists in compatibility-sensitive data or external content, preserve it unless the task explicitly requires migration or translation.

---

## 5. Compatibility-Sensitive Areas

komari-lite is a monitoring project. Treat the following areas as compatibility-sensitive:

- Node status
- Online/offline detection
- Heartbeats
- Metrics collection
- CPU, memory, disk, and network metrics
- Traffic statistics
- Latency
- Uptime
- Historical data
- Server and node lists
- Agent communication
- Polling intervals
- WebSocket or streaming connections
- Alerting logic
- Authentication and authorization
- Public APIs
- Configuration formats
- Existing storage and database behavior

Do not silently change:

- Metric meaning
- Units
- Thresholds
- Aggregation rules
- Update frequency
- API field semantics
- Agent protocol semantics

Any intentional behavior change must be directly related to the requested task and clearly documented in the final summary.

---

## 6. API, Configuration, and Database Compatibility

Avoid unnecessary changes to:

- Public API endpoints
- API routes
- Request and response formats
- JSON field names
- HTTP methods
- Status codes
- Authentication behavior
- Pagination behavior
- WebSocket message formats
- Agent protocol formats
- Environment variable names
- Configuration file names and formats
- Default values
- Service names
- CLI arguments
- Startup commands
- Docker behavior
- systemd behavior
- Paths used by existing deployments

If an API or configuration change is necessary:

1. Identify affected callers.
2. Preserve compatibility where practical.
3. Update all internal consumers.
4. Validate the changed behavior.
5. Document the compatibility impact.

Treat database changes as high risk.

Do not drop or destructively rewrite tables, columns, or production data unless explicitly requested.

Prefer additive and backward-compatible migrations. Preserve existing data and provide a rollback path where practical.

---

## 7. Security

Do not weaken existing security controls.

Preserve or improve:

- Authentication
- Authorization
- Input validation
- Output escaping
- CSRF protection
- XSS protection
- Secret handling
- Permission boundaries
- Token handling
- Session handling
- Rate limiting
- Path validation
- File access checks

Never hard-code or commit:

- API keys
- Tokens
- Passwords
- Private keys
- Credentials
- Production secrets

Do not print secrets in logs.

Be especially careful with:

- Theme and static asset paths
- Upload paths
- Relative paths
- Symlinks
- Temporary files
- User-controlled filenames
- Path traversal

---

## 8. Komari UI / UX Design Direction

For Komari frontend and administrative interfaces, use the **Hermes Agent dashboard** as the primary visual inspiration unless the user gives a different design direction for the current task.

The Hermes dashboard is a visual reference, not a requirement to copy Hermes branding, logos, text, product names, or exact page layout.

Preserve Komari's identity, workflows, information architecture, and monitoring purpose.

### Target visual language

Prefer:

- A dark deep-green / teal dashboard palette.
- Thin borders and subtle separators.
- Restrained contrast with clear information hierarchy.
- Rectangular cards and panels with small or minimal border radius.
- Compact, information-dense monitoring layouts.
- Strong typography hierarchy.
- Uppercase section labels where appropriate.
- Monospace or technical-style typography for metrics, IDs, versions, logs, timestamps, and system information.
- Clear left-side navigation when suitable for the existing application structure.
- Simple line-style icons.
- Dashboard cards for metrics, status, traffic, nodes, versions, runtime information, and system state.
- Consistent spacing, alignment, borders, button sizing, inputs, tables, and badges.
- Responsive behavior across desktop, tablet, and mobile.
- Accessible text and control contrast.

Avoid:

- Heavy shadows
- Strong decorative gradients
- Glassmorphism
- Excessive transparency or blur
- Decorative 3D effects
- Oversized rounded cards
- Excessive animation
- Visual noise
- Unnecessary DOM complexity

Use motion only when it improves feedback or comprehension.

### Design system first

Before broad visual changes:

1. Inspect the current frontend framework, component structure, routing, state management, styles, and build system.
2. Identify existing shared components and theme primitives.
3. Define or consolidate shared design tokens for:
   - colors
   - typography
   - spacing
   - borders
   - radii
   - control sizes
   - states
   - cards
   - tables
   - badges
   - charts
4. Refactor common layout and reusable components before duplicating page-specific styles.
5. Apply the visual language consistently across pages.

Do not create multiple slightly different versions of the same component without a clear reason.

### Visual references

When a Hermes dashboard screenshot or other reference image is provided in the current session, inspect it carefully for:

- Layout density
- Navigation structure
- Typography hierarchy
- Border treatment
- Card composition
- Spacing
- Status indicators
- Control styling
- Information density
- Overall visual atmosphere

Use the reference as design guidance rather than performing a literal pixel-for-pixel copy.

---

## 9. Frontend Refactoring Rules

Before modifying the frontend:

- Inspect the existing implementation first.
- Reuse the current architecture where practical.
- Preserve existing workflows and functionality.
- Preserve API compatibility whenever possible.
- Prefer reusable components over duplicated page-specific code.
- Keep desktop and mobile behavior intentional.
- Keep loading, empty, error, disabled, hover, and focus states consistent.
- Keep monitoring data readable and accurate.

Do not perform a full frontend rewrite unless the user explicitly requests it or the current architecture demonstrably prevents the requested result.

After meaningful frontend changes, verify at least the relevant subset of:

- Main pages load.
- Navigation works.
- Forms work.
- Buttons and dialogs work.
- Status badges render correctly.
- Monitoring data displays correctly.
- Loading, empty, and error states work.
- Desktop layout works.
- Mobile layout works.
- No major visual or functional regression was introduced.

A successful build alone is not sufficient validation for substantial UI work.

---

## 10. Backend and Dashboard Integration

Backend changes should be made only when required by the requested feature, redesigned UI, compatibility fix, security fix, or measurable backend improvement within scope.

When backend changes are necessary:

- Preserve existing API behavior whenever practical.
- Avoid breaking existing agents, clients, integrations, or stored data.
- Add or adjust APIs only when required by the requested functionality.
- Keep monitoring, node management, authentication, configuration, and administrative behavior intact unless the task explicitly changes them.
- Do not redesign backend architecture solely for visual reasons.
- Prefer incremental, testable, backward-compatible changes.

The frontend may expose existing backend information in a more structured dashboard-oriented form, including metrics, server status, node status, traffic, versions, runtime information, logs, and configuration state.

After backend changes, verify the relevant subset of:

- Service starts successfully.
- Configuration loads.
- API routes respond correctly.
- Authentication still works.
- Monitoring data is processed correctly.
- Existing storage remains compatible.
- Logging remains useful.
- Agent communication remains functional.
- No obvious new runtime errors appear.

Live restart and clean local redeployment are part of the default Komari delivery workflow after successful validation, unless the user explicitly asks not to deploy.

---

## 11. CSS, Components, and Code Cleanup

Cleanup is allowed when it directly supports the current task or is necessary to safely modify the affected code.

Look for:

- Duplicate selectors
- Conflicting styles
- Dead CSS
- Repeated inline styles
- Repeated color or spacing definitions
- Redundant responsive rules
- Unused imports
- Unused variables
- Unused functions
- Unused components
- Duplicate helpers
- Temporary debug code
- Commented-out obsolete code
- Redundant wrappers

Before deleting code, verify whether it may be used through:

- Dynamic imports
- Reflection
- Runtime configuration
- Plugins
- Build-time flags
- Conditional compilation
- Generated references
- External integrations
- Data-driven loading

Do not perform repository-wide cleanup merely because an unrelated task was requested.

Do not delete code simply because it appears unused at first glance.

---

## 12. Code Structure and Performance

Improve structure when it materially benefits the requested task.

Prefer:

- Clear naming
- Small focused functions
- Clear component responsibilities
- Clear module boundaries
- Reduced duplication
- Reduced nesting
- Predictable control flow
- Consistent error handling
- Separation of concerns
- Predictable state management

Avoid:

- Over-abstraction
- Unnecessary helper layers
- Premature architecture changes
- Large refactors without measurable benefit

For performance work, prefer simple and measurable improvements.

Potential frontend areas include:

- Unnecessary re-renders
- Repeated state updates
- Excessive polling
- Redundant API calls
- Repeated data transformation
- Unnecessary event listeners
- Excessive DOM updates
- Large unused assets
- Inefficient list rendering

Potential backend areas include:

- Repeated file I/O
- Repeated serialization
- Duplicate queries
- Redundant API calls
- Inefficient loops
- Repeated parsing
- Unnecessary allocations
- Excessive logging
- Repeated configuration loading
- Unnecessary subprocess calls

Do not sacrifice correctness, stability, readability, or maintainability for minor performance gains.

---

## 13. Dependencies, Build, and Tooling

Preserve the existing build system unless there is a strong task-specific reason to change it.

Avoid unnecessary changes to:

- Package manager
- Build scripts
- Bundler
- Compiler options
- TypeScript settings
- Lint configuration
- Formatting configuration
- Dockerfiles
- CI workflows

Before adding, removing, or upgrading a dependency:

1. Confirm that the change is necessary.
2. Check compatibility and breaking changes.
3. Prefer existing dependencies for trivial tasks.
4. Validate build and runtime behavior.

Use project-provided commands first.

Do not assume command names. Inspect the repository.

Run the relevant available checks, such as:

- lint
- test
- build
- type check
- format check
- static analysis
- unit tests
- integration tests

If automated tests are missing or incomplete, perform the best practical validation available.

---

## 14. Git Discipline

Before editing, inspect:

- `git status`
- current branch
- existing uncommitted changes
- relevant recent history when useful

Do not:

- Overwrite unrelated user changes.
- Discard user changes.
- Reset the working tree to remote state without explicit permission.
- Force-push unless explicitly requested.
- Rewrite published history unless explicitly requested.
- Commit unrelated files.

After each logical batch:

- Review `git diff`.
- Inspect modified files.
- Confirm the diff matches the intended scope.
- Run relevant validation.

When the user explicitly asks to use the **local repository as the source of truth**, preserve the local code and do not replace it with remote content merely because the remote branch or tag differs.

For normal Komari development tasks:

- Commit the completed validated change before deployment.
- Use a concise, task-focused commit message.
- Build and deploy from that committed source.
- Push the resulting commit to the configured GitHub remote only after successful post-deployment verification.
- Do not create a new tag or GitHub Release unless explicitly requested.

---

## 15. Release, Tag, and Version Discipline

When the user asks to publish the current local Komari code as a new release/tag:

1. Treat the validated local repository state as the source of truth.
2. Inspect the current branch, commit, tags, and remote configuration.
3. Determine the new version/tag requested by the user, or infer only when the project's existing versioning convention makes it unambiguous.
4. Update version metadata where required by the project.
5. Build and test the exact commit that will be tagged.
6. Create the new tag on that exact validated commit.
7. Push the intended branch/commit and the new tag to the remote repository.
8. Verify that the remote tag points to the intended commit.

Do not move or reuse an existing published tag unless explicitly requested.

Do not use `latest` as a substitute for a concrete version in version-sensitive installation instructions.

### Server and Agent version consistency

When generating Komari Agent installation commands for a released version:

- The Agent download version must match the deployed Komari server release version exactly, unless the upstream project explicitly uses a different versioning scheme.
- Prefer the concrete release tag/version.
- Do not silently use `latest`.
- Verify the actual release asset names and installation method before generating commands.

For a normal release, the expected relationship is:

```text
Source Tag == Server Version == Agent Version
```

If upstream packaging intentionally differs, explain and verify the exception rather than guessing.

---

## 16. Local Build and Clean Redeployment

For normal Komari development requests, clean local rebuild and redeployment are part of the default delivery workflow unless the user explicitly asks not to deploy.

When performing a **clean local rebuild and redeployment** of Komari:

1. Use the requested local Git commit/tag as the source of truth.
2. Inspect the current repository and build instructions.
3. Build from the exact intended source version.
4. Verify the build succeeds and identify the resulting version/artifacts.
5. Stop the existing Komari service before replacing deployed files.
6. For the established clean-redeploy workflow, remove `/opt/komari` and recreate it rather than preserving the old installation directory.
7. Do not back up the old `/opt/komari` data, configuration, or permissions unless the user explicitly asks for a backup for that deployment.
8. Deploy the newly built artifacts.
9. Recreate only the configuration, permissions, ownership, and systemd settings actually required by the current version and requested deployment.
10. Start the service and verify status, logs, runtime behavior, and reported version.
11. Generate Agent installation commands using the same concrete release version.

Never assume old permissions, service arguments, or file layout remain correct after a version change. Inspect the current project and deployment requirements first.

This clean-redeployment rule is authorized by the default delivery workflow for normal Komari development tasks unless the user explicitly disables deployment for the current task.

---

## 17. Iterative Workflows

For substantial implementation work, use logical phases appropriate to the task.

A typical UI redesign sequence is:

1. Repository inspection
2. Architecture and affected-page analysis
3. Shared theme/design-token work
4. Common layout and component refactoring
5. Page-by-page implementation
6. Backend/API adjustments only if required
7. Build and automated checks
8. Visual and functional validation
9. Diff review
10. Final summary

A typical backend task should use an equivalent task-specific sequence rather than performing unrelated UI or cleanup work.

Do not automatically run repository-wide English cleanup, dead-code cleanup, dependency cleanup, or performance refactoring unless those activities are part of the requested task.

---

## 18. Long-Running Task Progress

For genuinely long, multi-phase tasks, a temporary progress file may be useful:

`TASK_PROGRESS.md`

Create or update it only when it helps coordinate substantial work.

Suggested format:

```md
# Task Progress

## Completed

- Repository inspection
- Shared theme implementation

## In Progress

- Dashboard component refactoring

## Remaining

- Mobile validation
- Final build and tests

## Validation

- Frontend build: passed
- Backend tests: pending
```

Do not treat `TASK_PROGRESS.md` as a required artifact for small tasks.

Do not commit it unless the user requests it or the repository already intentionally tracks such progress files.

---

## 19. Final Review and Reporting

Before declaring a task complete:

1. Review the final diff.
2. Confirm only intended files changed.
3. Run the relevant available validation.
4. Check for regressions introduced by the work.
5. Confirm compatibility-sensitive behavior remains correct.
6. Confirm no secrets or accidental generated artifacts were added.

For substantial tasks, summarize:

- What changed
- Why it changed
- Important files affected
- Validation performed
- Git commit hash and commit message
- Local build result
- `/opt/komari` clean-deployment result
- Service/runtime verification result
- GitHub push result
- Any remaining limitations or follow-up work
- Release/tag/version information when relevant

## Long-Running Task Status Reporting

For long-running or multi-step tasks, always make the current execution state explicit.

Use one of these final status markers:

- `STATUS: RUNNING` — the task is still actively executing or waiting for an in-progress model/tool operation.
- `STATUS: WAITING` — execution is paused because user input, rate-limit recovery, external availability, or another dependency is required.
- `STATUS: COMPLETED` — all requested work for the current task has finished successfully.
- `STATUS: BLOCKED` — the task cannot continue without resolving a specific problem.
- `STATUS: FAILED` — execution stopped because of an unrecoverable error.

Never leave the user guessing whether work is still running.

When sending progress updates, include the current status and the next active step.

When the task finishes, always send an explicit `STATUS: COMPLETED` message even if a detailed summary was already provided.


## Safe Upgrade Architecture

Komari upgrades must be safe, data-preserving, repeatable, and rollback-capable.

Requirements:

- Separate versioned application files from persistent data.
- Upgrading Komari must preserve existing configuration, database, user data, tokens, certificates, and runtime settings.
- New frontend/theme/static assets must be updated with the new release.
- Do not blindly overwrite or delete persistent data during upgrades.
- Before every upgrade, automatically create a rollback snapshot containing:
  - current application version
  - configuration
  - database backup
  - required persistent data
  - current frontend/theme assets if needed
- Database schema changes must use explicit versioned migrations.
- Migrations must be tracked and safe against repeated execution.
- Do not assume application rollback alone is enough after a database migration.
- If the new version requires an incompatible DB migration, rollback must restore the pre-upgrade database backup.
- Use a staged deployment instead of overwriting the live installation directly.
- Prefer a release-based structure such as:

  /opt/komari/
    current -> releases/<version>
    releases/
    data/
    config/
    backups/
    upgrade/

- Build/download and validate the new version before stopping the running service.
- Upgrade sequence should be approximately:
  1. preflight checks
  2. verify disk space
  3. create backup
  4. prepare new release in staging
  5. stop service
  6. run required migrations
  7. switch release
  8. start service
  9. run health checks
  10. mark upgrade successful
- Health checks should verify service status, HTTP availability, database accessibility, expected schema version, frontend assets, and fatal startup errors.
- If any critical upgrade step or health check fails, automatically rollback to the last known-good version.
- Interrupted upgrades must be detectable and recoverable.
- Keep several recent rollback backups.
- Provide a manual rollback mechanism.
- Upgrade logic must be idempotent or explicitly state-tracked.
- Existing `/opt/komari` installations must be migrated safely to the new layout without losing data.
- Development reinstall workflows may still perform clean installs when explicitly requested, but production upgrade logic must never depend on deleting `/opt/komari`.

When implementing upgrade-related changes, first inspect the current installer, runtime layout, database migration logic, systemd service, frontend build output, and persistent storage locations. Avoid redesigning unrelated application behavior.
Do not claim tests, builds, deployments, pushes, or runtime checks succeeded unless they were actually performed.
