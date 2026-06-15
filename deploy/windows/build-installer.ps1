param(
    [string]$Version = "1.2.0",
    [string]$InnoSetupPath = ""
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$buildDir = Join-Path $repoRoot "build\windows"
$distDir = Join-Path $repoRoot "dist\windows"
$installerScript = Join-Path $PSScriptRoot "installer\sdwan.iss"
$wintunPath = Join-Path $buildDir "wintun.dll"

if (-not (Test-Path $wintunPath)) {
    throw "Missing $wintunPath. Copy the official x64 wintun.dll into build\windows first."
}

New-Item -ItemType Directory -Force -Path $buildDir, $distDir | Out-Null

Write-Host "Building Windows x64 binaries..."
$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH
try {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    & go build -trimpath -o (Join-Path $buildDir "sdwan-service.exe") .\cmd\windows-service
    if ($LASTEXITCODE -ne 0) { throw "sdwan-service build failed" }

    & go build -trimpath -ldflags "-H windowsgui" -o (Join-Path $buildDir "sdwan-tray.exe") .\cmd\windows-tray
    if ($LASTEXITCODE -ne 0) { throw "sdwan-tray build failed" }
}
finally {
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
}

if (-not $InnoSetupPath) {
    $candidates = @(
        "C:\Program Files (x86)\Inno Setup 6\ISCC.exe",
        "C:\Program Files\Inno Setup 6\ISCC.exe",
        (Join-Path $env:LOCALAPPDATA "Programs\Inno Setup 6\ISCC.exe")
    )
    $InnoSetupPath = $candidates | Where-Object { Test-Path $_ } | Select-Object -First 1
}

if (-not $InnoSetupPath -or -not (Test-Path $InnoSetupPath)) {
    throw "Inno Setup 6 was not found. Install it, then rerun this script."
}

Write-Host "Compiling installer..."
& $InnoSetupPath "/DAppVersion=$Version" $installerScript
if ($LASTEXITCODE -ne 0) { throw "Inno Setup compilation failed" }

$installer = Join-Path $distDir "SD-WAN-Setup-v$Version-x64.exe"
if (-not (Test-Path $installer)) {
    throw "Installer was not created at $installer"
}

$hash = Get-FileHash $installer -Algorithm SHA256
$hashFile = "$installer.sha256"
Set-Content -Path $hashFile -Value "$($hash.Hash)  $([System.IO.Path]::GetFileName($installer))" -Encoding ASCII
Write-Host "Installer: $installer"
Write-Host "SHA256:    $($hash.Hash)"
Write-Host "Hash file: $hashFile"
