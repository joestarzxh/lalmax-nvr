# Build bin/lalmax-nvr (and frontend assets when present).
$ErrorActionPreference = 'Stop'

. "$PSScriptRoot/env.ps1"

$Version = if ($env:VERSION) { $env:VERSION } else { 'dev' }
$Commit = $env:COMMIT
$Date = if ($env:DATE) { $env:DATE } else { (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ') }

if (-not $env:NODE_OPTIONS) {
    $env:NODE_OPTIONS = '--no-deprecation'
}
elseif ($env:NODE_OPTIONS -notmatch 'no-deprecation') {
    $env:NODE_OPTIONS = "$($env:NODE_OPTIONS) --no-deprecation"
}

$TargetOs = if ($env:GOOS) { $env:GOOS } else { (go env GOOS).Trim() }
$TargetArch = if ($env:GOARCH) { $env:GOARCH } else { (go env GOARCH).Trim() }

if (-not $Commit -and (Get-Command git -ErrorAction SilentlyContinue)) {
    try {
        git -C $RootDir rev-parse --is-inside-work-tree 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            $Commit = (git -C $RootDir rev-parse --short HEAD).Trim()
        }
    }
    catch {
        # Ignore git lookup failures.
    }
}
if (-not $Commit) {
    $Commit = 'none'
}

$HostOs = (go env GOOS).Trim()
$HostArch = (go env GOARCH).Trim()
if ($TargetOs -ne $HostOs -or $TargetArch -ne $HostArch) {
    $OutputBin = "$BinPath-$TargetArch"
    if ($TargetArch -eq 'arm' -and $env:GOARM) {
        $OutputBin = "$BinPath-armv$($env:GOARM)"
    }
}
else {
    $OutputBin = $BinPath
}

Write-Host "building $AppName"
Write-Host "  target: $TargetOs/$TargetArch"
Write-Host "  output: $OutputBin"
Write-Host "  cgo:    $CgoEnabled"

Push-Location $RootDir
try {
    $webPackage = Join-Path $RootDir 'web/package.json'
    if ((Test-Path -LiteralPath $webPackage) -and (Get-Command npm -ErrorAction SilentlyContinue)) {
        $nodeModules = Join-Path $RootDir 'web/node_modules'
        if (-not (Test-Path -LiteralPath $nodeModules)) {
            Write-Host 'installing frontend dependencies'
            npm --prefix web install
            if ($LASTEXITCODE -ne 0) { throw 'npm install failed' }
        }
        Write-Host 'building frontend'
        npm --prefix web run build
        if ($LASTEXITCODE -ne 0) { throw 'npm run build failed' }

        $assetsDir = Join-Path $RootDir 'internal/ui/static/assets'
        if (Test-Path -LiteralPath $assetsDir) {
            Remove-Item -LiteralPath $assetsDir -Recurse -Force
        }
        Copy-Item -Path (Join-Path $RootDir 'web/dist/*') -Destination (Join-Path $RootDir 'internal/ui/static/') -Recurse -Force
    }

    $env:CGO_ENABLED = $CgoEnabled
    $env:GOOS = $TargetOs
    $env:GOARCH = $TargetArch

    go build `
        -ldflags "-s -w -X main.appVersion=$Version" `
        -o $OutputBin `
        ./cmd/lalmax-nvr
    if ($LASTEXITCODE -ne 0) { throw 'go build failed' }

    $builtBin = Get-ActualBinPath $OutputBin
    & $builtBin -version
    if ($LASTEXITCODE -ne 0) { throw 'binary -version failed' }
}
finally {
    Pop-Location
}
