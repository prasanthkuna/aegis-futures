# Run once in PowerShell AS ADMINISTRATOR (outside Cursor).
# Fixes CodexSandboxUsers read-only ACL on Encore/PSReadLine folders so auth + git push work.

$ErrorActionPreference = 'Stop'
$paths = @(
    "$env:APPDATA\encore",
    "$env:APPDATA\Microsoft\Windows\PowerShell\PSReadLine"
)
$sandbox = 'LAP-HYD-IT-101\CodexSandboxUsers'

foreach ($p in $paths) {
    if (-not (Test-Path $p)) {
        New-Item -ItemType Directory -Path $p -Force | Out-Null
        Write-Host "Created $p"
    }
    Write-Host "Fixing ACL: $p"
    icacls $p /remove $sandbox 2>$null
    icacls $p /grant "${env:USERNAME}:(OI)(CI)F" /T
}

# Git-remote-encore writes sentinel files here during push
$localTemp = Join-Path $env:LOCALAPPDATA 'Temp'
if (Test-Path $localTemp) {
    Write-Host "Fixing ACL: $localTemp"
    icacls $localTemp /remove $sandbox 2>$null
    icacls $localTemp /grant "${env:USERNAME}:(OI)(CI)F"
}

Write-Host ""
Write-Host "Next (normal PowerShell, not admin):"
Write-Host "  encore auth login"
Write-Host "  cd $PSScriptRoot\.."
Write-Host "  git push encore main"
Write-Host ""
Write-Host "Success looks like: git output with objects counted, then builds at:"
Write-Host "  https://app.encore.cloud/aegis-futures-utk2"
