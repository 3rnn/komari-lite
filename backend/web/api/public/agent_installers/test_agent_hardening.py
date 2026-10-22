"""Offline Agent download and opt-in systemd hardening regressions."""
import pathlib
import shutil
import subprocess
import tempfile
import unittest
from test_installer_security import function, SH

class AgentHardening(unittest.TestCase):
    def test_restricted_binary_is_root_owned_and_executable_with_private_umask(self):
        definition = function(SH.read_text(), 'set_agent_permissions')
        with tempfile.TemporaryDirectory() as tmp:
            binary = pathlib.Path(tmp) / 'agent'
            binary.write_bytes(b'fixture')
            binary.chmod(0o600)
            script = definition + '''
umask 077
service_user=nobody; service_group=$(id -gn nobody)
set_agent_permissions "$1"
'''
            subprocess.run(['bash', '-c', script, '--', str(binary)], check=True)
            self.assertEqual(binary.stat().st_uid, 0)
            self.assertEqual(binary.stat().st_mode & 0o777, 0o750)
            group = subprocess.check_output(['id', '-g', 'nobody'], text=True)
            self.assertEqual(binary.stat().st_gid, int(group))

    def test_hermes_downloads_share_streaming_limit_and_deadlines(self):
        source = SH.read_text()
        definition = function(source, 'stage_hermes_helper')
        self.assertNotIn('curl ', definition)

        script = '''
enable_hermes_ai=true
staging_dir=$1; install_source=https://example.test/release
install_hermes_sha256=''
log_error() { :; }
download_https() {
    printf '%s %s\n' "$1" "$3"
    if [[ $2 == */SHA256SUMS.txt ]]; then
        printf '%064d  hermes_collector.py\n' 0 > "$2"
    else
        printf 'fixture' > "$2"
    fi
}
''' + definition + '\nstage_hermes_helper\n'
        with tempfile.TemporaryDirectory() as tmp:
            proc = subprocess.run(['bash', '-c', script, '--', tmp], capture_output=True, text=True)
        self.assertNotEqual(proc.returncode, 0, 'fixture digest must fail verification')
        self.assertEqual(proc.stdout.splitlines(), [
            'https://example.test/release/SHA256SUMS.txt 1048576',
            'https://example.test/release/hermes_collector.py 1048576'])

    def test_restricted_ai_requires_reviewed_spool_permission_migration(self):
        source = SH.read_text()
        definition = function(source, 'validate_service_mode')
        for user, ai, expected in [('root', 'true', 0), ('root', 'false', 0),
                                   ('nobody', 'false', 0), ('nobody', 'true', 1)]:
            proc = subprocess.run(['bash', '-c', 'log_error() { :; };\n' + definition +
                                   '\nvalidate_service_mode "$1" "$2"', '--', user, ai],
                                  capture_output=True, text=True)
            self.assertEqual(proc.returncode, expected)
        self.assertLess(source.index('validate_service_mode "$service_user" "$enable_hermes_ai"'),
                        source.index('install_dependencies\nlog_step'))

    def test_shell_download_caps_stream_and_pins_https_deadlines(self):
        source=SH.read_text()
        definition=function(source,'download_https')
        for option in ('--proto', '--proto-redir', '--connect-timeout', '--max-time', '--max-filesize'):
            self.assertIn(option,definition)
        with tempfile.TemporaryDirectory() as tmp:
            path=pathlib.Path(tmp)
            curl=path/'curl'
            curl.write_text('#!/usr/bin/python3\nimport sys\np=sys.argv[sys.argv.index("-o")+1]\nwith open(p,"wb",buffering=0) as f:\n for i in range(256): f.write(b"x"*4096)\n')
            curl.chmod(0o700)
            script=definition+'\nPATH="$1:$PATH"; download_https https://example.test/asset "$1/out" 4096\n'
            proc=subprocess.run(['bash','-c',script,'--',tmp],capture_output=True,text=True)
            self.assertNotEqual(proc.returncode,0)
            out=path/'out'
            self.assertTrue(not out.exists() or out.stat().st_size<=4096)

    def test_service_user_preserves_root_and_explicit_restricted_upgrades(self):
        definition=function(SH.read_text(),'select_service_user')
        for requested, existing, expected in [('', '', 'root'), ('', 'root', 'root'),
                                               ('', 'nobody', 'nobody'), ('nobody', '', 'nobody')]:
            proc=subprocess.run(['bash','-c',definition+'\nselect_service_user "$1" "$2"','--',requested,existing],capture_output=True,text=True)
            self.assertEqual(proc.returncode,0,proc.stderr)
            self.assertEqual(proc.stdout.strip(),expected)
        proc=subprocess.run(['bash','-c',definition+'\nselect_service_user nobody root'],capture_output=True,text=True)
        self.assertNotEqual(proc.returncode,0,'root to restricted requires manual traffic/state migration')

    @unittest.skipUnless(shutil.which('systemd-analyze'), 'systemd-analyze unavailable')
    def test_restricted_sandbox_keeps_monitoring_devices_network_and_raw_icmp(self):
        definition=function(SH.read_text(),'systemd_security')
        proc=subprocess.run(['bash','-c',definition+'\nsystemd_security nobody /opt/komari/state'],capture_output=True,text=True,check=True)
        security=proc.stdout
        for directive in ('NoNewPrivileges=yes','ProtectSystem=strict','ReadWritePaths="/opt/komari/state"',
                          'CapabilityBoundingSet=CAP_NET_RAW','AmbientCapabilities=CAP_NET_RAW',
                          'Environment=KOMARI_AGENT_PRIVILEGED_ICMP=1'):
            self.assertIn(directive,security)
        for incompatible in ('PrivateDevices=yes','PrivateNetwork=yes','ProtectProc=invisible'):
            self.assertNotIn(incompatible,security)
        with tempfile.TemporaryDirectory() as tmp:
            unit=pathlib.Path(tmp)/'agent-sandbox.service'
            unit.write_text('[Service]\nExecStart=/usr/bin/true\nUser=nobody\n'+security)
            result=subprocess.run(['systemd-analyze','verify',str(unit)],capture_output=True,text=True)
            self.assertEqual(result.returncode,0,result.stderr)
        root=subprocess.run(['bash','-c',definition+'\nsystemd_security root /opt/komari/state'],capture_output=True,text=True,check=True)
        self.assertNotIn('CapabilityBoundingSet',root.stdout,'legacy root monitoring must not lose capabilities silently')

if __name__=='__main__': unittest.main()
