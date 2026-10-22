"""Private synthetic SQLite lifetime regression; CONTROLLER selects exact artifact."""
from contextlib import closing
import gc
import importlib.util
import os
from pathlib import Path
import sqlite3
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(os.environ.get('CONTROLLER', Path(__file__).with_name('safe_upgrade.py')))
spec = importlib.util.spec_from_file_location('controller_under_test', SCRIPT)
assert spec is not None and spec.loader is not None
controller = importlib.util.module_from_spec(spec)
spec.loader.exec_module(controller)


class SQLiteLifetimeTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        (self.root/'data').mkdir()
        self.main = self.root/'data'/'komari.db'
        self.metrics = self.root/'data'/'metrics.db'
        with closing(sqlite3.connect(self.main)) as db:
            db.execute('create table configs (key text, value text)')
            db.execute('create table records (value integer)')
            db.commit()
        with closing(sqlite3.connect(self.metrics)) as db:
            db.execute('create table samples (value integer)')
            db.commit()
        gc.collect()
        self.gc_enabled = gc.isenabled()
        gc.disable()
        self.addCleanup(self.restore_gc)

    def restore_gc(self):
        gc.collect()
        if self.gc_enabled:
            gc.enable()

    def fds(self):
        result = []
        for fd in Path('/proc/self/fd').iterdir():
            try:
                target = os.readlink(fd)
            except FileNotFoundError:
                continue
            if target.startswith(str(self.root) + '/'):
                result.append(target)
        return sorted(result)

    def checks(self):
        return [('sqlite_check', lambda: controller.sqlite_check(self.main)),
                ('config_check', lambda: controller.config_check(self.root)),
                ('check_restricted_migrations', lambda: controller.check_restricted_migrations(self.root))]

    def test_success_has_zero_retained_fds_without_gc(self):
        for name, check in self.checks():
            with self.subTest(check=name):
                before = self.fds()
                for _ in range(20):
                    check()
                self.assertEqual(self.fds(), before, name + ' retained database FDs')

    def test_restricted_failure_closes_with_traceback_retained(self):
        with closing(sqlite3.connect(self.main)) as db:
            db.execute('insert into records values (1)')
            db.commit()
        errors = []
        for _ in range(10):
            try:
                controller.check_restricted_migrations(self.root)
            except RuntimeError as error:
                errors.append(error)  # Retain traceback: no reliance on finalizers.
        self.assertEqual(len(errors), 10)
        self.assertEqual(self.fds(), [])

    def test_corrupt_sqlite_error_labels_path_and_operation(self):
        self.main.write_bytes(b'not a database' * 100)
        for name, check in self.checks():
            with self.subTest(check=name):
                try:
                    check()
                except Exception as error:
                    self.assertIn(str(self.main), str(error))
                    self.assertIn('sqlite_check' if name == 'config_check' else name, str(error))
                    self.assertIsInstance(error.__cause__, sqlite3.Error)
                    self.assertEqual(self.fds(), [])
                else:
                    self.fail('corrupt database accepted')

    def test_null_or_invalid_config_labels_path_and_closes(self):
        for value in (None, 'invalid json', '"postgres"'):
            with closing(sqlite3.connect(self.main)) as db:
                db.execute('delete from configs')
                db.execute('insert into configs values (?, ?)', ('metric_db_driver', value))
                db.commit()
            with self.subTest(value=value):
                try:
                    controller.config_check(self.root)
                except Exception as error:
                    self.assertIsInstance(error, RuntimeError)
                    self.assertIn(str(self.main), str(error))
                    self.assertIn('config_check', str(error))
                    self.assertEqual(self.fds(), [])
                else:
                    self.fail('invalid config accepted')

    def test_config_validation_labels_path_and_closes(self):
        with closing(sqlite3.connect(self.main)) as db:
            db.execute('drop table configs')
            db.commit()
        try:
            controller.config_check(self.root)
        except RuntimeError as error:
            self.assertIn(str(self.main), str(error))
            self.assertIn('config_check', str(error))
            self.assertEqual(self.fds(), [])
        else:
            self.fail('unsupported config accepted')

    def test_wal_writer_stop_snapshot_rename_restart_lifecycle(self):
        writer = r'''
from contextlib import closing
from pathlib import Path
import select, sqlite3, sys, time
root = Path(sys.argv[1])
with closing(sqlite3.connect(root/'data'/'komari.db')) as main, closing(sqlite3.connect(root/'data'/'metrics.db')) as metrics:
    for db in (main, metrics):
        db.execute('pragma journal_mode=wal')
        db.execute('pragma wal_autocheckpoint=0')
    main.execute('create table if not exists activity (value integer)')
    main.commit()
    print('ready', flush=True)
    n = 0
    while not select.select([sys.stdin], [], [], 0)[0]:
        n += 1
        main.execute('insert into activity values (?)', (n,)); main.commit()
        metrics.execute('insert into samples values (?)', (n,)); metrics.commit()
        time.sleep(.001)
    print(n, flush=True)
'''
        for cycle in range(5):
            process = subprocess.Popen([sys.executable, '-u', '-c', writer, str(self.root)],
                                       stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            try:
                assert process.stdout is not None
                self.assertEqual(process.stdout.readline().strip(), 'ready')
                for _ in range(20):
                    for name, check in self.checks():
                        check()
                        self.assertEqual(self.fds(), [], (cycle, name))
                out, err = process.communicate('stop\n', timeout=10)
                self.assertEqual(process.returncode, 0, err)
                self.assertGreater(int(out.strip()), 0)
            finally:
                if process.poll() is None:
                    process.kill()
                    process.communicate()
            # No stale reader may retain sidecars after writer closure.
            self.assertFalse(Path(str(self.main)+'-wal').exists())
            self.assertFalse(Path(str(self.metrics)+'-wal').exists())
            previous = self.root/('snapshot-' + str(cycle))
            (self.root/'data').rename(previous)
            import shutil
            shutil.copytree(previous, self.root/'data')
            for name, check in self.checks():
                check()
                self.assertEqual(self.fds(), [], name)
            for path in (self.main, self.metrics):
                with closing(sqlite3.connect(path)) as db:
                    self.assertEqual(db.execute('pragma integrity_check').fetchall(), [('ok',)])


if __name__ == '__main__':
    unittest.main(verbosity=2)
