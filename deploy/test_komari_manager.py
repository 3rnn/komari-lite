"""Isolated manager contract tests; never touch the live service."""
import hashlib
import fcntl
import importlib.util
import json
import os
from pathlib import Path
import pwd
import stat
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock

SCRIPT = Path(__file__).with_name('komari-manager.py')


class ManagerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec = importlib.util.spec_from_file_location('komari_manager', SCRIPT)
        cls.manager = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.manager)

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.root = self.base / 'panel'
        self.units = self.base / 'units'
        self.units.mkdir()
        self.state = self.base / 'state'
        self.candidate = self.base / 'candidate'
        self.candidate.write_text('#!/bin/sh\nexit 0\n')
        self.candidate.chmod(0o755)
        self.sha = hashlib.sha256(self.candidate.read_bytes()).hexdigest()
        # tmpdir defaults 0700, root-owned; nobody cannot traverse it in fixture,
        # so fake service/HTTP startup rather than executing the fixture binary.
        self.user = pwd.getpwnam('nobody').pw_name

    def args(self, action='install', **overrides):
        defaults = dict(action=action, root=str(self.root), state_dir=str(self.state),
                        unit_dir=str(self.units), service='komari.service', systemctl='/usr/bin/true',
                        listen='127.0.0.1:25774', http_base='http://127.0.0.1:25774',
                        binary=str(self.candidate), sha256=self.sha, theme_bundle=None,
                        theme_sha256=None, service_user=self.user, hostname=None,
                        machine_id=None, database=None)
        defaults.update(overrides)
        return self.manager.argparse.Namespace(**defaults)

    def test_fresh_install_stages_release_gate_and_first_run_without_admin_secret(self):
        args = self.args()
        with mock.patch.object(self.manager.safe_upgrade, 'info', side_effect=[
            {'version': '2.0.0'}, {'schema_version': 4}]), \
             mock.patch.object(self.manager.safe_upgrade, 'install_gate') as gate, \
             mock.patch.object(self.manager.safe_upgrade, 'check_gate'), \
             mock.patch.object(self.manager, 'command') as command, \
             mock.patch.object(self.manager, 'check_fresh_unit') as effective, \
             mock.patch.object(self.manager, 'check_first_run') as first_run:
            self.manager.install(args)
        staged = self.root / 'releases' / '2.0.0' / 'komari'
        self.assertEqual(hashlib.sha256(staged.read_bytes()).hexdigest(), self.sha)
        self.assertEqual((self.root/'komari').resolve(), staged)
        self.assertEqual((self.root/'current').resolve(), staged.parent)
        self.assertIn('ExecStart='+str(self.root/'komari')+' server --listen 127.0.0.1:25774',
                      (self.units/'komari.service').read_text())
        self.assertEqual((self.root/'data').stat().st_uid, pwd.getpwnam(self.user).pw_uid)
        self.assertTrue((self.root/'data').stat().st_mode & stat.S_IWUSR)
        self.assertEqual((self.root/'backup').stat().st_uid, pwd.getpwnam(self.user).pw_uid)
        self.assertEqual(stat.S_IMODE((self.root/'backup').stat().st_mode), 0o700)
        self.assertIn('ReadWritePaths='+str(self.root/'data')+' '+str(self.root/'backup'),
                      (self.units/'komari.service').read_text())
        self.assertFalse(self.root.stat().st_mode & 0o022)
        self.assertFalse((self.root/'data'/'komari.db').exists())
        gate.assert_called_once()
        command.assert_any_call(['/usr/bin/true', 'enable', '--now', 'komari.service'])
        first_run.assert_called_once_with(args)
        effective.assert_called_once()
        self.assertIn('Group=nogroup', (self.units/'komari.service').read_text())
        self.assertNotIn('password', (self.units/'komari.service').read_text().lower())

    def test_existing_directory_and_unit_are_never_overwritten(self):
        self.root.mkdir()
        (self.root/'data').mkdir()
        with self.assertRaisesRegex(ValueError, 'empty|already'):
            self.manager.install(self.args())
        self.assertTrue((self.root/'data').exists())
        self.assertFalse((self.units/'komari.service').exists())


    def test_untrusted_binary_refused_before_any_writes(self):
        self.candidate.chmod(0o777)
        with self.assertRaisesRegex(RuntimeError, 'untrusted'):
            self.manager.install(self.args())
        self.assertFalse(self.root.exists())

    def test_mismatched_hash_refused_before_any_writes(self):
        with self.assertRaisesRegex(ValueError, 'SHA-256'):
            self.manager.install(self.args(sha256='0'*64))
        self.assertFalse(self.root.exists())

    def test_release_tag_must_match_binary_version_before_fresh_install(self):
        with mock.patch.object(self.manager.safe_upgrade, 'info', side_effect=[
            {'version': '2.0.0'}, {'schema_version': 4}]):
            with self.assertRaisesRegex(ValueError, 'release tag|version mismatch'):
                self.manager.install(self.args(expected_version='8.2.0'))
        self.assertFalse(self.root.exists())

    def test_update_requires_existing_trusted_install_before_gate_install(self):
        self.root.mkdir(mode=0o775)
        self.root.chmod(0o775)
        (self.root/'komari').write_text('old')
        with mock.patch.object(self.manager.safe_upgrade, 'install_gate') as gate:
            with self.assertRaisesRegex(RuntimeError, 'writable'):
                self.manager.update(self.args('update'))
            gate.assert_not_called()

    def test_update_delegates_to_safe_transaction_after_gate(self):
        self.root.mkdir()
        (self.root/'komari').write_text('old')
        (self.root/'komari').chmod(0o755)
        (self.root/'data').mkdir()
        (self.root/'data'/'theme'/'Glass').mkdir(parents=True)
        bundle = self.base/'Glass.zip'
        bundle.write_bytes(b'zip')
        args = self.args('update', theme_bundle=str(bundle), theme_sha256=hashlib.sha256(bundle.read_bytes()).hexdigest())
        with mock.patch.object(self.manager.safe_upgrade, 'preflight') as preflight, \
             mock.patch.object(self.manager.safe_upgrade, 'install_gate') as gate, \
             mock.patch.object(self.manager.safe_upgrade, 'upgrade') as upgrade, \
             mock.patch.object(self.manager, 'command', return_value='ExecStart=--listen 127.0.0.1:25774'):
            self.manager.update(args)
        gate.assert_called_once()
        preflight.assert_called_once_with(args, require_gate=False)
        upgrade.assert_called_once_with(args)

    def test_update_with_glass_requires_explicit_authenticated_bundle(self):
        self.root.mkdir()
        (self.root/'komari').write_text('old')
        (self.root/'komari').chmod(0o755)
        (self.root/'data'/'theme'/'Glass').mkdir(parents=True)
        with mock.patch.object(self.manager.safe_upgrade, 'install_gate') as gate:
            with self.assertRaisesRegex(ValueError, 'theme bundle'):
                self.manager.update(self.args('update'))
            gate.assert_not_called()

    def test_failed_fresh_start_keeps_data_and_stops_service(self):
        args = self.args()
        with mock.patch.object(self.manager.safe_upgrade, 'info', side_effect=[
            {'version': '2.0.0'}, {'schema_version': 4}]), \
             mock.patch.object(self.manager.safe_upgrade, 'install_gate'), \
             mock.patch.object(self.manager.safe_upgrade, 'check_gate'), \
             mock.patch.object(self.manager, 'check_fresh_unit'), \
             mock.patch.object(self.manager, 'command', side_effect=['', None, RuntimeError('start failed'), None]) as command:
            with self.assertRaises(RuntimeError):
                self.manager.install(args)
        self.assertTrue((self.root/'data').is_dir())
        self.assertTrue((self.root/'komari').is_symlink())
        self.assertEqual(command.call_args.args[0], ['/usr/bin/true', 'stop', 'komari.service'])

    def test_first_run_rejects_wrong_state(self):
        args = self.args()
        with mock.patch.object(self.manager.safe_upgrade, 'command', return_value='active'), \
             mock.patch.object(self.manager, 'read_http', return_value=(200, json.dumps({'data': {'state': 'completed', 'required': False}}).encode())):
            with self.assertRaisesRegex(ValueError, 'first-run'):
                self.manager.check_first_run(args)

    def test_first_run_checks_installer_status_and_page(self):
        args = self.args()
        with mock.patch.object(self.manager, 'command', return_value='active'), \
             mock.patch.object(self.manager, 'read_http', side_effect=[
                 (200, json.dumps({'data': {'state': 'ready', 'required': True}}).encode()),
                 (200, b'<html>first-run installer</html>')]) as request:
            self.manager.check_first_run(args)
        self.assertEqual(request.call_count, 2)

    def test_update_rejects_foreign_health_target_before_gate_install(self):
        self.root.mkdir()
        (self.root/'komari').write_text('old')
        (self.root/'komari').chmod(0o755)
        (self.root/'data').mkdir()
        with mock.patch.object(self.manager.safe_upgrade, 'install_gate') as gate:
            with self.assertRaisesRegex(ValueError, 'loopback'):
                self.manager.update(self.args('update', http_base='http://example.org:25774'))
            gate.assert_not_called()

    @unittest.skipUnless(shutil.which('systemd-analyze'), 'systemd-analyze unavailable')
    def test_generated_fresh_unit_passes_systemd_parser(self):
        args = self.args()
        self.root.mkdir()
        (self.root/'komari').write_bytes(self.candidate.read_bytes())
        (self.root/'komari').chmod(0o755)
        unit = self.units/'komari.service'
        unit.write_text(self.manager.service_unit(args, self.user))
        result = subprocess.run(['systemd-analyze', 'verify', str(unit)],
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_fresh_install_rejects_existing_override_before_writes(self):
        (self.units/'komari.service.d').mkdir()
        with self.assertRaisesRegex(ValueError, 'override'):
            self.manager.install(self.args())
        self.assertFalse(self.root.exists())

    def test_effective_unit_must_match_candidate_and_account(self):
        args = self.args()
        account = pwd.getpwnam(self.user)
        with mock.patch.object(self.manager, 'command', return_value=(
            'ExecStart={ path=/usr/bin/other ; argv[]=/usr/bin/other server ; }\n'
            'WorkingDirectory='+str(self.root)+'\nUser='+self.user+'\nGroup=nogroup\nRestart=always')):
            with self.assertRaisesRegex(ValueError, 'effective service'):
                self.manager.check_fresh_unit(args, account)

    def test_update_preflight_refusal_does_not_replace_gate(self):
        self.root.mkdir()
        (self.root/'komari').write_text('old')
        (self.root/'komari').chmod(0o755)
        (self.root/'data').mkdir()
        with mock.patch.object(self.manager.safe_upgrade, 'preflight', side_effect=RuntimeError('unsupported database')) as preflight, \
             mock.patch.object(self.manager.safe_upgrade, 'install_gate') as gate, \
             mock.patch.object(self.manager, 'command', return_value='ExecStart=--listen 127.0.0.1:25774'):
            with self.assertRaisesRegex(RuntimeError, 'unsupported database'):
                self.manager.update(self.args('update'))
        preflight.assert_called_once()
        gate.assert_not_called()

    def test_update_holds_lock_during_gate_and_transaction(self):
        self.root.mkdir()
        (self.root/'komari').write_text('old')
        (self.root/'komari').chmod(0o755)
        (self.root/'data').mkdir()
        args = self.args('update')
        def check_lock(_):
            with (self.state/'lock').open('a+') as lock:
                with self.assertRaises(BlockingIOError):
                    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        with mock.patch.object(self.manager.safe_upgrade, 'preflight') as preflight, \
             mock.patch.object(self.manager.safe_upgrade, 'install_gate', side_effect=check_lock) as gate, \
             mock.patch.object(self.manager.safe_upgrade, 'upgrade', side_effect=check_lock) as upgrade, \
             mock.patch.object(self.manager, 'command', return_value='ExecStart=--listen 127.0.0.1:25774'):
            self.manager.update(args)
        preflight.assert_called_once()
        gate.assert_called_once()
        upgrade.assert_called_once()


if __name__ == '__main__':
    unittest.main()
