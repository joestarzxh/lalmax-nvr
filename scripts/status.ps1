# Show process status and /api/health.
$ErrorActionPreference = 'Stop'

. "$PSScriptRoot/env.ps1"

if (Test-AppRunning) {
    Write-Host "$AppName running, pid=$(Get-AppPid)"
}
else {
    Write-Host "$AppName not running"
    exit 1
}

$listen = Get-ConfigListen
if ($listen -like ':*') {
    $url = "http://127.0.0.1$listen/api/health"
}
else {
    $url = "http://$listen/api/health"
}

Write-Host "health: $url"

try {
    if (Get-Command curl.exe -ErrorAction SilentlyContinue) {
        curl.exe -fsS $url
        Write-Host ''
    }
    else {
        $response = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 5
        Write-Host $response.Content
    }
}
catch {
    Write-Host $_.Exception.Message
}
