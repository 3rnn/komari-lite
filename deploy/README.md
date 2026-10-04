# Native safe upgrade transaction (not a production deployment)

## One entry point for fresh install or existing update

`komari-manager.py` provides explicit `install` and `update` subcommands; it
does **not** guess based on the contents of `/opt/komari`. These are host-changing
commands, not executed by repository tests. Review the target host, full unit
and overrides, storage and network exposure before authorizing either one.
It supports either local, independently pinned candidate files or a *single
GitHub Release tag* downloaded over HTTPS with `curl`/`wget`. `--tag latest`
explicitly resolves the stable Release to one concrete tag, prints it, and
uses that same tag throughout; prefer an explicitly reviewed `vX.Y.Z` tag.
GitHub API SHA-256 digests check download consistency but are **not an
independent signature**; optional `--sha256` and `--theme-sha256` can pin
digests distributed through a separate trusted channel. No admin password is
created or downloaded.

### One-command GitHub flow (requires a compatible published Release)

Once a new reviewed Release publishes `komari-oneclick.py`,
`komari-manager.py`, `safe_upgrade.py`, `komari`, and (for supported existing
Glass versions) `Glass.zip`, bootstrap from the **same immutable tag**:

```sh
TAG=v1.0.19 # verify this release and its asset checksums before running
curl -fL --proto '=https' --proto-redir '=https' \
  -o /root/komari-oneclick.py \
  "https://github.com/3rnn/komari-lite/releases/download/$TAG/komari-oneclick.py"
# Or: wget --https-only -O /root/komari-oneclick.py "https://github.com/3rnn/komari-lite/releases/download/$TAG/komari-oneclick.py"
# Obtain this hash via a separate trusted channel, then authenticate the
# bootstrap script BEFORE executing it (never compute the pin from the same
# untrusted file and treat that as an independent check):
BOOTSTRAP_SHA256=INDEPENDENTLY_PINNED_64_HEX_SHA256
printf '%s  %s\n' "$BOOTSTRAP_SHA256" /root/komari-oneclick.py | sha256sum -c -
sudo python3 /root/komari-oneclick.py install --tag "$TAG"
# On an EXISTING installation, use `update` instead of `install`:
sudo python3 /root/komari-oneclick.py update --tag "$TAG"
```

The bootstrap additionally accepts independently obtained `--manager-sha256`,
`--controller-sha256`, `--sha256` (binary) and `--theme-sha256` pins and refuses
a Release whose metadata disagrees *before* running downloaded code. Without
those pins, this workflow trusts the selected GitHub repository and Release
API; checking a hash delivered through that same API only detects corruption,
not a compromised Release. The candidate binary must report the same version
as the selected tag before installation or service stop.

The bootstrap checks release API digests and exact asset URLs **before it
executes** the downloaded manager. The manager downloads the program and
stock-theme transition from that tag to a root-private external staging
directory; it verifies size/digest, then runs the same safe transaction. It
does not run GitHub's older binary-only `install.sh`, install the Agent, or
guess whether a host should be erased. An unrecognized/custom installed
Glass tree fails the stock-hash check before service stop. Cross-version
upgrades require a compatible old→new stock theme bundle for that installed
version; one bundle cannot represent every previous theme. Existing database
and systemd prerequisites still apply.

The older `v1.0.16` Release does **not** include the controller scripts or
`Glass.zip`; its binary also predates the new schema/health commands. The
one-command path must refuse that Release. The `v1.0.17` Release has a broken
rollback and startup readiness check; `v1.0.18` still races database migration.
Do not deploy either superseded release. The `v1.0.19` Release must contain
all four assets and a theme bundle whose old stock inventory matches the
installed Glass tree exactly. Neither this README nor publication of a tag
proves any host has been upgraded. Do not execute an unverified `curl|sh`
command as root.

### Local pinned artifact flow (without GitHub download)

Build a matching program/UI bundle from the reviewed source, stage a root-owned
executable on a protected path, and independently verify its pinned SHA-256.

```sh
# Only if /opt/komari and komari.service do not already exist. Fresh install
# installs a dedicated unprivileged service, its recovery timer/gate and starts
# the built-in first-run UI. The *operator* finishes admin setup in a browser.
sudo python3 deploy/komari-manager.py install \
  --binary /secure/staging/komari --sha256 PINNED_BINARY_SHA256 \
  --hostname EXPECTED_HOST --machine-id EXPECTED_MACHINE_ID

# Existing installations are preserved, NOT reinstalled. The authenticated
# Glass bundle is mandatory when data/theme/Glass already exists.
sudo python3 deploy/komari-manager.py update \
  --binary /secure/staging/komari --sha256 PINNED_BINARY_SHA256 \
  --theme-bundle /secure/staging/Glass.zip --theme-sha256 PINNED_BUNDLE_SHA256 \
  --hostname EXPECTED_HOST --machine-id EXPECTED_MACHINE_ID

python3 deploy/komari-manager.py status
sudo python3 deploy/komari-manager.py recover
# Explicit manual rollback discards all data changes after the snapshot:
sudo python3 deploy/komari-manager.py rollback
```

`install` requires an **absent** installation root, unit and upgrade state. It
never erases an existing directory and does not create an administrator password.
It stages the candidate in `releases/<version>`, prepares a writable `data/`
owned by `komari` (creating a system user if absent), writes a localhost-only
systemd unit, installs the same external recovery gate/timer, starts the panel
and checks the real `/api/install/status` and `/install` endpoints. A failed
first start stops only the new panel and leaves its data/unit intact for
diagnosis; there is no prior installation to roll back to. The operator must
configure a reverse proxy/TLS and complete the first-run administrator setup.
Do not expose the first-run UI to untrusted clients before setup. Agent
installation/enrollment is a separate, version-pinned operation.

`update` refuses an untrusted or writable installed executable/root, checks
the pinned inputs and runs the DB/theme/service/disk preflight before installing
the recovery gate. One lock covers gate installation and the complete upgrade
transaction; there is no short wrapper timeout. It delegates snapshot, DB
migration, Glass replacement, health and rollback to
`safe_upgrade.py`. It does not convert arbitrary existing systemd units to the
required service layout; preflight may reject unsupported legacy layouts or
external databases. The health URL must use the verified local listener; units
with custom listen settings must include an explicit `--listen` flag. In
particular, the current development/test install at
`/opt/komari` has a group-writable root and will be refused until reviewed and
hardened without removing the service user's access to `data/`. The separate
production VPS has not been inspected or touched.

`safe_upgrade.py` is an offline, Python 3 stdlib transaction controller. It is **not** the old binary-only `update-native.sh` (which now refuses all updates). No production host, local development service or production data is changed by repository tests. Test with `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s deploy -p 'test_*.py' -v`.

## Required deployment contract

- The existing unit must execute `/opt/komari/komari` from `WorkingDirectory=/opt/komari`, with `Restart=always`; use only the panel unit. Check the full unit and any overrides manually, including `EnvironmentFile`, before authorizing. The controller rejects explicitly configured `--database`/`--db-type` in the loaded unit and SQLite metrics settings other than the default `./data/metrics.db`; it does **not** back up external MySQL/PostgreSQL metrics or external main databases. Refuse such installs until a coordinated database backup/restore plan exists. The controller also refuses **before stopping** when legacy monitoring tables contain rows: those trigger a restricted, interactive migration mode without the normal health endpoints; complete that migration separately under a coordinated backup/retention plan first.
- The candidate executable must support `version --json` (`version`), `schema-version --json` (`schema_version` integer) and `health --json` (`ok`, `schema_version`, `expected_schema_version`). Health must inspect the configured main SQLite **read-only** and exit nonzero for missing/corrupt/incompatible schema. The **old** executable need only support `version --json` (legacy v1.0.16). Versioned, idempotent migrations run in the candidate's normal server startup after the stopped backup; the controller does not run migrations on a live DB.
- Before a privileged controller can execute the existing binary for metadata, the installation root, all its ancestors, the executable and its release directories must be **root-owned and not writable by group or others**. Existing `/opt/komari` installations using `root:komari` mode `0775` are deliberately refused by `upgrade`; installing a root-owned external gate alone does not make that layout safe. Have an operator audit where the service writes, then restrict only the installation root and binary/release path while retaining the service user's necessary write permissions under `data/`; recheck the panel before retrying. The controller does not silently change these permissions.
- Pin the candidate SHA-256 out of band. If an installed `data/theme/Glass` exists, provide a locally obtained **authenticated** ZIP bundle plus its independently verified SHA-256. ZIP contains `stock.json` (mapping of *every* relative old Glass file path to SHA-256, including manifest, HTML and chunks) and `Glass/...` for the complete new tree. The supplied hash authenticates bytes only if independently verified against a trusted release; computing a hash from an untrusted download is insufficient. Every current Glass file must exactly match `stock.json`; custom/unknown trees abort before service stop. Other theme/data files are retained as-is. A missing Glass may be initialized by the candidate on startup; if another installed theme is active, inspect the HTTP asset-check implications before deployment.
- Put the pinned executable and ZIP in an **absolute, root-owned, non-group-writable staging path** with no symlink components, and keep them unchanged throughout the transaction. The controller checks every ancestor and the input files before executing the candidate and before stopping the service. A checksum alone cannot protect a root process from a writable candidate path between verification and execution.

Generate that bundle **off the production host** from two separately reviewed stock source trees (old release and candidate release), not from the installed production theme: `python3 deploy/build-theme-bundle.py --old /trusted/old/bundledThemes/Glass --new /trusted/new/bundledThemes/Glass --output /secure/staging/Glass.zip`. Record and independently distribute the printed SHA-256 alongside the pinned candidate hash. The builder refuses symlinks, unsafe output placement and replacement of an existing bundle. An installed theme with extra generated files or user customizations will not match the complete prior-release manifest; investigate and migrate it intentionally rather than regenerating `stock.json` from the production directory.

## Manual safe-upgrade controller procedure (operator review)

```sh
# On the actual target host, after manually reviewing service, paths, storage,
# trusted candidate and theme bundle, in a maintenance window:
sudo python3 deploy/safe_upgrade.py install-gate --root /opt/komari
# Verify systemctl show -p ExecStartPre komari.service contains the gate, and
# verify this host's identity and candidate SHA-256 via an independent channel.
sudo python3 deploy/safe_upgrade.py upgrade \
  --root /opt/komari --hostname EXPECTED_HOST --machine-id EXPECTED_MACHINE_ID \
  --binary /secure/staging/komari --sha256 PINNED_BINARY_SHA256 \
  --theme-bundle /secure/staging/Glass.zip --theme-sha256 PINNED_BUNDLE_SHA256
```

`install-gate` copies the controller into root-owned `/var/lib/komari-upgrade/safe_upgrade.py`, writes a systemd drop-in with `ExecStartPre=... gate` and `OnFailure=komari-upgrade-recover.service`, installs a root-run recovery unit and an enabled one-minute recovery timer, then reloads systemd. The timer also covers a controller crash while the panel is stopped and no service restart occurs. Recovery is a no-op for committed upgrades and live coordinators; an incomplete/corrupt snapshot blocks startup rather than replacing current data. A recovered service is checked before the journal is marked rolled back. The journal and lock are in that directory; snapshots are in its root-private `backups/` subdirectory. This separation is required because existing installations may have a group-writable `/opt/komari`, allowing the panel service user to rename any child directory there. The state directory and **all its ancestors** must be root-owned and not writable by group/others; a nondefault state path requires the identical `--state-dir /trusted/path` on *every* command and in the installed gate. This is an explicit **host mutation**, to be performed only on the intended host. Do not install the gate in this checkout's local development runtime. The upgrade refuses an absent, stale, or unloaded gate or an inactive recovery timer. An interrupted transaction blocks `Restart=always` startups unless the original controller process is still alive (PID + process start time); systemd recovery then restores the verified last-good snapshot. Inspect the journal at `/var/lib/komari-upgrade/journal.json` if automatic recovery fails, and then run:

```sh
sudo python3 deploy/safe_upgrade.py recover --root /opt/komari
# Or, to revert the most recent completed upgrade (discards ALL post-snapshot
# changes to DB, data, config and theme):
sudo python3 deploy/safe_upgrade.py rollback --root /opt/komari
```

`recover` restores the last complete snapshot; if interrupted before the snapshot completion marker, it checks that the original executable is still active, keeps the unmodified old installation and starts it. If rollback itself fails (missing/corrupt backup or old service cannot pass health), startup remains blocked; the timer retries recoverable failures and otherwise an operator must investigate rather than clearing the journal or forcing the service. Do not delete backups or `data.failed.*` holding directories during incident response. Completed snapshots are retained under `/var/lib/komari-upgrade/backups/<transaction-id>`; plan secure off-host retention and capacity separately. Rollback intentionally loses all changes made after the snapshot. Secrets/config, both SQLite files including WAL/SHM while stopped, and the entire existing theme/data tree are included. Paths with symlinks or special files in the persistent tree are rejected rather than followed. Ownership and restrictive modes are preserved via filesystem copies; new stock theme files inherit the existing theme owner. The initial legacy binary is adopted into `releases/legacy-<id>/komari`; subsequent `releases/<version>/komari` are immutable by convention. Rollback recreates a `releases/recovered-<transaction-id>/komari` **from the verified snapshot**, without trusting the old release directory. `current -> releases/<version>` and `/opt/komari/komari -> current/komari` retain the unchanged systemd `ExecStart` path. The active data remains `/opt/komari/data`, never beneath `current`.

Before restoring a snapshot, the controller verifies its complete file inventory and SHA-256 manifest. This protects against a partial or corrupted backup replacing the currently running database. After a candidate starts, health verification reads the schema and SQLite integrity, both public API endpoints, the served Glass JS/CSS bytes against the installed theme, and the administrator UI's entry assets. It does not replace external monitoring or a full browser test.

Preflight checks binary hash, version/schema metadata, hostname/machine ID if provided, service gate, paths, current DB quick-check, metrics settings, full Glass stock hashes, and estimated free space. Then it stages the candidate, journals intent, stops the panel, copies data/config and old binary, writes a complete marker, switches pointers/theme, starts the new server (migrations), and verifies systemd activity, HTTP `/api/version` and `/api/public`, CLI health/schema, and Glass HTML-referenced assets. Failure restores binary/pointers, complete persistent data/config/theme, and starts the previous version. A copied release is not a deployment until all health checks pass. This does not replace external monitoring, off-host disaster recovery, or operator verification of all theme routes/certificates.
