# GB28181 Device Guide

## Overview

lalmax-nvr supports the GB/T 28181 national standard protocol, allowing integration with compliant cameras, NVRs, and platforms. Features include device registration, recording query, playback, voice intercom, and platform cascading.

## Features

- **Device Management**: Auto-registration, heartbeat, online status monitoring
- **Recording Query**: Query device recordings by time range with visual timeline
- **Recording Playback**: Multi-protocol streaming (ws-flv, flv, hls, webrtc, etc.)
- **Playback Control**: Pause/resume, speed control (0.5x/1x/2x/4x), seek
- **Recording Download**: Single/batch download of device recordings
- **Voice Intercom**: SIP INVITE based, supports UDP/TCP transport
- **Cascade Platform**: Configure upstream platforms for cascading
- **Platform History**: Record register/unregister events

## Quick Start

### 1. Enable GB28181

Enable GB28181 in the configuration file:

```yaml
gb28181:
  enable: true
  id: "34020000002000000001"  # Local device ID
  domain: "3402000000"        # Local domain
  host: "192.168.1.100"       # Local SIP address
  port: 5060                  # SIP port
  media_ip: "192.168.1.100"   # Media IP
  media_port: 30000           # Media port
  password: "12345678"        # SIP auth password
```

### 2. Add Device

In the web interface:

1. Go to **Devices** page
2. Switch to **GB28181** tab
3. Devices will appear automatically after registration

### 3. Configure Device

Configure SIP parameters on the device side:

| Parameter | Value |
|-----------|-------|
| SIP Server IP | lalmax-nvr machine IP |
| SIP Port | 5060 (default) |
| Device ID | 20-digit national standard ID |
| Password | Same as configuration file |

## Usage

### Device List

Go to **Devices** → **GB28181** → **Device List** to:

- View device online status
- Play live video
- View device info (manufacturer, model, heartbeat time, etc.)

### Device Recording

#### Query Recordings

1. Switch to **Device Recording** tab
2. Select device and channel
3. Set start/end time
4. Click **Query Recordings**

#### Timeline

Query results display a 24-hour timeline with blue blocks indicating recording segments. Click a block to play the corresponding recording.

#### Playback Control

| Control | Description |
|---------|-------------|
| Pause/Resume | Pause or resume recording playback |
| Speed | Support 0.5x, 1x, 2x, 4x speed |
| Seek | Jump to specific time (start, 30s, 1min, 5min, 10min) |

#### Multi-Protocol Playback

Supports multiple playback protocols. After clicking play, use the protocol switch button to select:

| Protocol | Description |
|----------|-------------|
| ws-flv | WebSocket FLV (recommended) |
| flv | HTTP-FLV |
| hls | HLS |
| webrtc | WebRTC |
| fmp4 | Fragmented MP4 |

#### Download Recordings

- **Single Download**: Click the download button in the recording list
- **Batch Download**: Click "Batch Download", select multiple recordings, then click "Download Selected"

### Cascade Management

Switch to **Cascade Management** tab to:

- Add upstream platforms
- View platform status
- Delete platforms

#### Add Platform

1. Click **Add Platform**
2. Fill in platform info:
   - Platform name
   - Upstream SIP ID
   - Upstream IP and port
   - Transport protocol (UDP/TCP)
   - Username/password (optional)

### Platform History

Switch to **Platform History** tab to:

- View platform status overview
- View register/unregister events
- Filter by platform or event type

### Voice Intercom

In the device list, you can initiate voice intercom with online devices:

1. Click the device's intercom button
2. Allow browser microphone access
3. Start talking

## API Reference

### Device Management

```
GET  /api/gb28181/devices          # List devices
POST /api/gb28181/play             # Start live play
POST /api/gb28181/stop             # Stop play
```

### Recording Query & Playback

```
POST /api/gb28181/record_info      # Query recordings
POST /api/gb28181/playback         # Start playback
POST /api/gb28181/playback/pause   # Pause playback
POST /api/gb28181/playback/resume  # Resume playback
POST /api/gb28181/playback/speed   # Speed control
POST /api/gb28181/playback/seek    # Seek control
```

### Recording Download

```
POST /api/gb28181/download/start   # Start download
POST /api/gb28181/download/batch   # Batch download
POST /api/gb28181/download/stop    # Stop download
GET  /api/gb28181/downloads        # List downloads
```

### Cascade Platform

```
GET    /api/gb28181/platforms       # List platforms
POST   /api/gb28181/platforms       # Add platform
DELETE /api/gb28181/platforms       # Delete platform
GET    /api/gb28181/platform/events # Platform events
GET    /api/gb28181/platform/status # Platform status
```

### Voice Intercom

```
POST /api/gb28181/broadcast/start   # Start intercom
POST /api/gb28181/broadcast/stop    # Stop intercom
```

### Alarms

```
GET /api/gb28181/alarms             # List alarms
```

## Troubleshooting

### Device Cannot Register

1. Check device SIP configuration
2. Check network connectivity
3. Check if SIP port is occupied
4. Check SIP messages in logs

### Recording Query Failed

1. Confirm device is online
2. Confirm device supports recording query
3. Check time format is correct
4. Check RecordInfo messages in logs

### Playback 415 Error

415 error indicates device doesn't support the SDP format. Possible causes:

1. Device doesn't support playback
2. SDP format incompatible
3. Timestamp format incorrect

### Speed Control Failed

1. Confirm device supports SIP INFO commands
2. Confirm device supports speed playback
3. Check SIP INFO messages in logs

### Intercom No Audio

1. Check browser microphone permission
2. Check if device supports intercom
3. Check audio codec compatibility
4. Check Broadcast messages in logs

## Configuration Reference

```yaml
gb28181:
  enable: false                    # Enable GB28181
  id: ""                           # Local device ID (20 digits)
  domain: ""                       # Local domain (10 digits)
  host: ""                         # SIP listen address
  port: 5060                       # SIP listen port
  media_ip: ""                     # Media IP
  media_port: 30000                # Media port (0 for random)
  password: ""                     # SIP auth password
  expires: 3600                    # Registration expiry (seconds)
  keepalive_interval: 60           # Heartbeat interval (seconds)
  max_keepalive_count: 3           # Max heartbeat loss count
  transport: "udp"                 # Transport protocol (udp/tcp)
  charset: "GB2312"                # Character set
```
