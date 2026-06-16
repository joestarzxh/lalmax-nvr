# Run lalmax-nvr in the foreground.
$ErrorActionPreference = 'Stop'

. "$PSScriptRoot/env.ps1"

Ensure-Config

if (-not (Test-BinaryExists $BinPath)) {
    & "$PSScriptRoot/build.ps1"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "running $AppName"
Write-Host "  config: $ConfigFile"
Write-Host "  listen: $(Get-ConfigListen)"

Push-Location $RootDir
try {
    $actualBin = Get-ActualBinPath $BinPath
    & $actualBin -config $ConfigFile
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
