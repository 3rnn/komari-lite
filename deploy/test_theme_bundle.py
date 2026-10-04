"""A theme upgrade bundle must come from two reviewed, unmodified source trees."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import zipfile

SCRIPT = Path(__file__).with_name('build-theme-bundle.py')


class ThemeBundleTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        root = Path(self.tmp.name)
        self.old, self.new = root/'old'/'Glass', root/'new'/'Glass'
        for theme, content in ((self.old, b'old JS'), (self.new, b'new JS')):
            chunk = theme/'dist'/'_next'/'static'/'chunks'/'app.js'
            chunk.parent.mkdir(parents=True)
            chunk.write_bytes(content)
            (theme/'dist'/'index.html').write_text('<script src="/_next/static/chunks/app.js"></script>')
            (theme/'komari-theme.json').write_text('{"name":"Glass"}')
        self.output = root/'bundle.zip'

    def run_build(self):
        return subprocess.run([sys.executable, str(SCRIPT), '--old', str(self.old), '--new', str(self.new), '--output', str(self.output)], capture_output=True, text=True)

    def test_builds_complete_stock_manifest_and_new_theme(self):
        result = self.run_build()
        self.assertEqual(result.returncode, 0, result.stderr)
        with zipfile.ZipFile(self.output) as archive:
            stock = json.loads(archive.read('stock.json'))
            expected = {p.relative_to(self.old).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
                        for p in self.old.rglob('*') if p.is_file()}
            self.assertEqual(stock, expected)
            self.assertEqual(archive.read('Glass/dist/_next/static/chunks/app.js'), b'new JS')
            self.assertIn('Glass/dist/index.html', archive.namelist())
        self.assertNotEqual(self.run_build().returncode, 0, 'must not silently replace an approved bundle')

    def test_refuses_linked_old_or_new_theme(self):
        (self.old/'linked.js').symlink_to(self.old/'dist'/'index.html')
        self.assertNotEqual(self.run_build().returncode, 0)
        self.assertFalse(self.output.exists())
        (self.old/'linked.js').unlink()
        (self.new/'linked.js').symlink_to(self.new/'dist'/'index.html')
        self.assertNotEqual(self.run_build().returncode, 0)
        self.assertFalse(self.output.exists())


if __name__ == '__main__':
    unittest.main()
