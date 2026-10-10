"""Isolated systemd regression for atomic Hermes OAuth replacement.

A nested systemd runs in disposable user, mount and PID namespaces. Its chroot
contains only synthetic state/auth data and cannot reach host Hermes data or
installed services.
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

SH = pathlib.Path(__file__).with_name("install.sh")
SCRATCH = pathlib.Path("/root/.hermes/cache/scratch")


def function(source, name):
    match = re.search(r"(?m)^" + re.escape(name) + r"\(\) \{\n", source)
    if not match:
        raise AssertionError(f"missing {name}")
    end = re.search(r"(?m)^}\s*$", source[match.end():])
    if not end:
        raise AssertionError(f"unterminated {name}")
    return source[match.start():match.end() + end.end()]


@unittest.skipUnless(os.name == "posix" and os.geteuid() == 0 and all(shutil.which(x) for x in ("unshare", "nsenter")), "requires root plus Linux namespace tools")
class HermesAuthRefreshBlackBox(unittest.TestCase):
    def generated_units(self, units):
        source = SH.read_text()
        script = (
            "hermes_owner=root; hermes_group=root; hermes_home=/fixture/hermes\n"
            "ai_runtime=fixture-hermes; ai_base=/var/lib/fixture-hermes\n"
            "ai_private=$ai_base/private; ai_spool=$ai_base/spool\n"
            "ai_helper=/opt/fixture/hermes_collector.py\n"
            + function(source, "systemd_quote") + "\n" + function(source, "write_hermes_unit") + "\n"
            + function(source, "write_hermes_auth_refresh_service") + "\n" + function(source, "write_hermes_auth_refresh_path")
            + "\nwrite_hermes_unit > \"$1/fixture-hermes.service\"\n"
            + "write_hermes_auth_refresh_service > \"$1/fixture-hermes-auth-refresh.service\"\n"
            + "write_hermes_auth_refresh_path > \"$1/fixture-hermes-auth-refresh.path\"\n"
        )
        result = subprocess.run(["bash", "-c", script, "--", str(units)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_atomic_auth_replacements_restart_and_rebind_in_an_isolated_manager(self):
        with tempfile.TemporaryDirectory(dir=SCRATCH, prefix="komari-auth-refresh-") as temp:
            fixture = pathlib.Path(temp)
            units, root = fixture / "units", fixture / "rootfs"
            units.mkdir()
            self.generated_units(units)
            console, outer, launcher = fixture / "nested.log", fixture / "outer.log", fixture / "launch.sh"
            launcher.write_text("""#!/usr/bin/env bash
set -euo pipefail
root=$1; units=$2; console=$3
mkdir -p "$root"
mount --make-rprivate /
mount -t tmpfs tmpfs "$root"
mkdir -p "$root"/{usr,proc,sys,dev,etc/systemd/system,run,var/lib/fixture-hermes/private,var/lib/fixture-hermes/spool,opt/fixture,fixture/hermes,tmp}
mount --rbind /usr "$root/usr"; mount --make-rslave "$root/usr"
mount --rbind /sys "$root/sys"; mount --make-rslave "$root/sys"
ln -s usr/bin "$root/bin"; ln -s usr/lib "$root/lib"; ln -s usr/lib64 "$root/lib64"; ln -s usr/sbin "$root/sbin"
printf 'fixture:x:1000:1000::/tmp:/usr/sbin/nologin\\n' > "$root/etc/passwd"
printf 'fixture:x:1000:\\n' > "$root/etc/group"
printf 'fixture\\n' > "$root/etc/hostname"
printf '8de277067b3544d4b65c267d0edab928\\n' > "$root/etc/machine-id"
for d in null zero random urandom full console; do : > "$root/dev/$d"; mount --bind "/dev/$d" "$root/dev/$d"; done
mount -t proc proc "$root/proc"
printf state > "$root/fixture/hermes/state.db"
printf secret > "$root/fixture/hermes/private-note.txt"
printf '{"account":"A"}' > "$root/fixture/hermes/auth.json"
chmod 755 "$root/fixture" "$root/fixture/hermes"; chmod 640 "$root/fixture/hermes/auth.json" "$root/fixture/hermes/state.db"; chmod 600 "$root/fixture/hermes/private-note.txt"
cat > "$root/opt/fixture/hermes_collector.py" <<'PY'
import argparse,hashlib,json,os,pathlib,time
p=argparse.ArgumentParser(); p.add_argument('--database'); p.add_argument('--auth-path'); p.add_argument('--state-dir'); p.add_argument('--spool'); a=p.parse_args(); spool=pathlib.Path(a.spool); spool.mkdir(parents=True,exist_ok=True)
while True:
 account=json.load(open(a.auth_path))['account']
 report={'fingerprint':hashlib.sha256(('fixture:'+account).encode()).hexdigest(),'inode':os.stat(a.auth_path).st_ino,'private_unreadable':not os.path.exists('/fixture/hermes/private-note.txt'),'source_hidden':not os.path.exists('/fixture/hermes/auth.json')}
 tmp=spool/'.report'; tmp.write_text(json.dumps(report)); os.replace(tmp,spool/'report.json'); time.sleep(.02)
PY
chmod 755 "$root/opt/fixture/hermes_collector.py"
cp "$units"/* "$root/etc/systemd/system/"
exec chroot "$root" /usr/lib/systemd/systemd --system --log-target=console --log-level=warning --unit=basic.target > "$console" 2>&1
""")
            launcher.chmod(0o700)
            with outer.open("wb") as output:
                outer_process = subprocess.Popen(["unshare", "--user", "--map-root-user", "--mount", "--pid", "--fork", "--mount-proc", str(launcher), str(root), str(units), str(console)], stdout=output, stderr=subprocess.STDOUT)
            manager = None
            try:
                manager = self.wait_for_manager(root, outer, console)
                started = self.ns(manager, ["/usr/bin/systemctl", "start", "fixture-hermes.service", "fixture-hermes-auth-refresh.path"])
                self.assertEqual(started.returncode, 0, started.stdout + started.stderr + self.artifacts(outer, console))
                report = self.wait_report(manager, "A", None, outer, console)
                pid = self.show(manager, "MainPID")
                for account in ("B", "C", "D"):
                    replace = self.ns(manager, ["/usr/bin/python3", "-c", "import json,os,sys;p='/fixture/hermes/auth.json';t=p+'.tmp';f=open(t,'w');json.dump({'account':sys.argv[1]},f);f.flush();os.fsync(f.fileno());f.close();os.replace(t,p);d=os.open('/fixture/hermes',os.O_RDONLY);os.fsync(d);os.close(d)", account])
                    self.assertEqual(replace.returncode, 0, replace.stderr)
                    report = self.wait_report(manager, account, report["inode"], outer, console)
                    new_pid = self.show(manager, "MainPID")
                    source_inode = int(self.ns(manager, ["/usr/bin/stat", "-c", "%i", "/fixture/hermes/auth.json"]).stdout.strip())
                    self.assertNotEqual(new_pid, pid, self.artifacts(outer, console))
                    self.assertEqual(report["inode"], source_inode)
                    self.assertTrue(report["private_unreadable"])
                    self.assertTrue(report["source_hidden"])
                    self.assertEqual(report["fingerprint"], hashlib.sha256(("fixture:" + account).encode()).hexdigest())
                    pid = new_pid
                self.assertEqual(self.ns(manager, ["/usr/bin/systemctl", "is-active", "--quiet", "fixture-hermes-auth-refresh.path"]).returncode, 0)
            finally:
                if manager:
                    self.ns(manager, ["/usr/bin/systemctl", "poweroff"])
                try:
                    outer_process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    outer_process.kill(); outer_process.wait(timeout=5)

    def wait_for_manager(self, root, outer, console):
        wanted = str(root.resolve())
        for _ in range(300):
            for candidate in pathlib.Path("/proc").iterdir():
                if not candidate.name.isdigit():
                    continue
                try:
                    if os.readlink(candidate / "root") != wanted:
                        continue
                except OSError:
                    continue
                probe = self.ns(candidate.name, ["/usr/bin/systemctl", "show", "basic.target"])
                if probe.returncode == 0:
                    return candidate.name
            time.sleep(.05)
        self.skipTest("nested systemd cannot become ready in this host namespace configuration; run this black-box test on a systemd-capable CI runner")

    @staticmethod
    def ns(pid, command):
        return subprocess.run(["nsenter", "-t", str(pid), "-U", "-m", "-p", "-r", "--", *command], capture_output=True, text=True)

    def show(self, manager, key):
        result = self.ns(manager, ["/usr/bin/systemctl", "show", "fixture-hermes.service", "-p", key, "--value"])
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result.stdout.strip()

    def wait_report(self, manager, account, old_inode, outer, console):
        fingerprint = hashlib.sha256(("fixture:" + account).encode()).hexdigest()
        for _ in range(300):
            result = self.ns(manager, ["/usr/bin/cat", "/var/lib/fixture-hermes/spool/report.json"])
            try:
                report = json.loads(result.stdout)
            except json.JSONDecodeError:
                report = None
            if report and report.get("fingerprint") == fingerprint and report.get("inode") != old_inode:
                return report
            time.sleep(.05)
        status = self.ns(manager, ["/usr/bin/systemctl", "status", "fixture-hermes.service", "--no-pager", "--full"])
        self.fail("timed out waiting for replacement report\n" + status.stdout + status.stderr + self.artifacts(outer, console))

    @staticmethod
    def artifacts(outer, console):
        return "\n".join(f"--- {p.name} ---\n{p.read_text(errors='replace') if p.exists() else 'missing'}" for p in (outer, console))


if __name__ == "__main__":
    unittest.main()
