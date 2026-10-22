"""Safety tests for the v1.0.16/33-file Glass -> v1.0.19 updater."""
import hashlib
from contextlib import closing
from http.client import HTTPException
import io
import importlib.util
import json
import os
from pathlib import Path
import socket
import sqlite3
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
            with closing(sqlite3.connect(root/'data/komari.db')) as db:
                db.execute('create table configs (key text primary key, value text)')
                db.commit()
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
                 mock.patch.object(compat,'run_bounded',side_effect=fake_run), \
                 mock.patch.object(compat,'active_panel_identity',return_value=(42,'start')) as identity, \
                 mock.patch.object(compat,'download_verified',side_effect=AssertionError('status must not download')), \
                 mock.patch.object(compat,'HTTPConnection',create=True) as connection, \
                 redirect_stdout(io.StringIO()) as output:
                connection.return_value.getresponse.return_value = response
                self.assertEqual(compat.main(['status']), 0)
                connection.assert_called_once_with('127.0.0.1', 25774, timeout=5)
                connection.return_value.request.assert_called_once_with('GET', '/api/version')
            self.assertIn('Verified',output.getvalue())
            self.assertEqual(len(calls),1)
            self.assertEqual(identity.call_count,2)
            self.assertTrue((state/'journal.json').exists())

    def test_status_rejects_other_running_executable(self):
        fields = ('LoadState=loaded\nActiveState=active\nWorkingDirectory=/opt/komari\n'
                  'User=root\nMainPID=42\nExecStart={ path=/opt/other/komari ; }\n')
        with mock.patch.object(compat,'run_bounded',return_value=mock.Mock(stdout=fields)):
            with self.assertRaisesRegex(ValueError,'service executable'):
                compat.active_panel_identity(Path('/opt/komari/releases/1.0.19/komari'))

    def test_status_rejects_custom_database_unit_before_claiming_main_db_integrity(self):
        basic = ('LoadState=loaded\nActiveState=active\nWorkingDirectory=/opt/komari\n'
                 'User=root\nMainPID=42\nExecStart={ path=/opt/komari/komari ; '
                 'argv[]=/opt/komari/komari server ; }\n')
        for fields in (basic.replace(' server ;', ' server --database /other/db.sqlite ;'),
                       basic.replace(' server ;', ' server -d /other/db.sqlite ;'),
                       basic.replace(' server ;', ' server -d/other/db.sqlite ;'),
                       basic + 'EnvironmentFiles=/etc/komari.env\n',
                       basic + 'Environment=KOMARI_LISTEN=127.0.0.1:12345\n'):
            with self.subTest(fields=fields), mock.patch.object(compat,'run_bounded',return_value=mock.Mock(stdout=fields)), \
                 mock.patch.object(compat.os.path,'samefile',return_value=True), \
                 mock.patch.object(compat.Path,'read_text',return_value='42 (komari) S ' + '0 '*19 + '123'), \
                 mock.patch.object(compat,'panel_owns_listener',return_value=True):
                with self.assertRaisesRegex(ValueError,'custom|database|environment'):
                    compat.active_panel_identity(Path('/opt/komari/releases/1.0.19/komari'))

    def test_status_rejects_old_running_binary_inode(self):
        fields = ('LoadState=loaded\nActiveState=active\nWorkingDirectory=/opt/komari\n'
                  'User=root\nMainPID=42\nExecStart={ path=/opt/komari/komari ; }\n')
        with mock.patch.object(compat,'run_bounded',return_value=mock.Mock(stdout=fields)), \
             mock.patch.object(compat.os.path,'samefile',return_value=False):
            with self.assertRaisesRegex(ValueError,'running executable'):
                compat.active_panel_identity(Path('/opt/komari/releases/1.0.19/komari'))

    def test_status_rejects_unexpected_http_json_shapes(self):
        for payload in (b'null',b'[]',b'{"data":null}',b'{"data":[]}',b'{"data":{}}'):
            with self.subTest(payload=payload), self.assertRaisesRegex(ValueError,'HTTP version'):
                compat.check_http_version_payload(payload)

    def test_listener_must_belong_to_panel_process(self):
        with socket.socket(socket.AF_INET,socket.SOCK_STREAM) as server:
            server.bind(('127.0.0.1',0)); server.listen(1)
            port = server.getsockname()[1]
            self.assertTrue(compat.panel_owns_listener(os.getpid(),port))
        self.assertFalse(compat.panel_owns_listener(os.getpid(),port))

    def test_status_rejects_corrupt_main_sqlite_without_printing_rows(self):
        with tempfile.TemporaryDirectory() as tmp:
            data = Path(tmp)/'data'; data.mkdir()
            (data/'komari.db').write_bytes(b'not sqlite')
            with self.assertRaises((ValueError, sqlite3.DatabaseError)):
                compat.check_status_databases(Path(tmp))

    def test_status_rejects_external_metrics_database_configuration(self):
        with tempfile.TemporaryDirectory() as tmp:
            data = Path(tmp)/'data'; data.mkdir()
            with closing(sqlite3.connect(data/'komari.db')) as db:
                db.execute('create table configs (key text primary key, value text)')
                db.execute('insert into configs values (?,?)', ('metric_db_driver', '"postgres"'))
                db.commit()
            with self.assertRaisesRegex(ValueError,'metrics|external'):
                compat.check_status_databases(Path(tmp))

    def test_status_reports_http_protocol_failure_without_traceback(self):
        with mock.patch.object(compat,'verify_status',side_effect=HTTPException('invalid HTTP')):
            self.assertEqual(compat.main(['status']),1)

    def test_status_rejects_incomplete_transaction_before_claiming_success(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)/'panel'; root.mkdir(); (root/'komari').write_bytes(b'fixture')
            state = Path(tmp)/'state'; state.mkdir()
            (state/'journal.json').write_text('{"phase":"starting","target":"1.0.19"}')
            with mock.patch.object(compat,'ROOT',root), mock.patch.object(compat,'STATE',state), \
                 mock.patch.object(compat,'trusted_script'), \
                 mock.patch.object(compat,'trusted_file'), mock.patch.object(compat,'run_bounded',
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

    def test_streaming_cap_is_checked_before_any_disk_write(self):
        with tempfile.TemporaryFile() as destination:
            with self.assertRaisesRegex(ValueError, 'stdout size limit'):
                compat.run_bounded([compat.sys.executable, '-c',
                                    'import os; os.write(1,b"x"*8192)'],
                                   output=destination, stdout_limit=4096)
            self.assertLessEqual(destination.tell(), 4096)
            self.assertLessEqual(os.fstat(destination.fileno()).st_size, 4096)

    def test_bounded_capture_accepts_exact_limits_on_both_pipes(self):
        result = compat.run_bounded([compat.sys.executable, '-c',
                                    'import os; os.write(1,b"x"*65536); os.write(2,b"y"*65536)'])
        self.assertEqual(result.stdout, 'x'*65536)
        self.assertEqual(result.stderr, 'y'*65536)

    def test_bounded_capture_rejects_either_pipe_flood_without_echoing_output(self):
        original_popen = subprocess.Popen
        for fd, stream in ((1, 'stdout'), (2, 'stderr')):
            processes = []
            def launch(*args, **kwargs):
                process = original_popen(*args, **kwargs)
                processes.append(process)
                return process
            with self.subTest(stream=stream), \
                 mock.patch.object(compat.subprocess, 'Popen', side_effect=launch):
                with self.assertRaisesRegex(ValueError, stream + ' size limit') as caught:
                    compat.run_bounded([compat.sys.executable, '-c',
                                        f'import os; os.write({fd}, b"PRIVATE"*1000000)'])
                self.assertNotIn('PRIVATE', str(caught.exception))
                self.assertIsNotNone(processes[0].returncode)

    def test_bounded_capture_deadline_covers_idle_closed_and_inherited_pipes(self):
        import time
        for code in ('import time; time.sleep(30)',
                     'import os,time; os.close(1); os.close(2); time.sleep(30)',
                     'import os,time; pid=os.fork(); time.sleep(30) if pid==0 else None'):
            with self.subTest(code=code):
                start = time.monotonic()
                with self.assertRaisesRegex(ValueError, 'deadline'):
                    compat.run_bounded([compat.sys.executable, '-c', code], timeout=0.15)
                self.assertLess(time.monotonic() - start, 3)

    def test_failed_download_removes_only_its_own_partial_artifact(self):
        for body in (b'short', b'wrong-hash', b'x'*8192):
            with self.subTest(body_size=len(body)), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                downloader = root/'curl'
                downloader.write_text('#!/usr/bin/python3\nimport sys\nsys.stdout.buffer.write(' + repr(body) + ')\n')
                downloader.chmod(0o700)
                with mock.patch.object(compat, 'ASSETS', {'fixture': ('0'*64, 10, 'https://example.invalid/a')}), \
                     mock.patch.object(compat, 'trusted_downloader', return_value=('curl', str(downloader))):
                    with self.assertRaises(ValueError):
                        compat.download_verified('fixture', root)
                    self.assertFalse((root/'fixture').exists())
                    (root/'original').write_bytes(b'preserve')
                    (root/'fixture').symlink_to(root/'original')
                    with self.assertRaises(FileExistsError):
                        compat.download_verified('fixture', root)
                    self.assertTrue((root/'fixture').is_symlink())
                    self.assertEqual((root/'original').read_bytes(), b'preserve')

    def test_manager_version_capture_rejects_stdout_flood(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            binary = root/'komari'
            binary.write_text('#!/usr/bin/python3\nimport os\nos.write(1, b"x" * 1000000)\n')
            binary.chmod(0o700)
            with mock.patch.object(compat, 'ROOT', root):
                with self.assertRaisesRegex(ValueError, 'stdout size limit'):
                    compat.run_manager(root)

    def test_manager_failure_does_not_echo_host_identity_command_arguments(self):
        with tempfile.TemporaryDirectory() as tmp:
            stage = Path(tmp); (stage/'komari').write_bytes(b'fixture')
            result = mock.Mock(stdout='{"version":"1.0.19"}')
            failure = subprocess.CalledProcessError(1, ['manager', '--machine-id', 'HOST-IDENTITY'])
            with mock.patch.object(compat,'ROOT',stage), \
                 mock.patch.object(compat,'compare_stock',return_value=(True,33)), \
                 mock.patch.object(compat,'check_persistent_mounts'), \
                 mock.patch.object(compat,'run_bounded',return_value=result), \
                 mock.patch.object(compat.subprocess,'run',side_effect=failure):
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
            downloader = Path(tmp)/'curl'
            downloader.write_text('#!/usr/bin/python3\nimport sys\nsys.stdout.buffer.write(' + repr(payload) + ')\n')
            downloader.chmod(0o700)
            with mock.patch.object(compat, 'ASSETS', assets), \
                 mock.patch.object(compat, 'trusted_downloader', return_value=('curl', str(downloader))):
                target = compat.download_verified('komari', tmp)
            self.assertEqual(stat.S_IMODE(target.stat().st_mode), 0o755)

    def test_downloader_ignores_user_configuration(self):
        for tool, flag in (('curl', '-q'), ('wget', '--no-config')):
            with self.subTest(tool=tool), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                executable = root/'downloader'
                executable.write_text('#!/usr/bin/python3\nimport sys\n'
                                      'sys.exit(0 if sys.argv[1] == ' + repr(flag) + ' else 1)\n')
                executable.chmod(0o700)
                assets = {'fixture': (hashlib.sha256(b'').hexdigest(), 0, 'https://example.invalid/object')}
                with mock.patch.object(compat, 'ASSETS', assets), \
                     mock.patch.object(compat, 'trusted_downloader', return_value=(tool, str(executable))), \
                     mock.patch.object(compat, 'run_bounded', wraps=compat.run_bounded) as runner:
                    try:
                        compat.download_verified('fixture', root)
                    except ValueError:
                        pass  # wget fixture intentionally omits HTTP headers
                self.assertEqual(runner.call_args.args[0][1], flag)

    def test_non_https_asset_is_rejected_before_downloader(self):
        with tempfile.TemporaryDirectory() as tmp, \
             mock.patch.object(compat, 'ASSETS', {'fixture': ('0'*64, 10, 'http://127.0.0.1/object')}), \
             mock.patch.object(compat, 'trusted_downloader') as downloader:
            with self.assertRaisesRegex(ValueError, 'HTTPS'):
                compat.download_verified('fixture', tmp)
            downloader.assert_not_called()

    def test_wget_https_redirect_never_contacts_plain_http(self):
        import http.server
        import ssl
        import threading
        import shutil
        wget = shutil.which('wget')
        if not wget:
            self.skipTest('wget unavailable')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes',
                            '-keyout', str(root/'key'), '-out', str(root/'cert'),
                            '-days', '1', '-subj', '/CN=localhost'],
                           check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            contacted = []
            class Handler(http.server.BaseHTTPRequestHandler):
                def do_GET(self):
                    if self.path == '/redirect':
                        self.send_response(302)
                        self.send_header('Location', f'http://127.0.0.1:{plain.server_port}/insecure')
                    elif self.path == '/safe':
                        self.send_response(307)
                        self.send_header('Location', '/object')
                    elif self.path == '/loop':
                        self.send_response(302)
                        self.send_header('Location', '/loop')
                    else:
                        contacted.append(self.path)
                        self.send_response(200)
                        self.send_header('Content-Length', '10' if self.path == '/object' else '0')
                    self.end_headers()
                    if self.path == '/object':
                        self.wfile.write(b'valid-body')
                def log_message(self, format, *args):
                    pass
            plain = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
            server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
            ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            ctx.load_cert_chain(root/'cert', root/'key')
            server.socket = ctx.wrap_socket(server.socket, server_side=True)
            threads = [threading.Thread(target=s.serve_forever, daemon=True) for s in (plain, server)]
            for thread in threads: thread.start()
            wrapper = root/'wget'
            wrapper.write_text('#!/bin/sh\nexec ' + wget + ' --no-proxy --no-check-certificate "$@"\n')
            wrapper.chmod(0o700)
            assets = {'fixture': ('0'*64, 4096, f'https://127.0.0.1:{server.server_port}/redirect')}
            try:
                with mock.patch.object(compat, 'ASSETS', assets), \
                     mock.patch.object(compat, 'trusted_downloader', return_value=('wget', str(wrapper))):
                    with self.assertRaises(ValueError):
                        compat.download_verified('fixture', root)
                self.assertEqual(contacted, [], 'wget followed an HTTPS downgrade')
                for route in ('safe', 'loop'):
                    assets['fixture'] = (hashlib.sha256(b'valid-body').hexdigest(), 10,
                                         f'https://127.0.0.1:{server.server_port}/{route}')
                    with mock.patch.object(compat, 'ASSETS', assets), \
                         mock.patch.object(compat, 'trusted_downloader', return_value=('wget', str(wrapper))):
                        if route == 'safe':
                            result = compat.download_verified('fixture', root)
                            self.assertEqual(result.read_bytes(), b'valid-body')
                            self.assertEqual(stat.S_IMODE(result.stat().st_mode), 0o600)
                            result.unlink()
                        else:
                            with self.assertRaisesRegex(ValueError, 'redirect limit'):
                                compat.download_verified('fixture', root)
                            self.assertFalse((root/'fixture').exists())
            finally:
                for s in (plain, server): s.shutdown(); s.server_close()
                for thread in threads: thread.join()

    def test_chunked_https_download_never_writes_beyond_pin(self):
        # Reproduce the residual probe with real tools and no Content-Length.
        import http.server
        import ssl
        import threading
        for tool in ('curl', 'wget'):
            executable = __import__('shutil').which(tool)
            if not executable:
                self.skipTest(tool + ' unavailable')
            with self.subTest(tool=tool), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes',
                                '-keyout', str(root/'key'), '-out', str(root/'cert'),
                                '-days', '1', '-subj', '/CN=localhost'],
                               check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                class Handler(http.server.BaseHTTPRequestHandler):
                    protocol_version = 'HTTP/1.1'
                    def do_GET(self):
                        self.send_response(200)
                        self.send_header('Transfer-Encoding', 'chunked')
                        self.end_headers()
                        self.wfile.write(b'2000\r\n' + b'x'*8192 + b'\r\n0\r\n\r\n')
                    def log_message(self, format, *args):
                        pass
                server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
                ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
                ctx.load_cert_chain(root/'cert', root/'key')
                server.socket = ctx.wrap_socket(server.socket, server_side=True)
                thread = threading.Thread(target=server.serve_forever, daemon=True)
                thread.start()
                wrapper = root/'downloader'
                # Certificate bypass is loopback-fixture-only, never production.
                extra = '--insecure --noproxy *' if tool == 'curl' else '--no-check-certificate --no-proxy'
                wrapper.write_text('#!/bin/sh\nexec ' + executable + ' ' + extra.replace('*', "'*'") + ' "$@"\n')
                wrapper.chmod(0o700)
                assets = {'fixture': ('0'*64, 4096, f'https://127.0.0.1:{server.server_port}/asset')}
                try:
                    with mock.patch.object(compat, 'ASSETS', assets), \
                         mock.patch.object(compat, 'trusted_downloader', return_value=(tool, str(wrapper))):
                        with self.assertRaises((ValueError, subprocess.CalledProcessError)):
                            compat.download_verified('fixture', root)
                    target = root/'fixture'
                    self.assertTrue(not target.exists() or target.stat().st_size <= 4096,
                                    'chunked body exceeded pinned on-disk size')
                finally:
                    server.shutdown(); server.server_close(); thread.join()

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

    def test_preflight_rejects_short_database_and_driver_flags(self):
        fields = {'LoadState': 'loaded', 'ActiveState': 'active',
                  'WorkingDirectory': '/opt/komari', 'User': 'root',
                  'Restart': 'always'}
        for args in ('-d /srv/custom.db', '-d/srv/custom.db', '-t mysql',
                     '--database=/srv/custom.db', '--db-type mysql'):
            fields['ExecStart'] = '{ path=/opt/komari/komari ; argv[]=/opt/komari/komari server ' + args + ' ; }'
            with self.subTest(args=args), self.assertRaisesRegex(ValueError, 'database|driver'):
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
        expected_commits = {'safe_upgrade.py': '41fff50fd0926d4b2cd7b035816bcbb83f688fa0',
                            'Glass-33-to-38-v1.0.19.zip': '0216e11e2b120abde41c98b7887ee9c5fd209db3'}
        for name in expected_commits:
            target = SCRIPT.parent/'compat'/name
            digest, size, url = compat.ASSETS[name]
            self.assertEqual(digest, hashlib.sha256(target.read_bytes()).hexdigest())
            self.assertEqual(size, target.stat().st_size)
            self.assertIn('/' + expected_commits[name] + '/deploy/compat/', url)

if __name__ == '__main__': unittest.main()
