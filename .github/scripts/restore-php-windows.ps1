param(
    [Parameter(Mandatory)] [string] $Destination,
    [string] $Repository = 'php/frankenphp',
    [string] $AssetPattern = 'php-*-Win32-clang-x64.zip'
)

$ErrorActionPreference = 'Stop'
$Destination = [IO.Path]::GetFullPath($Destination)
if (Test-Path $Destination) { throw 'The PHP bundle destination must not exist.' }

# Release assets work across tags, branches, and fork PRs; Actions caches do not.
for ($page = 1; ; $page++) {
    $response = gh api "repos/$Repository/releases?per_page=100&page=$page"
    if ($LASTEXITCODE -ne 0) { throw 'Could not look up the released PHP bundle.' }
    $releases = @($response | ConvertFrom-Json)
    foreach ($release in $releases) {
        if ($release.draft) { continue }
        $asset = $release.assets | Where-Object { $_.name -like $AssetPattern -and $_.state -eq 'uploaded' } | Select-Object -First 1
        if (-not $asset) { continue }
        $archive = "$Destination.zip"
        New-Item -ItemType Directory -Force (Split-Path $Destination) | Out-Null
        gh release download $release.tag_name --repo $Repository --pattern $AssetPattern --output $archive
        if ($LASTEXITCODE -ne 0) { throw 'Could not download the released PHP bundle.' }
        if ("sha256:$((Get-FileHash $archive -Algorithm SHA256).Hash.ToLowerInvariant())" -ne $asset.digest) {
            throw 'Released PHP bundle checksum mismatch.'
        }
        Expand-Archive $archive $Destination
        Write-Host "Reusing PHP from $Repository release $($release.tag_name)"
        if ($env:GITHUB_OUTPUT) { 'restored=true' >> $env:GITHUB_OUTPUT }
        return
    }
    if ($releases.Count -lt 100) { break }
}

Write-Host 'No released PHP bundle exists yet; bootstrapping from source.'
if ($env:GITHUB_OUTPUT) { 'restored=false' >> $env:GITHUB_OUTPUT }
