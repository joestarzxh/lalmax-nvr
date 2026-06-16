# Run all Go tests.
$ErrorActionPreference = 'Stop'

. "$PSScriptRoot/env.ps1"

Push-Location $RootDir
try {
    $env:CGO_ENABLED = $CgoEnabled
    if ($env:GOCACHE) { $env:GOCACHE = $env:GOCACHE }
    if ($env:GOMODCACHE) { $env:GOMODCACHE = $env:GOMODCACHE }

    go test ./...
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
