"""Check the GitHub one-line compatibility bootstrap without touching services."""
import hashlib
from pathlib import Path
import subprocess
import unittest

ROOT = Path(__file__).parent
BOOTSTRAP = ROOT/'compat/upgrade.sh'
HELPER = ROOT/'komari-compat-v1019.py'

class BootstrapTests(unittest.TestCase):
    def test_single_command_bootstrap_pins_immutable_helper(self):
        body = BOOTSTRAP.read_text()
        self.assertIn('175843fa7d6a22c66488570e1847ced4ff5422d5', body)
        self.assertIn('7da9490e61eaa7c2eef1c1977b666feae77c2fb33e6b149030f722465fb94069', body)
        self.assertIn(hashlib.sha256(HELPER.read_bytes()).hexdigest(), body)
        self.assertIn('helper_size=' + str(HELPER.stat().st_size), body)
        self.assertNotIn('/main/', body)
        self.assertIn('sha256sum -c', body)
        self.assertIn('</dev/tty', body)
        self.assertIn('/var/lib/komari-', body)
        self.assertEqual(subprocess.run(['bash','-n',str(BOOTSTRAP)], capture_output=True).returncode, 0)

    def test_invalid_action_refuses_before_any_download(self):
        result = subprocess.run(['bash', str(BOOTSTRAP), 'install'], capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertRegex(result.stderr, r'check|update')

if __name__ == '__main__': unittest.main()
