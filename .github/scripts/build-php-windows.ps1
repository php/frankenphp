param(
    [Parameter(Mandatory)] [string] $SourceDirectory,
    [Parameter(Mandatory)] [string] $SdkDirectory,
    [Parameter(Mandatory)] [string] $Destination
)

$ErrorActionPreference = 'Stop'
$SourceDirectory = (Resolve-Path $SourceDirectory).Path
$SdkDirectory = (Resolve-Path $SdkDirectory).Path
$Destination = [IO.Path]::GetFullPath($Destination)
if ((Test-Path $Destination) -and (Get-ChildItem $Destination -Force | Select-Object -First 1)) {
    throw 'The destination must be empty so stale binaries cannot enter the release.'
}

$llvm = 'C:\Program Files\Microsoft Visual Studio\18\Enterprise\VC\Tools\Llvm\x64\bin'
$env:PATH = "$llvm;$env:PATH"
$compiler = & "$llvm\clang-cl.exe" --version

$deps = Join-Path (Split-Path $SourceDirectory) 'deps'
$task = Join-Path $SourceDirectory 'build-frankenphp.bat'
@"
@echo off
cd /d "$SourceDirectory"
call phpsdk_deps --update --no-backup --branch 8.6 --stability stable --deps "$deps"
if errorlevel 1 exit /b 1
rem mkdist follows transitive DLL imports through PATH when assembling the ZIP.
set "PATH=$deps\bin;%PATH%"
call buildconf.bat --force
if errorlevel 1 exit /b 1
call configure.bat --enable-snapshot-build --enable-zts --enable-embed --enable-com-dotnet=shared --without-analyzer --disable-debug-pack --disable-test-ini --with-toolset=clang --with-php-build="$deps"
if errorlevel 1 exit /b 1
rem The all target stops on compile errors; snap does not.
nmake /NOLOGO
if errorlevel 1 exit /b 1
nmake /NOLOGO build-devel
if errorlevel 1 exit /b 1
nmake /NOLOGO build-dist
if errorlevel 1 exit /b 1
"@ | Set-Content $task -Encoding ascii

& "$SdkDirectory\phpsdk-vs18-x64.bat" -t $task
if ($LASTEXITCODE -ne 0) { throw 'PHP source build failed.' }

$build = Join-Path $SourceDirectory 'x64\Release_TS'
$version = & "$build\php.exe" -n -r 'echo PHP_VERSION;'
if ($LASTEXITCODE -ne 0 -or $version -notmatch '^8\.6\.') { throw "Expected PHP 8.6, got: $version" }
$vm = & "$build\php.exe" -n -r 'echo ZEND_VM_KIND;'
if ($LASTEXITCODE -ne 0 -or $vm -ne 'ZEND_VM_KIND_TAILCALL') { throw "Expected the tailcall VM, got: $vm" }
if ((Get-Content "$SourceDirectory\Makefile" -Raw) -notmatch '(?m)^PHP_CL=.*clang-cl\.exe') {
    throw 'PHP was not built with clang-cl.'
}

$devel = @(Get-ChildItem $build -Directory -Filter "php-$version-devel-*")
if ($devel.Count -ne 1) { throw 'Expected exactly one PHP development pack.' }
New-Item -ItemType Directory -Force "$Destination\runtime", "$Destination\devel" | Out-Null
Copy-Item "$build\php-$version\*" "$Destination\runtime" -Recurse -Force
Copy-Item "$($devel[0].FullName)\*" "$Destination\devel" -Recurse -Force
foreach ($library in "$Destination\devel\lib\php8ts.lib", "$Destination\runtime\php8embed.lib") {
    if (-not (Test-Path $library)) { throw "Missing $library" }
}
@{
    version = $version
    vm = $vm
    compiler = $compiler -join "`n"
    source = git -C $SourceDirectory rev-parse HEAD
} | ConvertTo-Json | Set-Content "$Destination\build.json"
Write-Host "Built PHP $version"
