# `lalmax-nvr` Scripts

Platform-specific scripts live in subfolders. Run from the project root.

## Linux / macOS — `scripts/unix/`

```sh
./scripts/unix/build.sh      # build bin/lalmax-nvr
./scripts/unix/run.sh        # run in foreground
./scripts/unix/start.sh      # start in background
./scripts/unix/stop.sh       # stop background process
./scripts/unix/restart.sh    # restart background process
./scripts/unix/status.sh     # show pid and /api/health
./scripts/unix/logs.sh       # follow logs/lalmax-nvr.log
./scripts/unix/test.sh       # run all Go tests
```

Environment overrides:

```sh
CONFIG_FILE=./lalmax-nvr.dev.yaml ./scripts/unix/start.sh
BIN_PATH=./bin/lalmax-nvr-dev ./scripts/unix/build.sh
LINES=200 ./scripts/unix/logs.sh
```

## Windows — `scripts/windows/`

PowerShell（统一入口 `nvr.ps1`）：

```powershell
.\scripts\windows\nvr.ps1 build      # build bin/lalmax-nvr.exe
.\scripts\windows\nvr.ps1 run        # run in foreground
.\scripts\windows\nvr.ps1 start      # start in background
.\scripts\windows\nvr.ps1 stop       # stop background process
.\scripts\windows\nvr.ps1 restart    # restart background process
.\scripts\windows\nvr.ps1 status     # show pid and /api/health
.\scripts\windows\nvr.ps1 logs       # follow logs/lalmax-nvr.log
.\scripts\windows\nvr.ps1 test       # run all Go tests
```

Command Prompt（`.bat` 薄包装，内部同样调用 `nvr.ps1`）：

```cmd
scripts\windows\build.bat
scripts\windows\start.bat
scripts\windows\stop.bat
```

Environment overrides:

```powershell
$env:CONFIG_FILE = '.\config\lalmax-nvr.dev.yaml'; .\scripts\windows\nvr.ps1 start
$env:BIN_PATH = '.\bin\lalmax-nvr-dev'; .\scripts\windows\nvr.ps1 build
$env:LINES = '200'; .\scripts\windows\nvr.ps1 logs
```

Cross-compilation:

```powershell
$env:GOOS = 'linux'; $env:GOARCH = 'arm64'; .\scripts\windows\nvr.ps1 build
```

Implementation: `scripts/windows/nvr.ps1` + `scripts/windows/env.ps1`.

Unix helpers: `scripts/unix/env.sh`.

## Runtime files

```text
bin/lalmax-nvr          # bin/lalmax-nvr.exe on Windows
run/lalmax-nvr.pid
logs/lalmax-nvr.log
data/
```
