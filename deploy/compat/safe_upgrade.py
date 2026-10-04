#!/usr/bin/env python3
"""Offline-first, journaled native Komari upgrade. Python 3 standard library only."""
import argparse
import fcntl
import hashlib
import html.parser
import json
import os
from pathlib import Path
import re
import shutil
import socket
import sqlite3
import stat
import subprocess
import sys
import time
import urllib.request
import urllib.error
import uuid
import zipfile


def require(ok, message):
    if not ok:
        raise RuntimeError(message)


def digest(path):
    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for block in iter(lambda: f.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def atomic_json(path, obj):
    atomic_bytes(path, (json.dumps(obj, sort_keys=True) + '\n').encode())


def atomic_bytes(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_name('.' + path.name + '.' + uuid.uuid4().hex)
    with open(temp, 'xb') as f:
        f.write(data); f.flush(); os.fsync(f.fileno())
    os.chmod(temp, 0o644)
    os.replace(temp, path)
    fsync_dir(path.parent)


def fsync_dir(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try: os.fsync(fd)
    finally: os.close(fd)


def swap_link(path, target):
    temp = path.with_name('.' + path.name + '.' + uuid.uuid4().hex)
    temp.symlink_to(target)
    os.replace(temp, path)
    fsync_dir(path.parent)


def command(argv, cwd=None):
    return subprocess.run(list(map(str, argv)), cwd=cwd, check=True, capture_output=True, text=True, timeout=25).stdout


def info(binary, operation, root):
    return json.loads(command([binary, operation, '--json'], root))


def safe_tree(path):
    require(path.is_dir() and not path.is_symlink(), 'missing or linked directory: ' + str(path))
    for base, dirs, files in os.walk(path, followlinks=False):
        for item in dirs + files:
            p = Path(base) / item
            require(not p.is_symlink() and (p.is_dir() or p.is_file()), 'symlink/special file in persistent tree: ' + str(p))


def check_persistent_mounts(root, mountinfo=Path('/proc/self/mountinfo')):
    """Reject nested data/config mounts, including same-device bind mounts."""
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


def sqlite_check(path):
    require(path.is_file() and not path.is_symlink(), 'missing SQLite: ' + str(path))
    with sqlite3.connect('file:' + str(path) + '?mode=ro', uri=True) as db:
        require(db.execute('pragma quick_check').fetchone()[0] == 'ok', 'SQLite integrity failed: ' + str(path))


def config_check(root):
    dbpath = root / 'data' / 'komari.db'
    sqlite_check(dbpath)
    with sqlite3.connect('file:' + str(dbpath) + '?mode=ro', uri=True) as db:
        columns = {row[1] for row in db.execute('pragma table_info(configs)')}
        if {'key', 'value'} <= columns:
            rows = dict(db.execute("select key, value from configs where key in ('metric_db_driver','metric_db_dsn')"))
        else:
            require({'id', 'sitename'} <= columns, 'unrecognized legacy config schema')
            # Pre-key/value legacy settings had no configurable metrics backend.
            rows = {}
    driver = json.loads(rows.get('metric_db_driver', '"sqlite"'))
    dsn = json.loads(rows.get('metric_db_dsn', '"./data/metrics.db"'))
    require(driver == 'sqlite' and dsn == './data/metrics.db', 'external/custom metrics DSN is unsupported; coordinate separate backup')
    metrics = root / 'data' / 'metrics.db'
    if metrics.exists(): sqlite_check(metrics)
    return dbpath

def check_restricted_migrations(root):
    dbpath = root/'data'/'komari.db'
    with sqlite3.connect('file:' + str(dbpath) + '?mode=ro', uri=True) as db:
        for table in ('records', 'records_long_term', 'gpu_records', 'ping_records'):
            if db.execute('select 1 from sqlite_master where type=? and name=?', ('table', table)).fetchone():
                require(db.execute('select 1 from "' + table + '" limit 1').fetchone() is None,
                        'restricted migration required for legacy monitoring; complete it separately before safe upgrade')


def copy_tree(src, dst):
    safe_tree(src)
    shutil.copytree(src, dst, copy_function=shutil.copy2)
    # copytree/copy2 preserve modes and timestamps, not uid/gid. A rollback
    # snapshot owned by root would otherwise make service data inaccessible.
    for original in (src, *src.rglob('*')):
        target = dst / original.relative_to(src)
        owner = original.stat()
        os.chown(target, owner.st_uid, owner.st_gid)
    for base, dirs, files in os.walk(dst):
        for filename in files:
            fd = os.open(Path(base)/filename, os.O_RDONLY)
            try: os.fsync(fd)
            finally: os.close(fd)
    for base, dirs, files in os.walk(dst, topdown=False): fsync_dir(Path(base))
    fsync_dir(dst.parent)

def snapshot_files(snapshot):
    safe_tree(snapshot)
    return {p.relative_to(snapshot).as_posix(): digest(p) for p in snapshot.rglob('*')
            if p.is_file() and p.relative_to(snapshot).as_posix() not in ('metadata.json', 'manifest.json')}

def verify_snapshot(state, record):
    require(re.fullmatch(r'[0-9]{8}T[0-9]{6}-[0-9a-f]{8}', record['id']), 'invalid snapshot ID')
    backup = state/'backups'/record['id']
    require(backup.is_dir() and not backup.is_symlink(), 'missing rollback snapshot')
    metadata = backup/'metadata.json'
    manifest = backup/'manifest.json'
    require(metadata.is_file() and manifest.is_file() and
            (backup/'data').is_dir() and (backup/'release'/'komari').is_file(),
            'incomplete rollback snapshot')
    require(json.loads(metadata.read_text()).get('id') == record['id'], 'snapshot transaction mismatch')
    expected = json.loads(manifest.read_text())
    require(expected.get('id') == record['id'] and expected.get('files') == snapshot_files(backup),
            'rollback snapshot changed or corrupted')
    return backup


def check_host(a):
    if a.hostname: require(socket.gethostname() == a.hostname, 'hostname mismatch')
    if a.machine_id: require(Path('/etc/machine-id').read_text().strip() == a.machine_id, 'machine ID mismatch')

def check_executable_trust(root):
    # Executing the existing binary as root for version detection is unsafe if
    # the panel's service account can replace it through a writable ancestor.
    for directory in (root, *root.parents):
        meta = directory.lstat()
        require(stat.S_ISDIR(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
                'writable installation root or executable ancestor: ' + str(directory))
    binary = (root/'komari').resolve(strict=True)
    require(binary.is_relative_to(root), 'existing binary outside installation root')
    for directory in binary.parents:
        if directory == root: break
        meta = directory.lstat()
        require(stat.S_ISDIR(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
                'writable existing release directory: ' + str(directory))
    meta = binary.stat()
    require(stat.S_ISREG(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
            'existing executable is not root-owned and protected')
    release_dir = root/'releases'
    if release_dir.exists():
        meta = release_dir.lstat()
        require(stat.S_ISDIR(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
                'writable release storage')

def check_input_trust(path):
    require(path.is_absolute() and path == path.resolve(strict=True),
            'untrusted input path or symlink: ' + str(path))
    for ancestor in path.parents:
        meta = ancestor.lstat()
        require(stat.S_ISDIR(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
                'untrusted input ancestor: ' + str(ancestor))
    meta = path.stat()
    require(stat.S_ISREG(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
            'untrusted input file: ' + str(path))


def state_dir(a, create=False):
    path = Path(a.state_dir)
    require(path.is_absolute(), 'upgrade state path must be absolute')
    if create: path.mkdir(mode=0o755, parents=True, exist_ok=True)
    for directory in (path, *path.parents):
        mode = directory.lstat()
        require(stat.S_ISDIR(mode.st_mode) and mode.st_uid == 0 and not mode.st_mode & 0o022,
                'upgrade state has a non-root-owned, writable or linked ancestor: ' + str(directory))
    if path.stat().st_mode & 0o005 != 0o005:
        # The bootstrap uses umask 077. An explicit User=root panel runs its
        # ExecStartPre gate as root, so a root-private 0700 state directory is
        # readable without exposing the upgrade journal or changing modes.
        # A non-root panel still needs the ordinary traversable 0755 state.
        require(stat.S_IMODE(path.stat().st_mode) == 0o700 and
                command([a.systemctl, 'show', '-p', 'User', a.service]).strip() == 'User=root',
                'gate cannot read upgrade state directory')
    backups = path/'backups'
    if create: backups.mkdir(mode=0o700, exist_ok=True)
    require(backups.is_dir() and not backups.is_symlink() and backups.stat().st_uid == 0 and
            not backups.stat().st_mode & 0o077, 'rollback backups must be root-private')
    return path

def gate_text(a):
    return ('[Unit]\nOnFailure=' + recovery_unit_name(a) + '\n[Service]\nExecStartPre=/usr/bin/python3 ' +
            str(Path(a.state_dir)/'safe_upgrade.py') + ' gate --root ' + str(Path(a.root)) +
            ' --state-dir ' + str(Path(a.state_dir)) + '\n')

def recovery_unit_name(a):
    require(re.fullmatch(r'[A-Za-z0-9_@.-]+\.service', a.service), 'invalid panel unit name')
    return a.service[:-8] + '-upgrade-recover.service'

def recovery_unit_file(a):
    return Path(a.unit_dir)/recovery_unit_name(a)

def recovery_timer_name(a):
    return recovery_unit_name(a).removesuffix('.service') + '.timer'

def recovery_timer_file(a):
    return Path(a.unit_dir)/recovery_timer_name(a)

def recovery_timer_text(a):
    return ('[Unit]\nDescription=Check for interrupted Komari upgrades\n'
            '[Timer]\nOnBootSec=1min\nOnUnitActiveSec=1min\nAccuracySec=10s\n'
            'Unit=' + recovery_unit_name(a) + '\n'
            '[Install]\nWantedBy=timers.target\n')

def recovery_unit_text(a):
    values = (Path(a.state_dir)/'safe_upgrade.py', Path(a.root), Path(a.state_dir),
              Path(a.systemctl), a.service, a.http_base)
    require(all(re.fullmatch(r'[A-Za-z0-9_./:@-]+', str(v)) for v in values),
            'unsafe systemd recovery argument')
    return ('[Unit]\nDescription=Recover interrupted Komari upgrade\n'
            '[Service]\nType=oneshot\nUser=root\n'
            'ExecStart=/usr/bin/python3 ' + str(values[0]) + ' auto-recover --root ' + str(values[1]) +
            ' --state-dir ' + str(values[2]) + ' --systemctl ' + str(values[3]) +
            ' --service ' + str(values[4]) + ' --http-base ' + str(values[5]) + '\n')


def gate_file(a):
    return Path(a.unit_dir) / (a.service + '.d') / '10-safe-upgrade.conf'


def journal_path(state):
    return state / 'journal.json'


def journal(state):
    path = journal_path(state)
    return json.loads(path.read_text()) if path.exists() else None


def save(state, record, phase):
    record['phase'] = phase
    atomic_json(journal_path(state), record)


def process_identity():
    # /proc starttime defeats PID reuse; a killed coordinator must fail closed.
    return [os.getpid(), Path('/proc/self/stat').read_text().split(') ', 1)[1].split()[19]]


def coordinator_alive(identity):
    try:
        return Path('/proc/' + str(identity[0]) + '/stat').read_text().split(') ', 1)[1].split()[19] == identity[1]
    except (OSError, IndexError, KeyError, TypeError):
        return False


def gate(a):
    j = journal(state_dir(a))
    require(not j or j['phase'] in ('committed', 'rolled_back') or
            (j['phase'] in ('starting', 'rollback_starting') and coordinator_alive(j.get('coordinator', []))),
            'unfinished upgrade; run recover (startup blocked)')


def install_gate(a):
    root = Path(a.root)
    state = state_dir(a, create=True)
    require(root.is_dir() and not root.is_symlink() and (root / 'komari').is_file(), 'invalid existing installation')
    require(not journal(state) or journal(state)['phase'] in ('committed', 'rolled_back'), 'recover unfinished upgrade first')
    dest = state/'safe_upgrade.py'
    atomic_bytes(dest, Path(__file__).read_bytes())
    dest.chmod(0o755)
    atomic_bytes(recovery_unit_file(a), recovery_unit_text(a).encode())
    atomic_bytes(recovery_timer_file(a), recovery_timer_text(a).encode())
    unit = gate_file(a)
    unit.parent.mkdir(parents=True, exist_ok=True)
    atomic_bytes(unit, gate_text(a).encode())
    command([a.systemctl, 'daemon-reload'])
    command([a.systemctl, 'enable', '--now', recovery_timer_name(a)])
    require(unit.read_text() == gate_text(a) and
            recovery_unit_file(a).read_text() == recovery_unit_text(a) and
            recovery_timer_file(a).read_text() == recovery_timer_text(a), 'gate/recovery unit verification failed')


def check_gate(a, require_gate=True):
    root = Path(a.root)
    state = Path(a.state_dir)
    if require_gate:
        state_dir(a)
        require(gate_file(a).read_text() == gate_text(a), 'pre-start gate not installed')
        require(recovery_unit_file(a).read_text() == recovery_unit_text(a), 'automatic recovery unit missing or stale')
        require(recovery_timer_file(a).read_text() == recovery_timer_text(a), 'automatic recovery timer missing or stale')
        command([a.systemctl, 'is-enabled', '--quiet', recovery_timer_name(a)])
        command([a.systemctl, 'is-active', '--quiet', recovery_timer_name(a)])
        require((state/'safe_upgrade.py').is_file(), 'gate program absent')
        # Compare against deployment script to prevent stale gate versions.
        require(digest(state/'safe_upgrade.py') == digest(__file__), 'installed gate differs from upgrade program')
    unit_state = command([a.systemctl, 'show', '-p', 'OnFailure', '-p', 'ExecStartPre', '-p', 'ExecStart', '-p', 'WorkingDirectory', '-p', 'User', '-p', 'Restart', a.service])
    fields = dict(line.split('=', 1) for line in unit_state.splitlines() if '=' in line)
    if require_gate:
        require(recovery_unit_name(a) in fields.get('OnFailure', '').split(), 'automatic recovery not loaded by systemd')
        require(str(state/'safe_upgrade.py') + ' gate --root ' + str(root) + ' --state-dir ' + str(state) in fields.get('ExecStartPre', ''),
                'pre-start gate is not loaded by systemd')
        require(not re.search(r'\bignore_errors=(?:yes|true|1)\b|(?:^|[\s{])-/usr/bin/python3',
                              fields.get('ExecStartPre', ''), re.I),
                'pre-start gate or another preflight ignores errors')
    require(fields.get('WorkingDirectory') == str(root), 'unexpected WorkingDirectory')
    actual_binary = re.search(r'\bpath=([^\s;]+)', fields.get('ExecStart', ''))
    require(actual_binary and actual_binary.group(1) == str(root/'komari'), 'unexpected service executable')
    # Root-owned legacy installations keep their existing explicit User=root;
    # the upgrade controller is root already and must not change the unit or
    # attempt a privilege/ownership migration while preserving live data.
    require(bool(fields.get('User')), 'service user must be explicit')
    require(fields.get('Restart') == 'always', 'panel must restart after restore')
    require(not re.search(r'(?<![\w-])(?:--(?:database|db-type)(?:=|\s)|-[dt])',
                          fields.get('ExecStart', '')),
            'custom main database/driver in unit is unsupported')


def stock_theme(root, bundle, bundle_sha):
    require(bool(bundle) == bool(bundle_sha), 'theme bundle and SHA-256 must be supplied together')
    require(bundle or not (root/'data'/'theme'/'Glass').exists(), 'Glass installed: authenticated theme bundle required (unknown/custom theme must not be overwritten)')
    if not bundle: return None
    require(digest(bundle) == bundle_sha, 'theme bundle checksum mismatch')
    theme = root/'data'/'theme'/'Glass'
    safe_tree(theme)
    with zipfile.ZipFile(bundle) as z:
        names = z.namelist()
        require(len(names) == len(set(names)) and len(names) <= 4097, 'duplicate or oversized bundle inventory')
        require(all(n == 'stock.json' or n.startswith('Glass/') for n in names), 'unexpected theme bundle entry')
        require('stock.json' in names and z.getinfo('stock.json').file_size <= 1024 * 1024, 'missing or oversized stock manifest')
        total = 0
        for entry in z.infolist():
            segments = entry.filename.rstrip('/').split('/')
            require(all(s not in ('', '.', '..') for s in segments) and not entry.flag_bits & 1,
                    'unsafe or encrypted theme bundle entry')
            if not entry.is_dir():
                mode = stat.S_IFMT(entry.external_attr >> 16)
                require(mode in (0, stat.S_IFREG), 'non-regular theme bundle entry')
                total += entry.file_size
        require(total <= 512 * 1024 * 1024, 'theme bundle expands beyond safety limit')
        manifest = json.loads(z.read('stock.json'))
        require(isinstance(manifest, dict) and manifest and
                all(isinstance(k, str) and isinstance(v, str) and re.fullmatch(r'[0-9a-f]{64}', v)
                    for k, v in manifest.items()), 'invalid stock manifest')
        current = {p.relative_to(theme).as_posix(): digest(p) for p in theme.rglob('*') if p.is_file()}
        require(current == manifest, 'Glass modified or unknown; refusing overwrite')
        files = [n for n in names if n.startswith('Glass/') and not n.endswith('/')]
        require(files and all(not Path(n).is_absolute() and all(s not in ('', '.', '..') for s in n.split('/')) for n in files), 'unsafe bundle path')
        require('Glass/komari-theme.json' in files and 'Glass/dist/index.html' in files, 'new theme lacks manifest or index')
        require(isinstance(json.loads(z.read('Glass/komari-theme.json')), dict), 'invalid new theme manifest')
        require(z.getinfo('Glass/dist/index.html').file_size <= 2 * 1024 * 1024, 'new theme index too large')
        index = Assets(); index.feed(z.read('Glass/dist/index.html').decode('utf-8'))
        require(any(asset.startswith('_next/') for asset in index.paths), 'new theme JS entry missing')
        for asset in index.paths:
            if asset.startswith('system-assets/'): continue
            require('..' not in Path(asset).parts and '?' not in asset and '#' not in asset and
                    'Glass/dist/' + asset in files, 'new theme references missing or unsafe asset: ' + asset)
    return bundle


def apply_theme(root, bundle):
    if not bundle: return
    theme = root/'data'/'theme'/'Glass'
    staged = theme.parent / ('.Glass-stage-' + uuid.uuid4().hex)
    ownership = theme.stat()
    staged.mkdir()
    with zipfile.ZipFile(bundle) as z:
        for n in z.namelist():
            if n.startswith('Glass/') and not n.endswith('/'):
                target = staged / n[len('Glass/'):]
                target.parent.mkdir(parents=True, exist_ok=True)
                with z.open(n) as src, open(target, 'xb') as dst:
                    shutil.copyfileobj(src, dst)
                    dst.flush(); os.fsync(dst.fileno())
                previous = theme / n[len('Glass/'):]
                if previous.is_file(): shutil.copystat(previous, target)
                else: target.chmod(0o644)
                os.chown(target, ownership.st_uid, ownership.st_gid)
    for directory in sorted((p for p in staged.rglob('*') if p.is_dir()), key=lambda p: len(p.parts), reverse=True):
        original = theme / directory.relative_to(staged)
        if original.is_dir(): shutil.copystat(original, directory)
        else: directory.chmod(stat.S_IMODE((theme/'dist').stat().st_mode))
        os.chown(directory, ownership.st_uid, ownership.st_gid)
    shutil.copystat(theme, staged)
    os.chown(staged, ownership.st_uid, ownership.st_gid)
    for directory in sorted((p for p in staged.rglob('*') if p.is_dir()), key=lambda p: len(p.parts), reverse=True):
        fsync_dir(directory)
    fsync_dir(staged)
    old = theme.parent / ('.Glass-old-' + uuid.uuid4().hex)
    os.replace(theme, old)
    os.replace(staged, theme)
    fsync_dir(theme.parent)
    shutil.rmtree(old)


class Assets(html.parser.HTMLParser):
    def __init__(self): super().__init__(); self.paths = []
    def handle_starttag(self, tag, attrs):
        for k, v in attrs:
            if k in ('src', 'href') and v and (v.startswith('/_next/') or v.startswith('_next/') or
                                                v.startswith('/glass-visual-') or v.startswith('/glass-visual.') or
                                                v.startswith('/system-assets/')):
                self.paths.append(v.lstrip('/'))


def health(a, version, schema, legacy=False):
    root = Path(a.root)
    command([a.systemctl, 'is-active', '--quiet', a.service])
    for endpoint in ('/api/version', '/api/public'):
        deadline = time.monotonic() + 30 if endpoint.endswith('version') else None
        while True:
            try:
                with urllib.request.urlopen(a.http_base.rstrip('/') + endpoint, timeout=5) as response:
                    require(response.status == 200, 'HTTP failure: ' + endpoint)
                    if endpoint.endswith('version'):
                        payload = json.load(response)
                        require((payload.get('data') or payload.get('result') or payload).get('version') == version, 'HTTP version mismatch')
                break
            except urllib.error.URLError:
                if deadline is None or time.monotonic() >= deadline:
                    raise
                command([a.systemctl, 'is-active', '--quiet', a.service])
                time.sleep(0.5)
    # HTTP readiness means startup migrations have finished; checking the
    # on-disk schema immediately after systemctl start races that migration.
    config_check(root)
    if not legacy:
        result = info(root/'komari', 'health', root)
        require(result.get('ok') is True and result.get('schema_version') == schema and result.get('expected_schema_version') == schema, 'CLI schema/health mismatch')
    with urllib.request.urlopen(a.http_base.rstrip('/') + '/', timeout=5) as response:
        require(response.status == 200, 'selected public theme unavailable')
        public_html = response.read(1024 * 1024)
        require(bool(public_html), 'selected public theme is empty')
    public = Assets(); public.feed(public_html.decode('utf-8'))
    if not legacy:
        require(any(path.endswith('.js') for path in public.paths), 'selected public theme JS entry missing')
    require(len(public.paths) <= 256, 'selected public theme asset inventory too large')
    for asset in public.paths:
        require('..' not in Path(asset).parts and '?' not in asset and '#' not in asset,
                'unsafe selected public theme asset path')
        with urllib.request.urlopen(a.http_base.rstrip('/') + '/' + asset, timeout=5) as response:
            require(response.status == 200 and response.read(1),
                    'selected public theme asset unavailable: ' + asset)
    theme = root/'data'/'theme'/'Glass'
    dist = theme/'dist'
    if theme.is_dir():
        metadata = theme/'komari-theme.json'
        require(metadata.is_file() and isinstance(json.loads(metadata.read_text()), dict), 'Glass manifest missing or invalid')
        require(dist.is_dir(), 'Glass dist missing')
        index = dist/'index.html'
        require(index.is_file(), 'Glass index missing')
        p = Assets(); p.feed(index.read_text())
        if not legacy:
            require(any(asset.startswith('_next/') for asset in p.paths), 'Glass JS entry missing')
        for asset in p.paths:
            if asset.startswith('system-assets/'): continue
            require('..' not in Path(asset).parts and '?' not in asset and '#' not in asset, 'unsafe theme asset path')
            require((dist/asset).is_file(), 'missing theme asset: ' + asset)
            with urllib.request.urlopen(a.http_base.rstrip('/') + '/' + asset, timeout=5) as response:
                require(response.status == 200 and hashlib.sha256(response.read()).hexdigest() == digest(dist/asset),
                        'stale or unserved theme asset: ' + asset)
    with urllib.request.urlopen(a.http_base.rstrip('/') + '/admin', timeout=5) as response:
        require(response.status == 200, 'admin frontend unavailable')
        admin = Assets(); admin.feed(response.read().decode())
    entries = [p for p in admin.paths if p.startswith('system-assets/assets/')]
    require(any(p.endswith('.js') for p in entries), 'admin frontend JS entry missing')
    for asset in entries:
        require('..' not in Path(asset).parts and '?' not in asset and '#' not in asset, 'unsafe frontend asset path')
        with urllib.request.urlopen(a.http_base.rstrip('/') + '/' + asset, timeout=5) as response:
            require(response.status == 200 and response.read(1), 'admin frontend asset unavailable: ' + asset)


def stop(a): command([a.systemctl, 'stop', a.service])
def start(a): command([a.systemctl, 'start', a.service])


def restore(a, j):
    root = Path(a.root)
    check_persistent_mounts(root)
    state = state_dir(a)
    backup = verify_snapshot(state, j)
    stop(a)
    check_persistent_mounts(root)
    data = root/'data'
    displaced = root/('data.failed.' + j['id'] + '-' + uuid.uuid4().hex[:8])
    if data.exists(): os.replace(data, displaced)
    try:
        copy_tree(backup/'data', data)
        if (backup/'config').exists():
            target = root/'config'
            if target.exists(): os.replace(target, root/('config.failed.' + j['id'] + '-' + uuid.uuid4().hex[:8]))
            copy_tree(backup/'config', target)
        elif (root/'config').exists():
            os.replace(root/'config', root/('config.failed.' + j['id'] + '-' + uuid.uuid4().hex[:8]))
        config_check(root)
        recovered = root/'releases'/('recovered-' + j['id'])
        if recovered.exists():
            require(recovered.is_dir() and not recovered.is_symlink() and
                    digest(recovered/'komari') == digest(backup/'release'/'komari'),
                    'recovery release differs from verified snapshot')
        else:
            recovered.mkdir(mode=0o755, parents=True)
            shutil.copy2(backup/'release'/'komari', recovered/'komari')
            fsync_dir(recovered)
        swap_link(root/'current', Path('releases')/recovered.name)
        swap_link(root/'komari', Path('current')/'komari')
        j['coordinator'] = process_identity()
        save(state, j, 'rollback_starting')
        command([a.systemctl, 'reset-failed', a.service])
        start(a)
        health(a, j['old_version'], None, legacy=True)
        save(state, j, 'rolled_back')
    except Exception:
        save(state, j, 'rollback_failed')
        raise


def preflight(a, require_gate=True):
    root = Path(a.root)
    state = state_dir(a)
    check_host(a); check_gate(a, require_gate=require_gate)
    require(root.is_dir() and not root.is_symlink(), 'invalid root')
    check_persistent_mounts(root)
    check_executable_trust(root)
    for name in ('data', 'releases', 'backups', 'upgrade'):
        path = root/name
        require(not path.is_symlink() and (not path.exists() or (path.is_dir() and path.stat().st_dev == root.stat().st_dev and not os.path.ismount(path))), 'unsafe symlink/mount/non-directory: ' + str(path))
    require(not journal(state) or journal(state)['phase'] in ('committed', 'rolled_back'), 'unfinished transaction; run recover')
    require(not a.database or a.database == './data/komari.db', 'custom main --database unsupported')
    require(a.sha256 and re.fullmatch('[0-9a-f]{64}', a.sha256), 'require exact SHA-256')
    binary = Path(a.binary)
    check_input_trust(binary)
    require(binary.is_file() and os.access(binary, os.X_OK) and digest(binary) == a.sha256, 'binary hash/executable mismatch')
    require((root/'komari').is_file() and os.access(root/'komari', os.X_OK), 'existing executable missing')
    old_version = info(root/'komari', 'version', root)['version']
    version = info(binary, 'version', root)['version']
    schema = info(binary, 'schema-version', root)['schema_version']
    require(isinstance(schema, int) and schema > 0 and re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]*', version), 'invalid release metadata')
    if getattr(a, 'expected_version', None):
        require(version == a.expected_version, 'candidate version mismatch with release tag')
    require(version != old_version, 'identical version refused')
    config_check(root)
    check_restricted_migrations(root)
    safe_tree(root/'data')
    if (root/'config').exists(): safe_tree(root/'config')
    if a.theme_bundle: check_input_trust(Path(a.theme_bundle))
    bundle = stock_theme(root, a.theme_bundle, a.theme_sha256)
    require(not (root/'komari').is_symlink() or (root/'komari').resolve().is_relative_to(root/'releases'), 'unexpected binary symlink')
    require(not ((root/'current').exists() and not (root/'current').is_symlink()), 'current must be a release symlink')
    require(not ((root/'current').is_symlink() and not (root/'current').resolve().is_relative_to(root/'releases')), 'unexpected current target')
    require(not (root/'releases'/version).exists(), 'release already staged')
    size = sum(p.stat().st_size for p in (root/'data').rglob('*') if p.is_file())
    size += (root/'komari').stat().st_size + binary.stat().st_size
    if bundle:
        with zipfile.ZipFile(bundle) as z:
            size += sum(entry.file_size for entry in z.infolist() if not entry.is_dir()) * 3
    require(shutil.disk_usage(root).free > size * 2 + 16 * 1024 * 1024 and
            shutil.disk_usage(state).free > size * 2 + 16 * 1024 * 1024,
            'insufficient disk for snapshot and rollback')
    return root, state, binary, version, schema, bundle


def upgrade(a):
    root, state, binary, version, schema, bundle = preflight(a)
    old_version = info(root/'komari', 'version', root)['version']
    ident = time.strftime('%Y%m%dT%H%M%S') + '-' + uuid.uuid4().hex[:8]
    old_release = root/'current'
    previous = old_release.resolve().name if old_release.is_symlink() else 'legacy-' + ident
    new_release = root/'releases'/version
    new_release.mkdir(parents=True, exist_ok=False)
    shutil.copy2(binary, new_release/'komari')
    require(digest(new_release/'komari') == a.sha256, 'staged hash mismatch')
    # Persist intent before stopping; pre-start gate rejects reboot of partial transactions.
    j = dict(id=ident, previous=previous, target=version, old_version=old_version, schema=schema,
             sha256=a.sha256, coordinator=process_identity())
    save(state, j, 'prepared')
    try:
        stop(a)
        check_persistent_mounts(root)
        # A theme write between preflight and the stopped snapshot must not be
        # silently replaced by the candidate bundle.
        if bundle: stock_theme(root, bundle, a.theme_sha256)
        save(state, j, 'stopped')
        snapshot = state/'backups'/ident
        snapshot.mkdir(mode=0o700, exist_ok=False)
        copy_tree(root/'data', snapshot/'data')
        if (root/'config').exists(): copy_tree(root/'config', snapshot/'config')
        prior = snapshot/'release'; prior.mkdir()
        shutil.copy2(root/'komari', prior/'komari')
        if not old_release.is_symlink():
            dest = root/'releases'/previous
            dest.mkdir(parents=True, exist_ok=False)
            shutil.copy2(root/'komari', dest/'komari')
        # WAL/SHM, secrets, theme and other persistent data are copied while stopped.
        config_check(snapshot)
        atomic_json(snapshot/'manifest.json', {'id': ident, 'files': snapshot_files(snapshot)})
        atomic_json(snapshot/'metadata.json', {k: v for k, v in j.items() if k != 'coordinator'})
        save(state, j, 'backed_up')
        apply_theme(root, bundle)
        swap_link(root/'current', Path('releases')/version)
        swap_link(root/'komari', Path('current')/'komari')
        save(state, j, 'starting')
        start(a)  # versioned migrations execute only now, after the stopped snapshot.
        health(a, version, schema)
        save(state, j, 'committed')
    except BaseException:
        if (state/'backups'/ident/'metadata.json').is_file():
            restore(a, j)
        else:
            # No complete backup: old process can run only after removing transaction intent.
            if (root/'current').is_symlink() and (root/'current').resolve() == new_release:
                swap_link(root/'current', Path('releases')/previous)
            if (root/'komari').is_symlink(): swap_link(root/'komari', Path('current')/'komari')
            j['coordinator'] = process_identity()
            save(state, j, 'rollback_starting')
            try:
                command([a.systemctl, 'reset-failed', a.service])
                start(a)
                health(a, old_version, None, legacy=True)
                save(state, j, 'rolled_back')
            except BaseException:
                save(state, j, 'rollback_failed')
                raise
        raise


def recover_pending(a, state, j):
    require(j and j['phase'] not in ('committed', 'rolled_back'), 'no pending transaction')
    require(not coordinator_alive(j.get('coordinator', [])), 'upgrade coordinator is still active')
    root = Path(a.root)
    check_persistent_mounts(root)
    if not (state/'backups'/j['id']/'metadata.json').is_file():
        require(j['phase'] in ('prepared', 'stopped', 'rollback_starting', 'rollback_failed'),
                'snapshot missing after switch; manual intervention required')
        check_executable_trust(root)
        require(info(root/'komari', 'version', root)['version'] == j['old_version'],
                'snapshot absent and previous executable not active; manual intervention required')
        stop(a)
        config_check(root)
        j['coordinator'] = process_identity()
        save(state, j, 'rollback_starting')
        try:
            command([a.systemctl, 'reset-failed', a.service])
            start(a)
            health(a, j['old_version'], None, legacy=True)
            save(state, j, 'rolled_back')
        except Exception:
            save(state, j, 'rollback_failed')
            raise
    else:
        restore(a, j)

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('action', choices=['install-gate', 'gate', 'upgrade', 'recover', 'auto-recover', 'rollback'])
    p.add_argument('--root', default='/opt/komari')
    p.add_argument('--state-dir', default='/var/lib/komari-upgrade')
    p.add_argument('--unit-dir', default='/etc/systemd/system')
    p.add_argument('--service', default='komari.service')
    p.add_argument('--systemctl', default='systemctl')
    p.add_argument('--http-base', default='http://127.0.0.1:25774')
    p.add_argument('--binary'); p.add_argument('--sha256')
    p.add_argument('--theme-bundle'); p.add_argument('--theme-sha256')
    p.add_argument('--hostname'); p.add_argument('--machine-id'); p.add_argument('--database')
    p.add_argument('--expected-version')
    a = p.parse_args()
    root = Path(a.root)
    try:
        if a.action == 'gate': gate(a); return
        if a.action == 'install-gate': install_gate(a); return
        state = state_dir(a)
        if a.action == 'auto-recover':
            require(os.geteuid() == 0, 'automatic recovery requires root privileges')
            pending = journal(state)
            if not pending or pending['phase'] in ('committed', 'rolled_back') or coordinator_alive(pending.get('coordinator', [])):
                return
        lock = state/'lock'
        with open(lock, 'a+') as f:
            fcntl.flock(f, fcntl.LOCK_EX | fcntl.LOCK_NB)
            if a.action == 'upgrade':
                require(a.binary is not None, 'missing --binary'); upgrade(a)
            elif a.action in ('recover', 'auto-recover'):
                current = journal(state)
                if a.action == 'recover' or (current and current['phase'] not in ('committed', 'rolled_back') and
                                             not coordinator_alive(current.get('coordinator', []))):
                    recover_pending(a, state, current)
            else:
                j = journal(state)
                require(j and (state/'backups'/j['id']/'data').is_dir(), 'no snapshot for manual rollback')
                restore(a, j)
    except (Exception, KeyboardInterrupt) as exc:
        print('safe-upgrade: ' + str(exc), file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__': main()
