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

## On the production VPS (after a verified independent VPS backup)

Obtain the exact **immutable commit permalink** from the operator's separate
handoff message, not a moving `main` URL. The reviewed script's SHA-256 is
`e8e19c0970a0e9cca251abfe11fb4e3f1b2981accb1a162b3382783506c80b6f`;
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

After success, separately verify HTTPS, public/admin pages, node connectivity,
installed theme assets and the desired off-host backup. The transaction's
snapshot is **not** a substitute for a VPS backup. No production machine is
changed by committing or testing these files in this repository.
