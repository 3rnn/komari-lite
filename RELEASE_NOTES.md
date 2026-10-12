# Komari Lite v1.0.48

## Release scope

- Panel backend, embedded administrator UI, frontend package metadata, documentation, and Agent artifacts use version `1.0.48`.
- Agent artifacts are version `1.0.48`; administrator-generated install commands use immutable `v1.0.48` URLs and never use a moving `latest` download URL.
- Linux one-click commands stage the installer privately, clean up automatically, and do not depend on positional shell expansions that browser/UI layers can rewrite.
- The release builder uses `v1.0.26`, the latest existing release tag, as its default previous tag for the Glass transition bundle.

## Publish checklist

Publish the complete release from tag `v1.0.48` after verifying its panel and Agent binaries, embedded UI, `SHA256SUMS.txt`, `Glass.zip`, `update-komari.sh`, `komari-oneclick.py`, safe-controller assets, `hermes_collector.py`, and `release-manifest.json`.

Publishing a Git tag or GitHub Release does not update installed panels or Agents. Use the pinned panel upgrade command in `README.md`; new Agent installations use the verified `v1.0.48` artifacts.
