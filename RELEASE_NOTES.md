# Komari Lite v1.0.36

## Release scope

- Panel, Agent, administrator-generated install commands, frontend package metadata, and documentation use version `1.0.36`.
- Panel and Agent installation examples use the immutable `v1.0.36` GitHub Release URLs; they do not use a moving `latest` download URL.
- The release builder uses `v1.0.26`, the latest existing release tag, as its default previous tag for the Glass transition bundle.

## Publish checklist

Publish the release from tag `v1.0.36` with the complete output of:

```bash
./scripts/build-release.sh v1.0.36
```

Verify `SHA256SUMS.txt` before publishing. The release must include the Linux panel binaries, the generic `komari` binary, all Agent artifacts and installers, `Glass.zip`, `update-komari.sh`, `komari-oneclick.py`, the safe-controller assets, `hermes_collector.py`, and `release-manifest.json`.

Publishing a Git tag or GitHub Release does not update installed panels or Agents. Use the pinned upgrade/install commands in `README.md` after release assets are available.
