"""Regression checks for the pinned compatibility recovery controller."""
import hashlib
import importlib.util
from pathlib import Path
import stat
import tempfile
import types
import unittest
from unittest import mock
import zipfile

SCRIPT = Path(__file__).with_name('compat')/'safe_upgrade.py'
spec = importlib.util.spec_from_file_location('compat_safe_upgrade', SCRIPT)
controller = importlib.util.module_from_spec(spec)

class CompatControllerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec.loader.exec_module(controller)

    def test_preflight_rejects_nested_mount_before_snapshot(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)/'panel'
            (root/'data').mkdir(parents=True)
            mountinfo = Path(tmp)/'mountinfo'
            mountinfo.write_text('23 1 8:1 / ' + str(root/'data') + ' rw - ext4 /dev/sda1 rw\n')
            with self.assertRaisesRegex(RuntimeError, 'mount'):
                controller.check_persistent_mounts(root, mountinfo)

    def test_recheck_after_stop_blocks_snapshot_of_new_mount(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)/'panel'; state = Path(tmp)/'state'; binary = Path(tmp)/'candidate'
            (root/'releases').mkdir(parents=True)
            state.mkdir()
            binary.write_bytes(b'candidate')
            args = types.SimpleNamespace(root=str(root), systemctl='/usr/bin/true',
                                         service='komari.service', theme_sha256=None,
                                         sha256=hashlib.sha256(b'candidate').hexdigest())
            with mock.patch.object(controller, 'preflight', return_value=(root,state,binary,'1.0.19',1,None)), \
                 mock.patch.object(controller, 'info', return_value={'version':'1.0.16'}), \
                 mock.patch.object(controller, 'stop') as stop, \
                 mock.patch.object(controller, 'check_persistent_mounts', side_effect=RuntimeError('mounted data')) as mounts, \
                 mock.patch.object(controller, 'copy_tree') as snapshot, \
                 mock.patch.object(controller, 'save'), \
                 mock.patch.object(controller, 'command'), \
                 mock.patch.object(controller, 'start'), \
                 mock.patch.object(controller, 'health'):
                with self.assertRaisesRegex(RuntimeError, 'mounted data'):
                    controller.upgrade(args)
            stop.assert_called_once()
            mounts.assert_called()
            snapshot.assert_not_called()

    def test_restore_rejects_mount_before_replacing_live_data(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)/'panel'; root.mkdir()
            (root/'data').mkdir()
            args = types.SimpleNamespace(root=str(root), state_dir=str(Path(tmp)/'state'))
            with mock.patch.object(controller, 'state_dir', return_value=Path(tmp)/'state'), \
                 mock.patch.object(controller, 'verify_snapshot', return_value=Path(tmp)/'backup'), \
                 mock.patch.object(controller, 'check_persistent_mounts', side_effect=RuntimeError('mounted data')), \
                 mock.patch.object(controller, 'stop') as stop:
                with self.assertRaisesRegex(RuntimeError, 'mounted data'):
                    controller.restore(args, {'id':'20260101T000000-12345678'})
            stop.assert_not_called()
            self.assertTrue((root/'data').is_dir())

    def test_controller_accepts_existing_root_service_without_changing_unit(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)/'panel'; root.mkdir()
            args = types.SimpleNamespace(root=str(root), state_dir=str(Path(tmp)/'state'), systemctl='/usr/bin/systemctl', service='komari.service')
            fields = ('WorkingDirectory=' + str(root) + '\nUser=root\nRestart=always\n'
                      'ExecStart={ path=' + str(root/'komari') + ' ; argv[]=... }\n')
            with mock.patch.object(controller, 'command', return_value=fields):
                controller.check_gate(args, require_gate=False)

    def test_controller_still_rejects_missing_service_identity(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)/'panel'; root.mkdir()
            args = types.SimpleNamespace(root=str(root), state_dir=str(Path(tmp)/'state'), systemctl='/usr/bin/systemctl', service='komari.service')
            fields = ('WorkingDirectory=' + str(root) + '\nUser=\nRestart=always\n'
                      'ExecStart={ path=' + str(root/'komari') + ' ; argv[]=... }\n')
            with mock.patch.object(controller, 'command', return_value=fields):
                with self.assertRaisesRegex(RuntimeError, 'service user'):
                    controller.check_gate(args, require_gate=False)

    def test_root_owned_data_snapshot_and_private_glass_keep_ownership_and_modes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)/'panel'; theme = root/'data/theme/Glass'; theme.mkdir(parents=True)
            db = root/'data/komari.db'; db.write_bytes(b'database fixture')
            db.chmod(0o644); theme.chmod(0o700)
            (theme/'komari-theme.json').write_text('{}')
            (theme/'komari-theme.json').chmod(0o600)
            (theme/'dist').mkdir(mode=0o700)
            bundle = Path(tmp)/'Glass.zip'
            with zipfile.ZipFile(bundle, 'w') as archive:
                archive.writestr('Glass/komari-theme.json', '{}')
                archive.writestr('Glass/dist/index.html', '<html></html>')
            snapshot = Path(tmp)/'snapshot'
            controller.copy_tree(root/'data', snapshot)
            for original, saved in ((db, snapshot/'komari.db'),
                                    (theme, snapshot/'theme/Glass'),
                                    (theme/'komari-theme.json', snapshot/'theme/Glass/komari-theme.json')):
                self.assertEqual((original.stat().st_uid, original.stat().st_gid,
                                  stat.S_IMODE(original.stat().st_mode)),
                                 (saved.stat().st_uid, saved.stat().st_gid,
                                  stat.S_IMODE(saved.stat().st_mode)))
            controller.apply_theme(root, bundle)
            self.assertEqual(stat.S_IMODE(theme.stat().st_mode), 0o700)
            self.assertEqual((theme/'komari-theme.json').stat().st_uid, theme.stat().st_uid)
            self.assertEqual(db.read_bytes(), b'database fixture')

if __name__ == '__main__': unittest.main()