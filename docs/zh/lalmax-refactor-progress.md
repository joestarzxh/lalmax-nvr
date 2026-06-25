# lalmax / lalmax-nvr 改造进展

本文档记录当前分支上，围绕 `third/lal` 和 `third/lalmax` 对 `lalmax-nvr` 的媒体链路改造进度，方便后续继续施工时快速恢复上下文。

与 `RTMP/SRT push`、`外部流可见性`、`统一流管理` 相关的后续设计，已单独整理到：

- [stream-management-design.md](./stream-management-design.md)

## 目标

目标不是在旧链路上继续堆功能，而是逐步把项目收敛到下面这条主路径：

1. 摄像头输入统一进入 `lalmax`
2. HLS / HTTP-FLV / WebRTC 输出统一由 `lalmax` 提供
3. 录像链路尽量消费 `lalmax` 的统一流，减少对摄像头的重复拉流
4. ONVIF 的发现、信令、profile/encoding 决策继续使用项目自身已有实现
5. 旧的内建 live manager 退到 fallback 或残留职责，最终逐步退出主路径

## 当前状态

截至本次记录，主链路已经基本成形：

### 1. 依赖与目录

- 已引入本地依赖：
  - `third/lal`
  - `third/lalmax`
- `go.mod` 已通过 `replace` 指向本地 `third/` 目录
- 项目名已经统一为 `lalmax-nvr`

### 2. media engine 基座

- 已新增 `internal/media` 下的 engine 抽象与 `lalmax` 适配骨架
- 配置新增 `media:` 段，用于控制是否启用新的媒体主路径
- `cmd/lalmax-nvr/main.go` 已能创建并注入 `mediaEngine`

相关目录：

- `internal/media/`
- `cmd/lalmax-nvr/main.go`
- `internal/config/config.go`
- `config.example.yaml`

### 3. 输出侧已切到 lalmax 主路径

当前在 `media.enabled=true` 时：

- `GET /api/cameras/{id}/stream/*` 优先代理到 `lalmax` 的 HLS 输出
- `GET /api/cameras/{id}/stream.flv` 优先代理到 `lalmax` 的 HTTP-FLV 输出
- `POST/DELETE /api/cameras/{id}/stream/webrtc` 优先代理到 `lalmax` 的 WHEP
- `GET /api/cameras/{id}/protocols` 会返回：
  - `play_url`
  - `backend`
  - `stream_status`

其中：

- `hls / flv / webrtc` 的 backend 为 `lalmax`
- `wasm` 目前仍保留为 `builtin-ws`

关键文件：

- `internal/api/handler.go`
- `internal/api/handlers_hls.go`
- `internal/api/handlers_flv.go`
- `internal/api/handlers_webrtc.go`
- `internal/api/handlers_stream.go`

### 4. 输入侧已切到 lalmax relay pull

当前在 `media.enabled=true` 时：

- `rtsp + h264/h265` 相机启动时会先 `mediaEngine.StartPull(...)`
- `onvif` 相机也会优先解析出实际 RTSP URI 后交给 `lalmax`
- 停止、重启、禁用、删除等生命周期会同步 `StopPull(...)`

关键文件：

- `internal/camera/manager.go`

### 5. 录像链路已开始消费 lalmax 统一流

当前状态：

- `rtsp + h264/h265` recorder 已不再直接拉摄像头
- 录像仍然沿用现有 `H264Recorder/H265Recorder`
- 但其输入已切到 `lalmax` 输出的本地 RTSP 统一流
- `onvif` 在编码已知时，也走同样路径

这意味着：

- 对 RTSP H.264/H.265，相机双拉已经基本消掉
- 对 ONVIF，大部分场景也已切到统一流

### 5.1 断流追踪与恢复事件

所有 recorder（H264、H265、MJPEG、HTTP-JPEG）现已支持断流追踪：

- 断流发生时记录 `disconnectedAt` 时间戳和 `gapReason`（由错误类型自动分类）
- 恢复连接后，在首个录制段的 `Recording` 中填充 `ReconnectedAt` 和 `GapReason` 字段
- 通过 EventBus 发布 `TopicRecorderReconnected` 事件，携带 `Downtime`（断流时长）、`RetryCount`（重试次数）等信息
- DB schema 已升级至 v16，`recordings` 表新增 `reconnected_at` 和 `gap_reason` 列

断流原因分类（`gap_reason` 可选值）：
- `frame_watchdog` — RTSP 存活但无帧数据（30s 超时）
- `connection_lost` — 连接断开（EOF / 对端关闭）
- `connection_refused` — 连接被拒绝
- `connection_timeout` — 连接超时
- `rtsp_negotiation` — RTSP DESCRIBE/SETUP/PLAY 失败
- `connection_error` — 其他连接错误

关键文件：

- `internal/camera/manager.go`
- `internal/recorder/h264.go`
- `internal/recorder/h265.go`
- `internal/recorder/mjpeg.go`
- `internal/recorder/http_jpeg.go`
- `internal/recorder/backoff.go` — `classifyDisconnectReason()`
- `internal/event/types.go` — `RecorderReconnected`
- `internal/model/types.go` — `Recording.ReconnectedAt`, `Recording.GapReason`
- `internal/storage/db.go` — migration v15→v16

### 6. ONVIF 主路径已切到“项目自身信令 + lalmax 媒体”

当前 `media.enabled=true` 且 `protocol=onvif` 时：

1. 使用项目自己的 `internal/onvif` client 获取 profiles
2. 自动选择或复用 `profile_token`
3. 自动补齐 `encoding / stream_encoding`
4. 必要时解析实际 `stream URI`
5. 将补齐后的信息写回：
   - 内存配置
   - SQLite `cameras` 表
   - YAML 配置文件
6. 再走 `lalmax pull -> lalmax 本地 RTSP -> H264/H265 recorder`

这条路径已经是 ONVIF 的主路径。旧 `ONVIFRecorder` 目前只剩 fallback 意义。

关键文件：

- `internal/camera/manager.go`
- `internal/onvif/`

### 7. 旧 live manager 的职责已经收缩

当前在 `media.enabled=true` 时：

- 旧 `HLS / FLV / WebRTC` manager 不再初始化为主路径
- `StreamRegistry` 会直接把相关协议能力注册到 `mediaEngine`
- `ws` 仍然保留，用于 wasm / WebCodecs 本地链路
- `rtmp / srt` ingest 仍保留旧职责

关键文件：

- `internal/media/runtime.go`
- `cmd/lalmax-nvr/main.go`

## 当前架构判断

当前系统可以近似理解成：

### 已经统一到 lalmax 的部分

- 摄像头拉流入口：`rtsp / onvif`
- 在线播放出口：`hls / flv / webrtc`
- 录像主输入：`rtsp h264/h265` 和大多数 `onvif`

### 仍然保留独立实现的部分

- `wasm / ws`
- `rtmp / srt` ingest
- 少量 recorder / fallback 残留逻辑

## 已验证的测试基线

本轮除了媒体改造，还顺手处理了两类测试基线问题：

### 1. transcoding probe 并发缓存问题

`internal/transcoding/probe.go` 之前使用 `sync.Once` + `ResetProbe()`，在并发测试下会触发 `sync: unlock of unlocked mutex`。

现在已改为：

- `sync.Mutex`
- `probeReady bool`
- 显式缓存重置

这样测试重置和并发探测不会互相踩坏 `sync.Once` 内部状态。

### 2. ONVIF API 测试环境敏感问题

之前 `internal/api/onvif_test.go` 默认假设“测试环境里一定没有 ONVIF 设备”，这在真实局域网环境中不成立。

现在已改为：

- `Handler` 实例级注入：
  - `onvifDiscover`
  - `onvifProbeDevice`
  - `onvifNewClient`
- 测试统一使用 stub，不再依赖真实网络

已通过的验证：

- `go test ./internal/transcoding -run "TestProbe|TestNewTranscodingManager"`
- `go test ./internal/api -run ONVIF`
- `go test ./internal/api`

## 后续建议顺序

建议后续按下面顺序继续：

### 第一优先级

1. 继续处理 recorder 残留路径
2. 明确哪些场景仍会直连摄像头
3. 把旧 `ONVIFRecorder` 进一步降级为 fallback-only

#### 第一优先级进展（已完成）

**ONVIF 未知编码探测回退：**
当 `media.enabled=true` 且 ONVIF 编码未知时，系统现在会主动探测 ONVIF 设备以检测 H264/H265 编码，而不是直接放弃。如果检测成功，会将编码持久化到配置/数据库，重启媒体拉流，并创建基于 lalmax 中继的 H264/H265Recorder。此前此场景直接返回 `nil`（不录像）。详见 `internal/camera/manager.go:probeONVIFEncodingAndBuildURL`。

**残留直连摄像头路径识别：**
以下 recorder 类型在 `media.enabled=true` 时仍直连摄像头：
- **MJPEGRecorder** — lalmax 不转发 MJPEG 流；始终直接使用 `cam.URL`
- **HTTPJPEGRecorder** — 完全不同的传输协议（HTTP multipart）；始终直接使用 `cam.URL`
- **XiaomiRecorder** — 使用 TUTK P2P 协议；超出 lalmax 转发能力
- **ONVIFRecorder** — 仅在 `media.enabled=false` 时使用（已降级为 fallback）

所有 H264/H265 RTSP 和 ONVIF recorder 在 `media.enabled=true` 时均通过 lalmax。

**日志改进：**
- 为 MJPEG 和 HTTP/JPEG 直连摄像头添加了 `WARN` 日志
- 为 RTSP H264/H265 添加了区分 lalmax 中继 vs 直连的 `INFO` 日志
- 为旧 `ONVIFRecorder` 使用场景添加了 `WARN` 日志
- 为 ONVIF 编码探测失败添加了带操作建议的 `WARN` 日志

**新增测试：**
- `TestCreateRecorder_ONVIF_ProbesEncodingWhenMediaEngineEnabled` — 验证编码探测 + lalmax 中继 + 配置/数据库持久化
- `TestCreateRecorder_ONVIF_ProbeFailsReturnsNil` — 验证探测未找到 H264/H265 时的优雅回退
- 更新了 `TestCreateRecorder_ONVIFReturnsNilWhenMediaEngineEnabledButEncodingUnknown`，注入了正确的 stub

### 第二优先级

1. 评估 `wasm / ws` 是否继续保留独立链路
2. 如果要统一，优先评估 `fMP4 + WebCodecs` 或 `lalmax` 可直接承接的输出形式

#### 第二优先级进展（已完成）

**决策：在现有 wasm/ws 之外新增 HTTP fMP4 协议。**

wasm/ws 路径（自定义二进制协议 over WebSocket → WebCodecs）仍然保留，供支持 WebCodecs 的浏览器使用。新增了 HTTP fMP4 路径，使用 lalmax 内置的 HTTP fMP4 服务器，浏览器通过 MSE（Media Source Extensions）消费。

**服务端（`internal/api/`）：**
- 新增 `handlers_fmp4.go` — 将 `GET /api/cameras/{id}/stream.m4s` 代理到 lalmax 的 HTTP fMP4 端点
- 在 `handler.go` 中注册新路由（在 HLS catch-all 之前）
- 新增 `FMP4StreamHandler` 用于协议发现
- `protocolPlayURL()` 现在处理 `"fmp4"` 协议
- `main.go` 在媒体引擎启用时注册 `FMP4StreamHandler`

**客户端（`web/src/`）：**
- 新增 `FMP4Player.svelte` — 基于 MSE 的播放器，使用 `fetch()` + `ReadableStream` 消费 HTTP fMP4 流
  - 解析 init segment（ftyp+moov）并送入 `SourceBuffer`
  - 实时流式传输 moof+mdat 片段
  - 指数退避 + 抖动的自动重连
  - 全屏支持、标签页可见性处理
- 更新 `ProtocolSwitcher.svelte` — 添加 `fmp4` 选项及 MSE 可用性检查
- 更新 `Dashboard.svelte` 和 `LiveView.svelte` — 懒加载 `FMP4Player`，含加载/错误状态

**数据流：**
```
摄像头 → lalmax（拉流中继） → lalmax fMP4 muxer → HTTP fMP4 流
  → NVR 代理（/stream.m4s） → 浏览器 fetch() → SourceBuffer（MSE） → <video>
```

**浏览器兼容性：**
- MSE 在所有现代浏览器中受支持（Chrome、Firefox、Safari、Edge）
- H.264 in MSE：普遍支持
- H.265 in MSE：Chrome 107+、Safari（不支持 Firefox）

### 第三优先级

1. 继续裁掉旧 `media.Runtime` 中非必要的 live fan-out 逻辑
2. 梳理 `rtmp / srt` 是否也需要纳入新的统一媒体边界

#### 第三优先级进展（已完成）

**决策：修复现有 RTMP/SRT 的接线（方案 A）。**

lalmax 内置的 RTMP/SRT 服务器无法连接到 NVR 的摄像头/录像管线——需要桥接层。自定义实现已经有 StreamHub 集成和摄像头 ID 解析，只需要非 nil 回调。

**删除死代码（`internal/api/handlers_stream.go`）：**
- 删除 `handleHLSStreamViaRegistry`（84 行）— 从未在路由中注册
- 删除 `handleStopHLSStreamViaRegistry` — 同上
- 清理 3 个未使用的 import

**FMP4StreamHandler 注册修复（`cmd/lalmax-nvr/main.go`）：**
- 从 `else` 块（mediaEngine==nil）移到 `if` 块（mediaEngine!=nil）
- handler 需要 mediaEngine 才能工作，注册逻辑必须匹配

**RTMP/SRT ingest 接线（`internal/media/ingest.go` + `cmd/lalmax-nvr/main.go`）：**
- 实际 RTMP/SRT 接入由 lalmax/lal 负责。
- `internal/media.IngestHandler` 订阅 lalmax 的 publish/stop 事件，并把 stream name 映射到 camera ID。
- RTMP stream key 仍通过反向表支持 `camera_id -> stream_key` 配置。
- SRT 直接把 stream name 映射为 camera ID。

## 建议关注的文件

后续继续时，优先从这些文件入手：

- `internal/camera/manager.go`
- `internal/media/runtime.go`
- `internal/media/`
- `internal/api/handler.go`
- `internal/api/handlers_hls.go`
- `internal/api/handlers_flv.go`
- `internal/api/handlers_webrtc.go`
- `internal/api/handlers_stream.go`
- `internal/transcoding/probe.go`

## 一句话结论

当前项目已经不是“准备开始接 lalmax”，而是已经完成了第一阶段主链路迁移：输入、输出、以及录像主输入都已经开始围绕 `lalmax` 收敛，后面主要是清理残留路径和继续统一边界。
