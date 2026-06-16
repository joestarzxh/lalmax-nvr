# Follow logs/lalmax-nvr.log.
$ErrorActionPreference = 'Stop'

. "$PSScriptRoot/env.ps1"

Ensure-LogFile

$lines = if ($env:LINES) { [int]$env:LINES } else { 100 }
Get-Content -LiteralPath $LogFile -Tail $lines -Wait
