# lalmax-nvr lifecycle CLI for Windows.
# Usage: .\scripts\windows\nvr.ps1 <build|run|start|stop|restart|status|logs|test|help>

param(
    [Parameter(Position = 0)]
    [ValidateSet('build', 'run', 'start', 'stop', 'restart', 'status', 'logs', 'test', 'help')]
    [string]$Command = 'help'
)

$ErrorActionPreference = 'Stop'

. "$PSScriptRoot/env.ps1"

function Show-NvrHelp {
    Write-Host @"
lalmax-nvr scripts (Windows)

Usage:
  .\scripts\windows\nvr.ps1 build      build bin/lalmax-nvr.exe
  .\scripts\windows\nvr.ps1 run        run in foreground
  .\scripts\windows\nvr.ps1 start      start in background
  .\scripts\windows\nvr.ps1 stop       stop background process
  .\scripts\windows\nvr.ps1 restart    restart background process
  .\scripts\windows\nvr.ps1 status     show pid and /api/health
  .\scripts\windows\nvr.ps1 logs       follow logs/lalmax-nvr.log
  .\scripts\windows\nvr.ps1 test       run all Go tests

Command Prompt:
  scripts\windows\start.bat
  scripts\windows\build.bat
  ...

Environment overrides:
  `$env:CONFIG_FILE = '.\config\lalmax-nvr.dev.yaml'; .\scripts\windows\nvr.ps1 start
  `$env:BIN_PATH = '.\bin\lalmax-nvr-dev'; .\scripts\windows\nvr.ps1 build
  `$env:LINES = '200'; .\scripts\windows\nvr.ps1 logs
"@
}

function Invoke-NvrBuild {
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
}

function Invoke-NvrRun {
    Ensure-Config

    if (-not (Test-BinaryExists $BinPath)) {
        Invoke-NvrBuild
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
}

function Invoke-NvrStart {
    Ensure-Config

    $listenAddr = Get-ConfigListen

    if (Test-AppRunning) {
        Write-Host "$AppName already running, pid=$(Get-AppPid), listen=$listenAddr"
        return
    }

    if (-not (Test-BinaryExists $BinPath)) {
        Invoke-NvrBuild
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
}

function Invoke-NvrStop {
    if (-not (Test-AppRunning)) {
        Write-Host "$AppName is not running"
        Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
        try { Invoke-CleanupHlsTemp } catch { }
        return
    }

    $processId = Get-AppPid
    Write-Host "stopping $AppName, pid=$processId"

    Stop-Process -Id $processId -ErrorAction SilentlyContinue

    for ($i = 0; $i -lt 50; $i++) {
        if (-not (Get-Process -Id $processId -ErrorAction SilentlyContinue)) {
            Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
            try { Invoke-CleanupHlsTemp } catch { }
            Write-Host "$AppName stopped"
            return
        }
        Start-Sleep -Milliseconds 100
    }

    Write-Host "$AppName did not stop within 5s, sending force kill"
    Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
    try { Invoke-CleanupHlsTemp } catch { }
    Write-Host "$AppName stopped"
}

function Invoke-NvrRestart {
    Invoke-NvrStop
    Invoke-NvrStart
}

function Invoke-NvrStatus {
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
}

function Invoke-NvrLogs {
    Ensure-LogFile
    $lines = if ($env:LINES) { [int]$env:LINES } else { 100 }
    Get-Content -LiteralPath $LogFile -Tail $lines -Wait
}

function Invoke-NvrTest {
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
}

switch ($Command) {
    'build' { Invoke-NvrBuild }
    'run' { Invoke-NvrRun }
    'start' { Invoke-NvrStart }
    'stop' { Invoke-NvrStop }
    'restart' { Invoke-NvrRestart }
    'status' { Invoke-NvrStatus }
    'logs' { Invoke-NvrLogs }
    'test' { Invoke-NvrTest }
    default { Show-NvrHelp }
}
