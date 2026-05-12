<#
.SYNOPSIS
    Install the latest pscli CLI release on Windows.

.DESCRIPTION
    Downloads pscli-windows-amd64.zip from the GitHub release matching
    -Version (default: latest), extracts pscli.exe, and places it in
    -InstallDir (default: %LOCALAPPDATA%\Programs\pscli).

.PARAMETER Repo
    GitHub "owner/repo". Defaults to perfectscale/poc-cli or $env:pscli_REPO.

.PARAMETER Version
    Release tag (e.g. v1.2.3) or "latest". Defaults to "latest" or
    $env:pscli_VERSION.

.PARAMETER InstallDir
    Install destination. Defaults to %LOCALAPPDATA%\Programs\pscli or
    $env:pscli_INSTALL_DIR.

.PARAMETER AddToPath
    If set, prepends InstallDir to the current user's PATH (persistent).

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\install.ps1

.EXAMPLE
    .\install.ps1 -Version v1.2.3 -InstallDir C:\tools\pscli -AddToPath
#>

[CmdletBinding()]
param(
    [string]$Repo       = $(if ($env:pscli_REPO)        { $env:pscli_REPO }        else { 'perfectscale/poc-cli' }),
    [string]$Version    = $(if ($env:pscli_VERSION)     { $env:pscli_VERSION }     else { 'latest' }),
    [string]$InstallDir = $(if ($env:pscli_INSTALL_DIR) { $env:pscli_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\pscli' }),
    [switch]$AddToPath
)

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# Only windows-amd64 is published today.
$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -ne 'AMD64') {
    Write-Error "Unsupported architecture: $arch. Only windows-amd64 is published."
}

$asset  = 'pscli-windows-amd64.zip'
$binary = 'pscli.exe'

if ($Version -eq 'latest') {
    $url = "https://github.com/$Repo/releases/latest/download/$asset"
} else {
    $url = "https://github.com/$Repo/releases/download/$Version/$asset"
}

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("pscli-install-" + [Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp -Force | Out-Null

try {
    Write-Host "Downloading $url"
    $zipPath = Join-Path $tmp $asset
    Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing

    Write-Host "Extracting to $InstallDir"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Expand-Archive -Path $zipPath -DestinationPath $tmp -Force

    $src = Join-Path $tmp $binary
    if (-not (Test-Path $src)) {
        Write-Error "Extracted archive does not contain $binary"
    }
    Move-Item -Path $src -Destination (Join-Path $InstallDir $binary) -Force

    $installed = Join-Path $InstallDir $binary
    Write-Host "Installed $installed"

    & $installed --version

    if ($AddToPath) {
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $parts    = if ($userPath) { $userPath.Split(';') } else { @() }
        if ($parts -notcontains $InstallDir) {
            $newPath = ($InstallDir, $userPath -ne '' | Where-Object { $_ }) -join ';'
            [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
            Write-Host "Added $InstallDir to user PATH (open a new shell to pick it up)."
        }
    } else {
        $sessionPath = ";$env:Path;"
        if ($sessionPath -notlike "*;$InstallDir;*") {
            Write-Warning "$InstallDir is not on PATH. Re-run with -AddToPath, or add it manually."
        }
    }
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
