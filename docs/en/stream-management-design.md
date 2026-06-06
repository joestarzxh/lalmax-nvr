# Stream Management Design Draft

This document defines how `RTSP/ONVIF pull`, `RTMP/SRT push`, and `lalmax/lal` runtime state should be unified, so a stream that already exists inside the media engine becomes visible, bindable, and manageable in the NVR.

## Problem Summary

The project already routes most camera media through `lalmax`, but external ingest is still modeled inconsistently:

1. embedded `lalmax` already exposes built-in ingest and distribution
2. the project still contains a separate `internal/rtmp.Server`
3. most APIs and UI flows are built around `camera_id`
4. existing `lalmax` stream groups do not map to a first-class business object

Result:

- a stream can already be active in `lalmax`
- `mediaEngine.ListStreams()` can see it
- the NVR UI may still have no way to surface or manage it

## Goals

1. `lalmax/lal` becomes the only media ingest layer
2. `lalmax-nvr` becomes the only business management layer
3. both pull and push remain supported
4. a unified stream-management model is introduced
5. external ingest is decoupled from the `Camera` model

## Design Principles

### One media ingress boundary

All actual media ingest and protocol distribution should converge into `lalmax/lal`:

- RTSP pull
- ONVIF-resolved RTSP pull
- RTMP push
- SRT push

The project should not keep a second long-term RTMP main path.

### Separate device and stream concepts

Three layers should be distinguished:

1. `Camera`
   - physical device
   - credentials, ONVIF endpoint, RTSP transport, recording policy

2. `Source`
   - ingest definition
   - may come from a camera, RTMP ingest, SRT ingest, or relay

3. `LiveStream`
   - active runtime media stream
   - derived from `lalmax` runtime state

## Recommended Model

### Camera

Keep the current model for:

- RTSP cameras
- ONVIF cameras
- Xiaomi cameras

### Source

Recommended minimum fields:

- `id`
- `name`
- `type`: `camera | rtmp_ingest | srt_ingest | relay`
- `camera_id` (nullable)
- `ingest_key` / `stream_id` / `app_name`
- `enabled`

### LiveStream

Recommended runtime API object:

- `stream_id`
- `app_name`
- `source_type`
- `source_ref`
- `managed`
- `bound_camera_id`
- `publisher_protocol`
- `video_codec`
- `audio_codec`
- `active`
- `last_seen`
- `publisher`
- `subscribers`

## API Proposal

### Phase 1: Read-only visibility

Add:

- `GET /api/streams`
- `GET /api/streams/{stream_id}`

Purpose:

- expose all active streams already present in `lalmax`
- solve the immediate "the stream exists but is invisible" problem

### Phase 2: Binding and promotion

Add:

- `POST /api/streams/{stream_id}/bind-camera`
- `POST /api/streams/{stream_id}/unbind-camera`
- `POST /api/streams/{stream_id}/promote`

Meaning:

- bind an external stream to an existing camera
- promote an unmanaged stream into a formal managed source

### Phase 3: Control operations

Add:

- `DELETE /api/streams/{stream_id}`
- `POST /api/streams/{stream_id}/kick-publisher`

Meaning:

- stop relay pull for pull-based sources
- disconnect the publisher for push-based sources

## Frontend Proposal

Add a dedicated `Streams` page instead of expanding the `Cameras` page further.

Suggested sections:

1. `Managed Cameras`
2. `External Streams`

Each stream should support:

- preview
- publisher/subscriber state
- bind to camera
- promote to managed source
- disconnect

## Recommendation for internal/rtmp.Server

Conclusion:

- **do not keep it as a long-term primary path**
- **do not delete it immediately**

Reason:

1. it still provides transitional `stream key -> camera hub` behavior
2. it is not part of the unified `mediaEngine.ListStreams()` state surface
3. deleting it before stream-management APIs exist would cause feature regression

Recommended transition:

### Stage A

- stop adding new ingest features to `internal/rtmp.Server`
- prefer `lalmax` for all new ingest work

### Stage B

- ship `GET /api/streams` and a Streams page
- make existing `lalmax` streams visible first

### Stage C

- add binding and control operations
- convert `RTMP stream key -> camera ID` from a second-server concern into a business-layer binding rule

### Stage D

- retire `internal/rtmp.Server` after `lalmax` ingest is fully surfaced and managed

## What Not To Do

### 1. Disable push and keep only pull

Not recommended.

Reasons:

- OBS / FFmpeg / encoders naturally prefer push
- some sources cannot be pulled
- it reduces supported ingest scenarios instead of simplifying the model

### 2. Pretend every external stream is a Camera

Not recommended.

Reasons:

- Camera is a device model, not a stream model
- it pollutes device management with ingest-only concerns

## Phased Execution Plan

### Phase 1: Make streams visible

- add `GET /api/streams`
- add a `Streams` page
- render data from `mediaEngine.ListStreams()`

### Phase 2: Show ownership

- identify bound vs unmanaged streams
- expose camera/source relationship

### Phase 3: Add stream operations

- bind
- promote
- disconnect

### Phase 4: Retire legacy RTMP ingress

- remove `internal/rtmp.Server`
- keep `lalmax/lal` as the only ingest layer

## Recommended Immediate Next Steps

1. implement `GET /api/streams`
2. add a minimal `Streams` page
3. add bind / promote actions
4. retire `internal/rtmp.Server` last

This is the lowest-risk path with the fastest operational payoff.
