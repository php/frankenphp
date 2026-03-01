#Requires -Version 5.1
<#
.SYNOPSIS
    Downloads and installs the latest FrankenPHP release for Windows.
.DESCRIPTION
    This script downloads the latest FrankenPHP Windows release from GitHub
    and extracts it to the specified directory (current directory by default).

    Usage as a one-liner:
        irm https://github.com/php/frankenphp/raw/refs/heads/main/install.ps1 | iex
    Custom install directory:
        $env:BIN_DIR = 'C:\frankenphp'; irm https://github.com/php/frankenphp/raw/refs/heads/main/install.ps1 | iex
#>

$ErrorActionPreference = "Stop"

if ($env:BIN_DIR) {
    $BinDir = $env:BIN_DIR
} else {
    $BinDir = Get-Location
}

Write-Host "Querying latest FrankenPHP release..." -ForegroundColor Cyan

try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/php/frankenphp/releases/latest"
} catch {
    Write-Host "Could not query GitHub releases: $_" -ForegroundColor Red
    exit 1
}

$asset = $release.assets | Where-Object { $_.name -match "Win32-vs17-x64\.zip$" } | Select-Object -First 1

if (-not $asset) {
    Write-Host "Could not find a Windows release asset." -ForegroundColor Red
    Write-Host "Check https://github.com/php/frankenphp/releases for available downloads." -ForegroundColor Red
    exit 1
}

Write-Host "Downloading $($asset.name)..." -ForegroundColor Cyan

$tmpZip = Join-Path $env:TEMP "frankenphp-windows-$PID.zip"

try {
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $tmpZip -UseBasicParsing
} catch {
    Write-Host "Download failed: $_" -ForegroundColor Red
    exit 1
}

Write-Host "Extracting to $BinDir..." -ForegroundColor Cyan

if (-not (Test-Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
}

try {
    Expand-Archive -Force -Path $tmpZip -DestinationPath $BinDir
} finally {
    Remove-Item $tmpZip -Force -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "FrankenPHP downloaded successfully to $BinDir" -ForegroundColor Green

# Check if the directory is in PATH
$inPath = $env:PATH -split ";" | Where-Object { $_ -eq $BinDir -or $_ -eq "$BinDir\" }
if (-not $inPath) {
    Write-Host "Add $BinDir to your PATH to use frankenphp.exe globally:" -ForegroundColor Yellow
    Write-Host "  [Environment]::SetEnvironmentVariable('PATH', `"$BinDir;`" + [Environment]::GetEnvironmentVariable('PATH', 'User'), 'User')" -ForegroundColor Gray
}

Write-Host ""
Write-Host "If you like FrankenPHP, please give it a star on GitHub: https://github.com/php/frankenphp" -ForegroundColor Cyan
