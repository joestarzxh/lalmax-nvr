# Shared environment and helpers for lalmax-nvr Windows scripts.
$ErrorActionPreference = 'Stop'

$ScriptDir = $PSScriptRoot
$RootDir = Split-Path -Parent $ScriptDir

$AppName = 'lalmax-nvr'
$BinDir = if ($env:BIN_DIR) { $env:BIN_DIR } else { Join-Path $RootDir 'bin' }
$RunDir = if ($env:RUN_DIR) { $env:RUN_DIR } else { Join-Path $RootDir 'run' }
$LogDir = if ($env:LOG_DIR) { $env:LOG_DIR } else { Join-Path $RootDir 'logs' }
$DataDir = if ($env:DATA_DIR) { $env:DATA_DIR } else { Join-Path $RootDir 'data' }
$CgoEnabled = if ($env:CGO_ENABLED) { $env:CGO_ENABLED } else { '0' }

$BinPath = if ($env:BIN_PATH) { $env:BIN_PATH } else { Join-Path $BinDir $AppName }
if ($IsWindows -or $env:OS -match 'Windows') {
    if (-not $env:BIN_PATH -and -not $BinPath.EndsWith('.exe')) {
        $BinPath = "$BinPath.exe"
    }
}
$PidFile = if ($env:PID_FILE) { $env:PID_FILE } else { Join-Path $RunDir "$AppName.pid" }
$LogFile = if ($env:LOG_FILE) { $env:LOG_FILE } else { Join-Path $LogDir "$AppName.log" }
$ConfigFile = if ($env:CONFIG_FILE) { $env:CONFIG_FILE } else { Join-Path $RootDir 'config/lalmax-nvr.yaml' }
$DefaultConfigFile = Join-Path $RootDir 'config/config.example.yaml'

foreach ($dir in @($BinDir, $RunDir, $LogDir, $DataDir, (Split-Path $ConfigFile -Parent))) {
    if ($dir -and -not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
}

function Get-ActualBinPath {
    param([string]$Path)
    if (Test-Path -LiteralPath $Path) {
        return (Resolve-Path -LiteralPath $Path).Path
    }
    $exePath = "$Path.exe"
    if (Test-Path -LiteralPath $exePath) {
        return (Resolve-Path -LiteralPath $exePath).Path
    }
    return $Path
}

function Test-BinaryExists {
    param([string]$Path)
    return (Test-Path -LiteralPath $Path) -or (Test-Path -LiteralPath "$Path.exe")
}

function Ensure-LogFile {
    $logDir = Split-Path $LogFile -Parent
    if ($logDir -and -not (Test-Path -LiteralPath $logDir)) {
        New-Item -ItemType Directory -Path $logDir -Force | Out-Null
    }
    if (-not (Test-Path -LiteralPath $LogFile)) {
        New-Item -ItemType File -Path $LogFile -Force | Out-Null
    }
}

function Sync-StorageRoot {
    if (-not (Test-Path -LiteralPath $ConfigFile)) {
        return
    }

    $content = [System.IO.File]::ReadAllText($ConfigFile)
    $dataDirYaml = ($DataDir -replace '\\', '/')

    if ($content -match '(?m)^\s*root_dir:\s*') {
        $content = $content -replace '(?m)(^\s*root_dir:\s*).*$', "`${1}`"$dataDirYaml`""
    }
    if ($content -match 'lalmax_config_path:.*"/var/lib/lalmax-nvr/') {
        $content = $content -replace '(?m)^\s*lalmax_config_path:\s*"/var/lib/lalmax-nvr/.*$', '  # lalmax_config_path: auto-generated at {root_dir}/config/lalmax.conf.json'
    }

    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($ConfigFile, $content, $utf8NoBom)
}

function Ensure-Config {
    if (Test-Path -LiteralPath $ConfigFile) {
        Sync-StorageRoot
        return
    }
    if (-not (Test-Path -LiteralPath $DefaultConfigFile)) {
        throw "missing default config: $DefaultConfigFile"
    }
    $configDir = Split-Path $ConfigFile -Parent
    if ($configDir -and -not (Test-Path -LiteralPath $configDir)) {
        New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    }
    Copy-Item -LiteralPath $DefaultConfigFile -Destination $ConfigFile
    Sync-StorageRoot
    Write-Host "created $ConfigFile from config.example.yaml"
}

function Get-ConfigListen {
    $inServer = $false
    foreach ($line in Get-Content -LiteralPath $ConfigFile -ErrorAction SilentlyContinue) {
        if ($line -match '^\s*server:\s*$') {
            $inServer = $true
            continue
        }
        if ($inServer -and $line -match '^\S') {
            $inServer = $false
        }
        if ($inServer -and $line -match '^\s*listen:\s*(.+)') {
            return $matches[1].Trim().Trim('"').Trim("'")
        }
    }
    return ':9090'
}

function Get-ConfigListenPort {
    $listen = Get-ConfigListen
    if ($listen -match ':([^:]+)$') {
        return $matches[1]
    }
    return $listen
}

function Get-ConfigStorageRoot {
    $inStorage = $false
    foreach ($line in Get-Content -LiteralPath $ConfigFile -ErrorAction SilentlyContinue) {
        if ($line -match '^\s*storage:\s*$') {
            $inStorage = $true
            continue
        }
        if ($inStorage -and $line -match '^\S') {
            $inStorage = $false
        }
        if ($inStorage -and $line -match '^\s*root_dir:\s*(.+)') {
            return $matches[1].Trim().Trim('"').Trim("'")
        }
    }
    return $DataDir
}

function Get-ConfigHlsTempDir {
    $inHls = $false
    foreach ($line in Get-Content -LiteralPath $ConfigFile -ErrorAction SilentlyContinue) {
        if ($line -match '^\s*hls:\s*$') {
            $inHls = $true
            continue
        }
        if ($inHls -and $line -match '^\S') {
            $inHls = $false
        }
        if ($inHls -and $line -match '^\s*lal_temp_dir:\s*(.+)') {
            return $matches[1].Trim().Trim('"').Trim("'")
        }
    }
    return 'hls-temp'
}

function Resolve-HlsTempPath {
    $tempDir = Get-ConfigHlsTempDir
    if ($tempDir -match '^([A-Za-z]:[/\\]|/)') {
        return $tempDir
    }
    $rootDir = Get-ConfigStorageRoot
    return Join-Path $rootDir $tempDir
}

function Invoke-CleanupHlsTemp {
    $hlsTemp = Resolve-HlsTempPath
    if (-not (Test-Path -LiteralPath $hlsTemp)) {
        return
    }
    Write-Host "cleaning HLS temp dir: $hlsTemp"
    Remove-Item -LiteralPath $hlsTemp -Recurse -Force
}

function Get-LookupRunningPid {
    if (Test-Path -LiteralPath $PidFile) {
        $pidText = (Get-Content -LiteralPath $PidFile -Raw).Trim()
        if ($pidText -and (Get-Process -Id ([int]$pidText) -ErrorAction SilentlyContinue)) {
            return [int]$pidText
        }
    }

    $port = Get-ConfigListenPort
    try {
        $conn = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue |
            Select-Object -First 1
        if ($conn) {
            return [int]$conn.OwningProcess
        }
    }
    catch {
        # Get-NetTCPConnection may be unavailable; fall back to netstat.
    }

    $pattern = ":$port\s"
    $line = netstat -ano | Select-String -Pattern $pattern | Select-String -Pattern 'LISTENING' | Select-Object -First 1
    if ($line) {
        $parts = ($line.ToString() -split '\s+') | Where-Object { $_ }
        return [int]$parts[-1]
    }

    return $null
}

function Test-AppRunning {
    $runningPid = Get-LookupRunningPid
    if (-not $runningPid) {
        return $false
    }
    $currentPid = if (Test-Path -LiteralPath $PidFile) { (Get-Content -LiteralPath $PidFile -Raw).Trim() } else { '' }
    if ($currentPid -ne "$runningPid") {
        Set-Content -LiteralPath $PidFile -Value $runningPid -NoNewline
    }
    return $true
}

function Get-AppPid {
    return Get-LookupRunningPid
}

function Start-AppBackground {
    Ensure-LogFile
    $actualBin = Get-ActualBinPath $BinPath

    $processInfo = New-Object System.Diagnostics.ProcessStartInfo
    $processInfo.FileName = $actualBin
    $processInfo.Arguments = "-config `"$ConfigFile`""
    $processInfo.WorkingDirectory = $RootDir
    $processInfo.UseShellExecute = $false
    $processInfo.CreateNoWindow = $true
    $processInfo.RedirectStandardOutput = $true
    $processInfo.RedirectStandardError = $true

    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $processInfo
    $logWriter = [System.IO.StreamWriter]::new($LogFile, $true)

    $process.add_OutputDataReceived({
        param($sender, $eventArgs)
        if ($null -ne $eventArgs.Data) {
            $logWriter.WriteLine($eventArgs.Data)
            $logWriter.Flush()
        }
    })
    $process.add_ErrorDataReceived({
        param($sender, $eventArgs)
        if ($null -ne $eventArgs.Data) {
            $logWriter.WriteLine($eventArgs.Data)
            $logWriter.Flush()
        }
    })

    [void]$process.Start()
    $process.BeginOutputReadLine()
    $process.BeginErrorReadLine()
    Set-Content -LiteralPath $PidFile -Value $process.Id -NoNewline
    return $process
}
