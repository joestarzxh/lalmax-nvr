# Restart the background lalmax-nvr process.
$ErrorActionPreference = 'Stop'

& "$PSScriptRoot/stop.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

& "$PSScriptRoot/start.ps1"
exit $LASTEXITCODE
