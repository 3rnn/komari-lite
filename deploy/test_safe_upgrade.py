import hashlib
from contextlib import closing
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import threading
import json
import os
from pathlib import Path
import pwd
import shlex
import sqlite3
import subprocess
import sys
import tempfile
import time
import unittest
import zipfile

SCRIPT = Path(__file__).with_name('safe_upgrade.py')


class UpgradeTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.root = self.base / 'komari'
        self.state = self.base / 'upgrade-state'
        self.root.mkdir()
        (self.root / 'data' / 'theme' / 'Glass' / 'dist').mkdir(parents=True)
        (self.root / 'data' / 'theme' / 'Glass' / 'dist' / 'index.html').write_text('<html>old</html>')
        (self.root / 'data' / 'theme' / 'Glass' / 'komari-theme.json').write_text('{"name":"Glass"}')
        db = sqlite3.connect(self.root / 'data' / 'komari.db')
        db.execute('create table configs (key text, value text)')
        db.execute('insert into configs values (?,?)', ('metric_db_driver', json.dumps('sqlite')))
        db.execute('insert into configs values (?,?)', ('metric_db_dsn', json.dumps('./data/metrics.db')))
        db.execute('create table users (name text)')
        db.execute('insert into users values (?)', ('alice',))
        db.commit(); db.close()
        sqlite3.connect(self.root / 'data' / 'metrics.db').close()
        self.old = self.root / 'komari'
        self.binary(self.old, '1.0.16', legacy=True)
        self.candidate = self.base / 'candidate'
        self.binary(self.candidate, '2.0.0')
        self.bundle = self.base / 'glass.zip'
        old_index = self.root/'data'/'theme'/'Glass'/'dist'/'index.html'
        with zipfile.ZipFile(self.bundle, 'w') as z:
            z.writestr('stock.json', json.dumps({'dist/index.html': hashlib.sha256(old_index.read_bytes()).hexdigest(), 'komari-theme.json': hashlib.sha256((old_index.parent.parent/'komari-theme.json').read_bytes()).hexdigest()}))
            z.writestr('Glass/komari-theme.json', '{"name":"Glass"}')
            z.writestr('Glass/dist/index.html', '<script src="/_next/static/chunks/new.js"></script><link href="/glass-visual.css" rel="stylesheet">')
            z.writestr('Glass/dist/_next/static/chunks/new.js', 'new theme')
            z.writestr('Glass/dist/glass-visual.css', 'body{}')
        self.unitdir = self.base / 'systemd'
        self.unitdir.mkdir()
        self.systemctl = self.base / 'systemctl'
        self.systemctl.write_text('#!/usr/bin/env python3\nimport os,sys,pathlib,sqlite3\na=sys.argv[1]; p=pathlib.Path(os.environ["FAKE_STATE"]); p.write_text(a if a in ("start","stop") else p.read_text() if p.exists() else "start")\nif a=="enable" and "--now" in sys.argv:\n pathlib.Path(os.environ["FAKE_TIMER_ENABLED"]).write_text(sys.argv[-1])\nif a=="show": print(os.environ.get("FAKE_UNIT", "ExecStartPre=/usr/bin/python3 "+os.environ["FAKE_ROOT"]+"/upgrade/safe_upgrade.py gate")); sys.exit(0)\nif a=="stop" and os.environ.get("MUTATE_THEME_ON_STOP"):\n pathlib.Path(os.environ["FAKE_ROOT"]+"/data/theme/Glass/dist/index.html").write_text("operator customization")\nif a=="start" and os.environ.get("MUTATE_ON_START"):\n m=pathlib.Path(os.environ["MUTATE_ON_START"]);\n if not m.exists():\n  m.touch(); db=sqlite3.connect(os.environ["FAKE_ROOT"]+"/data/komari.db"); db.execute("update users set name = ?", ("MIGRATED",)); db.commit(); db.close()\nif a=="start" and os.environ.get("FAIL_START") and (not os.environ.get("MUTATE_ON_START") or pathlib.Path(os.environ["MUTATE_ON_START"]).exists()): sys.exit(1)\nif a=="is-active" and (not p.exists() or p.read_text()!="start"): sys.exit(1)\n')
        self.systemctl.chmod(0o755)
        self.env = dict(os.environ, PYTHONDONTWRITEBYTECODE='1', FAKE_STATE=str(self.base/'state'),
                        FAKE_ROOT=str(self.root), FAKE_TIMER_ENABLED=str(self.base/'timer-enabled'))
        root = self.root
        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                if self.path == '/api/version':
                    if time.monotonic() < getattr(self.server, 'ready_at', 0):
                        self.send_error(503); return
                    if marker := getattr(self.server, 'ready_marker', None):
                        Path(marker).touch()
                    body = subprocess.check_output([str(root/'komari'), 'version', '--json'], cwd=root)
                    body = json.dumps({'data': {'version': self.server.forced_version} if getattr(self.server, 'forced_version', None) and '2.0.0' in body.decode() else json.loads(body)}).encode()
                elif self.path == '/api/public':
                    body = b'{}'
                elif self.path == '/':
                    binary_version = subprocess.check_output([str(root/'komari'), 'version', '--json'], cwd=root)
                    body = b'' if getattr(self.server, 'broken_public', False) and b'2.0.0' in binary_version else \
                        (root/'data'/'theme'/'Glass'/'dist'/'index.html').read_bytes()
                elif self.path.startswith('/_next/'):
                    body = (root/'data'/'theme'/'Glass'/'dist'/self.path.lstrip('/')).read_bytes() if (root/'data'/'theme'/'Glass'/'dist'/self.path.lstrip('/')).is_file() else b''
                elif self.path == '/glass-visual.css':
                    body = b'outdated CSS' if getattr(self.server, 'stale_css', False) else (root/'data'/'theme'/'Glass'/'dist'/'glass-visual.css').read_bytes()
                elif self.path == '/admin':
                    body = b'<script src="/system-assets/assets/entry.js"></script>'
                elif self.path == '/system-assets/assets/entry.js':
                    body = b'admin app'
                else:
                    self.send_error(404); return
                self.send_response(200 if body else 404)
                self.end_headers(); self.wfile.write(body)
            def log_message(self, *args): pass
        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)
        self.url = 'http://127.0.0.1:%d' % self.server.server_port

    def binary(self, path, version, legacy=False):
        path.write_text('#!/usr/bin/env python3\nimport json,sys,os\na=sys.argv[1]\nif a=="health" and os.environ.get("REQUIRE_HTTP_READY") and not os.path.exists(os.environ["REQUIRE_HTTP_READY"]): sys.exit(1)\nif a=="version": print(json.dumps({"version":"'+version+'"}))\nelif a=="schema-version" and not '+str(legacy)+': print(json.dumps({"schema_version":1}))\nelif a=="health" and not '+str(legacy)+': print(json.dumps({"ok":True,"schema_version":1,"expected_schema_version":1}))\nelse: sys.exit(1)\n')
        path.chmod(0o755)

    def username(self):
        with closing(sqlite3.connect(self.root/'data'/'komari.db')) as db:
            return db.execute('select name from users').fetchone()[0]

    def runcli(self, action, *args, env=None):
        unit = ('ExecStartPre=/usr/bin/python3 ' + str(self.state/'safe_upgrade.py') + ' gate --root ' + str(self.root) + ' --state-dir ' + str(self.state) + '\n'
                'ExecStart={ path=' + str(self.root/'komari') + ' ; argv[]=' + str(self.root/'komari') + ' server ; }\n'
                'WorkingDirectory=' + str(self.root) + '\nUser=komari\nGroup=komari\nRestart=always\n'
                'OnFailure=komari-upgrade-recover.service')
        return subprocess.run([sys.executable, str(SCRIPT), action, '--root', str(self.root), '--state-dir', str(self.state), '--unit-dir', str(self.unitdir), '--systemctl', str(self.systemctl), '--http-base', self.url, *map(str,args)], env={**self.env, 'FAKE_UNIT': unit, **(env or {})}, text=True, capture_output=True)

    def install(self):
        r = self.runcli('install-gate')
        self.assertEqual(r.returncode, 0, r.stderr)

    def upgrade(self, *args, env=None):
        self.server.forced_version = (env or {}).get('FAKE_HTTP_VERSION')
        setattr(self.server, 'stale_css', (env or {}).get('FAKE_STALE_CSS') == '1')
        setattr(self.server, 'broken_public', (env or {}).get('FAKE_BROKEN_PUBLIC') == '1')
        return self.runcli('upgrade', '--binary', self.candidate, '--sha256', hashlib.sha256(self.candidate.read_bytes()).hexdigest(), '--theme-bundle', self.bundle, '--theme-sha256', hashlib.sha256(self.bundle.read_bytes()).hexdigest(), *args, env=env)

    def test_gate_required_before_upgrade(self):
        r = self.upgrade()
        self.assertNotEqual(r.returncode, 0)
        self.assertEqual(self.old.read_bytes(), self.old.read_bytes())

    def test_upgrade_preserves_data_and_legacy_binary(self):
        self.install()
        r = self.upgrade()
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(json.loads((self.state/'journal.json').read_text())['phase'], 'committed')
        self.assertEqual(self.username(), 'alice')
        self.assertTrue((self.root/'komari').is_symlink())
        self.assertTrue(list((self.state/'backups').iterdir()))
        self.assertTrue((self.root/'data'/'theme'/'Glass'/'dist'/'_next'/'static'/'chunks'/'new.js').is_file())

    def test_startup_health_waits_for_first_http_response(self):
        self.install()
        setattr(self.server, 'ready_at', time.monotonic() + 1.0)
        result = self.upgrade()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads((self.state/'journal.json').read_text())['phase'], 'committed')

    def test_cli_schema_check_runs_after_http_reports_migration_ready(self):
        self.install()
        marker = self.base/'ready'
        setattr(self.server, 'ready_at', time.monotonic() + 1.0)
        setattr(self.server, 'ready_marker', str(marker))
        result = self.upgrade(env={'REQUIRE_HTTP_READY': str(marker)})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(marker.is_file())

    @unittest.skipUnless(os.geteuid() == 0, 'ownership test needs root')
    def test_snapshot_and_rollback_keep_service_data_ownership(self):
        account = pwd.getpwnam('nobody')
        for p in (self.root/'data').rglob('*'):
            os.chown(p, account.pw_uid, account.pw_gid)
        os.chown(self.root/'data', account.pw_uid, account.pw_gid)
        self.install()
        result = self.upgrade(env={'FAKE_BROKEN_PUBLIC': '1'})
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(json.loads((self.state/'journal.json').read_text())['phase'], 'rolled_back')
        record = json.loads((self.state/'journal.json').read_text())
        for path in (self.root/'data', self.root/'data'/'komari.db',
                     self.root/'data'/'theme'/'Glass',
                     self.state/'backups'/record['id']/'data'):
            self.assertEqual(path.stat().st_uid, account.pw_uid, str(path))

    def test_broken_selected_public_page_rolls_back(self):
        self.install()
        result = self.upgrade(env={'FAKE_BROKEN_PUBLIC': '1'})
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(json.loads((self.state/'journal.json').read_text())['phase'], 'rolled_back')

    def test_theme_missing_bundle_and_customization_fail_closed(self):
        self.install()
        bare = self.runcli('upgrade', '--binary', self.candidate, '--sha256', hashlib.sha256(self.candidate.read_bytes()).hexdigest())
        self.assertNotEqual(bare.returncode, 0)
        (self.root/'data'/'theme'/'Glass'/'custom.txt').write_text('user changes')
        self.assertNotEqual(self.upgrade().returncode, 0)
        self.assertFalse((self.state/'journal.json').exists())

    def test_theme_modified_after_preflight_is_not_overwritten(self):
        self.install()
        result = self.upgrade(env={'MUTATE_THEME_ON_STOP': '1'})
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.root/'data'/'theme'/'Glass'/'dist'/'index.html').read_text(), 'operator customization')
        self.assertEqual((self.base/'state').read_text(), 'start')

    def test_failed_pre_snapshot_restart_keeps_gate_closed_for_recovery(self):
        self.install()
        result = self.upgrade(env={'MUTATE_THEME_ON_STOP': '1', 'FAIL_START': '1'})
        self.assertNotEqual(result.returncode, 0)
        record = json.loads((self.state/'journal.json').read_text())
        self.assertEqual(record['phase'], 'rollback_failed')
        self.assertNotEqual(self.runcli('gate').returncode, 0)

    def test_automatic_retry_recovers_failed_pre_snapshot_restart(self):
        self.install()
        self.assertNotEqual(self.upgrade(env={'MUTATE_THEME_ON_STOP': '1', 'FAIL_START': '1'}).returncode, 0)
        recovered = self.runcli('auto-recover')
        self.assertEqual(recovered.returncode, 0, recovered.stderr)
        self.assertEqual(self.runcli('gate').returncode, 0)
        self.assertEqual(self.username(), 'alice')

    def test_unloaded_gate_and_custom_unit_database_rejected(self):
        self.install()
        self.assertNotEqual(self.upgrade(env={'FAKE_UNIT':'ExecStartPre='}).returncode, 0)
        self.assertNotEqual(self.upgrade(env={'FAKE_UNIT':str(self.state/'safe_upgrade.py')+' --database /tmp/custom.db'}).returncode, 0)
        self.assertFalse((self.state/'journal.json').exists())

    def test_ignored_gate_failure_is_rejected_before_service_stop(self):
        self.install()
        before = (self.base/'state').read_text()
        gate = str(self.state/'safe_upgrade.py')
        unit = ('ExecStartPre={ path=/usr/bin/python3 ; argv[]=/usr/bin/python3 ' + gate +
                ' gate --root ' + str(self.root) + ' --state-dir ' + str(self.state) +
                ' ; ignore_errors=yes ; }\n'
                'ExecStart={ path=' + str(self.root/'komari') + ' ; }\n'
                'WorkingDirectory=' + str(self.root) + '\nUser=komari\nRestart=always\n'
                'OnFailure=komari-upgrade-recover.service')
        result = self.upgrade(env={'FAKE_UNIT': unit})
        self.assertNotEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.state/'journal.json').exists())
        self.assertEqual((self.base/'state').read_text(), before)

    def test_release_version_mismatch_is_rejected_before_service_stop(self):
        self.install()
        before = (self.base/'state').read_text()
        result = self.upgrade('--expected-version', '8.2.0')
        self.assertNotEqual(result.returncode, 0, result.stderr)
        self.assertIn('release tag', result.stderr)
        self.assertFalse((self.state/'journal.json').exists())
        self.assertEqual((self.base/'state').read_text(), before)

    def test_unverified_systemd_workdir_or_binary_rejected_before_stop(self):
        self.install()
        gate = str(self.state/'safe_upgrade.py')
        for unit in (
            'ExecStartPre=/usr/bin/python3 '+gate+' gate\nExecStart={ path='+str(self.root/'komari')+' ; }\n/another/working-directory',
            'ExecStartPre=/usr/bin/python3 '+gate+' gate\nExecStart={ path=/other/komari ; }\nWorkingDirectory='+str(self.root),
        ):
            with self.subTest(unit=unit):
                result = self.upgrade(env={'FAKE_UNIT': unit})
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse((self.state/'journal.json').exists())

    def test_start_failure_restores_binary_and_mutated_database(self):
        self.install()
        r = self.upgrade(env={'FAIL_START':'1', 'MUTATE_ON_START': str(self.base/'mutated')})
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('1.0.16', (self.root/'komari').read_text())
        self.assertEqual(self.username(), 'alice')
        self.assertNotEqual(self.runcli('gate').returncode, 0)

    def test_external_metrics_dsn_rejected_before_stop(self):
        self.install()
        with closing(sqlite3.connect(self.root/'data'/'komari.db')) as db, db:
            db.execute('update configs set value=? where key=?', (json.dumps('postgresql://example/db'), 'metric_db_dsn'))
        r = self.upgrade()
        self.assertNotEqual(r.returncode, 0)
        self.assertFalse((self.state/'journal.json').exists())

    def test_legacy_configs_can_be_snapshotted_before_candidate_migrates(self):
        self.install()
        with closing(sqlite3.connect(self.root/'data'/'komari.db')) as db, db:
            db.execute('drop table configs')
            db.execute('create table configs (id integer primary key, sitename text)')
            db.execute('insert into configs (sitename) values (?)', ('original',))
        result = self.upgrade()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(list((self.state/'backups').glob('*/data/komari.db')))

    def test_legacy_monitoring_rejected_before_service_stop(self):
        self.install()
        with closing(sqlite3.connect(self.root/'data'/'komari.db')) as db, db:
            db.execute('create table ping_records (id integer)')
            db.execute('insert into ping_records values (1)')
        result = self.upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('restricted migration', result.stderr)
        self.assertFalse((self.state/'journal.json').exists())
        self.assertNotEqual((self.base/'state').read_text() if (self.base/'state').exists() else '', 'stop')

    def test_gate_rejects_interrupted_transaction_until_recover(self):
        self.install()
        self.assertEqual(self.upgrade().returncode, 0)
        jpath = self.state/'journal.json'
        record = json.loads(jpath.read_text()); record['phase'] = 'starting'; record['coordinator'] = [99999999, '0']
        jpath.write_text(json.dumps(record))
        self.assertNotEqual(self.runcli('gate').returncode, 0)
        r = self.runcli('recover')
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(self.runcli('gate').returncode, 0)
        self.assertIn('1.0.16', (self.root/'komari').read_text())

    def test_systemd_failure_unit_restores_interrupted_upgrade_automatically(self):
        self.install()
        dropin = (self.unitdir/'komari.service.d'/'10-safe-upgrade.conf').read_text()
        recovery = (self.unitdir/'komari-upgrade-recover.service').read_text()
        timer = (self.unitdir/'komari-upgrade-recover.timer').read_text()
        self.assertIn('OnUnitActiveSec=', timer)
        self.assertEqual((self.base/'timer-enabled').read_text(), 'komari-upgrade-recover.timer')
        self.assertIn('OnFailure=komari-upgrade-recover.service', dropin)
        self.assertIn('Type=oneshot', recovery)
        self.assertIn('User=root', recovery)
        self.assertIn(' auto-recover ', recovery)
        self.assertEqual(self.upgrade().returncode, 0)
        with closing(sqlite3.connect(self.root/'data'/'komari.db')) as db, db:
            db.execute('update users set name="changed"')
        jpath = self.state/'journal.json'
        record = json.loads(jpath.read_text())
        record['phase'] = 'starting'; record['coordinator'] = [99999999, '0']
        jpath.write_text(json.dumps(record))
        self.assertNotEqual(self.runcli('gate').returncode, 0)
        command = recovery.split('ExecStart=', 1)[1].splitlines()[0]
        recovered = subprocess.run(shlex.split(command), env=self.env, capture_output=True, text=True)
        self.assertEqual(recovered.returncode, 0, recovered.stderr)
        self.assertEqual(self.username(), 'alice')
        self.assertIn('1.0.16', (self.root/'komari').read_text())
        self.assertEqual(self.runcli('gate').returncode, 0)

    def test_auto_recover_does_not_rollback_committed_upgrade(self):
        self.install(); self.assertEqual(self.upgrade().returncode, 0)
        result = self.runcli('auto-recover')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('2.0.0', (self.root/'komari').read_text())

    def test_auto_recover_refuses_corrupt_snapshot_without_replacing_data(self):
        self.install(); self.assertEqual(self.upgrade().returncode, 0)
        with closing(sqlite3.connect(self.root/'data'/'komari.db')) as db, db:
            db.execute('update users set name="current"')
        jpath = self.state/'journal.json'
        record = json.loads(jpath.read_text())
        record['phase'] = 'starting'; record['coordinator'] = [99999999, '0']
        jpath.write_text(json.dumps(record))
        (self.state/'backups'/record['id']/'data'/'komari.db').write_bytes(b'corrupt snapshot')
        result = self.runcli('auto-recover')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.username(), 'current')
        self.assertNotEqual(self.runcli('gate').returncode, 0)

    def test_auto_recover_without_snapshot_restores_old_service(self):
        self.install()
        (self.state/'journal.json').write_text(json.dumps({
            'id': 'partial', 'phase': 'stopped', 'previous': 'legacy', 'old_version': '1.0.16',
            'coordinator': [99999999, '0']}))
        result = self.runcli('auto-recover')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.username(), 'alice')
        self.assertEqual(self.runcli('gate').returncode, 0)

    def test_missing_recovery_timer_fails_preflight_before_stopping(self):
        self.install()
        (self.unitdir/'komari-upgrade-recover.timer').unlink()
        result = self.upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.state/'journal.json').exists())
        self.assertNotEqual((self.base/'state').read_text(), 'stop')

    def test_auto_recover_does_not_interrupt_live_coordinator(self):
        self.install(); self.assertEqual(self.upgrade().returncode, 0)
        jpath = self.state/'journal.json'
        record = json.loads(jpath.read_text())
        record['phase'] = 'starting'
        record['coordinator'] = [os.getpid(), Path('/proc/self/stat').read_text().split(') ', 1)[1].split()[19]]
        jpath.write_text(json.dumps(record))
        self.assertEqual(self.runcli('auto-recover').returncode, 0)
        self.assertIn('2.0.0', (self.root/'komari').read_text())
        self.assertEqual(json.loads(jpath.read_text())['phase'], 'starting')

    def test_wrong_hash_and_custom_database_rejected(self):
        self.install()
        self.assertNotEqual(self.runcli('upgrade', '--binary', self.candidate, '--sha256', '0'*64).returncode, 0)
        self.assertNotEqual(self.upgrade('--database', '/other/db').returncode, 0)
        self.assertFalse((self.state/'journal.json').exists())

    def test_mutable_candidate_rejected_before_executing_candidate(self):
        self.install()
        self.candidate.chmod(0o775)
        result = self.upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('untrusted input', result.stderr)
        self.assertFalse((self.state/'journal.json').exists())

    def test_mutable_bundle_rejected_before_service_stop(self):
        self.install()
        self.bundle.chmod(0o775)
        result = self.upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('untrusted input', result.stderr)
        self.assertFalse((self.state/'journal.json').exists())

    def test_legacy_helper_refuses_unverified_binary_only_update(self):
        script = SCRIPT.with_name('update-native.sh')
        r = subprocess.run(['bash', str(script), str(self.candidate)], env={**self.env, 'KOMARI_ROOT':str(self.root), 'KOMARI_SERVICE':'komari.service'}, capture_output=True, text=True)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('1.0.16', (self.root/'komari').read_text())
        self.assertFalse(list(self.root.glob('komari.prev.*')))

    def test_gate_files_traversable_by_service_user(self):
        self.install()
        self.assertTrue((self.state).stat().st_mode & 0o001)
        self.assertTrue((self.state/'safe_upgrade.py').stat().st_mode & 0o001)

    def test_symlinked_backup_directory_rejected_before_stop(self):
        self.install()
        outside = self.base/'outside'; outside.mkdir()
        (self.state/'backups').rmdir()
        (self.state/'backups').symlink_to(outside, target_is_directory=True)
        r = self.upgrade()
        self.assertNotEqual(r.returncode, 0)
        self.assertFalse((self.state/'journal.json').exists())
        self.assertFalse(list(outside.iterdir()))

    def test_theme_owner_and_mode_preserved(self):
        self.install()
        theme = self.root/'data'/'theme'/'Glass'
        theme.chmod(0o750)
        (theme/'dist').chmod(0o750)
        (theme/'dist'/'index.html').chmod(0o640)
        self.assertEqual(self.upgrade().returncode, 0)
        self.assertEqual(theme.stat().st_mode & 0o777, 0o750)
        self.assertEqual((theme/'dist'/'index.html').stat().st_mode & 0o777, 0o640)

    def test_missing_new_theme_manifest_triggers_rollback(self):
        self.install()
        broken = self.base/'broken.zip'
        with zipfile.ZipFile(self.bundle) as source, zipfile.ZipFile(broken, 'w') as target:
            for item in source.namelist():
                if item != 'Glass/komari-theme.json': target.writestr(item, source.read(item))
        r = self.runcli('upgrade', '--binary', self.candidate, '--sha256', hashlib.sha256(self.candidate.read_bytes()).hexdigest(), '--theme-bundle', broken, '--theme-sha256', hashlib.sha256(broken.read_bytes()).hexdigest())
        self.assertNotEqual(r.returncode, 0)
        self.assertTrue((self.root/'data'/'theme'/'Glass'/'komari-theme.json').is_file())
        self.assertFalse((self.state/'journal.json').exists(), 'invalid assets must fail before service stop')

    def test_manual_rollback_of_committed_upgrade(self):
        self.install(); self.assertEqual(self.upgrade().returncode, 0)
        with closing(sqlite3.connect(self.root/'data'/'komari.db')) as db, db: db.execute('update users set name="changed"')
        r = self.runcli('rollback')
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(self.username(), 'alice')

    def test_rollback_uses_verified_snapshot_if_previous_release_disappears(self):
        self.install(); self.assertEqual(self.upgrade().returncode, 0)
        previous = next(self.root.glob('releases/legacy-*/komari'))
        previous.unlink()
        result = self.runcli('rollback')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('1.0.16', (self.root/'komari').read_text())
        self.assertEqual(self.runcli('gate').returncode, 0)

    def test_group_writable_installation_cannot_replace_upgrade_journal(self):
        self.root.chmod(0o775)
        self.install()
        result = self.upgrade()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('writable installation root', result.stderr)
        self.assertFalse((self.state/'journal.json').exists())
        record = {'phase':'starting', 'coordinator':[99999999, '0']}
        (self.state/'journal.json').write_text(json.dumps(record))
        # The production service may replace children of this group-writable
        # root, but must not be able to forge the privileged gate's state.
        decoy = self.root/'upgrade'; decoy.mkdir()
        (decoy/'journal.json').write_text(json.dumps({**record, 'phase': 'committed'}))
        self.assertNotEqual(self.runcli('gate').returncode, 0)

    def test_corrupt_snapshot_cannot_replace_running_data_on_rollback(self):
        self.install(); self.assertEqual(self.upgrade().returncode, 0)
        with closing(sqlite3.connect(self.root/'data'/'komari.db')) as db, db: db.execute('update users set name="current"')
        transaction = json.loads((self.state/'journal.json').read_text())['id']
        (self.state/'backups'/transaction/'data'/'komari.db').write_bytes(b'corrupt snapshot')
        r = self.runcli('rollback')
        self.assertNotEqual(r.returncode, 0)
        self.assertEqual(self.username(), 'current')
        self.assertEqual((self.base/'state').read_text(), 'start')

    def test_recover_incomplete_snapshot_never_uses_partial_backup(self):
        self.install()
        (self.state/'journal.json').write_text(json.dumps({'id':'partial', 'phase':'stopped', 'previous':'legacy', 'old_version':'1.0.16'}))
        partial = self.state/'backups'/'partial'/'data'; partial.mkdir(parents=True)
        (partial/'komari.db').write_text('corrupt')
        self.assertNotEqual(self.runcli('gate').returncode, 0)
        r = self.runcli('recover')
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(self.username(), 'alice')

    def test_failed_health_restores_previous_theme(self):
        self.install()
        # The HTTP version check must fail after startup even if CLI health succeeds.
        r = self.upgrade(env={'FAKE_HTTP_VERSION': 'wrong'})
        self.assertNotEqual(r.returncode, 0)
        self.assertEqual((self.root/'data'/'theme'/'Glass'/'dist'/'index.html').read_text(), '<html>old</html>')

    def test_stale_served_glass_stylesheet_triggers_rollback(self):
        self.install()
        result = self.upgrade(env={'FAKE_STALE_CSS': '1'})
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.root/'data'/'theme'/'Glass'/'dist'/'index.html').read_text(), '<html>old</html>')


if __name__ == '__main__':
    unittest.main()
