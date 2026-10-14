# Komari Lite v1.0.60

## AI Monitoring audit corrections

- Read all overview dimensions in one bounded read-only database transaction so concurrent ingestion cannot mix range, pricing, model, task and daily totals from different snapshots. Reuse selected-range totals instead of scanning twice on the aggregate path.
- Show wholly unpriced usage as Unpriced, not $0. Mixed usage explicitly shows the priced subtotal and priced/unpriced calls. Daily detail tables omit the unavailable task dimension while retaining provider and billing mode; general usage tables retain actual tasks.
- Expose saved pricing provenance and effective time without representing operator-supplied metadata as independently verified official prices. Validate rates and exact custom model identifiers locally, disable invalid submissions, and retain server-side validation.
- Add request identifiers and server/database timing diagnostics without exposing SQL or parameters. Clarify loading, filter reset semantics, quota observation versus last success, and missing quota-refresh telemetry. Make mobile table scrolling discoverable.
- Add deterministic concurrent-ingestion regressions for raw and aggregate paths, duplicate-query checks, invalid-write preservation, cancellation, late-response/session-revocation and rendered UI regressions.

## Unified release and preservation

Panel, embedded administrator UI, package metadata, and Agent artifacts are version `1.0.60`. Agent artifacts are version `1.0.60`; installed Agents do not need updating for this Panel-only fix. No collector, gateway, credential routing, historical repricing, configuration or schema changes are included. Active request-account attribution remains unconfirmed and collection gaps remain null.

The prior quota failure's exact historical cause is not retained in telemetry. The previously observed 17.4-second request outlier was not reproduced; diagnostics improve investigation but do not establish that every source of latency is eliminated.

Known frontend dependency audit findings are tracked separately; this release does not change dependency versions.

## Assets and rollout

The release contains Linux panel binaries, 14 platform Agent binaries, installers, `hermes_collector.py`, `Glass.zip`, safe-controller assets, `update-komari.sh`, manifests, and `SHA256SUMS.txt`. Verify downloads before use. Safe updates preserve databases, credentials, configuration, history, and rollback snapshots; unknown/customized Glass trees must fail closed. Publishing does not itself upgrade installed services.
