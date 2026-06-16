# Start lalmax-nvr in the background.
$ErrorActionPreference = 'Stop'

. "$PSScriptRoot/env.ps1"

Ensure-Config

$listenAddr = Get-ConfigListen

if (Test-AppRunning) {
    Write-Host "$AppName already running, pid=$(Get-AppPid), listen=$listenAddr"
    exit 0
}

if (-not (Test-BinaryExists $BinPath)) {
    & "$PSScriptRoot/build.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "starting $AppName"
Write-Host "  config: $ConfigFile"
Write-Host "  listen: $listenAddr"
Write-Host "  pid:    $PidFile"
Write-Host "  log:    $LogFile"

$process = Start-AppBackground
Start-Sleep -Milliseconds 500

if ($process.HasExited) {
    Write-Error "$AppName failed to start, see $LogFile"
    Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
    exit 1
}

Write-Host "$AppName started, pid=$($process.Id)"
