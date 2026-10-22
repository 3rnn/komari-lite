"""Offline streaming download limits; never contacts a release server."""
import importlib.util
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).parent
sys.path.insert(0,str(ROOT))

class DownloadLimits(unittest.TestCase):
    def load(self, name):
        spec=importlib.util.spec_from_file_location(name.replace('-','_'),ROOT/(name+'.py'))
        assert spec is not None and spec.loader is not None
        module=importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_wget_cannot_stream_beyond_object_limit(self):
        for name in ('komari-oneclick','komari-manager'):
            with self.subTest(script=name), tempfile.TemporaryDirectory() as tmp:
                module=self.load(name)
                exe=Path(tmp)/'wget'
                exe.write_text('#!/usr/bin/python3\nimport sys\np=sys.argv[sys.argv.index("--output-document")+1]\nwith open(p,"wb",buffering=0) as f:\n for i in range(256): f.write(b"x"*4096)\n')
                exe.chmod(0o700)
                dest=Path(tmp)/'asset'
                patches = [mock.patch.object(module.subprocess,'run', wraps=subprocess.run)]
                if name=='komari-oneclick': patches += [mock.patch.object(module,'trusted_tool',return_value=str(exe))]
                elif name=='komari-manager': patches += [mock.patch.object(module.shutil,'which',return_value=str(exe)), mock.patch.object(module.safe_upgrade,'check_input_trust')]
                from contextlib import ExitStack
                with ExitStack() as stack:
                    for patch in patches: stack.enter_context(patch)
                    try:
                        module.download('https://example.test/asset',dest,'wget',4096)
                    except (ValueError,subprocess.CalledProcessError): pass
                self.assertTrue(not dest.exists() or dest.stat().st_size<=4096,'wget wrote an oversized response to disk before checking')

    def test_legacy_bootstrap_caps_its_pinned_helper_download(self):
        import re
        source=(ROOT/'compat/upgrade.sh').read_text()
        definition=re.search(r'(?ms)^bounded_download\(\) \{\n.*?^}',source)
        self.assertIsNotNone(definition,'bootstrap needs an in-flight cap and overall wget deadline')
        with tempfile.TemporaryDirectory() as tmp:
            path=Path(tmp)/'out'
            code='import sys; f=open(sys.argv[1],"wb",buffering=0); [f.write(b"x"*4096) for _ in range(256)]'
            script=definition.group(0)+'\nhelper_size=4096; bounded_download /usr/bin/python3 -c "$1" "$2"'
            proc=subprocess.run(['bash','-c',script,'--',code,str(path)],capture_output=True,text=True)
            self.assertNotEqual(proc.returncode,0)
            self.assertLessEqual(path.stat().st_size,4096)

if __name__=='__main__': unittest.main()
