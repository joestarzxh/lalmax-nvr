# Stop the background lalmax-nvr process.
$ErrorActionPreference = 'Stop'

. "$PSScriptRoot/env.ps1"

if (-not (Test-AppRunning)) {
    Write-Host "$AppName is not running"
    Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
    try {
        Invoke-CleanupHlsTemp
    }
    catch {
        # Best-effort cleanup.
    }
    exit 0
}

$processId = Get-AppPid
Write-Host "stopping $AppName, pid=$processId"

Stop-Process -Id $processId -ErrorAction SilentlyContinue

for ($i = 0; $i -lt 50; $i++) {
    if (-not (Get-Process -Id $processId -ErrorAction SilentlyContinue)) {
        Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
        try {
            Invoke-CleanupHlsTemp
        }
        catch {
            # Best-effort cleanup.
        }
        Write-Host "$AppName stopped"
        exit 0
    }
    Start-Sleep -Milliseconds 100
}

Write-Host "$AppName did not stop within 5s, sending force kill"
Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
try {
    Invoke-CleanupHlsTemp
}
catch {
    # Best-effort cleanup.
}
Write-Host "$AppName stopped"
