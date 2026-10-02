const shellUnsafePattern = /[\s"'\\$`!#&*();<>?[\]^{|}~]/;

export function quoteShellArg(value: string) {
  const trimmedValue = value.trim();

  if (trimmedValue === "") {
    return "''";
  }

  if (!shellUnsafePattern.test(trimmedValue)) {
    return trimmedValue;
  }

  return `'${trimmedValue.replace(/'/g, `'\\''`)}'`;
}

export function quoteShellArgs(args: string[]) {
  return args.map(quoteShellArg).join(" ");
}

export function quotePowerShellArg(value: string) {
  return `'${value.trim().replace(/'/g, "''")}'`;
}

/** Stage privately before sudo, so a partial failed download is never executed. */
export function linuxInstallCommand(scriptUrl: string, args: string[]): string {
  const program = 'script=$(mktemp) || exit 1; wget -qO "$script" "$1"; status=$?; if [ "$status" -eq 0 ]; then sudo bash "$script" "${@:2}"; status=$?; fi; rm -f "$script"; exit "$status"';
  return `bash -o pipefail -c '${program}' -- ${quoteShellArgs([scriptUrl, ...args])}`;
}

/** -EncodedCommand keeps the entire program outside cmd.exe's quoting grammar. */
export function windowsInstallCommand(scriptUrl: string, args: string[]): string {
  const program = [
    "$ErrorActionPreference = 'Stop'",
    "$privateDir = Join-Path $env:ProgramFiles ('komari-bootstrap-' + [guid]::NewGuid().ToString('N'))",
    "try {",
    "  New-Item -ItemType Directory -Path $privateDir -ErrorAction Stop | Out-Null",
    "  $acl = Get-Acl -LiteralPath $privateDir -ErrorAction Stop",
    "  $acl.SetAccessRuleProtection($true, $false)",
    "  $admins = [System.Security.Principal.SecurityIdentifier]::new('S-1-5-32-544')",
    "  $system = [System.Security.Principal.SecurityIdentifier]::new('S-1-5-18')",
    "  $acl.SetOwner($admins)",
    "  foreach ($identity in @($admins, $system)) { $acl.AddAccessRule([System.Security.AccessControl.FileSystemAccessRule]::new($identity, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')) }",
    "  Set-Acl -LiteralPath $privateDir -AclObject $acl -ErrorAction Stop",
    "  if (-not (Get-Acl -LiteralPath $privateDir -ErrorAction Stop).AreAccessRulesProtected) { throw 'Private installer directory ACL verification failed' }",
    "  $script = Join-Path $privateDir 'install.ps1'",
    `  Invoke-WebRequest -Uri ${quotePowerShellArg(scriptUrl)} -UseBasicParsing -OutFile $script -ErrorAction Stop`,
    `  & $script ${args.map(quotePowerShellArg).join(" ")}`,
    "  if ($LASTEXITCODE -ne 0) { throw \"Installer exited with code $LASTEXITCODE\" }",
    "} catch { Write-Error $_; exit 1 }",
    "finally { Remove-Item -LiteralPath $privateDir -Recurse -Force -ErrorAction SilentlyContinue }",
  ].join("\n");
  let utf16le = "";
  for (let index = 0; index < program.length; index++) {
    const code = program.charCodeAt(index);
    utf16le += String.fromCharCode(code & 0xff, code >> 8);
  }
  return `pwsh.exe -NoProfile -ExecutionPolicy Bypass -EncodedCommand ${btoa(utf16le)}`;
}