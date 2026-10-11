# Komari Lite v1.0.41

## Release scope

- This is a panel-only release: panel backend, embedded administrator UI, frontend package metadata, and documentation use version `1.0.41`.
- Agent artifacts remain at `1.0.37`; administrator-generated install commands continue to use the immutable `v1.0.37` Agent release URLs and never use a moving `latest` download URL.
- The release builder uses `v1.0.26`, the latest existing release tag, as its default previous tag for the Glass transition bundle.

## Publish checklist

Publish the panel release from tag `v1.0.41` after verifying its panel binaries, embedded UI, `SHA256SUMS.txt`, `Glass.zip`, `update-komari.sh`, `komari-oneclick.py`, safe-controller assets, `hermes_collector.py`, and `release-manifest.json`.

Publishing a Git tag or GitHub Release does not update installed panels or Agents. Use the pinned panel upgrade command in `README.md`; existing Agents stay on their verified `v1.0.37` artifacts.
