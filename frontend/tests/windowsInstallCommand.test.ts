import assert from "node:assert/strict";
import test from "node:test";
import { windowsInstallCommand } from "../src/utils/shellQuote.ts";

test("encoded Windows one-click command keeps shell metacharacters out of cmd.exe", () => {
  const args = ['-e', 'https://example.test/a?x="&calc&"', '-t', 'a" & echo hijack > file | %PATH% ^ ! ' + "'" + ' 🚀'];
  const command = windowsInstallCommand('https://example.test/install.ps1', args);
  assert.match(command, /^pwsh\.exe -NoProfile -ExecutionPolicy Bypass -EncodedCommand [A-Za-z0-9+/=]+$/);
  const program = Buffer.from(command.split(' ').at(-1)!, 'base64').toString('utf16le');
  assert.match(program, /Invoke-WebRequest -Uri 'https:\/\/example\.test\/install\.ps1' -UseBasicParsing -OutFile \$script -ErrorAction Stop/);
  assert.match(program, /Set-Acl -LiteralPath \$privateDir -AclObject \$acl -ErrorAction Stop/);
  assert.match(program, /\$acl\.SetAccessRuleProtection\(\$true, \$false\)/);
  assert.match(program, /\$script = Join-Path \$privateDir 'install\.ps1'/);
  assert.match(program, /& \$script /);
  assert.match(program, /finally \{ Remove-Item -LiteralPath \$privateDir -Recurse -Force -ErrorAction SilentlyContinue \}/);
  assert.ok(program.indexOf('Set-Acl -LiteralPath $privateDir') < program.indexOf('Invoke-WebRequest -Uri'));
  for (const arg of args) assert.ok(program.includes(` '${arg.replaceAll("'", "''")}'`), arg);
});

test("Windows download must finish before script execution, without a stale working-directory script", () => {
  const program = Buffer.from(windowsInstallCommand('https://example.test/install.ps1', ['-t', 'secret']).split(' ').at(-1)!, 'base64').toString('utf16le');
  assert.match(program, /try \{[\s\S]*Invoke-WebRequest[\s\S]*& \$script[\s\S]*\} catch \{/);
  assert.doesNotMatch(program, /&\s*['"]\.\\install\.ps1/);
  assert.match(program, /if \(\$LASTEXITCODE -ne 0\) \{ throw/);
});
