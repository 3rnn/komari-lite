import hashlib
import json
import os
from pathlib import Path
import shutil
import sqlite3
import subprocess
import sys
import tempfile
import unittest
import zipfile


REPOSITORY = Path(__file__).resolve().parents[1]
SCRIPT = REPOSITORY / "update-komari.sh"


class UpdateKomariScriptTests(unittest.TestCase):
    """Black-box fixtures for the standalone release updater."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.root = self.base / "komari"
        self.state = self.base / "state"
        self.release = self.base / "release"
        self.bin = self.base / "bin"
        self.root.mkdir()
        self.state.mkdir()
        self.release.mkdir()
        self.bin.mkdir()
        self._make_database(self.root / "data" / "komari.db")
        self._make_database(self.root / "data" / "metrics.db")
        self._make_release()
        self._make_fake_curl()
        self._make_fake_systemctl()

    def _make_database(self, database):
        database.parent.mkdir(parents=True, exist_ok=True)
        with sqlite3.connect(database) as db:
            db.execute("create table preserved (value text)")
            db.execute("insert into preserved values ('before')")

    def _make_binary(self, path, version):
        path.write_text(
            "#!/usr/bin/env python3\n"
            "import json, os, sqlite3, sys\n"
            "operation = sys.argv[1]\n"
            f"version = {version!r}\n"
            "if operation == 'version': print(json.dumps({'version': version}))\n"
            "elif operation == 'schema-version': print(json.dumps({'schema_version': 2}))\n"
            "elif operation == 'health':\n"
            " root=os.environ.get('KOMARI_ROOT')\n"
            " if os.environ.get('UPDATE_FIXTURE_MIGRATE_ON_HEALTH') and root and version == '1.0.26':\n"
            "  with sqlite3.connect(os.path.join(root, 'data', 'komari.db')) as db: db.execute('pragma user_version=3')\n"
            " if os.environ.get('UPDATE_FIXTURE_FAIL_HEALTH') and version == '1.0.26': raise SystemExit(1)\n"
            " print(json.dumps({'ok': True, 'version': version, 'schema_version': 2, 'expected_schema_version': 2}))\n"
            "else: raise SystemExit(1)\n"
        )
        path.chmod(0o755)

    def _make_release(self):
        candidate = self.release / "komari-linux-amd64"
        self._make_binary(candidate, "1.0.26")
        with zipfile.ZipFile(self.release / "Glass.zip", "w") as archive:
            archive.writestr("stock.json", "{}")
        checksums = []
        for name in ("komari-linux-amd64", "Glass.zip"):
            data = (self.release / name).read_bytes()
            checksums.append(f"{hashlib.sha256(data).hexdigest()}  {name}\n")
        (self.release / "SHA256SUMS.txt").write_text("".join(checksums))
        metadata = {
            "tag_name": "v1.0.26",
            "draft": False,
            "prerelease": False,
            "assets": [{"name": name} for name in ("komari-linux-amd64", "Glass.zip", "SHA256SUMS.txt")],
        }
        (self.release / "release.json").write_text(json.dumps(metadata))

    def _make_fake_curl(self):
        fake = self.bin / "curl"
        fake.write_text(
            "#!/usr/bin/env python3\n"
            "import os, pathlib, shutil, sys\n"
            "args=sys.argv[1:]\n"
            "dest=pathlib.Path(args[args.index('--output')+1])\n"
            "url=args[-1]\n"
            "source=pathlib.Path(os.environ['UPDATE_FIXTURE_RELEASE'])\n"
            "if '/releases/tags/' in url or url.endswith('/releases/latest'): item='release.json'\n"
            "else: item=url.rsplit('/', 1)[-1]\n"
            "shutil.copyfile(source/item, dest)\n"
        )
        fake.chmod(0o755)

    def _make_fake_systemctl(self):
        fake = self.bin / "systemctl"
        fake.write_text(
            "#!/usr/bin/env python3\n"
            "import os, pathlib, sys\n"
            "log=pathlib.Path(os.environ['UPDATE_FIXTURE_SYSTEMCTL_LOG'])\n"
            "args=sys.argv[1:]\n"
            "if args[0] == 'show':\n"
            " root=os.environ['KOMARI_ROOT']\n"
            " if '-p' in args and 'LoadState' in args: print('loaded')\n"
            " else: print('ExecStart={ path='+root+'/komari ; argv[]='+root+'/komari server ; }\\nWorkingDirectory='+root)\n"
            "elif args[0] == 'is-active': raise SystemExit(0)\n"
            "elif args[0] == 'start' and os.environ.get('UPDATE_FIXTURE_FAIL_START'): raise SystemExit(1)\n"
            "else:\n"
            " log.write_text((log.read_text() if log.exists() else '') + args[0]+'\\n')\n"
        )
        fake.chmod(0o755)

    def _make_installed_controller(self):
        controller = self.state / "safe_upgrade.py"
        controller.write_text(
            "#!/usr/bin/env python3\n"
            "import argparse, hashlib, json, os, pathlib, shutil, sqlite3\n"
            "def preflight(_):\n"
            " if os.environ.get('UPDATE_FIXTURE_FAIL_PREFLIGHT'): raise RuntimeError('fixture preflight rejection')\n"
            "def main():\n"
            " p=argparse.ArgumentParser(); p.add_argument('action'); p.add_argument('--root'); p.add_argument('--state-dir'); p.add_argument('--service'); p.add_argument('--binary'); p.add_argument('--sha256'); p.add_argument('--expected-version'); p.add_argument('--theme-bundle'); p.add_argument('--theme-sha256'); a=p.parse_args()\n"
            " root=pathlib.Path(a.root); state=pathlib.Path(a.state_dir)\n"
            " if a.action == 'gate':\n"
            "  j=state/'journal.json'\n"
            "  if j.exists() and json.loads(j.read_text()).get('phase') not in ('committed','rolled_back'): raise SystemExit('unfinished upgrade')\n"
            "  return\n"
            " if a.action != 'upgrade': raise SystemExit(2)\n"
            " if os.environ.get('UPDATE_FIXTURE_FAIL_CONTROLLER'): raise SystemExit('controller rejected upgrade')\n"
            " if os.environ.get('UPDATE_FIXTURE_INTERRUPT_CONTROLLER'):\n"
            "  (state/'journal.json').write_text(json.dumps({'phase':'starting','target':a.expected_version})); raise SystemExit('interrupted transaction')\n"
            " assert hashlib.sha256(pathlib.Path(a.binary).read_bytes()).hexdigest() == a.sha256\n"
            " target=root/'releases'/a.expected_version\n"
            " if target.exists(): raise SystemExit('release already staged')\n"
            " snapshot=state/'backups'/'fixture'; snapshot.mkdir(parents=True)\n"
            " for name in ('komari.db','metrics.db'):\n"
            "  src=root/'data'/name; dst=snapshot/name; srcdb=sqlite3.connect('file:'+str(src)+'?mode=ro', uri=True); dstdb=sqlite3.connect(dst); srcdb.backup(dstdb); srcdb.close(); dstdb.close()\n"
            " target.mkdir(parents=True); shutil.copy2(a.binary, target/'komari'); os.chmod(target/'komari', 0o755)\n"
            " tmp=root/'.current.fixture'; tmp.symlink_to(pathlib.Path('releases')/a.expected_version); os.replace(tmp, root/'current')\n"
            " tmp=root/'.komari.fixture'; tmp.symlink_to(pathlib.Path('current')/'komari'); os.replace(tmp, root/'komari')\n"
            " (state/'journal.json').write_text(json.dumps({'phase':'committed','target':a.expected_version}))\n"
            "if __name__ == '__main__': main()\n"
        )
        controller.chmod(0o755)
        return controller

    def _env(self):
        controller = self.state / "safe_upgrade.py"
        controller_hash = hashlib.sha256(controller.read_bytes()).hexdigest() if controller.exists() else ""
        return {
            **os.environ,
            "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
            "KOMARI_ROOT": str(self.root),
            "KOMARI_SERVICE": "komari.service",
            "KOMARI_UPGRADE_STATE_DIR": str(self.state),
            "KOMARI_SAFE_UPGRADE_SHA256_ALLOWLIST": controller_hash,
            "UPDATE_FIXTURE_RELEASE": str(self.release),
            "UPDATE_FIXTURE_SYSTEMCTL_LOG": str(self.base / "systemctl.log"),
        }

    def _run(self, *args, env=None):
        return subprocess.run(["bash", str(SCRIPT), *args], text=True, capture_output=True,
                              env={**self._env(), **(env or {})})

    def _assert_db_preserved(self, name):
        with sqlite3.connect(self.root / "data" / name) as db:
            self.assertEqual(db.execute("select value from preserved").fetchone()[0], "before")

    def test_production_controller_fingerprint_is_supported(self):
        self.assertIn(
            "1fe3c0525b3358fff7367ef5ef0b1174bcfa3e23f6f03a9d8fb9362301659cd1",
            SCRIPT.read_text(),
        )

    def test_versioned_dry_run_reports_layout_without_persistent_changes(self):
        releases = self.root / "releases" / "1.0.19"
        releases.mkdir(parents=True)
        self._make_binary(releases / "komari", "1.0.19")
        (self.root / "current").symlink_to(Path("releases") / "1.0.19")
        (self.root / "komari").symlink_to(Path("current") / "komari")
        controller = self._make_installed_controller()
        controller_before = controller.read_bytes()
        links_before = (os.readlink(self.root / "current"), os.readlink(self.root / "komari"))
        result = self._run("--dry-run", "v1.0.26")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Installation layout: versioned", result.stdout)
        self.assertIn("Safe-upgrade gate compatibility: accepted", result.stdout)
        self.assertIn("Safe-upgrade transaction preflight: accepted", result.stdout)
        self.assertEqual(links_before, (os.readlink(self.root / "current"), os.readlink(self.root / "komari")))
        self.assertEqual(controller.read_bytes(), controller_before)
        self.assertFalse((self.state / "journal.json").exists())
        self.assertFalse((self.base / "systemctl.log").exists())

    def test_versioned_preflight_failure_is_reported_without_mutation(self):
        releases = self.root / "releases" / "1.0.19"
        releases.mkdir(parents=True)
        self._make_binary(releases / "komari", "1.0.19")
        (self.root / "current").symlink_to(Path("releases") / "1.0.19")
        (self.root / "komari").symlink_to(Path("current") / "komari")
        self._make_installed_controller()
        links_before = (os.readlink(self.root / "current"), os.readlink(self.root / "komari"))
        result = self._run("--dry-run", "v1.0.26", env={"UPDATE_FIXTURE_FAIL_PREFLIGHT": "1"})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Safe-upgrade transaction preflight failed", result.stderr)
        self.assertIn("fixture preflight rejection", result.stderr)
        self.assertEqual(links_before, (os.readlink(self.root / "current"), os.readlink(self.root / "komari")))
        self.assertFalse((self.state / "journal.json").exists())
        self.assertFalse((self.base / "systemctl.log").exists())

    def test_versioned_upgrade_delegates_to_controller_and_preserves_databases(self):
        releases = self.root / "releases" / "1.0.19"
        releases.mkdir(parents=True)
        self._make_binary(releases / "komari", "1.0.19")
        (self.root / "current").symlink_to(Path("releases") / "1.0.19")
        (self.root / "komari").symlink_to(Path("current") / "komari")
        controller = self._make_installed_controller()
        controller_before = controller.read_bytes()
        result = self._run("v1.0.26")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(os.readlink(self.root / "current"), "releases/1.0.26")
        self.assertEqual(os.readlink(self.root / "komari"), "current/komari")
        self.assertTrue((self.root / "releases" / "1.0.19" / "komari").is_file())
        self.assertTrue((self.root / "releases" / "1.0.26" / "komari").is_file())
        self.assertEqual(controller.read_bytes(), controller_before)
        self.assertEqual(json.loads((self.state / "journal.json").read_text())["phase"], "committed")
        self._assert_db_preserved("komari.db")
        self._assert_db_preserved("metrics.db")
        again = self._run("v1.0.26")
        self.assertEqual(again.returncode, 0, again.stderr)
        self.assertIn("already installed", again.stdout)

    def test_versioned_layout_rejects_symlink_escaping_root(self):
        outside = self.base / "outside"
        outside.mkdir()
        self._make_binary(outside / "komari", "1.0.19")
        (self.root / "current").symlink_to(outside)
        (self.root / "komari").symlink_to(Path("current") / "komari")
        self._make_installed_controller()
        result = self._run("--dry-run", "v1.0.26")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unsafe", result.stderr.lower())

    def test_flat_dry_run_and_upgrade_preserve_databases(self):
        self._make_binary(self.root / "komari", "1.0.19")
        before = (self.root / "komari").read_bytes()
        dry = self._run("--dry-run", "v1.0.26")
        self.assertEqual(dry.returncode, 0, dry.stderr)
        self.assertIn("Installation layout: flat", dry.stdout)
        self.assertEqual((self.root / "komari").read_bytes(), before)
        self.assertFalse((self.base / "systemctl.log").exists())
        result = self._run("v1.0.26")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.root / "komari").is_symlink())
        self.assertEqual(json.loads(subprocess.check_output([self.root / "komari", "version", "--json"]))["version"], "1.0.26")
        self._assert_db_preserved("komari.db")
        self._assert_db_preserved("metrics.db")

    def test_versioned_controller_rejection_keeps_active_links_and_journal_clean(self):
        releases = self.root / "releases" / "1.0.19"
        releases.mkdir(parents=True)
        self._make_binary(releases / "komari", "1.0.19")
        (self.root / "current").symlink_to(Path("releases") / "1.0.19")
        (self.root / "komari").symlink_to(Path("current") / "komari")
        self._make_installed_controller()
        result = self._run("v1.0.26", env={"UPDATE_FIXTURE_FAIL_CONTROLLER": "1"})
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(os.readlink(self.root / "current"), "releases/1.0.19")
        self.assertEqual(os.readlink(self.root / "komari"), "current/komari")
        self.assertFalse((self.state / "journal.json").exists())
    def test_flat_start_failure_restores_previous_executable_and_keeps_sqlite_data(self):
        self._make_binary(self.root / "komari", "1.0.19")
        previous = (self.root / "komari").read_bytes()
        result = self._run("v1.0.26", env={"UPDATE_FIXTURE_FAIL_START": "1"})
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.root / "komari").read_bytes(), previous)
        self._assert_db_preserved("komari.db")
        self._assert_db_preserved("metrics.db")
        self.assertIn("stop", (self.base / "systemctl.log").read_text())

    def test_versioned_interruption_preserves_journal_and_next_run_fails_closed(self):
        releases = self.root / "releases" / "1.0.19"
        releases.mkdir(parents=True)
        self._make_binary(releases / "komari", "1.0.19")
        (self.root / "current").symlink_to(Path("releases") / "1.0.19")
        (self.root / "komari").symlink_to(Path("current") / "komari")
        self._make_installed_controller()
        interrupted = self._run("v1.0.26", env={"UPDATE_FIXTURE_INTERRUPT_CONTROLLER": "1"})
        self.assertNotEqual(interrupted.returncode, 0)
        self.assertEqual(json.loads((self.state / "journal.json").read_text())["phase"], "starting")
        self.assertEqual(os.readlink(self.root / "current"), "releases/1.0.19")
        retry = self._run("v1.0.26")
        self.assertNotEqual(retry.returncode, 0)
        self.assertIn("unfinished transaction", retry.stderr)

    def test_versioned_gate_rejection_prevents_downloaded_release_selection(self):
        releases = self.root / "releases" / "1.0.19"
        releases.mkdir(parents=True)
        self._make_binary(releases / "komari", "1.0.19")
        (self.root / "current").symlink_to(Path("releases") / "1.0.19")
        (self.root / "komari").symlink_to(Path("current") / "komari")
        self._make_installed_controller()
        (self.state / "journal.json").write_text(json.dumps({"phase": "starting"}))
        result = self._run("--dry-run", "v1.0.26")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(os.readlink(self.root / "current"), "releases/1.0.19")
        self.assertFalse((self.root / "releases" / "1.0.26").exists())

    def test_bad_release_checksum_and_unsafe_state_are_rejected_before_service_stop(self):
        self._make_binary(self.root / "komari", "1.0.19")
        checksums = (self.release / "SHA256SUMS.txt").read_text()
        (self.release / "SHA256SUMS.txt").write_text(checksums.replace("komari-linux-amd64", "other"))
        bad_checksum = self._run("v1.0.26")
        self.assertNotEqual(bad_checksum.returncode, 0)
        self.assertFalse((self.base / "systemctl.log").exists())
        self._make_release()
        self.state.chmod(0o775)
        releases = self.root / "releases" / "1.0.19"
        releases.mkdir(parents=True)
        (self.root / "komari").unlink()
        self._make_binary(releases / "komari", "1.0.19")
        (self.root / "current").symlink_to(Path("releases") / "1.0.19")
        (self.root / "komari").symlink_to(Path("current") / "komari")
        self._make_installed_controller()
        unsafe_state = self._run("--dry-run", "v1.0.26")
        self.assertNotEqual(unsafe_state.returncode, 0)
        self.assertIn("unsafe safe-upgrade state directory", unsafe_state.stderr)
    def test_flat_migration_then_failed_health_restores_binary_but_keeps_service_stopped(self):
        self._make_binary(self.root / "komari", "1.0.19")
        previous = (self.root / "komari").read_bytes()
        result = self._run("v1.0.26", env={
            "UPDATE_FIXTURE_MIGRATE_ON_HEALTH": "1",
            "UPDATE_FIXTURE_FAIL_HEALTH": "1",
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.root / "komari").read_bytes(), previous)
        with sqlite3.connect(self.root / "data" / "komari.db") as db:
            self.assertEqual(db.execute("pragma user_version").fetchone()[0], 3)
        self.assertEqual((self.base / "systemctl.log").read_text().splitlines()[-1], "stop")

    def test_tmpdir_is_ignored_and_existing_flat_backup_directory_is_rejected(self):
        self._make_binary(self.root / "komari", "1.0.19")
        unsafe_tmp = self.base / "unsafe-tmp"
        unsafe_tmp.mkdir(mode=0o777)
        tmp_result = self._run("--dry-run", "v1.0.26", env={"TMPDIR": str(unsafe_tmp)})
        self.assertEqual(tmp_result.returncode, 0, tmp_result.stderr)
        self.assertIn("Dry run complete", tmp_result.stdout)
        backup = self.root / "backup"
        backup.mkdir()
        backup.chmod(0o777)
        backup_result = self._run("v1.0.26")
        self.assertNotEqual(backup_result.returncode, 0)
        self.assertIn("backup", backup_result.stderr.lower())
        self.assertFalse((self.base / "systemctl.log").exists())


if __name__ == "__main__":
    unittest.main()
