#!/usr/bin/env pwsh
#Requires -Version 5.1
<#
moca installer — Windows.

Downloads a release package (the latest one, or -Version <v>), verifies it
against the release's checksums.txt, and installs moca.exe.

Re-running is safe and idempotent: an existing installation is updated in
place — the script never fails because moca is already installed. The
default target is the directory of the moca already on PATH, else
%LOCALAPPDATA%\Programs\moca (which is added to the user PATH).

With -Uninstall the installed moca.exe is removed instead — the config,
data and state directories are deliberately left untouched. -RemoveData
(which implies -Uninstall) removes the whole installation: the binary plus
%USERPROFILE%\.config\moca, %USERPROFILE%\.local\share\moca and
%USERPROFILE%\.local\state\moca (or their $env:XDG_*_HOME equivalents —
moca applies the XDG rules on every platform).

usage:
    .\install.ps1 [-Version <vX.Y.Z>] [-Dir <path>] [-Uninstall] [-RemoveData]

env: MOCA_REPO (default adeotek/moca), MOCA_API_BASE (default the GitHub API)
#>
[CmdletBinding()]
param(
    [string]$Version = '',
    [string]$Dir = '',
    [switch]$Uninstall,
    [switch]$RemoveData,
    [string]$Repo = $(if ($env:MOCA_REPO) { $env:MOCA_REPO } else { 'adeotek/moca' }),
    [string]$ApiBase = $(if ($env:MOCA_API_BASE) { $env:MOCA_API_BASE } else { 'https://api.github.com' })
)

$ErrorActionPreference = 'Stop'
# Windows PowerShell 5.1: its progress bar slows downloads to a crawl, and
# older .NET defaults may not offer TLS 1.2 (which GitHub requires).
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

# $IsWindows is $null on Windows PowerShell 5.1 (which only runs on Windows).
if ($IsWindows -eq $false) {
    throw 'this installer is for Windows — on Linux use install.sh'
}

if ($Uninstall -or $RemoveData) {
    if ($Version) { throw '-Version has no effect together with -Uninstall/-RemoveData' }
    if (-not $Dir) {
        # -CommandType Application: an alias or function named moca has no
        # .Source path, and we must not delete a random moca.exe in the cwd.
        $existing = Get-Command moca -CommandType Application -ErrorAction SilentlyContinue
        if (-not $existing) { throw 'moca is not on PATH — pass -Dir <path> to name the installation directory' }
        $Dir = Split-Path -Parent $existing.Source
    }
    $target = Join-Path $Dir 'moca.exe'
    if (-not (Test-Path $target)) { throw "no moca.exe at $target" }
    # Make sure the file we are about to delete really is moca.
    $versionOut = ''
    try { $versionOut = (& $target --version 2>$null | Out-String).Trim() } catch { $versionOut = '' }
    if ($versionOut -notmatch '^moca') { throw "$target is not moca (its --version output does not start with 'moca') — pass -Dir <path> to name the installation directory explicitly" }
    try {
        Remove-Item -Path $target -Force
    } catch {
        throw "cannot remove $target — close all moca instances and re-run ($_)"
    }
    # A leftover from a previous update, if any.
    Remove-Item -Path "$target.old" -Force -ErrorAction SilentlyContinue
    Write-Host "removed moca from $target"
    if ($RemoveData) {
        $configDir = if ($env:XDG_CONFIG_HOME) { Join-Path $env:XDG_CONFIG_HOME 'moca' } else { Join-Path $env:USERPROFILE '.config/moca' }
        $dataDir = if ($env:XDG_DATA_HOME) { Join-Path $env:XDG_DATA_HOME 'moca' } else { Join-Path $env:USERPROFILE '.local/share/moca' }
        $stateDir = if ($env:XDG_STATE_HOME) { Join-Path $env:XDG_STATE_HOME 'moca' } else { Join-Path $env:USERPROFILE '.local/state/moca' }
        Remove-Item -Path $configDir, $dataDir, $stateDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-Host "removed moca data: $configDir, $dataDir, $stateDir"
    } else {
        Write-Host 'note: left the config, data and state directories untouched (configuration and credentials) — pass -RemoveData for a full uninstall'
    }
    # return, not exit: `irm ... | iex` runs this in the caller's session.
    return
}

switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    default { throw "unsupported architecture: $env:PROCESSOR_ARCHITECTURE (windows/amd64 is published; 32-bit is not)" }
}

# Tolerate both "0.2.0-beta" and "v0.2.0-beta".
if ($Version -and -not $Version.StartsWith('v')) { $Version = "v$Version" }

if ($Version) {
    Write-Host "moca installer: release $Version, windows/$arch"
    $release = Invoke-RestMethod -Uri "$ApiBase/repos/$Repo/releases/tags/$Version"
} else {
    Write-Host "moca installer: latest release, windows/$arch"
    $release = @(Invoke-RestMethod -Uri "$ApiBase/repos/$Repo/releases?per_page=1")[0]
}
if (-not $release) { throw "no releases found in $Repo" }

$tag = $release.tag_name
$assetName = "moca-$tag-windows-$arch.zip"
$asset = $release.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1
if (-not $asset) { throw "release $tag has no package for windows/$arch" }
$sumsAsset = $release.assets | Where-Object { $_.name -eq 'checksums.txt' } | Select-Object -First 1

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("moca-install-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    $zip = Join-Path $tmp $assetName
    Write-Host "downloading $assetName..."
    Invoke-WebRequest -UseBasicParsing -Uri $asset.browser_download_url -OutFile $zip

    if ($sumsAsset) {
        Write-Host 'verifying checksum...'
        # To a file: release assets are served as application/octet-stream,
        # for which .Content is a byte[] (PowerShell 7), not text.
        $sumsFile = Join-Path $tmp 'checksums.txt'
        Invoke-WebRequest -UseBasicParsing -Uri $sumsAsset.browser_download_url -OutFile $sumsFile
        $sumsText = Get-Content -Raw -Path $sumsFile
        $line = ($sumsText -split "`n") | Where-Object { $_ -match [regex]::Escape("  $assetName") } | Select-Object -First 1
        if (-not $line) { throw "checksums.txt has no entry for $assetName" }
        $want = ($line -split '\s+')[0].ToLower()
        $got = (Get-FileHash -Path $zip -Algorithm SHA256).Hash.ToLower()
        if ($got -ne $want) { throw "checksum mismatch for ${assetName}: got $got, want $want — aborting (nothing was installed)" }
    } else {
        Write-Warning "release $tag has no checksums.txt — skipping checksum verification"
    }

    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    $newExe = Join-Path $tmp 'moca.exe'
    if (-not (Test-Path $newExe)) { throw 'the package contains no moca.exe' }

    if (-not $Dir) {
        $existing = Get-Command moca -ErrorAction SilentlyContinue
        if ($existing) {
            $Dir = Split-Path -Parent $existing.Source
        } else {
            $Dir = Join-Path $env:LOCALAPPDATA 'Programs\moca'
        }
    }
    if (-not (Test-Path $Dir)) { New-Item -ItemType Directory -Path $Dir -Force | Out-Null }

    $target = Join-Path $Dir 'moca.exe'
    if (Test-Path $target) { Write-Host "updating existing installation at $target" } else { Write-Host "installing to $target" }

    # A running moca.exe cannot be overwritten in place; rename it aside and
    # clean the leftover up now or on the next run.
    $old = "$target.old"
    Remove-Item -Path $old -Force -ErrorAction SilentlyContinue
    if (Test-Path $target) {
        try { Rename-Item -Path $target -NewName (Split-Path -Leaf $old) -Force }
        catch { throw "cannot replace $target — close all moca instances and re-run ($_)" }
    }
    try { Move-Item -Path $newExe -Destination $target -Force }
    catch {
        if (Test-Path $old) { Rename-Item -Path $old -NewName (Split-Path -Leaf $target) -Force } # put the old one back
        throw
    }
    Remove-Item -Path $old -Force -ErrorAction SilentlyContinue

    # Add the directory to the user PATH once (idempotent).
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $userPath) { $userPath = '' }
    if (($userPath -split ';') -notcontains $Dir) {
        $trimmed = $userPath.TrimEnd(';')
        $newPath = $(if ($trimmed) { "$trimmed;$Dir" } else { $Dir })
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Host "added $Dir to the user PATH (open a new terminal to use it)"
    }

    & $target --version
    Write-Host "installed moca $tag -> $target"
}
finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
