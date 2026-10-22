# Komari Lite v1.0.62

## Security and session boundaries

- Bind pending two-factor enrollment to the authenticated account and session on the server; require the existing factor before replacement and consume enrollment atomically.
- Revalidate RPC credentials on each dispatch so revoked sessions and rotated API keys cannot retain stale WebSocket privileges. Sensitive WebSocket methods cannot reuse a handshake OTP.
- Bound JSON-RPC request size, batch count, execution duration, and serialized response size. Stop subsequent dispatch when a batch is canceled.
- Make legacy GET logout non-mutating while retaining same-origin POST logout.

## Reliability and performance

- Bound queued, in-flight, and retry metric reports under one admission budget; limit write batches and preserve accepted reports during recoverable storage outages.
- Propagate history-query cancellation, batch entity history reads, coalesce calibration cache rebuilds, and avoid unused calibration-history reads.
- Read per-node runtime state without copying every node; add compatible browser-status WebSocket heartbeat/read/write deadlines and HTTP header/body-read limits without a global stream write timeout.
- Abort dashboard requests on revocation and apply a deadline through response-body processing, retaining existing cache and account isolation.
- Preserve current AI monitoring and the compact Glass card/list interface.

## Agent and installers

- Defer GPU discovery and bound external command time/output. Bound probe concurrency with an optional local destination policy; preserve explicit private-network monitoring compatibility.
- Harden public-IP discovery with HTTPS, validated addresses, response limits, and shared time budgets.
- Add bounded installer downloads and an opt-in restricted service account without changing existing root deployments automatically.
- Make traffic-retention regression tests independent of the wall clock.

## Dependencies and release

Panel, embedded UI, and Agent artifacts are version `1.0.62`. Agent artifacts are version `1.0.62`; install the matching artifacts to obtain Agent-side changes. Build with Go 1.27.3, updated Go networking dependencies, Vite 6.4.4, and corrected source-map/tooling dependencies.

The release contains Linux panel binaries, platform Agent binaries, installers, `hermes_collector.py`, `Glass.zip`, safe-controller assets, `update-komari.sh`, manifests, and `SHA256SUMS.txt`. Verify hashes before use. Upgrades preserve databases, credentials, configuration, historical monitoring, and rollback snapshots. Historical AI pricing is unchanged. Publishing release files alone does not upgrade installed services.

## Upgrade-controller follow-up

Explicitly close SQLite health-check connections and retain path-labelled original upgrade tracebacks before rollback. This release includes the tested controller follow-up discovered during the staged rollout.
