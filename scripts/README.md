# `lalmax-nvr` Scripts

All scripts run from the project root automatically.

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

Runtime files:

```text
bin/lalmax-nvr
run/lalmax-nvr.pid
logs/lalmax-nvr.log
data/
```
