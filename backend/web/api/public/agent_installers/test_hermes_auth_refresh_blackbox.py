"""Systemd black-box regression for atomic Hermes OAuth replacement.

The fixture loads only randomly named units into the disposable test host's
runtime unit directory. It uses fake state/auth files and a unique private
runtime tree, never an installed Hermes home or Komari service.
"""
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import tempfile
import time
import unittest
import uuid

SH = pathlib.Path(__file__).with_name("install.sh")
SCRATCH = pathlib.Path("/root/.hermes/cache/scratch")
UNIT_DIR = pathlib.Path("/run/systemd/system")


def function(source, name):
    match = re.search(r"(?m)^" + re.escape(name) + r"\(\) \{\n", source)
    if not match:
        raise AssertionError(f"missing {name}")
    end = re.search(r"(?m)^}\s*$", source[match.end():])
    if not end:
        raise AssertionError(f"unterminated {name}")
    return source[match.start():match.end() + end.end()]


def systemctl(*args):
    return subprocess.run(["systemctl", *args], capture_output=True, text=True)


def systemd_available():
    return bool(
        os.name == "posix"
        and os.geteuid() == 0
        and shutil.which("systemctl")
        and UNIT_DIR.is_dir()
        and systemctl("show", "basic.target").returncode == 0
    )


@unittest.skipUnless(systemd_available(), "requires root and a running system manager")
class HermesAuthRefreshBlackBox(unittest.TestCase):
    def generated_units(self, units, runtime, source, helper, private, spool):
        installer = SH.read_text()
        script = (
            f"hermes_owner=root; hermes_group=root; hermes_home={source}\n"
            f"ai_runtime={runtime}; ai_private={private}; ai_spool={spool}\n"
            f"ai_helper={helper}\n"
            + function(installer, "shell_quote")
            + "\n"
            + function(installer, "systemd_quote")
            + "\n"
            + function(installer, "write_hermes_unit")
            + "\n"
            + function(installer, "write_hermes_runtime_setup_service")
            + "\n"
            + function(installer, "write_hermes_auth_refresh_service")
            + "\n"
            + function(installer, "write_hermes_auth_refresh_path")
            + f"\nwrite_hermes_unit > {units / (runtime + '.service')}\n"
            + f"write_hermes_runtime_setup_service > {units / (runtime + '-runtime-setup.service')}\n"
            + f"write_hermes_auth_refresh_service > {units / (runtime + '-auth-refresh.service')}\n"
            + f"write_hermes_auth_refresh_path > {units / (runtime + '-auth-refresh.path')}\n"
        )
        result = subprocess.run(["bash", "-c", script], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_atomic_auth_replacements_restart_and_rebind_in_isolated_sandbox(self):
        runtime = "komari-auth-fixture-" + uuid.uuid4().hex
        service = runtime + ".service"
        refresh_service = runtime + "-auth-refresh.service"
        refresh_path = runtime + "-auth-refresh.path"
        private = pathlib.Path("/var/lib") / runtime / "private"
        spool = pathlib.Path("/var/lib") / runtime / "spool"
        unit_paths = [
            UNIT_DIR / service,
            UNIT_DIR / (runtime + "-runtime-setup.service"),
            UNIT_DIR / refresh_service,
            UNIT_DIR / refresh_path,
        ]
        with tempfile.TemporaryDirectory(dir=SCRATCH, prefix="komari-auth-refresh-") as temp:
            root = pathlib.Path(temp)
            source, units = root / "synthetic-hermes", root / "units"
            helper = private.parent / "collector.py"
            source.mkdir(mode=0o755)
            units.mkdir(mode=0o700)
            private.mkdir(parents=True, mode=0o700)
            spool.mkdir(parents=True, mode=0o700)
            (source / "state.db").write_text("synthetic state")
            (source / "private-note.txt").write_text("must not be visible")
            self.replace_auth(source / "auth.json", "A")
            os.chmod(source / "state.db", 0o640)
            os.chmod(source / "private-note.txt", 0o600)
            helper.write_text(self.helper_source(source))
            helper.chmod(0o755)
            self.generated_units(units, runtime, source, helper, private, spool)
            for target in unit_paths:
                target.write_bytes((units / target.name).read_bytes())
            reloaded = systemctl("daemon-reload")
            self.assertEqual(reloaded.returncode, 0, reloaded.stderr)
            try:
                started = systemctl("start", service, refresh_path)
                self.assertEqual(started.returncode, 0, started.stderr)
                report = self.wait_report(spool, "A", None, service)
                previous_pid = self.show(service, "MainPID")
                for account in ("B", "C", "D"):
                    self.replace_auth(source / "auth.json", account)
                    report = self.wait_report(spool, account, report["inode"], service)
                    current_pid = self.show(service, "MainPID")
                    self.assertNotEqual(current_pid, previous_pid)
                    self.assertEqual(report["inode"], os.stat(source / "auth.json").st_ino)
                    self.assertTrue(report["private_unreadable"])
                    self.assertTrue(report["source_hidden"])
                    self.assertEqual(report["fingerprint"], self.fingerprint(account))
                    previous_pid = current_pid
                active = systemctl("is-active", "--quiet", refresh_path)
                self.assertEqual(active.returncode, 0, active.stderr)
            finally:
                systemctl("stop", refresh_path, refresh_service, service)
                for path in unit_paths:
                    path.unlink(missing_ok=True)
                reloaded = systemctl("daemon-reload")
                self.assertEqual(reloaded.returncode, 0, reloaded.stderr)
                shutil.rmtree(private.parent, ignore_errors=True)

    @staticmethod
    def helper_source(source):
        private_note = repr(str(source / "private-note.txt"))
        auth_file = repr(str(source / "auth.json"))
        return f"""import argparse, hashlib, json, os, pathlib, time
parser=argparse.ArgumentParser()
parser.add_argument('--database')
parser.add_argument('--auth-path')
parser.add_argument('--state-dir')
parser.add_argument('--spool')
args=parser.parse_args()
spool=pathlib.Path(args.spool)
spool.mkdir(parents=True, exist_ok=True)
while True:
    account=json.load(open(args.auth_path))['account']
    report={{'fingerprint': hashlib.sha256(('fixture:'+account).encode()).hexdigest(),
            'inode': os.stat(args.auth_path).st_ino,
            'private_unreadable': not os.path.exists({private_note}),
            'source_hidden': not os.path.exists({auth_file})}}
    temp=spool/'.report'
    temp.write_text(json.dumps(report))
    os.replace(temp, spool/'report.json')
    time.sleep(.02)
"""

    @staticmethod
    def replace_auth(path, account):
        temporary = path.with_name(path.name + ".tmp")
        with temporary.open("w") as out:
            json.dump({"account": account}, out)
            out.flush()
            os.fsync(out.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)

    @staticmethod
    def fingerprint(account):
        return hashlib.sha256(("fixture:" + account).encode()).hexdigest()

    def show(self, service, key):
        result = systemctl("show", service, "-p", key, "--value")
        self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout.strip()

    def wait_report(self, spool, account, previous_inode, service):
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            try:
                report = json.loads((spool / "report.json").read_text())
            except (FileNotFoundError, json.JSONDecodeError):
                report = None
            if report and report.get("fingerprint") == self.fingerprint(account) and report.get("inode") != previous_inode:
                return report
            time.sleep(.05)
        status = systemctl("status", service, "--no-pager", "--full")
        self.fail(f"timed out waiting for {account}: {status.stdout}{status.stderr}")


if __name__ == "__main__":
    unittest.main()
