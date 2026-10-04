"""Regression checks for the pinned compatibility recovery controller."""
import hashlib
import importlib.util
from pathlib import Path
import tempfile
import types
import unittest
from unittest import mock

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

if __name__ == '__main__': unittest.main()