# Komari Lite v1.0.59

## AI Monitoring overview correction

- Fix intermittent HTTP 422 on a full rolling 24H overview with more than 50,000 detailed samples. A non-minute-aligned day intersects 1,441 minute buckets, not 1,440; the database aggregation guard rejected the valid final edge bucket.
- Keep bounded SQL results and overflow detection, including both partial edge buckets without rounding or shortening the requested interval. Dimension, retention, authentication, rate-limit and query-timeout controls are unchanged.
- Add an HTTP bridge regression using real SQL aggregation above the raw-row threshold, frontend query encoding, exact totals and null collection-gap semantics. Both fixed boundaries and the default 24H preset are exercised.

## Unified release and preservation

Panel, embedded administrator UI, package metadata, and Agent artifacts are version `1.0.59`. Agent artifacts are version `1.0.59`; installed Agents do not need updating for this Panel-only fix. No collector, gateway, credential routing, historical data, pricing, configuration or schema changes are included. Active request-account attribution remains unconfirmed and collection gaps remain null.

Known unchanged frontend dependency audit findings remain: source-map-js high-severity indexed source-map denial of service and esbuild low-severity Windows development-server file exposure. No dependency upgrades are included.

## Assets and rollout

The release contains Linux panel binaries, 14 platform Agent binaries, installers, `hermes_collector.py`, `Glass.zip`, safe-controller assets, `update-komari.sh`, manifests, and `SHA256SUMS.txt`. Verify downloads before use. Safe updates preserve databases, credentials, configuration, history, and rollback snapshots; unknown/customized Glass trees must fail closed. Publishing does not itself upgrade installed services.
