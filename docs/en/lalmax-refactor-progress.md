# lal / lalmax Refactor Progress

This document records the current refactor status for moving `lalmax-nvr` onto vendored `third/lal` and `third/lalmax`, so future work can resume without reconstructing context from the codebase.

Follow-up design work for `RTMP/SRT push`, `external stream visibility`, and unified stream management is tracked separately in:

- [stream-management-design.md](./stream-management-design.md)

## Goal

The target architecture is:

1. Camera ingest goes through `lalmax`
2. HLS / HTTP-FLV / WebRTC playback is served by `lalmax`
3. Recording consumes the unified `lalmax` stream whenever possible
4. ONVIF discovery, signaling, profile, and encoding decisions remain in the project's own ONVIF layer
5. Legacy built-in live managers are reduced to fallback or residual responsibilities

## Current status

### Vendor and module setup

- `third/lal` and `third/lalmax` are vendored locally
- `go.mod` already uses local `replace` directives
- the project name has been normalized to `lalmax-nvr`

### Media engine foundation

- an engine boundary and `lalmax` adapter skeleton now exist under `internal/media`
- a new `media:` config section controls the new media path
- `cmd/lalmax-nvr/main.go` can create and inject `mediaEngine`

### Playback path moved to lalmax

With `media.enabled=true`:

- camera HLS endpoints proxy to `lalmax`
- camera HTTP-FLV endpoints proxy to `lalmax`
- camera WebRTC endpoints proxy to `lalmax` WHEP
- `GET /api/cameras/{id}/protocols` now includes:
  - `play_url`
  - `backend`
  - `stream_status`

Current backend split:

- `hls / flv / webrtc` => `lalmax`
- `wasm` => `builtin-ws`

### Ingest path moved to lalmax pull

With `media.enabled=true`:

- `rtsp + h264/h265` cameras start a `mediaEngine.StartPull(...)`
- `onvif` cameras resolve the actual RTSP URI first, then hand it to `lalmax`
- camera lifecycle operations also stop the pull with `StopPull(...)`

### Recording has started consuming the lalmax unified RTSP output

Current behavior:

- `rtsp + h264/h265` recorders no longer pull directly from the camera
- existing `H264Recorder/H265Recorder` implementations are still reused
- their input now points to the local RTSP stream exposed by `lalmax`
- `onvif` follows the same path when encoding is known

This means duplicate pulls are mostly removed for RTSP H.264/H.265, and largely removed for ONVIF as well.

#### Reconnection tracking and recovery events

All recorders (H264, H265, MJPEG, HTTP-JPEG) now track disconnections:

- On disconnect, records `disconnectedAt` timestamp and `gapReason` (auto-classified from error type)
- On recovery, the first recording segment's `Recording` struct is populated with `ReconnectedAt` and `GapReason`
- Publishes a `TopicRecorderReconnected` event via EventBus with `Downtime` (duration of outage) and `RetryCount`
- DB schema upgraded to v16 — `recordings` table has new `reconnected_at` and `gap_reason` columns

Disconnect reason tags (`gap_reason` values):
- `frame_watchdog` — RTSP alive but no frame data (30s timeout)
- `connection_lost` — Connection dropped (EOF / peer closed)
- `connection_refused` — Connection refused by target
- `connection_timeout` — Connection timed out
- `rtsp_negotiation` — RTSP DESCRIBE/SETUP/PLAY failed
- `connection_error` — Other connection error

Key files:

- `internal/recorder/h264.go`, `h265.go`, `mjpeg.go`, `http_jpeg.go`
- `internal/recorder/backoff.go` — `classifyDisconnectReason()`
- `internal/event/types.go` — `RecorderReconnected`
- `internal/model/types.go` — `Recording.ReconnectedAt`, `Recording.GapReason`
- `internal/storage/db.go` — migration v15→v16

### ONVIF now uses "project signaling + lalmax media"

With `media.enabled=true` and `protocol=onvif`:

1. the project ONVIF client loads profiles
2. `profile_token` is selected or reused
3. `encoding / stream_encoding` are filled in
4. stream URI is resolved if needed
5. prepared state is written back to:
   - in-memory config
   - SQLite `cameras` table
   - YAML config file
6. the runtime path becomes:
   `lalmax pull -> local lalmax RTSP -> H264/H265 recorder`

Legacy `ONVIFRecorder` is now effectively a fallback path.

### Legacy live manager responsibilities are reduced

With `media.enabled=true`:

- legacy `HLS / FLV / WebRTC` managers are no longer initialized as the main path
- `StreamRegistry` registers those capabilities through `mediaEngine`
- `ws` still remains for the wasm / WebCodecs local path
- `rtmp / srt` ingest still keeps legacy responsibilities

## Test baseline updates

Two test-baseline issues were fixed alongside the refactor work.

### 1. transcoding probe concurrency issue

`internal/transcoding/probe.go` previously used `sync.Once` together with `ResetProbe()`, which caused `sync: unlock of unlocked mutex` under concurrent tests.

It now uses:

- `sync.Mutex`
- `probeReady bool`
- explicit cache reset state

### 2. ONVIF API tests no longer depend on real LAN state

`internal/api/onvif_test.go` previously assumed there would be no discoverable ONVIF devices in the test environment.

It now uses handler-level injectable dependencies:

- `onvifDiscover`
- `onvifProbeDevice`
- `onvifNewClient`

Tests use stubs instead of real network discovery.

Validated commands:

- `go test ./internal/transcoding -run "TestProbe|TestNewTranscodingManager"`
- `go test ./internal/api -run ONVIF`
- `go test ./internal/api`

## Recommended next steps

### Priority 1

1. continue reducing recorder residual direct-camera paths
2. identify exactly which scenarios still pull directly from cameras
3. demote legacy `ONVIFRecorder` to fallback-only more aggressively

#### Priority 1 progress (completed)

**ONVIF unknown-encoding probe fallback:**
When `media.enabled=true` and ONVIF encoding is unknown, the system now probes the ONVIF device to detect H264/H265 encoding before giving up. If detection succeeds, it persists the encoding to config/DB, restarts the media pull, and creates an H264/H265Recorder backed by the lalmax relay. Previously this case returned `nil` (no recording). See `internal/camera/manager.go:probeONVIFEncodingAndBuildURL`.

**Residual direct-camera path identification:**
The following recorder types still pull directly from cameras even when `media.enabled=true`:
- **MJPEGRecorder** — lalmax does not relay MJPEG streams; always uses `cam.URL` directly
- **HTTPJPEGRecorder** — fundamentally different transport (HTTP multipart); always uses `cam.URL` directly
- **XiaomiRecorder** — uses TUTK P2P protocol; outside lalmax's relay capabilities
- **ONVIFRecorder** — only used when `media.enabled=false` (already demoted to fallback)

All H264/H265 RTSP and ONVIF recorders now go through lalmax when `media.enabled=true`.

**Logging improvements:**
- Added `WARN` logs for MJPEG and HTTP/JPEG direct-camera pulls
- Added `INFO` logs distinguishing lalmax-mediated vs direct camera pulls for RTSP H264/H265
- Added `WARN` log when legacy `ONVIFRecorder` is used (media engine disabled)
- Added `WARN` log when ONVIF encoding probe fails with actionable hint

**Tests added:**
- `TestCreateRecorder_ONVIF_ProbesEncodingWhenMediaEngineEnabled` — verifies encoding probe + lalmax relay + config/DB persistence
- `TestCreateRecorder_ONVIF_ProbeFailsReturnsNil` — verifies graceful fallback when probe finds no H264/H265
- Updated `TestCreateRecorder_ONVIFReturnsNilWhenMediaEngineEnabledButEncodingUnknown` with proper stub injection

### Priority 2

1. decide whether `wasm / ws` remains a separate path
2. if not, evaluate `fMP4 + WebCodecs` or another `lalmax`-aligned output path

#### Priority 2 progress (completed)

**Decision: Add HTTP fMP4 as a new protocol alongside existing wasm/ws.**

The wasm/ws path (custom binary protocol over WebSocket → WebCodecs) remains available for browsers with WebCodecs support. A new HTTP fMP4 path has been added that uses lalmax's built-in HTTP fMP4 server, consumed via MSE (Media Source Extensions) in the browser.

**Server-side (`internal/api/`):**
- New `handlers_fmp4.go` — proxies `GET /api/cameras/{id}/stream.m4s` to lalmax's HTTP fMP4 endpoint
- New route registered in `handler.go` (before HLS catch-all)
- New `FMP4StreamHandler` for protocol discovery
- `protocolPlayURL()` now handles `"fmp4"` protocol
- `main.go` registers `FMP4StreamHandler` when media engine is enabled

**Client-side (`web/src/`):**
- New `FMP4Player.svelte` — MSE-based player using `fetch()` + `ReadableStream` to consume HTTP fMP4 stream
  - Parses init segment (ftyp+moov) and feeds to `SourceBuffer`
  - Streams moof+dat fragments in real-time
  - Auto-reconnect with exponential backoff + jitter
  - Fullscreen support, tab visibility handling
- Updated `ProtocolSwitcher.svelte` — added `fmp4` option with MSE availability check
- Updated `Dashboard.svelte` and `LiveView.svelte` — lazy-loaded `FMP4Player` with loading/error states

**Data flow:**
```
Camera → lalmax (relay pull) → lalmax fMP4 muxer → HTTP fMP4 stream
  → NVR proxy (/stream.m4s) → Browser fetch() → SourceBuffer (MSE) → <video>
```

**Browser compatibility:**
- MSE is supported in all modern browsers (Chrome, Firefox, Safari, Edge)
- H.264 in MSE: universal support
- H.265 in MSE: Chrome 107+, Safari (not Firefox)

### Priority 3

1. continue removing unnecessary legacy live fan-out logic from `media.Runtime`
2. decide whether `rtmp / srt` should also move behind the same media boundary

#### Priority 3 progress (completed)

**Decision: Wire custom RTMP/SRT with proper callbacks (Option A).**

Lalmax's built-in RTMP/SRT servers don't connect to the NVR's camera/recorder pipeline — they'd need a bridge layer. The custom implementations already have StreamHub integration and camera ID parsing, they just needed non-nil callbacks.

**Dead code removed (`internal/api/handlers_stream.go`):**
- Deleted `handleHLSStreamViaRegistry` (84 lines) — never wired in routes
- Deleted `handleStopHLSStreamViaRegistry` — same
- Cleaned 3 unused imports (`net/http`, `chi`, `hls`)

**FMP4StreamHandler registration fix (`cmd/lalmax-nvr/main.go`):**
- Moved from `else` block (mediaEngine==nil) to `if` block (mediaEngine!=nil)
- Handler requires mediaEngine to function, so registration must match

**RTMP/SRT ingest wiring (`internal/media/ingest.go` + `cmd/lalmax-nvr/main.go`):**
- Actual RTMP/SRT ingest is handled by lalmax/lal.
- `internal/media.IngestHandler` subscribes to lalmax publish/stop events and maps stream names to camera IDs.
- RTMP stream keys still map `camera_id -> stream_key` through a reverse lookup.
- SRT maps stream names directly to camera IDs.

**SRT wiring (`cmd/lalmax-nvr/main.go`):**
- Registered existing camera hubs with SRT listener before `Start()`
- Iterates enabled cameras, gets their recorders, extracts StreamHub
- Before this fix: SRT created orphaned hubs disconnected from recorders
- After: SRT publish → hub delivery → recording via existing camera pipeline

## Files to resume from

- `internal/camera/manager.go`
- `internal/media/runtime.go`
- `internal/media/`
- `internal/api/handler.go`
- `internal/api/handlers_hls.go`
- `internal/api/handlers_flv.go`
- `internal/api/handlers_webrtc.go`
- `internal/api/handlers_stream.go`
- `internal/transcoding/probe.go`

## Bottom line

The project is already past the "prepare for lalmax" phase. The first-stage migration is in place: ingest, playback, and the main recording input path are already converging on `lalmax`. The next stage is mostly about cleaning residual paths and tightening the remaining boundaries.
