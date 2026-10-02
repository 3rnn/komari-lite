# Windows PowerShell installation script for Komari Agent

# Logging functions with colors
function Log-Info { param([string]$Message) Write-Host "$Message"    -ForegroundColor Cyan }
function Log-Success { param([string]$Message) Write-Host "$Message"    -ForegroundColor Green }
function Log-Warning { param([string]$Message) Write-Host "[WARNING] $Message"    -ForegroundColor Yellow }
function Log-Error { param([string]$Message) Write-Host "[ERROR] $Message"    -ForegroundColor Red }
function Log-Step { param([string]$Message) Write-Host "$Message"    -ForegroundColor Magenta }
function Log-Config { param([string]$Message) Write-Host "- $Message"    -ForegroundColor White }

# Default parameters
$InstallDir = Join-Path $Env:ProgramFiles "Komari"
$ServiceName = "komari-agent"
$InstallSource = ""
$KomariArgs = @()
$InstallVersion = ""
$InstallSha256 = "" # caller-pinned digest for custom download sources

# Parse script arguments
for ($i = 0; $i -lt $args.Count; $i++) {
    if ($args[$i] -in @('--install-dir', '--install-service-name', '--install-source', '--install-ghproxy', '--install-version', '--install-sha256') -and
        ($i + 1 -ge $args.Count -or [string]::IsNullOrEmpty($args[$i + 1]) -or $args[$i + 1] -like '--install*')) {
        Log-Error "$($args[$i]) requires a value"
        exit 1
    }
    switch ($args[$i]) {
        "--install-dir" { $InstallDir = $args[$i + 1]; $i++; continue }
        "--install-service-name" { $ServiceName = $args[$i + 1]; $i++; continue }
        "--install-source" { $InstallSource = $args[$i + 1].TrimEnd('/'); $i++; continue }
        "--install-ghproxy" { $i++; continue } # old copied commands: accepted but ignored
        "--install-version" { $InstallVersion = $args[$i + 1]; $i++; continue }
        "--install-sha256" { $InstallSha256 = $args[$i + 1]; $i++; continue }
        Default { $KomariArgs += $args[$i] }
    }
}

# Service identifiers must not address a different service or escape a service path.
if ($ServiceName -cnotmatch '^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$' -or $ServiceName -eq '.' -or $ServiceName -eq '..') {
    Log-Error 'Invalid --install-service-name (letters, digits, dot, underscore, hyphen only).'
    exit 1
}

# Validate the integrity policy before installing NSSM or changing the service.
# Only official release URLs may use the existing HTTPS checksum manifest flow.
if ([string]::IsNullOrWhiteSpace($InstallSource)) {
    Log-Error 'Missing --install-source (expected: a pinned GitHub release URL)'
    exit 1
}
if ($InstallSource -cnotmatch '^https://[^/]+(/.*)?$') {
    Log-Error '--install-source must be an HTTPS URL'
    exit 1
}
# Check before printing URLs or invoking a client that might echo credentials.
$SourceUri = $null
if (-not [Uri]::TryCreate($InstallSource, [UriKind]::Absolute, [ref]$SourceUri) -or
    $SourceUri.Scheme -ne 'https' -or $SourceUri.UserInfo -or
    $InstallSource -match '[?#\\]' -or $InstallSource -match '[\x00-\x1f\x7f]') {
    Log-Error 'Invalid --install-source: URL userinfo, query, and fragment are not allowed'
    exit 1
}
$AgentToken = ''
$AgentEndpoint = ''
$AgentCfSecret = ''
$ForwardArgs = [System.Collections.Generic.List[string]]::new()
for ($i = 0; $i -lt $KomariArgs.Count; $i++) {
    $arg = [string]$KomariArgs[$i]
    $key = $null
    $value = $null
    if ($arg -in @('-t', '--token', '-token', '-e', '--endpoint', '-endpoint', '--cf-access-client-secret', '-cf-access-client-secret')) {
        if ($i + 1 -ge $KomariArgs.Count -or -not $KomariArgs[$i + 1]) {
            Log-Error 'Missing Agent credential/endpoint value'; exit 1
        }
        $key = $arg; $value = [string]$KomariArgs[++$i]
    } elseif ($arg -cmatch '^(--token|-token|--endpoint|-endpoint|--cf-access-client-secret|-cf-access-client-secret|-t|-e)=(.*)$') {
        $key = $Matches[1]; $value = $Matches[2]
    } elseif ($arg -cmatch '^-[te](.+)$') {
        $key = $arg.Substring(0, 2); $value = $arg.Substring(2)
    } elseif ($arg -cmatch '^--?(config|token|endpoint|cf-access-client-secret|auto-discovery)') {
        Log-Error 'Unsupported Agent config/credential syntax'; exit 1
    } else {
        $ForwardArgs.Add($arg); continue
    }
    if (-not $value) { Log-Error 'Empty Agent credential/endpoint'; exit 1 }
    switch ($key) {
        { $_ -in @('-t', '--token', '-token') } { $AgentToken = $value; break }
        { $_ -in @('-e', '--endpoint', '-endpoint') } { $AgentEndpoint = $value; break }
        { $_ -in @('--cf-access-client-secret', '-cf-access-client-secret') } { $AgentCfSecret = $value; break }
    }
}
$KomariArgs = @($ForwardArgs.ToArray())
if ($AgentEndpoint) {
    $EndpointUri = $null
    if (-not [Uri]::TryCreate($AgentEndpoint, [UriKind]::Absolute, [ref]$EndpointUri) -or
        $EndpointUri.Scheme -ne 'https' -or $EndpointUri.UserInfo) {
        Log-Error 'Invalid Agent endpoint: HTTPS without userinfo required'; exit 1
    }
}
if ($InstallSha256 -and $InstallSha256 -cnotmatch '^[0-9a-fA-F]{64}$') {
    Log-Error 'Invalid --install-sha256 (expected 64 hexadecimal characters)'
    exit 1
}
if ($InstallSource -cnotmatch '^https://github[.]com/3rnn/komari-lite/releases/download/v[^/?#]+$' -and -not $InstallSha256) {
    Log-Error 'Custom --install-source requires --install-sha256 from a trusted, independent source'
    exit 1
}
if ($InstallVersion) {
    if ($InstallVersion -cnotmatch '^[0-9]+(\.[0-9]+){1,3}([-+][A-Za-z0-9.-]+)?$' -or
        $InstallSource -cne "https://github.com/3rnn/komari-lite/releases/download/v$InstallVersion") {
        Log-Error 'Invalid --install-version: requested version must match the official release URL exactly'
        exit 1
    }
}

# Windows PowerShell 5.1 rewrites native-process quotes; unverified NSSM
# AppParameters would be unsafe. Require the testable modern argument contract.
if ($PSVersionTable.PSVersion.Major -lt 7 -or
    ($PSVersionTable.PSVersion.Major -eq 7 -and $PSVersionTable.PSVersion.Minor -lt 3)) {
    Log-Error 'PowerShell 7.3+ required for safe NSSM argument quoting; Windows PowerShell 5.1 is unsupported.'
    exit 1
}
$PSNativeCommandArgumentPassing = 'Standard'

# Ensure running as Administrator
if (-not ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()
    ).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)) {
    Log-Error "Please run this script as Administrator."
    exit 1
}

# Detect architecture early for constructing binary name
switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    'x86' { $arch = '386' }
    Default { Log-Error "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE"; exit 1 }
}

# Ensure installation directory exists for nssm and agent. Reject junctions,
# reparse points and writable parent ACLs before a privileged binary is copied.
function Assert-TrustedDirectory {
    param([string]$Path)
    $absolute = [System.IO.Path]::GetFullPath($Path)
    if (-not [System.IO.Path]::IsPathFullyQualified($Path) -or
        $Path -match '[*?\[\]]' -or $Path -match '(^|[\\/])\.\.?([\\/]|$)' -or
        $absolute.StartsWith('\\') -or $absolute -eq [System.IO.Path]::GetPathRoot($absolute)) {
        throw 'Unsafe installation path component, relative path or network path'
    }
    $root = [System.IO.Path]::GetPathRoot($absolute)
    $trustedInstaller = ([System.Security.Principal.NTAccount]::new('NT SERVICE\TrustedInstaller')).Translate(
        [System.Security.Principal.SecurityIdentifier]).Value
    $trustedOwners = @('S-1-5-32-544', 'S-1-5-18', $trustedInstaller)
    # Inspect every existing ancestor before creating anything beneath it.
    $current = $root
    foreach ($segment in @('') + $absolute.Substring($root.Length).Split([char]'\', [System.StringSplitOptions]::RemoveEmptyEntries)) {
        if ($segment) { $current = Join-Path $current $segment }
        if (-not (Test-Path -LiteralPath $current)) { break }
        $item = Get-Item -LiteralPath $current -Force -ErrorAction Stop
        if (-not $item.PSIsContainer -or ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
            throw 'Install directory chain contains a file or reparse point'
        }
        $acl = Get-Acl -LiteralPath $current -ErrorAction Stop
        $owner = $acl.GetOwner([System.Security.Principal.SecurityIdentifier]).Value
        if ($owner -notin $trustedOwners) { throw 'Untrusted directory owner' }
        foreach ($rule in $acl.GetAccessRules($true, $true, [System.Security.Principal.SecurityIdentifier])) {
            if ($rule.AccessControlType -ne 'Allow' -or
                ($rule.PropagationFlags -band [System.Security.AccessControl.PropagationFlags]::InheritOnly)) { continue }
            $writeRights = [int][System.Security.AccessControl.FileSystemRights]::Modify -bor
                [int][System.Security.AccessControl.FileSystemRights]::ChangePermissions -bor
                [int][System.Security.AccessControl.FileSystemRights]::TakeOwnership
            $write = [int]$rule.FileSystemRights -band $writeRights
            if ($write -and $rule.IdentityReference.Value -notin $trustedOwners) {
                throw 'Untrusted writable directory ACL'
            }
        }
    }
    $current = $root
    foreach ($segment in $absolute.Substring($root.Length).Split([char]'\', [System.StringSplitOptions]::RemoveEmptyEntries)) {
        $current = Join-Path $current $segment
        if (-not (Test-Path -LiteralPath $current)) {
            New-Item -ItemType Directory -Path $current -ErrorAction Stop | Out-Null
            $newAcl = Get-Acl -LiteralPath $current -ErrorAction Stop
            $newAcl.SetAccessRuleProtection($true, $false)
            $newAcl.SetOwner([System.Security.Principal.SecurityIdentifier]::new('S-1-5-32-544'))
            foreach ($sid in @('S-1-5-32-544', 'S-1-5-18')) {
                $rule = [System.Security.AccessControl.FileSystemAccessRule]::new(
                    [System.Security.Principal.SecurityIdentifier]::new($sid), 'FullControl',
                    'ContainerInherit,ObjectInherit', 'None', 'Allow')
                $newAcl.AddAccessRule($rule)
            }
            Set-Acl -LiteralPath $current -AclObject $newAcl -ErrorAction Stop
        }
    }
    $current = $root
    foreach ($segment in @('') + $absolute.Substring($root.Length).Split([char]'\', [System.StringSplitOptions]::RemoveEmptyEntries)) {
        if ($segment) { $current = Join-Path $current $segment }
        $item = Get-Item -LiteralPath $current -Force -ErrorAction Stop
        if (-not $item.PSIsContainer -or ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
            throw 'Install directory chain contains a file or reparse point'
        }
        $acl = Get-Acl -LiteralPath $current -ErrorAction Stop
        $owner = $acl.GetOwner([System.Security.Principal.SecurityIdentifier]).Value
        if ($owner -notin $trustedOwners) { throw 'Untrusted directory owner' }
        foreach ($rule in $acl.GetAccessRules($true, $true, [System.Security.Principal.SecurityIdentifier])) {
            if ($rule.AccessControlType -ne 'Allow' -or
                ($rule.PropagationFlags -band [System.Security.AccessControl.PropagationFlags]::InheritOnly)) { continue }
            $sid = $rule.IdentityReference.Value
            $writeRights = [int][System.Security.AccessControl.FileSystemRights]::Modify -bor
                [int][System.Security.AccessControl.FileSystemRights]::ChangePermissions -bor
                [int][System.Security.AccessControl.FileSystemRights]::TakeOwnership
            $write = [int]$rule.FileSystemRights -band $writeRights
            if ($write -and $sid -notin $trustedOwners) {
                throw 'Untrusted writable directory ACL'
            }
        }
    }
    foreach ($name in @('nssm.exe', 'komari-agent.exe', 'agent-config.json')) {
        $candidate = Join-Path $absolute $name
        if ((Test-Path -LiteralPath $candidate) -and
            ((Get-Item -LiteralPath $candidate -Force).Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
            throw 'Reparse point at installed artifact'
        }
    }
    return $absolute
}
try { $InstallDir = Assert-TrustedDirectory $InstallDir }
catch { Log-Error "Unsafe --install-dir: $_"; exit 1 }

function Set-PrivateDirectory {
    param([string]$Path)
    $acl = Get-Acl -LiteralPath $Path -ErrorAction Stop
    $acl.SetAccessRuleProtection($true, $false)
    $admins = [System.Security.Principal.SecurityIdentifier]::new('S-1-5-32-544')
    $acl.SetOwner($admins)
    foreach ($sid in @('S-1-5-32-544', 'S-1-5-18')) {
        $rule = [System.Security.AccessControl.FileSystemAccessRule]::new(
            [System.Security.Principal.SecurityIdentifier]::new($sid), 'FullControl',
            'ContainerInherit,ObjectInherit', 'None', 'Allow')
        $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $Path -AclObject $acl -ErrorAction Stop
    $check = Get-Acl -LiteralPath $Path -ErrorAction Stop
    if (-not $check.AreAccessRulesProtected) { throw 'Private staging ACL verification failed' }
}

# Paths and existing service provenance: never replace a foreign service.
$AgentPath = Join-Path $InstallDir 'komari-agent.exe'
$ExpectedNssm = Join-Path $InstallDir 'nssm.exe'
try {
    $ExistingService = Get-CimInstance Win32_Service -Filter "Name='$ServiceName'" -ErrorAction Stop
    if ($ExistingService) {
        $nssmImage = '^\s*"?' + [regex]::Escape($ExpectedNssm) + '"?(?:\s|$)'
        if ($ExistingService.DisplayName -cne 'Komari Agent Service' -or
            $ExistingService.PathName -notmatch $nssmImage) {
            throw 'Existing service is not an installer-managed NSSM service'
        }
        $registered = (Get-ItemProperty -LiteralPath "HKLM:\SYSTEM\CurrentControlSet\Services\$ServiceName\Parameters" -Name Application -ErrorAction Stop).Application
        if (-not [string]::Equals([System.IO.Path]::GetFullPath($registered), $AgentPath,
            [System.StringComparison]::OrdinalIgnoreCase)) {
            throw 'Existing service points to another executable'
        }
    }
} catch { Log-Error "Existing service provenance cannot be verified: $_"; exit 1 }

# Check for nssm and download if not present
$nssmExeToUse = Join-Path $InstallDir "nssm.exe"
$NssmStageDir = Join-Path $InstallDir ('.nssm-stage-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $NssmStageDir -ErrorAction Stop | Out-Null
Set-PrivateDirectory $NssmStageDir

# Do not execute an existing PATH/local NSSM binary: version output is not provenance.
# Always obtain the pinned archive and verify it before running the extracted exe.
$nssmCmd = $null

# Download a pinned NSSM binary into the installation directory.
if (-not $nssmCmd) {
    Log-Info "nssm not found or not usable. Attempting to download to $InstallDir..."
    $NssmVersion = "2.24"
    $NssmZipUrl = "https://nssm.cc/release/nssm-$NssmVersion.zip"
    $TempExtractDir = Join-Path $env:TEMP ("nssm_extract_" + [guid]::NewGuid().ToString('N'))
    $TempNssmZipPath = Join-Path $TempExtractDir "nssm-$NssmVersion.zip"

    try {
        New-Item -ItemType Directory -Path $TempExtractDir -Force -ErrorAction Stop | Out-Null
        Log-Info "Downloading nssm from $NssmZipUrl..."
        Invoke-WebRequest -Uri $NssmZipUrl -OutFile $TempNssmZipPath -UseBasicParsing -ErrorAction Stop
        # SHA-256 of the official nssm 2.24 ZIP fetched from https://nssm.cc/release/nssm-2.24.zip.
        # Pin the archive, not a checksum fetched alongside the potentially compromised download.
        $NssmZipSha256 = '727d1e42275c605e0f04aba98095c38a8e1e46def453cdffce42869428aa6743'
        if ((Get-FileHash -Path $TempNssmZipPath -Algorithm SHA256 -ErrorAction Stop).Hash -ne $NssmZipSha256) {
            throw 'NSSM archive checksum mismatch'
        }

        Expand-Archive -Path $TempNssmZipPath -DestinationPath $TempExtractDir -Force -ErrorAction Stop
        
        $NssmSourceDirInsideZip = "nssm-$NssmVersion" # Used for Get-ChildItem search path
        # The path part within the extracted nssm folder, e.g., "nssm-2.24\win32"
        # 'win32' nssm is used for both 'amd64' and 'arm64' PowerShell architectures.
        $NssmArchSubDir = Join-Path "nssm-$NssmVersion" "win32"
        $NssmSourceExePath = Join-Path (Join-Path $TempExtractDir $NssmArchSubDir) "nssm.exe"

        if (-not (Test-Path $NssmSourceExePath)) {
            Log-Error "Could not find nssm.exe at expected path: $NssmSourceExePath after extraction."
            # Fallback search for nssm.exe within the extracted directory
            $foundNssmFallback = Get-ChildItem -Path $TempExtractDir -Recurse -Filter "nssm.exe" | 
            Where-Object { $_.FullName -like "*$NssmArchSubDir\nssm.exe" } | 
            Select-Object -First 1
            if ($foundNssmFallback) {
                Log-Warning "Found nssm.exe at $($foundNssmFallback.FullName) using fallback search. Using this."
                $NssmSourceExePath = $foundNssmFallback.FullName
            }
            else {
                Log-Error "nssm.exe ($NssmArchSubDir) still not found in $TempExtractDir. Please install nssm manually (from https://nssm.cc) and ensure it's in your PATH."
                exit 1
            }
        }
        
        $TrustedNssmHash = (Get-FileHash -Path $NssmSourceExePath -Algorithm SHA256 -ErrorAction Stop).Hash
        $nssmExeToUse = Join-Path $NssmStageDir 'nssm.exe'
        Copy-Item -Path $NssmSourceExePath -Destination $nssmExeToUse -Force -ErrorAction Stop
        if ((Get-FileHash -Path $nssmExeToUse -Algorithm SHA256 -ErrorAction Stop).Hash -ne $TrustedNssmHash) {
            throw 'Copied NSSM binary checksum mismatch'
        }
        $nssmVersionOutput = & $nssmExeToUse version 2>&1
        if ($LASTEXITCODE -ne 0) { throw 'Pinned NSSM failed to execute' }
        $nssmCmd = $nssmExeToUse
    }
    catch {
        Log-Error "Failed to download or configure nssm: $_"
        Log-Error "Please install nssm manually from https://nssm.cc and ensure nssm.exe is in your PATH."
        exit 1
    }
    finally {
        if (Test-Path $TempNssmZipPath) { Remove-Item $TempNssmZipPath -Force -ErrorAction SilentlyContinue }
        if (Test-Path $TempExtractDir) { Remove-Item $TempExtractDir -Recurse -Force -ErrorAction SilentlyContinue }
    }
}

# Final check that nssm is operational
try {
    $nssmVersionOutput = & $nssmExeToUse version 2>&1
    if ($LASTEXITCODE -ne 0) { throw 'Pinned NSSM version command failed' }
}
catch {
    Log-Error "nssm command failed to execute even after setup attempts. Please check the nssm installation and PATH. Error: $_"
    exit 1
}

Log-Step "Installation configuration:"
Log-Config "Service name: $ServiceName"
Log-Config "Install directory: $InstallDir"
Log-Config "Agent source: $InstallSource"
Log-Config "Agent arguments: [redacted] ($($KomariArgs.Count) arguments)"
if ($InstallVersion -ne "") {
    Log-Config "Specified agent version: $InstallVersion"
} else {
    Log-Config "Agent version: panel-managed"
}

# Paths
$BinaryName = "komari-agent-windows-$arch.exe"
$AgentPath = Join-Path $InstallDir "komari-agent.exe"

# Existing services are kept in place; replacement is transactional below.
# Agent binaries come from the explicitly configured release source.
$BinaryName = "komari-agent-windows-$arch.exe"
if ([string]::IsNullOrWhiteSpace($InstallSource)) {
    Log-Error "Missing --install-source (expected: a pinned GitHub release URL)"
    exit 1
}
$versionToInstall = if ($InstallVersion) { $InstallVersion } else { "panel-managed" }
$DownloadUrl = "$($InstallSource.TrimEnd('/'))/$BinaryName"
Log-Success "Installing Komari Agent version: $versionToInstall"

# Download and verify before stopping the existing service.
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Log-Info "URL: $DownloadUrl"
$StagingDir = Join-Path $InstallDir (".agent-download-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $StagingDir -Force -ErrorAction Stop | Out-Null
Set-PrivateDirectory $StagingDir
$StagedAgent = Join-Path $StagingDir $BinaryName
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $StagedAgent -UseBasicParsing -ErrorAction Stop
    if ($InstallSha256) {
        # Caller-supplied fixed checksum takes precedence over a mutable manifest.
        $expected = $InstallSha256
    }
    else {
        $ChecksumFile = Join-Path $StagingDir 'SHA256SUMS.txt'
        Invoke-WebRequest -Uri "$InstallSource/SHA256SUMS.txt" -OutFile $ChecksumFile -UseBasicParsing -ErrorAction Stop
        $escapedName = [regex]::Escape($BinaryName)
        $matches = @(Get-Content -Path $ChecksumFile | Where-Object { $_ -match "(?i)^[0-9a-f]{64}\s+$escapedName$" })
        if ($matches.Count -ne 1) { throw "Release checksum missing or ambiguous for $BinaryName" }
        $expected = ($matches[0] -split '\s+')[0]
    }
    $actual = (Get-FileHash -Path $StagedAgent -Algorithm SHA256 -ErrorAction Stop).Hash
    if ($actual -ne $expected) { throw "Agent checksum mismatch for $BinaryName" }
    Log-Success "Verified $BinaryName against SHA-256 checksum"
}
catch {
    Remove-Item -Path $StagingDir -Recurse -Force -ErrorAction SilentlyContinue
    Log-Error "Download or checksum verification failed: $_"
    exit 1
}
# Create private config before replacing the working service. Do not put token on
# NSSM AppParameters (readable in service registry and process command lines).
$ConfigPath = Join-Path $InstallDir 'agent-config.json'
$ConfigTemp = Join-Path $InstallDir ('.agent-config-' + [guid]::NewGuid().ToString('N'))
try {
    # Create an empty file only; lock down inheritance before secrets exist on disk.
    $file = [System.IO.File]::Open($ConfigTemp, [System.IO.FileMode]::CreateNew,
        [System.IO.FileAccess]::Write, [System.IO.FileShare]::None)
    $file.Dispose()
    $acl = Get-Acl -LiteralPath $ConfigTemp -ErrorAction Stop
    $acl.SetAccessRuleProtection($true, $false)
    $admins = New-Object System.Security.Principal.SecurityIdentifier('S-1-5-32-544')
    $system = New-Object System.Security.Principal.SecurityIdentifier('S-1-5-18')
    $acl.SetOwner($admins)
    foreach ($identity in @($admins, $system)) {
        $rule = New-Object System.Security.AccessControl.FileSystemAccessRule($identity, 'FullControl', 'Allow')
        $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $ConfigTemp -AclObject $acl -ErrorAction Stop
    if ((Get-Acl -LiteralPath $ConfigTemp -ErrorAction Stop).AreAccessRulesProtected -ne $true) {
        throw 'Private config ACL verification failed'
    }
    $configJson = @{ token = $AgentToken; endpoint = $AgentEndpoint; cf_access_client_secret = $AgentCfSecret } | ConvertTo-Json -Compress
    [System.IO.File]::WriteAllText($ConfigTemp, $configJson, (New-Object System.Text.UTF8Encoding($false)))

} catch {
    Remove-Item -LiteralPath $ConfigTemp -Force -ErrorAction SilentlyContinue
    Log-Error 'Cannot install private Agent config; existing service was not changed.'
    exit 1
}
$KomariArgs += @('--config', $ConfigPath)

# Format each original argument as one Windows command-line argument for NSSM's
# AppParameters string (the Agent is ultimately started via CreateProcess).
function Quote-WindowsArg {
    param([string]$Value)
    $escaped = [regex]::Replace($Value, '(\\*)"', {
        param($match)
        return ('\' * ($match.Groups[1].Value.Length * 2 + 1)) + '"'
    })
    $escaped = [regex]::Replace($escaped, '(\\+)$', {
        param($match)
        return '\' * ($match.Groups[1].Value.Length * 2)
    })
    return '"' + $escaped + '"'
}

# Register and start service
Log-Step "Configuring Windows service with nssm..."
$argString = (@($KomariArgs | ForEach-Object { Quote-WindowsArg $_ }) -join ' ')
$InstalledNssm = Join-Path $InstallDir 'nssm.exe'
$PriorFiles = @{}
$PriorRunning = $false
$PriorParameters = $null
$MutationStarted = $false
function Restore-Previous {
    $failed = $false
    $current = Get-CimInstance Win32_Service -Filter "Name='$ServiceName'" -ErrorAction Stop
    if ($current) {
        if ($current.State -cne 'Stopped') {
            & $nssmExeToUse stop $ServiceName 2>&1 | Out-Null
            if ($LASTEXITCODE -ne 0) { $failed = $true }
        }
        if (-not $ExistingService) {
            & $nssmExeToUse remove $ServiceName confirm 2>&1 | Out-Null
            if ($LASTEXITCODE -ne 0) { $failed = $true }
        }
    }
    foreach ($entry in @(@($AgentPath, 'agent'), @($ConfigPath, 'config'), @($InstalledNssm, 'nssm'))) {
        $path = $entry[0]; $name = $entry[1]
        try {
            if ($PriorFiles[$name]) {
                Copy-Item -LiteralPath (Join-Path $StagingDir "backup-$name") -Destination $path -Force -ErrorAction Stop
                if ($name -eq 'config') {
                    Set-Acl -LiteralPath $path -AclObject (Get-Acl -LiteralPath (Join-Path $StagingDir 'backup-config') -ErrorAction Stop) -ErrorAction Stop
                    if (-not (Get-Acl -LiteralPath $path -ErrorAction Stop).AreAccessRulesProtected) { throw 'Restored config ACL is not private' }
                }
            } else {
                Remove-Item -LiteralPath $path -Force -ErrorAction Stop
            }
        } catch {
            if ($PriorFiles[$name] -or (Test-Path -LiteralPath $path)) { $failed = $true }
        }
    }
    if ($ExistingService) {
        & $nssmExeToUse set $ServiceName AppParameters $PriorParameters 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { $failed = $true }
        if ($PriorRunning) {
            & $nssmExeToUse start $ServiceName 2>&1 | Out-Null
            if ($LASTEXITCODE -ne 0) { $failed = $true }
            else {
                Start-Sleep -Seconds 2
                $restored = Get-CimInstance Win32_Service -Filter "Name='$ServiceName'" -ErrorAction Stop
                $restoredApp = (& $nssmExeToUse status $ServiceName 2>&1 | Out-String).Trim()
                if ($LASTEXITCODE -ne 0 -or -not $restored -or $restored.State -cne 'Running' -or
                    $restoredApp -cne 'SERVICE_RUNNING') { $failed = $true }
            }
        }
    }
    if ($failed) { throw "Rollback incomplete; recovery copies retained in $StagingDir" }
}
try {
    if ($ExistingService) {
        $application = & $nssmExeToUse get $ServiceName Application 2>&1
        if ($LASTEXITCODE -ne 0 -or
            -not [string]::Equals(([string]$application).Trim(), $AgentPath, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw 'NSSM application provenance mismatch'
        }
        $PriorParameters = (& $nssmExeToUse get $ServiceName AppParameters 2>&1 | Out-String).Trim()
        if ($LASTEXITCODE -ne 0 -or $PriorParameters -match '(?i)(?:^|\s)-{1,2}(?:t|token|cf-access-client-secret)(?:\s|=)') {
            throw 'Cannot safely restore a legacy service with credentials in process arguments'
        }
        $PriorRunning = $ExistingService.State -ceq 'Running'
    } elseif ((Test-Path -LiteralPath $AgentPath) -or (Test-Path -LiteralPath $ConfigPath)) {
        throw 'Existing Agent artifacts without a verified service; manual migration required'
    }
    foreach ($entry in @(@($AgentPath, 'agent'), @($ConfigPath, 'config'), @($InstalledNssm, 'nssm'))) {
        $path = $entry[0]; $name = $entry[1]
        $PriorFiles[$name] = Test-Path -LiteralPath $path
        if ($PriorFiles[$name]) { Copy-Item -LiteralPath $path -Destination (Join-Path $StagingDir "backup-$name") -ErrorAction Stop }
    }
    $MutationStarted = $true
    if ($PriorRunning) {
        & $nssmExeToUse stop $ServiceName 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Failed to stop existing service' }
    }
    Move-Item -LiteralPath $StagedAgent -Destination $AgentPath -Force -ErrorAction Stop
    Move-Item -LiteralPath $ConfigTemp -Destination $ConfigPath -Force -ErrorAction Stop
    if (-not (Get-Acl -LiteralPath $ConfigPath -ErrorAction Stop).AreAccessRulesProtected) { throw 'Installed config ACL verification failed' }
    Copy-Item -LiteralPath $nssmExeToUse -Destination $InstalledNssm -Force -ErrorAction Stop
    if ((Get-FileHash -Path $InstalledNssm -Algorithm SHA256 -ErrorAction Stop).Hash -ne $TrustedNssmHash) { throw 'Installed NSSM checksum mismatch' }
    if (-not $ExistingService) {
        & $nssmExeToUse install $ServiceName $AgentPath
        if ($LASTEXITCODE -ne 0) { throw 'NSSM failed to install the service' }
    }
    & $nssmExeToUse set $ServiceName AppParameters $argString
    if ($LASTEXITCODE -ne 0) { throw 'NSSM failed to set Agent arguments' }
    if (-not $ExistingService) {
        & $nssmExeToUse set $ServiceName DisplayName "Komari Agent Service"
        if ($LASTEXITCODE -ne 0) { throw 'NSSM failed to set display name' }
        & $nssmExeToUse set $ServiceName Start SERVICE_AUTO_START
        if ($LASTEXITCODE -ne 0) { throw 'NSSM failed to enable service startup' }
        & $nssmExeToUse set $ServiceName AppExit Default Restart
        if ($LASTEXITCODE -ne 0) { throw 'NSSM failed to set restart policy' }
        & $nssmExeToUse set $ServiceName AppRestartDelay 5000
        if ($LASTEXITCODE -ne 0) { throw 'NSSM failed to set restart delay' }
        & $nssmExeToUse set $ServiceName AppNoConsole 1
        if ($LASTEXITCODE -ne 0) { throw 'NSSM failed to configure console handling' }
    }
    & $nssmExeToUse start $ServiceName
    if ($LASTEXITCODE -ne 0) { throw 'NSSM failed to start service' }
    Start-Sleep -Seconds 2
    $serviceState = Get-CimInstance Win32_Service -Filter "Name='$ServiceName'" -ErrorAction Stop
    $appState = (& $nssmExeToUse status $ServiceName 2>&1 | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or -not $serviceState -or $serviceState.State -cne 'Running' -or
        $appState -cne 'SERVICE_RUNNING') { throw 'Agent service did not remain running after start' }
} catch {
    $reason = $_.Exception.Message
    if ($MutationStarted) {
        try { Restore-Previous } catch { Log-Error $_.Exception.Message; exit 1 }
    }
    Log-Error "Installation failed: $reason"
    exit 1
}
Remove-Item -LiteralPath $StagingDir -Recurse -Force -ErrorAction Stop
Remove-Item -LiteralPath $NssmStageDir -Recurse -Force -ErrorAction Stop
Log-Success "Service $ServiceName installed and verified running."

Log-Success "Komari Agent installation completed!"
Log-Config "Service name: $ServiceName"
Log-Config "Agent arguments: [redacted] ($($KomariArgs.Count) arguments)"
