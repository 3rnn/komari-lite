"""GitHub release downloader tests; never mutate a live installation."""
import hashlib
import importlib.util
import json
import sys
from pathlib import Path
import tempfile
import unittest
from unittest import mock

SCRIPT = Path(__file__).with_name('komari-manager.py')


class ReleaseTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec = importlib.util.spec_from_file_location('komari_manager_remote', SCRIPT)
        cls.manager = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.manager)

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.payload = b'#!/bin/sh\nexit 0\n'
        self.bundle = b'fixture-theme-bundle'
        self.tag = 'v8.2.0'
        self.release = {
            'tag_name': self.tag, 'draft': False, 'prerelease': False,
            'assets': [self.asset('komari', self.payload), self.asset('Glass.zip', self.bundle)],
        }

    def asset(self, name, contents):
        return {'name': name, 'size': len(contents),
                'digest': 'sha256:' + hashlib.sha256(contents).hexdigest(),
                'browser_download_url': 'https://github.com/3rnn/komari-lite/releases/download/' + self.tag + '/' + name}

    def fake_download(self, url, dest, tool, limit):
        if 'api.github.com' in url:
            dest.write_text(json.dumps(self.release))
        elif url.endswith('/komari'):
            dest.write_bytes(self.payload)
        elif url.endswith('/Glass.zip'):
            dest.write_bytes(self.bundle)
        else:
            raise AssertionError('unexpected URL: '+url)

    def test_fetches_binary_and_installed_glass_from_same_pinned_release(self):
        with mock.patch.object(self.manager, 'download', side_effect=self.fake_download) as download:
            result = self.manager.fetch_release(self.tag, 'curl', self.base, needs_theme=True)
        self.assertEqual(result.tag, self.tag)
        self.assertEqual(result.binary.read_bytes(), self.payload)
        self.assertEqual(result.theme_bundle.read_bytes(), self.bundle)
        self.assertEqual(result.sha256, hashlib.sha256(self.payload).hexdigest())
        self.assertEqual(result.theme_sha256, hashlib.sha256(self.bundle).hexdigest())
        self.assertEqual(download.call_count, 3)

    def test_fails_closed_when_theme_asset_missing_before_binary_download(self):
        self.release['assets'].pop()
        with mock.patch.object(self.manager, 'download', side_effect=self.fake_download) as download:
            with self.assertRaisesRegex(ValueError, 'Glass.zip'):
                self.manager.fetch_release(self.tag, 'curl', self.base, needs_theme=True)
        self.assertEqual(download.call_count, 1)

    def test_detects_changed_download_against_github_asset_digest(self):
        self.release['assets'][0]['digest'] = 'sha256:' + '0'*64
        with mock.patch.object(self.manager, 'download', side_effect=self.fake_download):
            with self.assertRaisesRegex(ValueError, 'checksum'):
                self.manager.fetch_release(self.tag, 'wget', self.base, needs_theme=False)

    def test_rejects_unpinned_or_malicious_release_asset(self):
        with self.assertRaisesRegex(ValueError, 'tag'):
            self.manager.fetch_release('../main', 'curl', self.base, False)
        self.release['assets'][0]['browser_download_url'] = 'https://attacker.example/komari'
        with mock.patch.object(self.manager, 'download', side_effect=self.fake_download) as download:
            with self.assertRaisesRegex(ValueError, 'URL'):
                self.manager.fetch_release(self.tag, 'curl', self.base, False)
        self.assertEqual(download.call_count, 1)

    def test_rejects_draft_and_mismatched_release(self):
        self.release['draft'] = True
        with mock.patch.object(self.manager, 'download', side_effect=self.fake_download) as download:
            with self.assertRaisesRegex(ValueError, 'draft'):
                self.manager.fetch_release(self.tag, 'curl', self.base, False)
        self.assertEqual(download.call_count, 1)

    def test_explicit_sha_must_match_release_digest(self):
        with mock.patch.object(self.manager, 'download', side_effect=self.fake_download) as download:
            with self.assertRaisesRegex(ValueError, 'pinned SHA-256'):
                self.manager.fetch_release(self.tag, 'curl', self.base, False, pinned_binary='0'*64)
        self.assertEqual(download.call_count, 1)

    def test_cli_one_command_stages_and_invokes_existing_upgrade(self):
        stage = self.base/'stage'
        args = ['komari-manager.py', 'update', '--tag', self.tag,
                '--download-dir', str(stage), '--root', str(self.base/'panel')]
        def fake_fetch(tag, tool, directory, needs_theme, pinned_binary=None, pinned_theme=None):
            self.assertEqual(tag, self.tag)
            self.assertTrue(needs_theme)
            self.assertEqual(tool, 'curl')
            candidate = Path(directory)/'komari'
            candidate.write_bytes(self.payload)
            return self.manager.Release(self.tag, candidate, hashlib.sha256(self.payload).hexdigest(),
                                        Path(directory)/'Glass.zip', hashlib.sha256(self.bundle).hexdigest())
        root = self.base/'panel'/'data'/'theme'/'Glass'
        root.mkdir(parents=True)
        with mock.patch.object(sys, 'argv', args), \
             mock.patch.object(self.manager, 'fetch_release', side_effect=fake_fetch), \
             mock.patch.object(self.manager, 'update') as upgrade:
            self.assertEqual(self.manager.main(), 0)
        upgrade.assert_called_once()
        called = upgrade.call_args.args[0]
        self.assertEqual(called.expected_version, '8.2.0')
        self.assertEqual(called.sha256, hashlib.sha256(self.payload).hexdigest())
        self.assertEqual(called.theme_sha256, hashlib.sha256(self.bundle).hexdigest())
        self.assertFalse(Path(called.binary).exists())

    def test_release_mode_refuses_mixed_local_binary_argument(self):
        args = ['komari-manager.py', 'install', '--tag', self.tag,
                '--binary', str(self.base/'candidate')]
        with mock.patch.object(sys, 'argv', args), \
             mock.patch.object(self.manager, 'fetch_release') as fetch:
            self.assertEqual(self.manager.main(), 1)
        fetch.assert_not_called()


if __name__ == '__main__':
    unittest.main()
