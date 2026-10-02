"""Offline regression checks for privileged Agent installer input handling."""
import hashlib
import json
import pathlib
import re
import shutil
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[5]
SH = pathlib.Path(__file__).with_name("install.sh")
PS = pathlib.Path(__file__).with_name("install.ps1")


def function(source, name):
    match = re.search(r"(?m)^" + re.escape(name) + r"\(\) \{\n", source)
    if not match:
        raise AssertionError(f"missing {name}")
    end = re.search(r"(?m)^}\s*$", source[match.end():])
    if end:
        return source[match.start():match.end() + end.end()]
    raise AssertionError(f"unterminated {name}")


class InstallerSecurity(unittest.TestCase):
    def test_windows_acl_mask_contains_only_explicit_mutation_rights(self):
        source = PS.read_text()
        directory = source[source.index('function Assert-TrustedDirectory'):source.index('try { $InstallDir =')]
        masks = re.findall(r'\$writeRights = (.*?)\n\s*\$write =', directory, re.S)
        self.assertEqual(len(masks), 2, 'check both pre-create and post-create ACL passes')
        expected = {'WriteData', 'AppendData', 'WriteExtendedAttributes', 'WriteAttributes',
                    'Delete', 'DeleteSubdirectoriesAndFiles', 'ChangePermissions', 'TakeOwnership'}
        for mask in masks:
            rights = set(re.findall(r'FileSystemRights\]::(\w+)', mask))
            self.assertEqual(rights, expected, 'read/execute composite rights must not count as mutation')
            self.assertNotIn('Modify', mask)

    def test_windows_config_has_private_acl_before_serializing_secrets(self):
        src = PS.read_text()
        config = src[src.index('# Create private config'):src.index("$KomariArgs += @('--config'")]
        self.assertIn('SetAccessRuleProtection($true, $false)', config)
        self.assertIn('Set-Acl -LiteralPath $ConfigTemp', config)
        self.assertIn('WriteAllText($ConfigTemp', config)
        self.assertLess(config.index('Set-Acl -LiteralPath $ConfigTemp'),
                        config.index('WriteAllText($ConfigTemp'))
        self.assertLess(config.index('Set-Acl -LiteralPath $ConfigTemp'),
                        config.index('ConvertTo-Json -Compress'))

    def test_windows_upgrade_keeps_service_and_rolls_back_after_health_failure(self):
        source = PS.read_text()
        upgrade = source[source.index('# Register and start service'):]
        self.assertNotIn('Uninstall-Previous', source)
        self.assertIn('function Restore-Previous', source)
        self.assertIn('catch {', upgrade)
        self.assertIn('Restore-Previous', upgrade)
        self.assertLess(source.index('Copy-Item -LiteralPath $path -Destination (Join-Path $StagingDir "backup-$name")'), source.index('Move-Item -LiteralPath $StagedAgent'))
        self.assertIn("$appState -cne 'SERVICE_RUNNING'", upgrade)

    def test_windows_backup_directory_is_private_before_credentials_are_copied(self):
        src = PS.read_text()
        self.assertIn('function Set-PrivateDirectory', src)
        self.assertLess(src.index('Set-PrivateDirectory $StagingDir'),
                        src.index('Copy-Item -LiteralPath $path -Destination (Join-Path $StagingDir "backup-$name")'))

    def test_shell_transaction_restores_artifacts_and_service_on_injected_failure(self):
        source = SH.read_text()
        transaction = function(source, 'commit_install')
        rollback = function(source, 'rollback_install')
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            for failure in ('stop', 'binary', 'config', 'unit', 'reload', 'enable', 'start', 'health'):
                with self.subTest(failure=failure):
                    target = root / failure
                    target.mkdir()
                    for name in ('agent', 'agent-config.json', 'agent.service'):
                        (target / name).write_text('old-' + name)
                        (target / ('new-' + name)).write_text('new-' + name)
                    script = '''
log_error() { printf '%s\\n' "$1" >&2; }
service_name=agent; init_system=systemd; state=running
komari_agent_path=$1/agent; config_path=$1/agent-config.json; service_file=$1/agent.service
staged_agent=$1/new-agent; config_tmp=$1/new-agent-config.json; staged_service=$1/new-agent.service
staging_dir=$1/stage; mkdir "$staging_dir"
systemctl() {
    case "$1" in
        is-active) [[ $FAIL != health && $state == running ]] ;;
        is-enabled) [[ $FAIL != enable ]] ;;
        stop) [[ $FAIL != stop ]] || return 1; state=stopped ;;
        start) [[ $FAIL != start ]] || return 1; state=running ;;
        daemon-reload) [[ $FAIL != reload ]] ;;
        enable) [[ $FAIL != enable ]] ;;
        *) return 0 ;;
    esac
}
mv() { if [[ ${!#} == "$FAIL_PATH" ]]; then return 1; fi; command mv "$@"; }
sleep() { :; }
verify_started_service() { [[ $FAIL != health && $state == running ]]; }
''' + rollback + '\n' + transaction + '''
commit_install
'''
                    path = {'binary': 'agent', 'config': 'agent-config.json', 'unit': 'agent.service'}.get(failure, '')
                    proc = subprocess.run(['bash', '-c', script, '--', str(target)],
                                          env={'FAIL': failure, 'FAIL_PATH': str(target / path), 'PATH': '/usr/bin:/bin'},
                                          capture_output=True, text=True)
                    self.assertNotEqual(proc.returncode, 0, (failure, proc.stdout, proc.stderr))
                    for name in ('agent', 'agent-config.json', 'agent.service'):
                        self.assertEqual((target / name).read_text(), 'old-' + name, failure)

    def test_openrc_fresh_failure_removes_new_artifacts(self):
        source = SH.read_text()
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            for name in ('new-agent', 'new-agent-config.json', 'new-service', 'new-wrapper'):
                (root / name).write_text(name)
            script = '''
log_error() { :; }; service_name=agent; init_system=openrc
komari_agent_path=$1/agent; config_path=$1/agent-config.json
service_file=$1/service; wrapper=$1/agent-service.sh
staged_agent=$1/new-agent; config_tmp=$1/new-agent-config.json
staged_service=$1/new-service; staged_wrapper=$1/new-wrapper
staging_dir=$1/stage; mkdir "$staging_dir"
rc-update() { return 0; }; rc-service() { return 0; }
verify_started_service() { return 1; }
''' + function(source, 'rollback_install') + '\n' + function(source, 'commit_install') + '\ncommit_install\n'
            proc = subprocess.run(['bash', '-c', script, '--', str(root)], capture_output=True, text=True)
            self.assertNotEqual(proc.returncode, 0)
            for name in ('agent', 'agent-config.json', 'service', 'agent-service.sh'):
                self.assertFalse((root / name).exists(), name)

    def test_agent_secret_aliases_never_reach_shell_argv(self):
        src = SH.read_text()
        extract = function(src, 'extract_runtime_credentials')
        script = ('log_error() { printf "%s\\n" "$1" >&2; }; komari_args=("$@"); ' + extract +
                  '\nextract_runtime_credentials || exit 1; printf "%s\\n" "$agent_token" "$agent_cf_secret"; printf "argv:%s\\n" "${komari_args[@]}"')
        for token_flag in ('-t', '--token', '-token'):
            for cf_flag in ('--cf-access-client-secret', '-cf-access-client-secret'):
                for joined in (False, True):
                    args = ([f'{token_flag}=token-secret', f'{cf_flag}=cf-secret'] if joined else
                            [token_flag, 'token-secret', cf_flag, 'cf-secret'])
                    result = subprocess.run(['bash', '-c', script, '--', *args, '--gpu'], capture_output=True, text=True)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stdout, 'token-secret\ncf-secret\nargv:--gpu\n')
        for flag in ('-auto-discovery', '--auto-discovery', '-config', '--config'):
            result = subprocess.run(['bash', '-c', script, '--', flag, 'secret'], capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0, flag)
            self.assertNotIn('secret', result.stdout + result.stderr)

    def test_effective_systemd_execstart_rejects_comment_decoys(self):
        source = SH.read_text()
        check = function(source, 'verify_systemd_execstart')
        with tempfile.TemporaryDirectory() as temp:
            path = str(pathlib.Path(temp) / 'agent')
            script = ('log_error() { printf "%s\\n" "$1" >&2; }; komari_agent_path=$1; ' + check +
                      '\nverify_systemd_execstart "$2"')
            for effective, ok in ((f'{{ path={path} ; argv[]={path} --config config ; }}', True),
                                  (f'# {path}\n{{ path=/foreign/agent ; argv[]=/foreign/agent ; }}', False),
                                  (f'{{ path={path}-other ; argv[]={path}-other ; }}', False),
                                  (f'{{ path={path} ; argv[]={path} ; }} {{ path=/foreign/agent ; argv[]=/foreign/agent ; }}', False),
                                  ('', False)):
                result = subprocess.run(['bash', '-c', script, '--', path, effective], capture_output=True, text=True)
                self.assertEqual(result.returncode == 0, ok, effective)
        self.assertIn('ExecStart --value', function(source, 'verify_existing_service'))

    def test_legacy_systemd_credential_argv_is_rejected_before_upgrade(self):
        source = function(SH.read_text(), 'verify_existing_service')
        self.assertIn('credential arguments', source)
        self.assertLess(source.index('credential arguments'), SH.read_text().index('commit_install ||'))

    def test_prevalidation_precedes_dependencies(self):
        src = SH.read_text()
        self.assertLess(src.index('validate_release_version || exit 1'), src.index('\ninstall_dependencies\n'))
        self.assertLess(src.index('Custom --install-source requires --install-sha256'), src.index('\ninstall_dependencies\n'))
        ps = PS.read_text()
        self.assertLess(ps.index('Invalid --install-version'), ps.index('Invoke-WebRequest -Uri $NssmZipUrl'))

    def test_post_start_checks_are_required_on_all_supported_platforms(self):
        src = SH.read_text()
        for name in ('systemd', 'openrc'):
            self.assertIn(f'verify_started_service {name} || {{ rollback_install; return 1; }}', src)
        self.assertIn('SERVICE_RUNNING', PS.read_text())
        self.assertLess(src.index('case $init_system in\n    systemd|openrc)'), src.index('install_dependencies\n'))
        self.assertNotIn('elif [ "$init_system" = "procd" ]', src)

    def test_post_start_shell_status_fails_closed(self):
        check = function(SH.read_text(), 'verify_started_service')
        script = ('service_name=komari-agent; target_dir=/opt/komari; sleep() { :; }; '
                  'systemctl() { [[ $SIMULATED == running ]]; }; '
                  'rc-service() { [[ $SIMULATED == running ]]; }; '
                  'initctl() { printf "%s\\n" "komari-agent $SIMULATED"; }; '
                  'launchctl() { printf "state = %s\\n" "$SIMULATED"; }; ' + check +
                  '\nverify_started_service "$1"')
        for platform in ('systemd', 'openrc', 'upstart', 'launchd'):
            result = subprocess.run(['bash', '-c', script, '--', platform],
                                    env={'SIMULATED': 'stopped', 'PATH': '/usr/bin:/bin'}, capture_output=True)
            self.assertNotEqual(result.returncode, 0, platform)
        for platform, state in (('systemd', 'running'), ('openrc', 'running')):
            result = subprocess.run(['bash', '-c', script, '--', platform],
                                    env={'SIMULATED': state, 'PATH': '/usr/bin:/bin'}, capture_output=True)
            self.assertEqual(result.returncode, 0, (platform, result.stderr))

    def test_windows_secret_aliases_are_consumed_or_rejected(self):
        text = PS.read_text()
        for flag in ('-token', '-endpoint', '-cf-access-client-secret', 'auto-discovery'):
            self.assertIn(flag, text)
        self.assertIn('$AgentCfSecret = $value', text)
        self.assertIn("$appState -cne 'SERVICE_RUNNING'", text)

    def test_shell_token_and_endpoint_roundtrip_private_config(self):
        src = SH.read_text()
        extract = function(src, 'extract_runtime_credentials')
        escape = function(src, 'json_escape')
        config = src[src.index('config_path="${target_dir}/agent-config.json"'):src.index('# Serialize each Agent argument')]
        with tempfile.TemporaryDirectory() as temp:
            script = '''
log_error() { printf '%s\\n' "$1" >&2; }
komari_args=("$@")
''' + extract + '\n' + escape + '''
extract_runtime_credentials || exit 1
target_dir=$TEST_CONFIG_DIR
umask 077
''' + config + '''
printf '%s\\0' "${komari_args[@]}"
'''
            secret = 'a"b\\c$(touch never)'
            result = subprocess.run(['bash', '-c', script, '--', '-t', secret, '--endpoint=https://panel.test',
                                     '--interval', '5 seconds'], env={'TEST_CONFIG_DIR': temp, 'PATH': '/usr/bin:/bin'},
                                    capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.split(b'\0')[:-1],
                             [b'--interval', b'5 seconds', b'--config', str(pathlib.Path(temp) / 'agent-config.json').encode()])
            config_file = next(pathlib.Path(temp).glob('.agent-config.*'))
            self.assertEqual(json.loads(config_file.read_text())['token'], secret)
            self.assertEqual(config_file.stat().st_mode & 0o777, 0o600)
            self.assertNotIn(secret.encode(), result.stdout + result.stderr)

    def test_source_userinfo_rejected_before_logging(self):
        src = SH.read_text()
        validation = src[src.index('# Validate before any logs or network requests'):src.index('# Single-quote one shell word')]
        script = '''
log_error() { printf '%s\\n' "$1"; }
install_source=$1
agent_endpoint=''
''' + validation + '\nprintf "accepted\\n"\n'
        secret = 'do-not-log-this'
        proc = subprocess.run(['bash', '-c', script, '--', f'https://user:{secret}@mirror.test/v1'],
                              capture_output=True, text=True)
        self.assertNotEqual(proc.returncode, 0)
        self.assertNotIn(secret, proc.stdout + proc.stderr)
        self.assertNotIn('accepted', proc.stdout)

    def test_runtime_token_is_removed_from_service_arguments(self):
        src = SH.read_text()
        self.assertIn('extract_runtime_credentials', src)
        self.assertIn('"--config"', src)
        self.assertIn('"token"', src)
        self.assertNotIn('killall agent', src)
        self.assertNotIn('command_args=$(shell_quote "${shell_args# }")', src)
        self.assertNotIn('procd_set_param command', src)

    def test_nixos_config_argument_and_rejects_unrepresented_options(self):
        src = SH.read_text()
        self.assertIn('if [ -f /etc/NIXOS ]; then', src)
        self.assertIn('systemd|openrc) ;;', src)
        self.assertNotIn('systemd.services.', src)

    def test_shell_non_linux_rejected_before_any_install_mutation(self):
        src = SH.read_text()
        self.assertLess(src.index('[[ $os_name == linux ]]'), src.index('install_dependencies\n'))
        self.assertLess(src.index('[[ $os_name == linux ]]'), src.index('validate_install_directory "$target_dir"'))

    def test_windows_private_config_and_nssm_fail_closed_checks(self):
        text = PS.read_text()
        self.assertIn('ConvertTo-Json', text)
        self.assertIn('Set-Acl', text)
        self.assertIn('Get-FileHash -Path $nssmExeToUse', text)
        self.assertIn('Quote-WindowsArg', text)
        self.assertIn('AppParameters', text)
        self.assertIn('userinfo', text.lower())
        self.assertNotIn('sc.exe delete $ServiceName', text)
        self.assertIn("PowerShell 7.3+ required", text)

    def test_windows_nssm_verified_before_extraction(self):
        text = PS.read_text()
        download = text.index("Invoke-WebRequest -Uri $NssmZipUrl")
        extract = text.index("Expand-Archive -Path $TempNssmZipPath")
        verify = text.find("Get-FileHash -Path $TempNssmZipPath", download, extract)
        self.assertGreater(verify, download)
        self.assertIn("throw", text[verify:extract])
        self.assertRegex(text[verify:extract], r"(?i)727d1e42275c605e0f04aba98095c38a8e1e46def453cdffce42869428aa6743|NssmZipSha256")

    def test_service_name_is_validated_before_side_effects(self):
        shell = SH.read_text()
        self.assertLess(shell.index('validate_service_name "$service_name"'), shell.index("install_dependencies\n"))
        defs = function(shell, "validate_service_name")
        for value in ("../../evil", "a b", "a;touch x", "-oops", "", "a\n[Service]"):
            p = subprocess.run(["bash", "-c", defs + '\nvalidate_service_name "$1"', "--", value], capture_output=True, text=True)
            self.assertNotEqual(p.returncode, 0, value)
        for value in ("komari-agent", "agent_2.test"):
            p = subprocess.run(["bash", "-c", defs + '\nvalidate_service_name "$1"', "--", value], capture_output=True, text=True)
            self.assertEqual(p.returncode, 0, value)
        ps = PS.read_text()
        self.assertLess(ps.index("$ServiceName -cnotmatch"), ps.index("# Ensure installation directory exists"))

    def test_missing_install_option_value_fails_without_looping(self):
        src = SH.read_text().split("# macOS doesn't always require sudo for everything")[0]
        for option in ("--install-dir", "--install-service-name", "--install-source", "--install-version", "--install-sha256"):
            proc = subprocess.run(["bash", "-c", src, "--", option], capture_output=True, text=True, timeout=2)
            self.assertNotEqual(proc.returncode, 0, option)
            self.assertIn("requires a value", proc.stdout + proc.stderr)

    def test_shell_arguments_preserve_boundaries_and_escape_config_formats(self):
        src = SH.read_text()
        for name in ("shell_quote", "systemd_quote", "xml_escape", "nix_escape"):
            function(src, name)
        defs = "\n".join(function(src, n) for n in ("shell_quote", "systemd_quote", "xml_escape", "nix_escape"))
        script = defs + '\nshell_quote "$1"; echo; systemd_quote "$1"; echo; xml_escape "$1"; echo; nix_escape "$1"; echo\n'
        value = "a b'\"&<${oops}%$(printf payload)"
        p = subprocess.run(["bash", "-c", script, "--", value], capture_output=True, text=True, check=True)
        shell, systemd, xml, nix = p.stdout.splitlines()
        self.assertEqual(subprocess.run(["bash", "-c", "printf '%s' " + shell], capture_output=True, text=True, check=True).stdout, value)
        self.assertIn('%%', systemd)
        self.assertIn('$${oops}', systemd)
        self.assertIn('&amp;&lt;', xml)
        self.assertIn(r'\${oops}', nix)
        self.assertIn('komari_args+=("$1")', src)
        self.assertNotIn('komari_args="$komari_args $1"', src)
        self.assertNotIn('echo "$komari_args" | xargs', src)
        self.assertNotIn('ExecStart=${komari_agent_path} ${komari_args}', src)
        self.assertNotIn('command_args="${komari_args}"', src)
        self.assertNotIn('procd_set_param command \\$PROG \\$ARGS', src)
        self.assertNotIn('procd_set_param command', src)
        self.assertIn('ExecStart=${systemd_command}', src)

    def test_nixos_snippet_never_prints_agent_credentials(self):
        src = SH.read_text()
        self.assertNotIn('if [ "$init_system" = "nixos" ]; then', src)
        self.assertLess(src.index('systemd|openrc) ;;'), src.index('validate_install_directory "$target_dir"'))

    def test_custom_source_requires_fixed_checksum_and_verifies_binary(self):
        src = SH.read_text()
        fragment = src[src.index('# Construct the Agent download URL'):src.index('# Set executable permissions')]
        with tempfile.TemporaryDirectory(dir=pathlib.Path.home()) as tmp:
            root = pathlib.Path(tmp)
            binary = root / 'fixture'
            binary.write_bytes(b'known good agent fixture')
            digest = hashlib.sha256(binary.read_bytes()).hexdigest()
            (root / 'SHA256SUMS.txt').write_text(f'{digest}  komari-agent-linux-amd64\n')
            harness = '''
log_error() { printf '%s\\n' "$1"; }
log_step() { :; }
log_info() { :; }
log_success() { printf '%s\\n' "$1"; }
GREEN='' NC='' CYAN=''
os_name=linux
arch=amd64
install_source=$4
file_name=komari-agent-linux-amd64
install_sha256=$1
target_dir=$2
fixture=$3
curl() {
    local output=''
    while [[ $# -gt 0 ]]; do
        if [[ $1 == -o ]]; then output=$2; shift 2; else shift; fi
    done
    if [[ $output == */SHA256SUMS.txt ]]; then
        cp "$(dirname "$fixture")/SHA256SUMS.txt" "$output"
    else
        cp "$fixture" "$output"
    fi
}
''' + fragment
            def run(checksum, source='https://mirror.example.test/release'):
                return subprocess.run(['bash', '-c', harness, '--', checksum, str(root / 'installed'), str(binary), source],
                                      capture_output=True, text=True)
            missing = run('')
            self.assertNotEqual(missing.returncode, 0)
            self.assertIn('sha256', missing.stdout.lower())
            self.assertFalse((root / 'installed').exists())
            invalid = run('not-a-sha256')
            self.assertNotEqual(invalid.returncode, 0)
            self.assertIn('sha256', invalid.stdout.lower())
            mismatch = run('0' * 64)
            self.assertNotEqual(mismatch.returncode, 0)
            self.assertIn('mismatch', mismatch.stdout.lower())
            success = run(digest.upper())
            self.assertEqual(success.returncode, 0, success.stdout + success.stderr)
            self.assertIn('Verified', success.stdout)
            official = run('', 'https://github.com/3rnn/komari-lite/releases/download/v1.2.3')
            self.assertEqual(official.returncode, 0, official.stdout + official.stderr)
            self.assertIn('Verified', official.stdout)
            lookalike = run('', 'https://github.com/3rnn/komari-lite/releases/download/v1.2.3/other')
            self.assertNotEqual(lookalike.returncode, 0)
            insecure = run(digest, 'http://mirror.example.test/release')
            self.assertNotEqual(insecure.returncode, 0)
            self.assertIn('HTTPS', insecure.stdout)

    def test_windows_custom_source_requires_pinned_checksum(self):
        text = PS.read_text()
        self.assertIn('"--install-sha256"', text)
        self.assertIn('$InstallSha256', text)
        self.assertLess(text.index('Invalid --install-sha256'), text.index('# Check for nssm'))
        verification = text[text.index('Invoke-WebRequest -Uri $DownloadUrl'):text.index('# Create private config')]
        self.assertIn('Get-FileHash -Path $StagedAgent', verification)
        self.assertIn('if ($InstallSha256)', verification)
        self.assertIn('checksum mismatch', verification.lower())
        self.assertIn('requires --install-sha256', text)

    def test_windows_registered_nssm_path_is_checked_before_cleanup(self):
        source = PS.read_text()
        transaction = source[source.index('# Register and start service'):]
        self.assertIn('$registeredNssm = ([string]$serviceState.PathName).Trim()', transaction)
        check = transaction[transaction.index('$registeredNssm ='):transaction.index('} catch {\n    $reason')]
        self.assertIn('[string]::Equals($registeredNssm, $InstalledNssm,', check)
        self.assertIn('''[string]::Equals($registeredNssm, '"' + $InstalledNssm + '"',''', check)
        self.assertEqual(check.count('[System.StringComparison]::OrdinalIgnoreCase'), 2)
        self.assertIn('-and', check)
        self.assertIn("throw 'Registered service executable is not the permanent NSSM binary'", check)
        self.assertLess(transaction.index('$serviceState = Get-CimInstance Win32_Service'),
                        transaction.index('$registeredNssm ='))
        self.assertLess(transaction.index('$registeredNssm ='), transaction.index('Restore-Previous } catch'))
        for stage in ('$StagingDir', '$NssmStageDir'):
            self.assertLess(transaction.index('$registeredNssm ='),
                            transaction.index('Remove-Item -LiteralPath ' + stage + ' -Recurse'))

    def test_windows_nssm_receives_separate_arguments(self):
        text = PS.read_text()
        self.assertNotIn("$argString = $KomariArgs -join ' '", text)
        self.assertIn('& $InstalledNssm install $ServiceName $AgentPath', text)
        self.assertNotRegex(text, r'& \$nssmExeToUse install\b')
        self.assertLess(text.index('Installed NSSM checksum mismatch'),
                        text.index('& $InstalledNssm install $ServiceName $AgentPath'))
        self.assertIn('& $nssmExeToUse set $ServiceName AppParameters $argString', text)
        self.assertIn('Quote-WindowsArg', text)

    def test_shell_directory_chain_rejects_symlink_and_writable_parent(self):
        source = SH.read_text()
        validator = function(source, 'validate_install_directory')
        with tempfile.TemporaryDirectory(dir=pathlib.Path.home()) as temp:
            base = pathlib.Path(temp)
            safe = base / 'safe'
            safe.mkdir(mode=0o700)
            link = safe / 'link'
            link.symlink_to(base)
            script = 'log_error() { printf "%s\\n" "$1"; }; ' + validator + '\nvalidate_install_directory "$1"\n'
            # tempfile parent is ordinarily 0700; an explicit writable parent must fail.
            for path in (str(link / 'agent'), str(base / 'writable' / 'agent'), '/'):
                if 'writable' in path:
                    (base / 'writable').mkdir(mode=0o777)
                    (base / 'writable').chmod(0o777)
                result = subprocess.run(['bash', '-c', script, '--', path], capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0, path)
            result = subprocess.run(['bash', '-c', script, '--', str(safe / 'new')], capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertLess(source.index('validate_install_directory "$target_dir"'), source.index('staging_dir=$(mktemp'))

    def test_shell_service_provenance_and_preflight_order(self):
        source = SH.read_text()
        self.assertIn('verify_existing_service', source)
        self.assertLess(source.index('init_system=$(detect_init_system)'), source.index('commit_install ||'))
        self.assertLess(source.index('verify_existing_service ||'), source.index('commit_install ||'))
        self.assertNotIn('systemctl list-unit-files | grep -q', source)
        self.assertIn('FragmentPath', function(source, 'verify_existing_service'))
        self.assertIn('komari-agent installer', function(source, 'verify_existing_service'))
        self.assertIn('verify_systemd_execstart "$effective"', function(source, 'verify_existing_service'))
        self.assertIn('OpenRC upgrades require manual migration', function(source, 'verify_existing_service'))
        self.assertLess(source.index('init_system == openrc'), source.index('commit_install ||'))

    def test_shell_lifecycle_failures_cannot_report_success(self):
        source = SH.read_text()
        for command in ('systemctl daemon-reload', 'systemctl start', 'rc-update add', 'rc-service "$service_name" start'):
            self.assertIn(command, function(source, 'commit_install'))
        self.assertIn('rollback_install', function(source, 'commit_install'))

    def test_requested_version_requires_bound_release_source(self):
        shell = SH.read_text()
        windows = PS.read_text()
        self.assertIn('validate_release_version', shell)
        self.assertLess(shell.index('validate_release_version'), shell.index('staging_dir=$(mktemp'))
        self.assertRegex(windows, r'(?s)if \(\$InstallVersion\).*?InstallSource.*?exit 1')
        self.assertLess(windows.index('Invalid --install-version'), windows.index('# Check for nssm'))

    def test_shell_requested_version_rejects_mismatch_and_custom_source(self):
        definition = function(SH.read_text(), 'validate_release_version')
        script = ('log_error() { printf "%s\\n" "$1"; }; '
                  'install_version=$1; install_source=$2; ' + definition + '\nvalidate_release_version\n')
        for source in ('https://github.com/3rnn/komari-lite/releases/download/v1.2.4',
                       'https://mirror.test/v1.2.3'):
            result = subprocess.run(['bash', '-c', script, '--', '1.2.3', source],
                                    capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0, source)
        result = subprocess.run(['bash', '-c', script, '--', '1.2.3',
                                 'https://github.com/3rnn/komari-lite/releases/download/v1.2.3'],
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_windows_directory_service_and_lifecycle_guards(self):
        text = PS.read_text()
        self.assertIn('ReparsePoint', text)
        self.assertIn('Get-Acl', text[:text.index('# Check for nssm')])
        self.assertIn('NT SERVICE\\TrustedInstaller', text)
        self.assertIn('InheritOnly', text)
        self.assertIn('Get-CimInstance Win32_Service', text)
        self.assertIn('get $ServiceName Application', text)
        self.assertLess(text.index('Get-CimInstance Win32_Service'), text.index('function Restore-Previous'))
        self.assertRegex(text, r'& \$nssmExeToUse start \$ServiceName\s*\n\s*if \(\$LASTEXITCODE -ne 0\)')
        for key in ('DisplayName', 'Start SERVICE_AUTO_START', 'AppExit Default Restart', 'AppRestartDelay 5000'):
            self.assertRegex(text, re.escape('set $ServiceName ' + key) + r'[^\n]*\n\s*if \(\$LASTEXITCODE -ne 0\)')

    def test_preflight_blocks_root_runtime_paths(self):
        src = (ROOT / "scripts/github-preflight.sh").read_text()
        self.assertNotIn("'^/(data|secrets", src)
        self.assertNotIn("'^/komari($|", src)
        for path in ("data/panel.json", "release/artifact", "komari-agent-linux-amd64"):
            self.assertTrue(any(re.search(p, path) for p in re.findall(r"^reject_path .*? '([^']+)'$", src, re.M)), path)

    def test_preflight_rejects_staged_root_release_directory(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            (root / "scripts").mkdir()
            shutil.copyfile(ROOT / "scripts/github-preflight.sh", root / "scripts/github-preflight.sh")
            (root / "release").mkdir()
            (root / "release/artifact").write_text("fixture")
            subprocess.run(["git", "init", "-q", str(root)], check=True, capture_output=True)
            subprocess.run(["git", "add", "release/artifact"], cwd=root, check=True, capture_output=True)
            proc = subprocess.run(["bash", "scripts/github-preflight.sh"], cwd=root, capture_output=True, text=True)
            self.assertNotEqual(proc.returncode, 0)
            self.assertIn("Blocked runtime directory", proc.stderr)


if __name__ == "__main__":
    unittest.main()
