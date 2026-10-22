"""Black-box bootstrap probes using real curl/wget and loopback TLS only."""
import hashlib
import http.server
import os
from pathlib import Path
import re
import shutil
import ssl
import subprocess
import tempfile
import threading
import unittest

BOOTSTRAP = Path(__file__).parent / 'compat/upgrade.sh'


@unittest.skipUnless(os.geteuid() == 0 and shutil.which('openssl'), 'root and openssl required')
class BootstrapNetworkTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.root = Path(cls.tmp.name)
        cls.cert, cls.key = cls.root / 'cert.pem', cls.root / 'key.pem'
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes',
                        '-keyout', str(cls.key), '-out', str(cls.cert), '-days', '1',
                        '-subj', '/CN=localhost', '-addext', 'subjectAltName=IP:127.0.0.1'],
                       check=True, capture_output=True)
        cls.payload = b'print("LOCAL_VERIFIED_HELPER")\n' + b'# padding\n' * 400
        cls.requests = []

        class Handler(http.server.BaseHTTPRequestHandler):
            protocol_version = 'HTTP/1.1'

            def do_GET(self):
                cls.requests.append((isinstance(self.connection, ssl.SSLSocket), self.path, self.headers.get('X-Config')))
                if self.path == '/downgrade':
                    self.send_response(302)
                    self.send_header('Location', cls.http_url + '/helper')
                elif self.path == '/relative':
                    self.send_response(302)
                    self.send_header('Location', '/helper')
                elif self.path == '/loop':
                    self.send_response(302)
                    self.send_header('Location', '/loop')
                else:
                    self.send_response(200)
                self.send_header('Transfer-Encoding', 'chunked')
                self.send_header('Connection', 'close')
                self.close_connection = True
                self.end_headers()
                if self.path in ('/helper', '/oversize', '/wronghash'):
                    try:
                        body = cls.payload if self.path == '/helper' else (b'x' * (len(cls.payload) * 8) if self.path == '/oversize' else b'x' * len(cls.payload))
                        self.wfile.write(('%x\r\n' % len(body)).encode() + body + b'\r\n0\r\n\r\n')
                    except (BrokenPipeError, ConnectionResetError):
                        pass
                else:
                    self.wfile.write(b'0\r\n\r\n')

            def log_message(self, format, *args):
                pass

        cls.servers, cls.threads = [], []
        for tls in (False, True):
            server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
            if tls:
                context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
                context.load_cert_chain(cls.cert, cls.key)
                server.socket = context.wrap_socket(server.socket, server_side=True)
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            cls.servers.append(server)
            cls.threads.append(thread)
        cls.http_url = 'http://127.0.0.1:' + str(cls.servers[0].server_port)
        cls.https_url = 'https://127.0.0.1:' + str(cls.servers[1].server_port)

    @classmethod
    def tearDownClass(cls):
        for server in cls.servers:
            server.shutdown()
            server.server_close()
        for thread in cls.threads:
            thread.join()
        cls.tmp.cleanup()

    def run_bootstrap(self, tool, route='/helper', hostile_config=False):
        if not Path('/usr/bin/' + tool).exists():
            self.skipTest(tool + ' unavailable')
        self.requests.clear()
        with tempfile.TemporaryDirectory(dir=self.root) as work:
            work = Path(work)
            # Wrappers add only our test CA; the actual transfer is the system binary.
            wrapper = work / tool
            ca_flag = '--cacert' if tool == 'curl' else '--ca-certificate'
            wrapper.write_text('#!/bin/bash\nexec /usr/bin/' + tool + ' "$1" ' + ca_flag + ' ' + str(self.cert) + ' "${@:2}"\n')
            wrapper.chmod(0o700)
            source = BOOTSTRAP.read_text()
            source = re.sub(r"^helper_url=.*$", "helper_url='" + self.https_url + route + "'", source, flags=re.M)
            source = re.sub(r"^helper_sha=.*$", "helper_sha='" + hashlib.sha256(self.payload).hexdigest() + "'", source, flags=re.M)
            source = re.sub(r'^helper_size=.*$', 'helper_size=' + str(len(self.payload)), source, flags=re.M)
            source = source.replace('/var/lib/komari-compat.', str(work / 'stage.'))
            source = source.replace('/usr/bin/' + tool, str(wrapper))
            if tool == 'wget':
                source = source.replace('/usr/bin/curl', str(work / 'absent-curl'))
            script = work / 'upgrade.sh'
            # Observe the real partial file immediately before normal cleanup.
            cleanup_probe = work / 'cleanup-probe'
            sizes = work / 'download-sizes'
            cleanup_probe.write_text('#!/bin/bash\nfor path in "$@"; do\n'
                                     ' if [[ -f "$path" ]]; then /usr/bin/stat -c %s -- "$path" >> "' + str(sizes) + '"; fi\n'
                                     'done\nexec /usr/bin/rm "$@"\n')
            cleanup_probe.chmod(0o700)
            source = source.replace('/usr/bin/rm ', str(cleanup_probe) + ' ')
            script.write_text(source)
            env = dict(os.environ, HOME=str(work), CURL_HOME=str(work), XDG_CONFIG_HOME=str(work),
                       WGETRC=str(work / '.wgetrc'), no_proxy='127.0.0.1', NO_PROXY='127.0.0.1')
            (work / '.wgetrc').write_text('')
            if hostile_config:
                (work / '.curlrc').write_text('header = "X-Config: loaded"\n')
                (work / '.wgetrc').write_text('header = X-Config: loaded\n')
            result = subprocess.run(['bash', str(script), 'status'], env=env, capture_output=True, text=True, timeout=15)
            self.assertEqual(list(work.glob('stage.*')), [], 'bootstrap leaked private staging files')
            if route in ('/helper', '/relative', '/oversize', '/wronghash'):
                self.assertTrue(sizes.exists(), 'cleanup must observe the downloaded object')
            if sizes.exists():
                for size in sizes.read_text().splitlines():
                    self.assertLessEqual(int(size), len(self.payload), 'helper exceeded its streaming disk ceiling')
            return result

    def test_curl_ignores_user_configuration(self):
        result = self.run_bootstrap('curl', hostile_config=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('LOCAL_VERIFIED_HELPER', result.stdout)
        self.assertTrue(self.requests)
        self.assertTrue(all(header is None for _, _, header in self.requests), self.requests)


    def test_wget_ignores_user_configuration(self):
        result = self.run_bootstrap('wget', hostile_config=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('LOCAL_VERIFIED_HELPER', result.stdout)
        self.assertTrue(self.requests)
        self.assertTrue(all(header is None for _, _, header in self.requests), self.requests)


    def test_wget_never_contacts_a_downgraded_redirect(self):
        result = self.run_bootstrap('wget', '/downgrade')
        self.assertFalse(any(not tls for tls, _, _ in self.requests), self.requests)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('LOCAL_VERIFIED_HELPER', result.stdout)


    def test_https_relative_redirects_preserve_verified_execution(self):
        for tool in ('curl', 'wget'):
            with self.subTest(tool=tool):
                result = self.run_bootstrap(tool, '/relative')
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn('LOCAL_VERIFIED_HELPER', result.stdout)
                self.assertEqual([path for _, path, _ in self.requests], ['/relative', '/helper'])
                self.assertTrue(all(tls for tls, _, _ in self.requests))

    def test_curl_never_contacts_a_downgraded_redirect(self):
        result = self.run_bootstrap('curl', '/downgrade')
        self.assertFalse(any(not tls for tls, _, _ in self.requests), self.requests)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('LOCAL_VERIFIED_HELPER', result.stdout)

    def test_redirect_loops_are_bounded(self):
        for tool in ('curl', 'wget'):
            with self.subTest(tool=tool):
                result = self.run_bootstrap(tool, '/loop')
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn('LOCAL_VERIFIED_HELPER', result.stdout)
                self.assertLessEqual(len(self.requests), 6)

    def test_chunked_oversize_is_rejected_before_execution(self):
        for tool in ('curl', 'wget'):
            with self.subTest(tool=tool):
                result = self.run_bootstrap(tool, '/oversize')
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn('LOCAL_VERIFIED_HELPER', result.stdout)

    def test_same_length_wrong_digest_is_rejected(self):
        for tool in ('curl', 'wget'):
            with self.subTest(tool=tool):
                result = self.run_bootstrap(tool, '/wronghash')
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn('LOCAL_VERIFIED_HELPER', result.stdout)


if __name__ == '__main__':
    unittest.main()
