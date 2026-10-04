"""Safety tests for the v1.0.16/33-file Glass -> v1.0.19 updater."""
import hashlib
import io
import importlib.util
import json
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest
from unittest import mock
import zipfile
from contextlib import redirect_stdout

SCRIPT = Path(__file__).with_name('komari-compat-v1019.py')
spec = importlib.util.spec_from_file_location('komari_compat_v1019', SCRIPT)
compat = importlib.util.module_from_spec(spec)

class CompatTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec.loader.exec_module(compat)

    def test_complete_stock_required_extra_and_changed_files_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp); theme = root/'Glass'; theme.mkdir()
            stock = {}
            for number in range(33):
                name = f'stock-{number}.js'
                content = f'original-{number}'.encode()
                (theme/name).write_bytes(content)
                stock[name] = hashlib.sha256(content).hexdigest()
            bundle = root/'bundle.zip'
            with zipfile.ZipFile(bundle,'w') as z: z.writestr('stock.json',json.dumps(stock))
            self.assertEqual(compat.compare_stock(theme,bundle),(True,33))
            (theme/'custom.js').write_bytes(b'custom')
            self.assertEqual(compat.compare_stock(theme,bundle),(False,34))
            (theme/'custom.js').unlink()
            (theme/'stock-0.js').write_bytes(b'altered')
            self.assertEqual(compat.compare_stock(theme,bundle),(False,33))

    def test_symlink_theme_file_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); theme=root/'Glass'; theme.mkdir()
            (root/'outside').write_bytes(b'secret')
            (theme/'stock.js').symlink_to(root/'outside')
            bundle=root/'bundle.zip'
            with zipfile.ZipFile(bundle,'w') as z: z.writestr('stock.json','{"stock.js":"'+hashlib.sha256(b'secret').hexdigest()+'"}')
            with self.assertRaises(ValueError): compat.compare_stock(theme,bundle)

    def test_version_mismatch_aborts_before_any_download_or_manager(self):
        with mock.patch.object(compat,'preflight',side_effect=ValueError('old version mismatch')), \
             mock.patch.object(compat,'download_verified') as download, \
             mock.patch.object(compat,'run_manager') as manager:
            self.assertEqual(compat.main(['update']),1)
            download.assert_not_called(); manager.assert_not_called()

    def test_matching_check_does_not_run_upgrade(self):
        with mock.patch.object(compat,'preflight',return_value=True), \
             mock.patch.object(compat,'run_manager') as manager:
            self.assertEqual(compat.main(['check']),0)
            manager.assert_not_called()

    def test_status_checks_committed_version_service_http_and_exact_target_theme(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)/'panel'; theme = root/'data/theme/Glass'; theme.mkdir(parents=True)
            with zipfile.ZipFile(SCRIPT.parent/'compat/Glass-33-to-38-v1.0.19.zip') as archive:
                for name in archive.namelist():
                    if name.startswith('Glass/') and not name.endswith('/'):
                        dest = theme/name[len('Glass/'):]
                        dest.parent.mkdir(parents=True, exist_ok=True)
                        dest.write_bytes(archive.read(name))
            (root/'komari').write_text('fixture')
            state = Path(tmp)/'state'; state.mkdir()
            (state/'journal.json').write_text(json.dumps({'phase':'committed', 'target':'1.0.19'}))
            calls = []
            def fake_run(argv, **kwargs):
                calls.append(argv)
                return mock.Mock(stdout=json.dumps({'version':'1.0.19'}))
            response = mock.Mock(status=200)
            response.read.return_value = b'{"data":{"version":"1.0.19"}}'
            with mock.patch.object(compat,'ROOT',root), mock.patch.object(compat,'STATE',state), \
                 mock.patch.object(compat,'trusted_script'), mock.patch.object(compat,'trusted_file'), \
                 mock.patch.object(compat.subprocess,'run',side_effect=fake_run), \
                 mock.patch.object(compat,'download_verified',side_effect=AssertionError('status must not download')), \
                 mock.patch.object(compat,'HTTPConnection',create=True) as connection, \
                 redirect_stdout(io.StringIO()) as output:
                connection.return_value.getresponse.return_value = response
                self.assertEqual(compat.main(['status']), 0)
                connection.assert_called_once_with('127.0.0.1', 25774, timeout=5)
            self.assertIn('Verified',output.getvalue())
            self.assertEqual(len(calls),2)
            self.assertTrue((state/'journal.json').exists())

    def test_status_rejects_incomplete_transaction_before_claiming_success(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)/'panel'; root.mkdir(); (root/'komari').write_bytes(b'fixture')
            state = Path(tmp)/'state'; state.mkdir()
            (state/'journal.json').write_text('{"phase":"starting","target":"1.0.19"}')
            with mock.patch.object(compat,'ROOT',root), mock.patch.object(compat,'STATE',state), \
                 mock.patch.object(compat,'trusted_script'), \
                 mock.patch.object(compat,'trusted_file'), mock.patch.object(compat.subprocess,'run',
                 return_value=mock.Mock(stdout='{"version":"1.0.19"}')), \
                 mock.patch.object(compat,'download_verified') as download:
                self.assertEqual(compat.main(['status']),1)
                download.assert_not_called()

    def test_status_rejects_changed_target_theme(self):
        with tempfile.TemporaryDirectory() as tmp:
            theme = Path(tmp)/'Glass'; theme.mkdir()
            (theme/'modified.js').write_bytes(b'not official')
            with self.assertRaisesRegex(ValueError,'Glass'):
                compat.check_target_theme(theme)

    def test_manager_failure_does_not_echo_host_identity_command_arguments(self):
        with tempfile.TemporaryDirectory() as tmp:
            stage = Path(tmp); (stage/'komari').write_bytes(b'fixture')
            result = mock.Mock(stdout='{"version":"1.0.19"}')
            failure = subprocess.CalledProcessError(1, ['manager', '--machine-id', 'HOST-IDENTITY'])
            with mock.patch.object(compat,'ROOT',stage), \
                 mock.patch.object(compat,'compare_stock',return_value=(True,33)), \
                 mock.patch.object(compat,'check_persistent_mounts'), \
                 mock.patch.object(compat.subprocess,'run',side_effect=[result,failure]):
                with self.assertRaises(ValueError) as caught:
                    compat.run_manager(stage)
            self.assertNotIn('HOST-IDENTITY',str(caught.exception))

    def test_update_requires_interactive_confirmation_before_download(self):
        with mock.patch.object(compat,'preflight',return_value=True), \
             mock.patch.object(compat,'trusted_script',return_value=True), \
             mock.patch.object(compat.sys.stdin,'isatty',return_value=False), \
             mock.patch.object(compat,'download_verified') as download:
            self.assertEqual(compat.main(['update']),1)
            download.assert_not_called()

    def test_asset_pins_and_version_explicit(self):
        self.assertEqual(compat.TAG,'v1.0.19')
        self.assertEqual(compat.OLD_VERSION,'1.0.16')
        self.assertEqual(set(compat.ASSETS),{'komari','komari-manager.py','safe_upgrade.py','Glass-33-to-38-v1.0.19.zip'})
        for digest, size, url in compat.ASSETS.values():
            self.assertRegex(digest,r'^[0-9a-f]{64}$')
            self.assertGreater(size,100)
            self.assertTrue(url.startswith('https://'))

    def test_candidate_binary_remains_executable_by_non_root_service(self):
        with tempfile.TemporaryDirectory() as tmp:
            payload = b'verified-candidate'
            expected = hashlib.sha256(payload).hexdigest()
            assets = dict(compat.ASSETS)
            assets['komari'] = (expected, len(payload), 'https://example.invalid/komari')
            def fake_download(args, **kwargs):
                Path(args[args.index('--output') + 1]).write_bytes(payload)
            with mock.patch.object(compat, 'ASSETS', assets), \
                 mock.patch.object(compat, 'trusted_downloader', return_value=('curl', '/usr/bin/curl')), \
                 mock.patch.object(compat.subprocess, 'run', side_effect=fake_download):
                target = compat.download_verified('komari', tmp)
            self.assertEqual(stat.S_IMODE(target.stat().st_mode), 0o755)

    def test_nested_bind_mount_in_data_is_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)/'panel'
            (root/'data'/'external').mkdir(parents=True)
            mountinfo = Path(tmp)/'mountinfo'
            mountinfo.write_text('23 1 8:1 / ' + str(root/'data'/'external') + ' rw - ext4 /dev/sda1 rw\n')
            with self.assertRaisesRegex(ValueError, 'mount'):
                compat.check_persistent_mounts(root, mountinfo)

    def test_systemd_private_or_bind_mount_view_is_rejected(self):
        fields = {'RootDirectory':'', 'RootImage':'', 'BindPaths':'',
                  'BindReadOnlyPaths':'', 'TemporaryFileSystem':'',
                  'PrivateMounts':'no', 'JoinsNamespaceOf':'',
                  'ExtensionImages':'', 'ExtensionDirectories':'',
                  'ProtectSystem':'full', 'ReadWritePaths':'/opt/komari'}
        compat.check_unit_mount_view(fields)
        for property_name, value in (('PrivateMounts','yes'),
                                     ('BindPaths','/mnt/other:/opt/komari/data'),
                                     ('RootDirectory','/srv/chroot')):
            other = dict(fields, **{property_name: value})
            with self.subTest(property_name=property_name), self.assertRaisesRegex(ValueError, 'mount|namespace'):
                compat.check_unit_mount_view(other)

    def test_existing_root_run_service_layout_is_accepted_without_user_migration(self):
        fields = {'LoadState': 'loaded', 'ActiveState': 'active',
                  'WorkingDirectory': '/opt/komari', 'User': 'root',
                  'Restart': 'always',
                  'ExecStart': '{ path=/opt/komari/komari ; argv[]=... }'}
        compat.check_standard_unit_layout(fields)

    def test_root_layout_does_not_allow_inactive_or_missing_service_identity(self):
        fields = {'LoadState': 'loaded', 'ActiveState': 'active',
                  'WorkingDirectory': '/opt/komari', 'User': 'root',
                  'Restart': 'always',
                  'ExecStart': '{ path=/opt/komari/komari ; argv[]=... }'}
        for changes in ({'ActiveState': 'inactive'}, {'User': ''},
                        {'ExecStart': '{ path=/other/komari ; }'}):
            with self.subTest(changes=changes), self.assertRaisesRegex(ValueError, 'unsupported unit'):
                compat.check_standard_unit_layout(dict(fields, **changes))

    def test_custom_controller_and_theme_pins_match_checked_in_artifacts(self):
        expected_commits = {'safe_upgrade.py': '0216e11e2b120abde41c98b7887ee9c5fd209db3',
                            'Glass-33-to-38-v1.0.19.zip': '0216e11e2b120abde41c98b7887ee9c5fd209db3'}
        for name in expected_commits:
            target = SCRIPT.parent/'compat'/name
            digest, size, url = compat.ASSETS[name]
            self.assertEqual(digest, hashlib.sha256(target.read_bytes()).hexdigest())
            self.assertEqual(size, target.stat().st_size)
            self.assertIn('/' + expected_commits[name] + '/deploy/compat/', url)

if __name__ == '__main__': unittest.main()
