"""Standalone one-command bootstrap tests; no live install/update."""
import hashlib
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

SCRIPT = Path(__file__).with_name('komari-oneclick.py')


class BootstrapTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec = importlib.util.spec_from_file_location('komari_oneclick', SCRIPT)
        cls.script = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.script)

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.stage = Path(self.tmp.name)
        self.tag = 'v8.2.0'
        self.assets = {name: (name + '-contents').encode() for name in
                       ('komari-manager.py', 'safe_upgrade.py', 'komari', 'Glass.zip')}
        self.metadata = {'tag_name': self.tag, 'draft': False, 'prerelease': False,
                         'assets': [dict(name=name, digest='sha256:'+hashlib.sha256(data).hexdigest(),
                                         size=len(data), browser_download_url='https://github.com/3rnn/komari-lite/releases/download/'+self.tag+'/'+name)
                                    for name, data in self.assets.items()]}

    def fake_download(self, url, target, tool, limit):
        if 'api.github.com' in url:
            target.write_text(json.dumps(self.metadata))
        else:
            target.write_bytes(self.assets[url.rsplit('/', 1)[-1]])

    def test_bootstrap_checks_both_scripts_and_hands_pinned_tag_to_manager(self):
        with mock.patch.object(self.script, 'download', side_effect=self.fake_download) as download:
            found = self.script.fetch_controller(self.tag, 'curl', self.stage)
        self.assertEqual(found.tag, self.tag)
        self.assertEqual(found.manager.read_bytes(), self.assets['komari-manager.py'])
        self.assertEqual(found.controller.read_bytes(), self.assets['safe_upgrade.py'])
        self.assertEqual(found.binary_sha, hashlib.sha256(self.assets['komari']).hexdigest())
        self.assertEqual(download.call_count, 3)

    def test_missing_controller_asset_fails_before_code_execution(self):
        self.metadata['assets'] = [x for x in self.metadata['assets'] if x['name'] != 'safe_upgrade.py']
        with mock.patch.object(self.script, 'download', side_effect=self.fake_download) as download:
            with self.assertRaisesRegex(ValueError, 'safe_upgrade.py'):
                self.script.fetch_controller(self.tag, 'curl', self.stage)
        self.assertEqual(download.call_count, 1)

    def test_controller_hash_mismatch_rejected(self):
        self.metadata['assets'][1]['digest'] = 'sha256:'+'0'*64
        with mock.patch.object(self.script, 'download', side_effect=self.fake_download):
            with self.assertRaisesRegex(ValueError, 'checksum'):
                self.script.fetch_controller(self.tag, 'wget', self.stage)

    def test_out_of_band_controller_pin_refuses_release_before_execution(self):
        with mock.patch.object(self.script, 'download', side_effect=self.fake_download) as download:
            with self.assertRaisesRegex(ValueError, 'pinned controller'):
                self.script.fetch_controller(self.tag, 'curl', self.stage,
                                             pinned_controller='0'*64)
        self.assertEqual(download.call_count, 1)

    def test_out_of_band_binary_pin_refuses_release_before_script_download(self):
        with mock.patch.object(self.script, 'download', side_effect=self.fake_download) as download:
            with self.assertRaisesRegex(ValueError, 'pinned binary'):
                self.script.fetch_controller(self.tag, 'curl', self.stage,
                                             pinned_binary='0'*64)
        self.assertEqual(download.call_count, 1)

    def test_main_runs_only_verified_controller_with_exact_tag(self):
        with mock.patch.object(sys, 'argv', ['komari-oneclick.py', 'update', '--tag', self.tag,
                                             '--root', str(self.stage/'panel'),
                                             '--download-dir', str(self.stage/'bootstrap-cache')]), \
             mock.patch.object(self.script, 'fetch_controller') as fetch, \
             mock.patch.object(self.script, 'run_manager', return_value=0) as execute:
            fetch.return_value = self.script.Controller(self.tag, self.stage/'komari-manager.py',
                                                         self.stage/'safe_upgrade.py', 'a'*64)
            self.assertEqual(self.script.main(), 0)
        self.assertEqual(Path(fetch.call_args.args[2]).parent, self.stage/'bootstrap-cache')
        passed = execute.call_args.args[1]
        self.assertIn('--tag', passed)
        self.assertIn(self.tag, passed)
        self.assertIn('--sha256', passed)
        self.assertIn('a'*64, passed)

    def test_bootstrap_accepts_independent_binary_pin_and_forwards_it(self):
        with mock.patch.object(sys, 'argv', ['komari-oneclick.py', 'install', '--tag', self.tag,
                                             '--sha256', 'a'*64, '--download-dir', str(self.stage/'cache')]), \
             mock.patch.object(self.script, 'fetch_controller') as fetch, \
             mock.patch.object(self.script, 'run_manager', return_value=0) as execute:
            fetch.return_value = self.script.Controller(self.tag, self.stage/'komari-manager.py',
                                                         self.stage/'safe_upgrade.py', 'a'*64)
            self.assertEqual(self.script.main(), 0)
        self.assertEqual(fetch.call_args.kwargs['pinned_binary'], 'a'*64)
        self.assertIn('a'*64, execute.call_args.args[1])


if __name__ == '__main__':
    unittest.main()
