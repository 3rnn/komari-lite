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
        self.assertIn('e8c41f12ed3a87821677925ed4095280aa7dd073', body)
        self.assertIn('37153fa134b0395855a212c0595250c2aed294859b01f575bdbb83ac871c7e32', body)
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

    def test_status_is_supported_without_interactive_confirmation(self):
        body = BOOTSTRAP.read_text()
        self.assertIn('status', body)
        self.assertIn('"$mode" == update', body)
        self.assertIn('"$stage/k19a.py" "$mode"', body)

if __name__ == '__main__': unittest.main()
