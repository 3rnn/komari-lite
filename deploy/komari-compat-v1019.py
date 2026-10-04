#!/usr/bin/env python3
"""Pinned v1.0.16 + 33-file Glass -> v1.0.19 production update.

Run from a root-owned, non-writable path on the intended VPS. Default: check.
The official v1.0.19 release's 38-file stock bundle is NOT used. This script
fetches an exact 33-file old-stock compatibility bundle from this repository.
"""
import argparse
from contextlib import closing
import hashlib
from http.client import HTTPConnection, HTTPException
import json
import os
from pathlib import Path
import platform
import re
import shutil
import socket
import sqlite3
import stat
import subprocess
import sys
import tempfile
import zipfile

TAG = 'v1.0.19'
OLD_VERSION = '1.0.16'
ROOT = Path('/opt/komari')
STATE = Path('/var/lib/komari-upgrade')
BUNDLE = 'Glass-33-to-38-v1.0.19.zip'
# SHA-256 of canonical {relative_path: sha256(file_bytes)} for the 38-file
# target in the pinned compatibility bundle. No network needed for status.
TARGET_THEME_INVENTORY_SHA = 'ba2e823004b4aa03918e921de0de4278eb245b956e1c1f5e439a0a40788885ff'
RELEASE = 'https://github.com/3rnn/komari-lite/releases/download/' + TAG + '/'
COMPAT_SOURCE_COMMIT = '0216e11e2b120abde41c98b7887ee9c5fd209db3'
COMPAT_SOURCE = ('https://raw.githubusercontent.com/3rnn/komari-lite/' +
                 COMPAT_SOURCE_COMMIT + '/deploy/compat/')
CONTROLLER_SOURCE = ('https://raw.githubusercontent.com/3rnn/komari-lite/' +
                     '41fff50fd0926d4b2cd7b035816bcbb83f688fa0/deploy/compat/')
ASSETS = {
    'komari': ('68d1c5cd8f879e152269da36920cfecba677e8145e83ad774bd385d2ec1dff21', 29197640, RELEASE + 'komari'),
    'komari-manager.py': ('92a492be21753db126a14123304be7794787b864f039172d47e8c3ea2eb3c0fa', 22020, RELEASE + 'komari-manager.py'),
    'safe_upgrade.py': ('6db6232b7f01cb641c8a0bda842c7467d7374b59ccdaa163d45456b6acdebb92', 37606,
                        CONTROLLER_SOURCE + 'safe_upgrade.py'),
    BUNDLE: ('71cc410af2b997c3e9dde065cdd35df6dd49cc37519c2dd481cc970c9090b4c6', 1597320,
             COMPAT_SOURCE + BUNDLE),
}


def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def sha256(path):
    digest = hashlib.sha256()
    with Path(path).open('rb') as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b''):
            digest.update(chunk)
    return digest.hexdigest()


def trusted_file(path):
    path = Path(path)
    require(path.is_absolute() and path.resolve(strict=True) == path, 'file must be an absolute non-symlink path: ' + str(path))
    for directory in path.parents:
        meta = directory.lstat()
        require(stat.S_ISDIR(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
                'writable or non-root-owned ancestor: ' + str(directory))
    meta = path.lstat()
    require(stat.S_ISREG(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
            'file is not root-owned, regular and protected: ' + str(path))
    return str(path)


def trusted_script():
    trusted_file(Path(__file__).absolute())


def trusted_downloader():
    for name in ('curl', 'wget'):
        path = shutil.which(name)
        if path:
            return name, trusted_file(Path(path).resolve(strict=True))
    raise ValueError('curl or wget is required')


def download_verified(name, directory):
    expected, size, url = ASSETS[name]
    tool, executable = trusted_downloader()
    output = Path(directory)/name
    if tool == 'curl':
        argv = [executable, '--fail', '--location', '--silent', '--show-error',
                '--proto', '=https', '--proto-redir', '=https', '--connect-timeout', '15',
                '--max-time', '600', '--max-filesize', str(size), '--output', str(output), url]
    else:
        argv = [executable, '--quiet', '--https-only', '--max-redirect=5',
                '--timeout=25', '--tries=2', '--quota=32m',
                '--output-document', str(output), url]
    subprocess.run(argv, check=True, capture_output=True, timeout=630)
    require(output.is_file() and output.stat().st_size == size and sha256(output) == expected,
            name + ' size/SHA-256 mismatch; refused')
    # The controller copies this mode into the release. The panel runs as a
    # non-root user, so 0700 would make the new service fail to start.
    output.chmod(0o755 if name == 'komari' else 0o600)
    return output


def compare_stock(theme, bundle):
    theme = Path(theme)
    require(theme.is_dir() and not theme.is_symlink(), 'Glass directory missing or linked')
    with zipfile.ZipFile(bundle) as archive:
        info = archive.getinfo('stock.json')
        require(0 < info.file_size < 1024 * 1024, 'invalid stock manifest size')
        expected = json.loads(archive.read('stock.json'))
    require(isinstance(expected, dict) and len(expected) == 33 and all(
        isinstance(name, str) and name and not name.startswith('/') and
        all(part not in ('', '.', '..') for part in name.split('/')) and
        isinstance(value, str) and re.fullmatch('[0-9a-f]{64}', value)
        for name, value in expected.items()), 'invalid 33-file old-stock manifest')
    current = {}
    for base, dirs, files in os.walk(theme, followlinks=False):
        for name in dirs:
            require(not (Path(base)/name).is_symlink(), 'Glass has a symlink directory')
        for name in files:
            path = Path(base)/name
            require(stat.S_ISREG(path.lstat().st_mode), 'Glass has symlink or special file')
            current[path.relative_to(theme).as_posix()] = sha256(path)
    return current == expected, len(current)

def check_target_theme(theme):
    theme = Path(theme)
    require(theme.is_dir() and not theme.is_symlink(), 'Glass directory missing or linked')
    current = {}
    for base, dirs, files in os.walk(theme, followlinks=False):
        for name in dirs:
            require(not (Path(base)/name).is_symlink(), 'Glass contains linked directory')
        for name in files:
            path = Path(base)/name
            require(stat.S_ISREG(path.lstat().st_mode), 'Glass contains linked/special file')
            current[path.relative_to(theme).as_posix()] = sha256(path)
            require(len(current) <= 38, 'Glass has extra files')
    canonical = json.dumps(current, sort_keys=True, separators=(',', ':')).encode()
    require(len(current) == 38 and hashlib.sha256(canonical).hexdigest() == TARGET_THEME_INVENTORY_SHA,
            'Glass is not exact v1.0.19 stock')

def panel_owns_listener(pid, port=25774):
    """Tie the loopback HTTP check to the systemd MainPID, not another server."""
    sockets = set()
    for fd in (Path('/proc')/str(pid)/'fd').iterdir():
        try:
            match = re.fullmatch(r'socket:\[(\d+)\]', os.readlink(fd))
            if match: sockets.add(match.group(1))
        except OSError:  # descriptors can close while we inspect them
            continue
    if not sockets: return False
    for table in ('tcp', 'tcp6'):
        for line in (Path('/proc/net')/table).read_text().splitlines()[1:]:
            fields = line.split()
            if len(fields) < 10 or fields[3] != '0A' or fields[9] not in sockets:
                continue
            address, hex_port = fields[1].split(':', 1)
            allowed = (('00000000', '0100007F') if table == 'tcp' else
                       ('00000000000000000000000000000000', '00000000000000000000000001000000'))
            if address in allowed and int(hex_port, 16) == port:
                return True
    return False

def active_panel_identity(binary):
    result = subprocess.run([trusted_file(Path('/usr/bin/systemctl')), 'show',
                             '-p', 'LoadState', '-p', 'ActiveState', '-p', 'WorkingDirectory',
                             '-p', 'User', '-p', 'MainPID', '-p', 'ExecStart',
                             '-p', 'Environment', '-p', 'EnvironmentFiles', 'komari.service'],
                            check=True, capture_output=True, text=True, timeout=20)
    fields = dict(row.split('=', 1) for row in result.stdout.splitlines() if '=' in row)
    require(fields.get('LoadState') == 'loaded' and fields.get('ActiveState') == 'active' and
            fields.get('WorkingDirectory') == str(ROOT) and bool(fields.get('User')),
            'panel unit is not active in the expected layout')
    require(re.search(r'\bpath=/opt/komari/komari(?:\s|;)', fields.get('ExecStart', '')),
            'unexpected service executable')
    require(not re.search(r'(?<![\w-])(?:--(?:database|db-type)(?:=|\s)|-[dt])',
                          fields.get('ExecStart', '')) and
            fields.get('EnvironmentFiles', '') in ('', 'n/a') and
            not re.search(r'KOMARI_LISTEN\s*=', fields.get('Environment', '')),
            'custom database or service environment needs review')
    raw_pid = fields.get('MainPID', '')
    require(raw_pid.isdecimal() and int(raw_pid) > 1, 'panel MainPID is unavailable')
    pid = int(raw_pid)
    require(os.path.samefile(Path('/proc')/str(pid)/'exe', binary),
            'running executable is not the installed v1.0.19 binary')
    stat_line = (Path('/proc')/str(pid)/'stat').read_text()
    require(') ' in stat_line, 'panel process identity is unavailable')
    parts = stat_line.rsplit(') ', 1)[1].split()
    require(len(parts) > 19, 'panel process identity is unavailable')
    start = parts[19]
    require(panel_owns_listener(pid), 'panel process does not own the loopback HTTP listener')
    return pid, start

def check_http_version_payload(payload):
    result = json.loads(payload)
    require(isinstance(result, dict) and isinstance(result.get('data'), dict) and
            result['data'].get('version') == '1.0.19',
            'local HTTP version differs from installed binary')

def check_status_databases(root):
    for name in ('komari.db', 'metrics.db'):
        path = root/'data'/name
        require((name != 'komari.db' or path.is_file()) and not path.is_symlink(),
                'missing or linked SQLite database: ' + name)
        if not path.exists(): continue  # metrics.db is optional
        require(path.is_file(), 'invalid SQLite database: ' + name)
        with closing(sqlite3.connect('file:' + str(path) + '?mode=ro', uri=True)) as db:
            require(db.execute('pragma quick_check').fetchone()[0] == 'ok',
                    'SQLite integrity failed: ' + name)
            if name == 'komari.db':
                columns = {row[1] for row in db.execute('pragma table_info(configs)')}
                if {'key', 'value'} <= columns:
                    rows = dict(db.execute("select key, value from configs where key in ('metric_db_driver','metric_db_dsn')"))
                else:
                    require({'id', 'sitename'} <= columns, 'unrecognized config schema')
                    rows = {}
                driver = json.loads(rows.get('metric_db_driver', '"sqlite"'))
                dsn = json.loads(rows.get('metric_db_dsn', '"./data/metrics.db"'))
                require(driver == 'sqlite' and dsn == './data/metrics.db',
                        'external/custom metrics database is not covered by this status check')

def verify_status():
    require(os.geteuid() == 0, 'status requires root to inspect the upgrade journal')
    trusted_script()
    binary = (ROOT/'komari').resolve(strict=True)
    require(binary.is_relative_to(ROOT), 'installed binary resolves outside panel root')
    trusted_file(binary)
    installed = subprocess.run([str(binary), 'version', '--json'], cwd=ROOT,
                               capture_output=True, text=True, check=True, timeout=20)
    require(json.loads(installed.stdout).get('version') == '1.0.19',
            'panel binary is not v1.0.19')
    identity = active_panel_identity(binary)
    require(STATE.is_dir() and not STATE.is_symlink() and
            STATE.stat().st_uid == 0 and not STATE.stat().st_mode & 0o022,
            'upgrade state directory missing or untrusted')
    journal = STATE/'journal.json'
    require(journal.is_file() and not journal.is_symlink() and journal.stat().st_uid == 0 and
            not journal.stat().st_mode & 0o022, 'upgrade journal missing or untrusted')
    trusted_file(journal)
    record = json.loads(journal.read_text())
    require(isinstance(record, dict) and record.get('phase') == 'committed' and
            record.get('target') == '1.0.19',
            'v1.0.19 transaction is not committed')
    require((ROOT/'data').is_dir() and not (ROOT/'data').is_symlink() and
            (ROOT/'data/theme').is_dir() and not (ROOT/'data/theme').is_symlink(),
            'persistent theme parent is missing or linked')
    check_status_databases(ROOT)
    check_target_theme(ROOT/'data/theme/Glass')
    connection = HTTPConnection('127.0.0.1', 25774, timeout=5)
    try:
        connection.request('GET', '/api/version')
        response = connection.getresponse()
        require(response.status == 200, 'local HTTP version endpoint failed')
        payload = response.read(64 * 1024 + 1)
        require(len(payload) <= 64 * 1024, 'local HTTP version response is oversized')
        check_http_version_payload(payload)
    finally:
        connection.close()
    require(active_panel_identity(binary) == identity,
            'panel restarted during status verification')
    print('Verified: panel v1.0.19, active service and running binary, committed transaction, '
          'SQLite integrity, 38-file stock Glass and panel-owned loopback HTTP. '
          'Agent and public HTTPS not checked.', flush=True)


def read_only_database_check():
    database = ROOT/'data/komari.db'
    require(database.is_file() and not database.is_symlink(), 'default SQLite database missing')
    with sqlite3.connect('file:' + str(database) + '?mode=ro', uri=True) as db:
        require(db.execute('pragma quick_check').fetchone()[0] == 'ok', 'main SQLite quick_check failed')
        columns = {row[1] for row in db.execute('pragma table_info(configs)')}
        if {'key', 'value'} <= columns:
            rows = dict(db.execute("select key, value from configs where key in ('metric_db_driver','metric_db_dsn')"))
        else:
            require({'id', 'sitename'} <= columns, 'unknown database config schema')
            rows = {}
        require(json.loads(rows.get('metric_db_driver', '"sqlite"')) == 'sqlite' and
                json.loads(rows.get('metric_db_dsn', '"./data/metrics.db"')) == './data/metrics.db',
                'custom/external metrics storage requires a separate plan')
        for table in ('records', 'records_long_term', 'gpu_records', 'ping_records'):
            if db.execute('select 1 from sqlite_master where type=? and name=?', ('table', table)).fetchone():
                require(db.execute('select 1 from "' + table + '" limit 1').fetchone() is None,
                        'legacy monitoring rows require a separate migration before upgrade')
    metrics = ROOT/'data/metrics.db'
    if metrics.exists():
        require(metrics.is_file() and not metrics.is_symlink(), 'metrics SQLite is invalid')
        with sqlite3.connect('file:' + str(metrics) + '?mode=ro', uri=True) as db:
            require(db.execute('pragma quick_check').fetchone()[0] == 'ok', 'metrics SQLite quick_check failed')


def check_persistent_mounts(root, mountinfo=Path('/proc/self/mountinfo')):
    """Refuse nested mounts, including bind mounts on the same filesystem."""
    targets = (Path(root)/'data', Path(root)/'config')
    with Path(mountinfo).open('r', encoding='utf-8', errors='surrogateescape') as source:
        for line in source:
            fields = line.split()
            require(len(fields) >= 10 and fields[2].count(':') == 1,
                    'cannot safely inspect mount table')
            mount_path = re.sub(r'\\([0-7]{3})',
                                lambda m: chr(int(m.group(1), 8)), fields[4])
            mounted = Path(mount_path)
            require(not any(mounted == path or mounted.is_relative_to(path)
                            for path in targets),
                    'nested/persistent data mount needs a separate backup plan')


def check_unit_mount_view(fields):
    """Refuse units that can see different persistent paths than the snapshot."""
    for property_name in ('RootDirectory', 'RootImage', 'BindPaths',
                          'BindReadOnlyPaths', 'TemporaryFileSystem',
                          'JoinsNamespaceOf', 'ExtensionImages',
                          'ExtensionDirectories'):
        require(not fields.get(property_name),
                'custom systemd mount namespace requires separate review')
    require(fields.get('PrivateMounts', 'no') in ('no', 'false', '0', ''),
            'private systemd mount namespace requires separate review')

def check_standard_unit_layout(fields):
    # Production installations can explicitly run their existing panel as
    # User=root. This root-run compatibility path does not change that unit or
    # migrate ownership. An omitted User= is not an explicit reviewed layout.
    require(fields.get('LoadState') == 'loaded' and fields.get('ActiveState') == 'active' and
            fields.get('WorkingDirectory') == str(ROOT) and
            bool(fields.get('User')) and fields.get('Restart') == 'always' and
            re.search(r'\bpath=/opt/komari/komari(?:\s|;)', fields.get('ExecStart', '')),
            'unsupported unit layout or inactive service')
    require(not re.search(r'(?<![\w-])(?:--(?:database|db-type)(?:=|\s)|-[dt])',
                          fields.get('ExecStart', '')),
            'custom main database/driver in unit is unsupported')


def host_check():
    require(sys.version_info >= (3, 9), 'Python 3.9 or newer required')
    require(os.geteuid() == 0, 'run with sudo/root so protected database can be inspected')
    trusted_script()
    require(platform.system() == 'Linux' and platform.machine() == 'x86_64',
            'only Linux x86_64 is supported')
    trusted_file(Path('/usr/bin/systemctl'))
    for directory in (ROOT, *ROOT.parents):
        meta = directory.lstat()
        require(stat.S_ISDIR(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
                'installation root/ancestor is writable or untrusted; audit, do not chmod blindly')
    binary = (ROOT/'komari').resolve(strict=True)
    require(binary.is_relative_to(ROOT), 'binary resolves outside installation')
    trusted_file(binary)
    require((ROOT/'data').is_dir() and not (ROOT/'data').is_symlink(), 'data directory missing/linked')
    require((ROOT/'data/theme').is_dir() and not (ROOT/'data/theme').is_symlink(),
            'theme parent directory missing/linked')
    require(not (ROOT/'data/theme/Glass').is_symlink(), 'Glass path is a symlink')
    if (ROOT/'releases').exists():
        meta = (ROOT/'releases').lstat()
        require(stat.S_ISDIR(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
                'release storage is untrusted')
    check_persistent_mounts(ROOT)
    result = subprocess.run([trusted_file(Path('/usr/bin/systemctl')), 'show', '-p', 'LoadState',
                             '-p', 'ActiveState', '-p', 'WorkingDirectory', '-p', 'User',
                             '-p', 'Restart', '-p', 'ExecStart', '-p', 'Environment',
                             '-p', 'EnvironmentFiles', '-p', 'RootDirectory',
                             '-p', 'RootImage', '-p', 'BindPaths', '-p', 'BindReadOnlyPaths',
                             '-p', 'TemporaryFileSystem', '-p', 'PrivateMounts',
                             '-p', 'JoinsNamespaceOf', '-p', 'ExtensionImages',
                             '-p', 'ExtensionDirectories', 'komari.service'],
                            check=True, capture_output=True, text=True, timeout=20)
    fields = dict(row.split('=', 1) for row in result.stdout.splitlines() if '=' in row)
    check_unit_mount_view(fields)
    check_standard_unit_layout(fields)
    require(fields.get('EnvironmentFiles', '') in ('', 'n/a') and
            not re.search(r'KOMARI_LISTEN\s*=', fields.get('Environment', '')),
            'custom database/listener environment needs review; no secrets printed')
    listen = re.search(r'--listen(?:=|\s+)([^\s;]+)', fields['ExecStart'])
    require(not listen or listen.group(1) in ('127.0.0.1:25774', '0.0.0.0:25774'),
            'custom listen port needs review')
    version = subprocess.run([str(ROOT/'komari'), 'version', '--json'], cwd=ROOT,
                             check=True, capture_output=True, text=True, timeout=20)
    require(json.loads(version.stdout).get('version') == OLD_VERSION,
            'installed binary is not 1.0.16; refuse to downgrade or repeat update')
    read_only_database_check()
    if STATE.exists():
        require(STATE.is_dir() and not STATE.is_symlink(), 'unsafe upgrade state path')
        journal = STATE/'journal.json'
        if journal.exists():
            require(journal.is_file() and not journal.is_symlink() and
                    json.loads(journal.read_text()).get('phase') in ('committed','rolled_back'),
                    'unfinished upgrade; recover before attempting another')
    return ROOT/'data/theme/Glass'


def preflight():
    theme = host_check()
    # A private staging directory is used even in check mode; never touch panel data.
    with tempfile.TemporaryDirectory(prefix='komari-compat-check-', dir='/var/lib') as stage:
        bundle = download_verified(BUNDLE, stage)
        match, count = compare_stock(theme, bundle)
    print(f'Glass: {count} files; 33-file stock exact match: {"yes" if match else "no"}', flush=True)
    require(match, 'Glass differs from v1.0.13-v1.0.15 stock; service unchanged')
    print('Check passed for 1.0.16/default service/SQLite/Glass; service unchanged.', flush=True)
    print('The transaction controller will recheck paths, disk, DB and theme before stopping.', flush=True)
    return True


def run_manager(stage):
    binary = Path(stage)/'komari'
    version = subprocess.run([str(binary), 'version', '--json'], cwd=ROOT,
                             check=True, capture_output=True, text=True, timeout=20)
    require(json.loads(version.stdout).get('version') == TAG.removeprefix('v'),
            'downloaded release binary version mismatch')
    bundle = Path(stage)/BUNDLE
    match, _ = compare_stock(ROOT/'data/theme/Glass', bundle)
    require(match, 'Glass changed since preflight; service unchanged')
    check_persistent_mounts(ROOT)
    args = [sys.executable, str(Path(stage)/'komari-manager.py'), 'update',
            '--binary', str(binary), '--sha256', ASSETS['komari'][0],
            '--theme-bundle', str(bundle), '--theme-sha256', ASSETS[BUNDLE][0],
            '--hostname', socket.gethostname(),
            '--machine-id', Path('/etc/machine-id').read_text().strip()]
    try:
        subprocess.run(args, check=True)  # no timeout: controller owns recovery and rollback
    except subprocess.CalledProcessError:
        # CalledProcessError echoes every argv token, including host identity.
        # The manager already reports its own error without dumping arguments.
        raise ValueError('upgrade manager failed; inspect the service and transaction journal before retrying') from None
    result = subprocess.run([str(ROOT/'komari'), 'version', '--json'], cwd=ROOT,
                            capture_output=True, text=True, check=True, timeout=20)
    require(json.loads(result.stdout).get('version') == TAG.removeprefix('v'),
            'upgrade returned but installed version differs; inspect recovery status')
    print('Verified: v1.0.19 installed. Snapshot retained in /var/lib/komari-upgrade/backups.', flush=True)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('check','update','status'), nargs='?', default='check')
    args = parser.parse_args(argv)
    try:
        if args.action == 'status':
            verify_status()
            return 0
        preflight()
        if args.action == 'check':
            return 0
        require(sys.stdin.isatty(), 'update requires interactive terminal confirmation')
        print('Only the panel will be updated. Make an independent VPS backup first.', flush=True)
        require(input('Type yes to update 1.0.16 and old Glass to 1.0.19: ').strip() == 'yes',
                'not confirmed; service unchanged')
        with tempfile.TemporaryDirectory(prefix='komari-compat-', dir='/var/lib') as stage:
            for name in (BUNDLE, 'komari-manager.py', 'safe_upgrade.py', 'komari'):
                download_verified(name, stage)
            run_manager(stage)
        return 0
    except (OSError, ValueError, RuntimeError, subprocess.CalledProcessError,
            subprocess.TimeoutExpired, sqlite3.DatabaseError, zipfile.BadZipFile, HTTPException,
            json.JSONDecodeError, EOFError) as exc:
        print(('Status failed: ' if args.action == 'status' else 'Stopped/failed: ') + str(exc), file=sys.stderr)
        if args.action == 'update':
            print('If the transaction started, inspect komari.service and the upgrade journal before retrying.', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
