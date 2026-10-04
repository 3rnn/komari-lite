# v1.0.16 + 33-file Glass compatibility update (production operator)

This is a **special-case** v1.0.16 → v1.0.19 data-preserving update for an
installation whose complete `data/theme/Glass` tree exactly matches the
v1.0.13–v1.0.15 stock (33 files). The standard v1.0.19 `Glass.zip` expects a
different 38-file source tree and must not be forced onto this installation.
No GitHub tag or Release is created for this helper. The v1.0.19 program and
manager come from the fixed Release. The compatibility ZIP and a narrowly
patched recovery controller come from an exact reviewed repository commit.
The script pins the SHA-256 and exact size of **every** downloaded asset.
GitHub remains the trust root: repository-supplied digests are not an
independent signature.

Do **not** use it for a fresh install, for a different existing binary, or for
unknown/customized Glass. It does not update Agents and it does not erase data.
It accepts only Linux x86_64 with Python 3.9 or newer, the standard root-owned
`/opt/komari` installation, `komari.service`, default SQLite databases and
default port 25774. It refuses unsafe writable binary ancestors, symlinks,
mounts anywhere under persistent `data/` or `config/` (including bind mounts),
service-specific private/bind mount views, custom database/listener units,
restricted legacy monitoring tables containing rows, an unfinished upgrade, wrong stock files, checksum differences, and
unsupported versions. Refusals are not invitations to bypass a guard. Do not
blindly change ownership or delete files to make a check pass. Full
service/DB/disk/theme checks run again under the transaction controller
**before service stop**. Do not add or change mounts during the maintenance
window. Review the full unit, overrides, external stores and VPS backup
independently; the script cannot verify an off-host disaster-recovery backup.
An existing unit with explicit `User=root` is supported without changing
the service account or file ownership. This preserves a legacy root-owned data
tree (including a private `Glass` directory), but does not make running the
panel as root a recommended configuration for new installations. The updater
itself already runs as root; that alone did not make the old controller accept
root-run systemd units. The dedicated compatibility controller now handles
both explicit root and non-root service users while retaining its other guards.
The bootstrap runs with umask 077, so an explicit root-run service may use a
root-private mode 0700 upgrade state directory; its root-run pre-start gate and
recovery process can read it. A non-root service still requires the original
traversable upgrade state directory; do not weaken permissions manually.

## On the production VPS (after a verified independent VPS backup)

For the Agent-installer-style one-line flow, use the **immutable GitHub commit
permalink** for `deploy/compat/upgrade.sh` from the separate operator handoff.
Do not substitute the moving `main` branch. Download the entire shell
bootstrap to `/root` before executing it (replace the placeholder with the
exact reviewed commit URL):

```sh
sudo curl -fLsS --proto '=https' --proto-redir '=https' -o /root/k19.sh https://raw.githubusercontent.com/3rnn/komari-lite/<PINNED_COMMIT>/deploy/compat/upgrade.sh && sudo bash /root/k19.sh
```

This is **one command** to copy, but unlike `curl | sudo bash`, a failed
download cannot start an incomplete shell stream. The bootstrap stages a
fixed-commit Python helper under a root-private path, verifies its exact size
and SHA-256 before execution, then starts `update`, which repeats the
environment checks and requires interactive `yes`. To run its preflight
separately, use `sudo bash /root/k19.sh check` with the **newly downloaded**
bootstrap. After the transaction, run `sudo bash /root/k19.sh status` using
that same new bootstrap (older copies lack this action). The `status` mode
never stops or restarts the panel: it reads the installed binary version,
active unit, committed journal, exact 38-file v1.0.19 stock Glass inventory,
and direct loopback version API. The hardened status check also binds the
systemd MainPID executable and listener socket to the installed v1.0.19
binary and checks SQLite integrity without updating application records. SQLite
in WAL mode may create or update its `-shm` auxiliary file even when opened with
`mode=ro`; therefore `status` must not be described as strictly filesystem
read-only. It does not stop/restart the service or replace the database. A
version string from an unrelated process on the same port must not count as a
successful rollout. It does **not** verify public HTTPS, Agents, or a restorable
off-host backup.
A GitHub-hosted shell
is still root code: trust the exact commit and review it before running. The
backend's `/agent/install.sh` installs Agents, not this panel updater, and the
existing production backend cannot serve a new updater endpoint before it has
itself been upgraded.

### Alternative: stage and verify the Python helper yourself

Obtain the exact **immutable commit permalink** from the operator's separate
handoff message, not a moving `main` URL. The reviewed script's SHA-256 is
`c2d9a50385b2584134f5391f57ba36539a2cf5ed3fb6f78d60f67d6d33229ba5`;
compare this with the independently supplied handoff hash as well. Download
that file into `/root/k19a.py` with curl/wget or transfer it from a browser.
Do not execute a root script from `/tmp` or pipe a download to a shell. Run:

```sh
sha256sum /root/k19a.py
```

Confirm that the output matches the independent hash **before executing the
script with sudo**. Without both the pinned commit URL and the hash check,
STOP. Once verified and after an off-host backup, use these separate commands:

```sh
sudo python3 /root/k19a.py check
sudo python3 /root/k19a.py update
```
`check` downloads and verifies the 33→38 stock ZIP into a private temporary
location, reads the existing version/service/SQLite/Glass, and does not alter
the panel service. `update` repeats the check, requires an interactive `yes`,
downloads the fixed v1.0.19 assets and pinned compatibility controller into a private location, verifies
hashes/size/version, and invokes `komari-manager.py` with the authenticated
custom ZIP. The transaction snapshots complete data and binary outside the
installation, installs recovery gate/timer, updates only the panel and Glass,
starts the service and verifies health; automatic rollback uses a verified
snapshot. Keep the terminal connected until the command exits. If a refusal or
rollback occurs, do not retry blindly: inspect `komari.service` and
`/var/lib/komari-upgrade/journal.json` without disclosing credentials.

## Later releases

This helper is permanently pinned to **v1.0.16 + 33-file stock Glass ->
v1.0.19**. Do **not** reuse it to update v1.0.19 to a later version, or point
it at a moving `main`/`latest` URL. Its preflight intentionally rejects an
already-upgraded binary. A later release needs its own reviewed, immutable
one-command bootstrap and complete compatible theme stock inventory. The
future release must also be tested against the root-run service layout and
root-private upgrade state if those remain in use. After that release exists,
publish its pinned GitHub commit URL and provide a new copyable download-and-
run command with separate `check`, `update`, and post-upgrade `status` modes.
There is no safe universal update command for an as-yet unpublished version;
the release assets and actual theme migration determine the command.

After success, separately verify HTTPS, public/admin pages, node connectivity,
installed theme assets and the desired off-host backup. The transaction's
snapshot is **not** a substitute for a VPS backup. No production machine is
changed by committing or testing these files in this repository.
