#!/usr/bin/env python3
"""Explicit fresh-install and data-preserving update entry points for Komari.

Optional pinned GitHub Release downloads via curl/wget; no data removal or administrator credentials.
Run from a reviewed source checkout on the intended systemd host as root.
"""
import argparse
from collections import namedtuple
import fcntl
import ipaddress
import json
import os
from pathlib import Path
import pwd
import grp
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import time
from urllib.error import URLError
from urllib.request import urlopen

import safe_upgrade


class Refusal(ValueError):
    pass


def require(condition, reason):
    if not condition:
        raise Refusal(reason)


def command(argv):
    return subprocess.run([str(v) for v in argv], check=True, capture_output=True,
                          text=True, timeout=30).stdout


Release = namedtuple('Release', 'tag binary sha256 theme_bundle theme_sha256')
REPO = '3rnn/komari-lite'


def download(url, dest, tool, limit):
    """Fetch one HTTPS object using the selected external downloader."""
    require(url.startswith('https://'), 'HTTPS download required')
    executable = shutil.which(tool)
    require(tool in ('curl', 'wget') and executable, 'curl or wget is required')
    safe_upgrade.check_input_trust(Path(executable).resolve())
    if tool == 'curl':
        argv = [executable, '--fail', '--location', '--silent', '--show-error',
                '--proto', '=https', '--proto-redir', '=https', '--retry', '2',
                '--connect-timeout', '15', '--max-time', '300', '--max-filesize', str(limit),
                '--output', str(dest), url]
    else:
        argv = [executable, '--quiet', '--https-only', '--max-redirect=5',
                '--timeout=20', '--tries=2', '--output-document', str(dest), url]
    subprocess.run(argv, check=True, capture_output=True, timeout=330)
    require(dest.is_file() and 0 < dest.stat().st_size <= limit,
            'download missing or exceeds size limit')


def fetch_release(tag, tool, workdir, needs_theme, pinned_binary=None, pinned_theme=None):
    """Resolve once, validate the complete release inventory, then stage files."""
    require(tag == 'latest' or re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+(?:[.-][A-Za-z0-9.-]+)?', tag),
            'explicit version tag or latest required')
    url = ('https://api.github.com/repos/' + REPO + '/releases/' +
           ('latest' if tag == 'latest' else 'tags/' + tag))
    workdir = Path(workdir)
    meta_file = workdir/'release.json'
    download(url, meta_file, tool, 1024 * 1024)
    try:
        release = json.loads(meta_file.read_text())
    finally:
        meta_file.unlink(missing_ok=True)
    resolved = release.get('tag_name')
    require(isinstance(resolved, str) and re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+(?:[.-][A-Za-z0-9.-]+)?', resolved) and
            (tag == 'latest' or tag == resolved), 'release tag mismatch')
    require(not release.get('draft') and not release.get('prerelease'), 'draft/prerelease refused')
    assets = release.get('assets')
    require(isinstance(assets, list) and len(assets) <= 128, 'invalid release assets')
    names = [x.get('name') for x in assets if isinstance(x, dict)]
    require(len(names) == len(assets) and len(names) == len(set(names)), 'duplicate/invalid release assets')
    inventory = {x['name']: x for x in assets}
    required = ['komari'] + (['Glass.zip'] if needs_theme else [])
    for name in required:
        require(name in inventory, 'release ' + resolved + ' lacks required ' + name + ' asset')
    expected_url = 'https://github.com/' + REPO + '/releases/download/' + resolved + '/'
    validated = {}
    for name in required:
        item = inventory[name]
        match = re.fullmatch(r'sha256:([0-9a-f]{64})', str(item.get('digest', '')))
        size = item.get('size')
        require(item.get('browser_download_url') == expected_url + name, 'unexpected release asset URL')
        require(match and isinstance(size, int) and not isinstance(size, bool) and
                0 < size <= (150 * 1024 * 1024 if name == 'komari' else 100 * 1024 * 1024),
                'missing digest or invalid asset size: ' + name)
        validated[name] = (match.group(1), size)
    require(not pinned_binary or pinned_binary == validated['komari'][0], 'pinned SHA-256 differs from release binary')
    require(not pinned_theme or (needs_theme and pinned_theme == validated['Glass.zip'][0]),
            'pinned SHA-256 differs from release theme')
    for name in required:
        sha, size = validated[name]
        dest = workdir/name
        download(expected_url + name, dest, tool, size)
        require(dest.stat().st_size == size and safe_upgrade.digest(dest) == sha,
                name + ' checksum or size mismatch')
        dest.chmod(0o755 if name == 'komari' else 0o600)
    print('Selected GitHub release ' + resolved + ' (asset SHA-256 verified against GitHub API)')
    return Release(resolved, workdir/'komari', validated['komari'][0],
                   workdir/'Glass.zip' if needs_theme else None,
                   validated['Glass.zip'][0] if needs_theme else None)


def protected_dir(path):
    """Root-owned, non-writable ancestor chain; don't trust symlinks/mounts."""
    require(path.is_absolute(), 'absolute path required: ' + str(path))
    for part in (path, *path.parents):
        meta = part.lstat()
        require(stat.S_ISDIR(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
                'untrusted or writable directory: ' + str(part))


def shared_preflight(a):
    require(os.geteuid() == 0, 'install/update require root')
    require(re.fullmatch(r'[A-Za-z0-9_.@-]+\.service', a.service), 'invalid service name')
    require(re.fullmatch(r'[A-Za-z0-9_./:@-]+', a.root) and Path(a.root).is_absolute(),
            'unsafe installation path')
    require(re.fullmatch(r'[A-Za-z0-9_./:@-]+', a.state_dir) and Path(a.state_dir).is_absolute(),
            'unsafe state path')
    require(re.fullmatch(r'[A-Za-z0-9_./:@-]+', a.unit_dir) and Path(a.unit_dir).is_absolute(),
            'unsafe unit path')
    require(re.fullmatch(r'[A-Za-z0-9_./:@-]+', a.systemctl) and Path(a.systemctl).is_absolute(),
            'absolute systemctl executable required')
    safe_upgrade.check_input_trust(Path(a.systemctl))
    protected_dir(Path(a.unit_dir))
    # A state path inside the installation can be renamed by the panel user;
    # prohibit this even when an empty, root-owned installation is available.
    require(not Path(a.state_dir).is_relative_to(Path(a.root)), 'state must be outside installation')
    require(a.sha256 and re.fullmatch(r'[0-9a-f]{64}', a.sha256), 'pinned SHA-256 required')
    require(a.binary is not None, 'candidate binary required')
    binary = Path(a.binary)
    safe_upgrade.check_input_trust(binary)
    require(os.access(binary, os.X_OK) and safe_upgrade.digest(binary) == a.sha256,
            'candidate SHA-256 or executable mismatch')
    safe_upgrade.check_host(a)


def service_unit(a, username):
    # No EnvironmentFile: an operator must review/add custom settings separately.
    return ('[Unit]\nDescription=Komari panel\nAfter=network-online.target\n'
            'Wants=network-online.target\n[Service]\nType=simple\n'
            'User=' + username + '\nGroup=' + grp.getgrgid(pwd.getpwnam(username).pw_gid).gr_name + '\n'
            'WorkingDirectory=' + a.root + '\n'
            'ExecStart=' + a.root + '/komari server --listen ' + a.listen + '\n'
            'Restart=always\nRestartSec=3s\n'
            'NoNewPrivileges=yes\nProtectSystem=strict\n'
            'ReadWritePaths=' + a.root + '/data ' + a.root + '/backup\n'
            '[Install]\nWantedBy=multi-user.target\n')


def read_http(url):
    with urlopen(url, timeout=2) as response:
        return response.status, response.read(64 * 1024)


def check_fresh_unit(a, account):
    """Check systemd's loaded unit, not only the file we just wrote."""
    output = command([a.systemctl, 'show', '-p', 'ExecStart', '-p', 'WorkingDirectory',
                      '-p', 'User', '-p', 'Group', '-p', 'Restart', a.service])
    fields = dict(row.split('=', 1) for row in output.splitlines() if '=' in row)
    expected = (r'\bpath=' + re.escape(a.root + '/komari') + r'(?:\s|;)')
    require(re.search(expected, fields.get('ExecStart', '')) and
            ('server --listen ' + a.listen) in fields.get('ExecStart', '') and
            fields.get('WorkingDirectory') == a.root and
            fields.get('User') == account.pw_name and
            fields.get('Group') == grp.getgrgid(account.pw_gid).gr_name and
            fields.get('Restart') == 'always', 'effective service does not match fresh installation')


def check_first_run(a):
    """Do not mark a fresh install healthy unless its *installer* is available."""
    last = 'first-run installer did not become ready'
    for _ in range(30):
        try:
            command([a.systemctl, 'is-active', '--quiet', a.service])
            code, body = read_http(a.http_base + '/api/install/status')
            if code == 200:
                result = json.loads(body)
                state = result.get('data', {})
                require(state.get('state') == 'ready' and state.get('required') is True,
                        'first-run installer is not ready (already initialized or wrong listener)')
                page_code, page = read_http(a.http_base + '/install')
                require(page_code == 200 and b'<html' in page.lower(),
                        'first-run installer page unavailable')
                return
        except (URLError, subprocess.CalledProcessError, subprocess.TimeoutExpired,
                json.JSONDecodeError, KeyError, ConnectionError) as exc:
            last = type(exc).__name__
        time.sleep(1)
    raise Refusal(last)


def install(a):
    shared_preflight(a)
    require(not a.database or a.database == './data/komari.db',
            'fresh install supports only the default main SQLite database')
    root = Path(a.root)
    require(root != Path('/') and not Path(a.unit_dir).is_relative_to(root), 'invalid installation root')
    require(not root.exists() and not root.is_symlink(), 'installation directory already exists; install requires an empty, absent root')
    protected_dir(root.parent)
    protected_dir(Path(a.unit_dir))
    state = Path(a.state_dir)
    require(not state.exists() and not state.is_symlink(), 'upgrade state already exists; refusing to reuse it for fresh install')
    protected_dir(state.parent)
    unit = Path(a.unit_dir) / a.service
    require(not unit.exists() and not unit.is_symlink(), 'panel service unit already exists')
    require(not (Path(a.unit_dir)/(a.service + '.d')).exists(), 'panel service override already exists')
    loaded = command([a.systemctl, 'show', '-p', 'LoadState', '-p', 'ActiveState',
                      '-p', 'FragmentPath', a.service])
    fields = dict(row.split('=', 1) for row in loaded.splitlines() if '=' in row)
    require(fields.get('LoadState') in ('not-found', None) and
            fields.get('ActiveState') in ('inactive', None) and
            not fields.get('FragmentPath'), 'panel service already loaded or active')
    require(not safe_upgrade.gate_file(a).exists() and
            not safe_upgrade.recovery_unit_file(a).exists() and
            not safe_upgrade.recovery_timer_file(a).exists(), 'recovery/gate unit already exists')
    require(re.fullmatch(r'[A-Za-z_][A-Za-z0-9_-]*', a.service_user) and a.service_user != 'root',
            'invalid service user')
    host, sep, port = a.listen.rpartition(':')
    require(sep and port.isdecimal() and 0 < int(port) <= 65535 and
            ipaddress.ip_address(host).is_loopback, 'fresh install must listen on an explicit loopback IPv4 address and port')
    require(':' not in host and a.http_base == 'http://' + a.listen,
            'first-run health URL must match the listen address')
    # Probe metadata without creating the target directory or DB. The binary
    # is trusted by both path ownership and an independently pinned digest.
    version = safe_upgrade.info(Path(a.binary), 'version', root.parent)['version']
    schema = safe_upgrade.info(Path(a.binary), 'schema-version', root.parent)['schema_version']
    require(isinstance(version, str) and re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]*', version) and
            isinstance(schema, int) and not isinstance(schema, bool) and schema > 0,
            'candidate lacks safe upgrade metadata')
    if getattr(a, 'expected_version', None):
        require(version == a.expected_version, 'candidate version mismatch with GitHub release tag')
    safe_upgrade.check_input_trust(Path(a.binary))
    require(safe_upgrade.digest(Path(a.binary)) == a.sha256, 'candidate SHA-256 changed')
    try:
        account = pwd.getpwnam(a.service_user)
    except KeyError:
        command(['useradd', '--system', '--user-group', '--home-dir', '/nonexistent',
                 '--shell', '/usr/sbin/nologin', a.service_user])
        account = pwd.getpwnam(a.service_user)
    require(account.pw_uid != 0, 'service must run unprivileged')
    require(not root.exists(), 'installation destination changed')
    root.mkdir(mode=0o755)
    root.chmod(0o755)
    releases = root / 'releases'
    releases.mkdir(mode=0o755)
    release = releases / version
    release.mkdir(mode=0o755)
    staged = release / 'komari'
    shutil.copyfile(a.binary, staged)
    staged.chmod(0o755)
    require(safe_upgrade.digest(staged) == a.sha256, 'staged executable changed')
    (root/'current').symlink_to(Path('releases') / version)
    (root/'komari').symlink_to(Path('current') / 'komari')
    data = root/'data'
    data.mkdir(mode=0o750)
    os.chown(data, account.pw_uid, account.pw_gid)
    # The backend still writes its own best-effort version archives under
    # ./backup; keep that tree private and writable without relaxing root.
    legacy_backups = root/'backup'
    legacy_backups.mkdir(mode=0o700)
    os.chown(legacy_backups, account.pw_uid, account.pw_gid)
    safe_upgrade.atomic_bytes(unit, service_unit(a, account.pw_name).encode())
    safe_upgrade.install_gate(a)
    try:
        command([a.systemctl, 'daemon-reload'])
        safe_upgrade.check_gate(a)
        check_fresh_unit(a, account)
        command([a.systemctl, 'enable', '--now', a.service])
        check_first_run(a)
    except BaseException:
        # Never remove a newly created database, setup state, binary or unit.
        # An operator can diagnose and resume without losing first-run data.
        try:
            command([a.systemctl, 'stop', a.service])
        except Exception:
            pass
        raise
    print('Fresh install ready. Finish administrator setup at ' + a.http_base + '/install')


def update(a):
    shared_preflight(a)
    root = Path(a.root)
    require(root.is_dir() and not root.is_symlink(), 'existing installation required for update')
    safe_upgrade.check_executable_trust(root)
    require((root/'data').is_dir(), 'persistent data missing')
    require(bool(a.theme_bundle) == bool(a.theme_sha256), 'theme bundle and SHA-256 must be supplied together')
    if (root/'data'/'theme'/'Glass').exists():
        require(a.theme_bundle, 'installed Glass requires an authenticated theme bundle')
    if a.theme_bundle:
        safe_upgrade.check_input_trust(Path(a.theme_bundle))
        require(re.fullmatch(r'[0-9a-f]{64}', a.theme_sha256) and
                safe_upgrade.digest(Path(a.theme_bundle)) == a.theme_sha256,
                'theme bundle SHA-256 mismatch')
    # A health check against a foreign host/port must not commit a transaction.
    # Explicit --listen is required for non-default listeners on existing units.
    require(a.http_base.startswith('http://127.0.0.1:') and
            re.fullmatch(r'http://127\.0\.0\.1:[1-9][0-9]{0,4}', a.http_base) and
            0 < int(a.http_base.rsplit(':', 1)[1]) <= 65535,
            'update health target must be a loopback HTTP listener')
    effective = command([a.systemctl, 'show', '-p', 'ExecStart', '-p', 'Environment',
                         '-p', 'EnvironmentFiles', a.service])
    fields = dict(row.split('=', 1) for row in effective.splitlines() if '=' in row)
    start = fields.get('ExecStart', '')
    configured = re.search(r'--listen(?:=|\s+)([^\s;]+)', start)
    if configured:
        require(configured.group(1) in ('127.0.0.1:' + a.http_base.rsplit(':', 1)[1],
                                       '0.0.0.0:' + a.http_base.rsplit(':', 1)[1]),
                'health port differs from the effective service listener')
    else:
        require(not re.search(r'KOMARI_LISTEN\s*=', fields.get('Environment', '')) and
                fields.get('EnvironmentFiles', '') in ('', 'n/a') and
                a.http_base == 'http://127.0.0.1:25774',
                'unverified listener: use a service with explicit --listen')
    # Take the very same lock used by safe_upgrade's CLI, through preflight,
    # gate installation and the whole synchronous transaction. No 30s timeout.
    state = safe_upgrade.state_dir(a, create=True)
    with (state/'lock').open('a+') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        safe_upgrade.preflight(a, require_gate=False)
        safe_upgrade.install_gate(a)
        safe_upgrade.upgrade(a)
    print('Update committed; snapshot retained in ' + a.state_dir + '/backups')


def status(a):
    root = Path(a.root)
    state = Path(a.state_dir)
    phase = 'none'
    if state.exists():
        phase = (safe_upgrade.journal(safe_upgrade.state_dir(a)) or {}).get('phase', 'none')
    active = subprocess.run([a.systemctl, 'is-active', '--quiet', a.service],
                            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                            timeout=10).returncode == 0
    print(json.dumps({'installed': root.is_dir() and (root/'komari').is_file(),
                      'service_active': active, 'upgrade_phase': phase}, sort_keys=True))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('install', 'update', 'status', 'recover', 'rollback'))
    parser.add_argument('--root', default='/opt/komari')
    parser.add_argument('--state-dir', default='/var/lib/komari-upgrade')
    parser.add_argument('--unit-dir', default='/etc/systemd/system')
    parser.add_argument('--service', default='komari.service')
    parser.add_argument('--systemctl', default='/usr/bin/systemctl')
    parser.add_argument('--binary')
    parser.add_argument('--tag', help='GitHub Release tag, or explicit latest (resolved once)')
    parser.add_argument('--downloader', choices=('auto', 'curl', 'wget'), default='auto')
    parser.add_argument('--download-dir', default='/var/lib/komari-downloads')
    parser.add_argument('--sha256')
    parser.add_argument('--theme-bundle')
    parser.add_argument('--theme-sha256')
    parser.add_argument('--database', help='Only the default ./data/komari.db is supported for update')
    parser.add_argument('--listen', default='127.0.0.1:25774')
    parser.add_argument('--http-base', default='http://127.0.0.1:25774')
    parser.add_argument('--service-user', default='komari')
    parser.add_argument('--hostname')
    parser.add_argument('--machine-id')
    a = parser.parse_args()
    try:
        if a.action in ('install', 'update') and a.tag:
            require(not a.binary and not a.theme_bundle,
                    '--tag cannot be combined with local --binary or --theme-bundle')
            require(os.geteuid() == 0, 'GitHub release installation requires root')
            parent = Path(a.download_dir)
            require(parent.is_absolute() and not parent.is_relative_to(Path(a.root)) and
                    not parent.is_relative_to(Path(a.state_dir)) and
                    re.fullmatch(r'[A-Za-z0-9_./:@-]+', str(parent)),
                    'unsafe external download directory')
            protected_dir(parent.parent)
            if not parent.exists() and not parent.is_symlink():
                parent.mkdir(mode=0o700)
            protected_dir(parent)
            tool = ('curl' if shutil.which('curl') else 'wget') if a.downloader == 'auto' else a.downloader
            require(shutil.which(tool), 'curl or wget required for GitHub release download')
            with tempfile.TemporaryDirectory(prefix='release-', dir=parent) as staging:
                needs_theme = a.action == 'update' and (Path(a.root)/'data'/'theme'/'Glass').exists()
                release = fetch_release(a.tag, tool, staging, needs_theme,
                                        pinned_binary=a.sha256, pinned_theme=a.theme_sha256)
                a.binary = str(release.binary)
                a.sha256 = release.sha256
                a.theme_bundle = str(release.theme_bundle) if release.theme_bundle else None
                a.theme_sha256 = release.theme_sha256
                a.expected_version = release.tag.removeprefix('v')
                (install if a.action == 'install' else update)(a)
        elif a.action == 'install':
            install(a)
        elif a.action == 'update':
            update(a)
        elif a.action == 'status':
            require(not a.tag, '--tag applies only to install/update')
            status(a)
        else:
            require(not a.tag, '--tag applies only to install/update')
            require(os.geteuid() == 0, 'recovery/rollback require root')
            command([sys.executable, str(Path(safe_upgrade.__file__).resolve()), a.action,
                     '--root', a.root, '--state-dir', a.state_dir,
                     '--systemctl', a.systemctl, '--service', a.service,
                     '--http-base', a.http_base])
    except (Exception, KeyboardInterrupt) as exc:
        print('komari-manager: ' + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
