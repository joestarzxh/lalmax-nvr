# `lalmax-nvr` Scripts

All scripts run from the project root automatically.

## Linux / macOS

```sh
./scripts/build.sh      # build bin/lalmax-nvr
./scripts/run.sh        # run in foreground
./scripts/start.sh      # start in background
./scripts/stop.sh       # stop background process
./scripts/restart.sh    # restart background process
./scripts/status.sh     # show pid and /api/health
./scripts/logs.sh       # follow logs/lalmax-nvr.log
./scripts/test.sh       # run all Go tests
```

Environment overrides:

```sh
CONFIG_FILE=./lalmax-nvr.dev.yaml ./scripts/start.sh
BIN_PATH=./bin/lalmax-nvr-dev ./scripts/build.sh
LINES=200 ./scripts/logs.sh
```

## Windows

PowerShell (recommended):

```powershell
.\scripts\build.ps1      # build bin/lalmax-nvr.exe
.\scripts\run.ps1        # run in foreground
.\scripts\start.ps1      # start in background
.\scripts\stop.ps1       # stop background process
.\scripts\restart.ps1    # restart background process
.\scripts\status.ps1     # show pid and /api/health
.\scripts\logs.ps1       # follow logs/lalmax-nvr.log
.\scripts\test.ps1       # run all Go tests
```

Command Prompt wrappers (`.cmd` files call the PowerShell scripts above):

```cmd
scripts\build.cmd
scripts\start.cmd
scripts\stop.cmd
```

Environment overrides:

```powershell
$env:CONFIG_FILE = '.\config\lalmax-nvr.dev.yaml'; .\scripts\start.ps1
$env:BIN_PATH = '.\bin\lalmax-nvr-dev'; .\scripts\build.ps1
$env:LINES = '200'; .\scripts\logs.ps1
```

Cross-compilation examples:

```powershell
$env:GOOS = 'linux'; $env:GOARCH = 'arm64'; .\scripts\build.ps1
```

## Runtime files

```text
bin/lalmax-nvr          # bin/lalmax-nvr.exe on Windows
run/lalmax-nvr.pid
logs/lalmax-nvr.log
data/
```
