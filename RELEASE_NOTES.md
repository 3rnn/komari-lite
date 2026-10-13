# Komari Lite v1.0.58

## Unified release

- Panel, embedded administrator UI, package metadata, and Agent artifacts are version `1.0.58`. Administrator-generated installers use immutable `v1.0.58` GitHub Release URLs.
- Agent artifacts are version `1.0.58`; existing installations require a separate authorized upgrade.

## AI Monitoring

- Reliable filters use independent authenticated node/model/task inventories. Node choices remain available after filtering; successful inventory refreshes reconcile deleted nodes. Same-query failures are visibly stale; changed queries clear old scoped results. Refresh preserves the selected display timezone, and session revocation clears private state.
- A compact toolbar, closed-filter summary/reset, deduplicated latest quota snapshots, aggregate API-equivalent estimates, token/call trends, expandable daily rows, and By model detail keep the main dashboard compact. Settings separates future-only pricing from read-only collection and retention policy.
- Additive interval-coverage evidence distinguishes historical gaps from current collection/report/delivery status. Hidden pages pause polling and retries use capped backoff without weakening server work limits.
- Quota eligibility skips exhausted/rate-limited credentials only while a valid reset remains in the future; boolean reset timestamps are rejected.

## Limits and preservation

Active request-account attribution remains unconfirmed: quota identity is not proof of request routing. Unproved historical gaps retain unknown causes and unverified trend values remain null. Retention editing is intentionally read-only. No retrospective repricing, collector enablement, authentication-pool changes, or dependency upgrades are included.

Known unchanged frontend dependency audit findings remain: source-map-js high-severity indexed source-map denial of service and esbuild low-severity Windows development-server file exposure. These are recorded rather than silently auto-upgraded in this release.

## Assets and rollout

The release contains Linux panel binaries, 14 platform Agent binaries, installers, `hermes_collector.py`, `Glass.zip`, safe-controller assets, `update-komari.sh`, manifests, and `SHA256SUMS.txt`. Verify downloads before use. Safe updates preserve databases, credentials, configuration, history, and rollback snapshots; unknown/customized Glass trees must fail closed. Publishing does not itself upgrade installed services.
