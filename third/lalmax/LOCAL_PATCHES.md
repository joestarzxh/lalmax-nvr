# Local patches

Upstream: git@github.com:q191201771/lalmax.git
Pinned commit: ffdfe24e1a84d1b97e4ed50a527833e4f7efb283

This file records local changes made to `third/lalmax` for lalmax-nvr.

## Current patches

### embed-lifecycle

- Reason: lalmax-nvr embeds lalmax as an in-process media engine and needs explicit start, readiness and shutdown hooks.
- Files:
  - `server/server.go`
  - `srt/server.go`
  - `rtc/server.go`
  - `gb28181/rtppub/manager.go`
  - `server/zlm_compat_ffmpeg.go`
- Upstreamable: Yes, as a generic embedded lifecycle API.
- Test: `go test ./server ./srt ./rtc ./gb28181/rtppub`

## Patch template

### patch-name

- Reason:
- Files:
- Upstreamable:
- Test:
