# Komari Lite v1.0.50

## Release scope

- Panel backend, embedded administrator UI, frontend package metadata, documentation, and Agent artifacts use version `1.0.50`.
- The AI Monitoring default dashboard is reduced to Account Quotas, Usage Overview, Usage Trend, and Daily Usage Summary. Advanced and Pricing & Retention remain collapsed.
- Usage Overview shows compact collected Tokens, collected calls, and average Tokens per call. Token totals explicitly include cache usage and do not mean generated output tokens.
- Trend coverage is now evaluated bucket-by-bucket: verified zero-activity buckets remain zero, while missing or partial collection intervals return explicit gaps and render as `null` rather than fabricated zeroes.
- Range overview retains collected partial totals with an explicit coverage label; it never upgrades partial collection into verified usage.
- Daily cards show compact Tokens, calls, proportional model bars, and verified API-equivalent estimates to two decimals. Partial pricing is labeled `Partial / Unpriced`; output token-per-second is not shown by default.
- Agent artifacts are version `1.0.50`; administrator-generated install commands use immutable `v1.0.50` URLs and never use a moving `latest` download URL.

## Publish checklist

Publish the complete release from tag `v1.0.50` after verifying its panel and Agent binaries, embedded UI, `SHA256SUMS.txt`, `Glass.zip`, `update-komari.sh`, `komari-oneclick.py`, safe-controller assets, `hermes_collector.py`, and `release-manifest.json`.

Publishing a Git tag or GitHub Release does not update installed panels or Agents. Use the pinned panel upgrade command in `README.md`; new Agent installations use the verified `v1.0.50` artifacts.
