# ONVIF 设备录像查询功能实现方案

## 概述

实现 ONVIF 设备（摄像头 SD 卡）的录像查询和回放功能。

## 技术架构

### ONVIF 服务

1. **Recording Service** - 管理设备上的录像
   - GetRecordings - 获取录像列表
   - GetRecordingJobs - 获取录制任务
   
2. **Search Service** - 搜索录像
   - FindRecordings - 开始搜索录像
   - GetRecordingSearchResults - 获取搜索结果
   
3. **Replay Service** - 回放录像
   - GetReplayUri - 获取回放流地址

### 数据流

```
前端 → API → ONVIF Client → 设备 SOAP 请求 → 设备响应 → 解析 → 返回前端
```

## 实现步骤

### 1. 后端 ONVIF 录像客户端

**新增文件**: `internal/onvif/recording.go`

```go
// Recording 表示设备上的一个录像
type Recording struct {
    Token       string
    Name        string
    Description string
    Source      RecordingSource
    StartTime   time.Time
    EndTime     time.Time
    Status      string
}

// RecordingSegment 表示录像片段
type RecordingSegment struct {
    Token       string
    RecordingToken string
    StartTime   time.Time
    EndTime     time.Time
    FilePath    string
    Duration    time.Duration
    Size        int64
}

// SearchRequest 搜索请求
type SearchRequest struct {
    RecordingToken string
    StartTime      time.Time
    EndTime        time.Time
    MaxResults     int
}

// GetRecordings 获取设备上的所有录像
func (c *Client) GetRecordings(ctx context.Context) ([]Recording, error)

// SearchRecordings 搜索录像片段
func (c *Client) SearchRecordings(ctx context.Context, req SearchRequest) ([]RecordingSegment, error)

// GetReplayURI 获取回放流地址
func (c *Client) GetReplayURI(ctx context.Context, recordingToken string) (string, error)
```

### 2. 后端 API 接口

**新增文件**: `internal/api/handlers_onvif_recording.go`

```go
// handleGetONVIFRecordings 查询 ONVIF 设备录像
// GET /api/cameras/:id/onvif/recordings?start_time=xxx&end_time=xxx
func (h *Handler) handleGetONVIFRecordings(w http.ResponseWriter, r *http.Request)

// handleGetONVIFReplayURI 获取回放地址
// GET /api/cameras/:id/onvif/replay/:token
func (h *Handler) handleGetONVIFReplayURI(w http.ResponseWriter, r *http.Request)
```

### 3. 前端 API

**新增文件**: `web/src/lib/api/onvif-recording.ts`

```typescript
export interface ONVIFRecording {
  token: string;
  name: string;
  description: string;
  start_time: string;
  end_time: string;
  status: string;
}

export interface ONVIFRecordingSegment {
  token: string;
  recording_token: string;
  start_time: string;
  end_time: string;
  file_path: string;
  duration: number;
  size: number;
}

export interface ONVIFReplayResponse {
  uri: string;
  protocol: string;
}

export async function getONVIFRecordings(
  cameraId: string,
  params?: { start_time?: string; end_time?: string }
): Promise<{ recordings: ONVIFRecording[] }>

export async function searchONVIFRecordings(
  cameraId: string,
  params: { start_time: string; end_time?: string; max_results?: number }
): Promise<{ segments: ONVIFRecordingSegment[] }>

export async function getONVIFReplayURI(
  cameraId: string,
  recordingToken: string
): Promise<ONVIFReplayResponse>
```

### 4. 前端组件

**新增文件**: `web/src/lib/components/ONVIFRecordingQuery.svelte`

功能：
- 设备选择（自动加载已连接的 ONVIF 设备）
- 时间范围选择
- 录像列表显示
- 回放播放器集成

### 5. 集成到设备管理页面

在 ONVIF 设备标签页中添加「设备录像」子标签，类似于 GB28181 的实现。

## 测试要点

1. **录像查询**
   - 查询设备上的所有录像
   - 按时间范围筛选
   - 显示录像基本信息

2. **录像搜索**
   - 按时间范围搜索
   - 显示搜索结果
   - 分页支持

3. **录像回放**
   - 获取回放地址
   - 集成 FLV 播放器
   - 播放控制

## 已知限制

1. 不同厂商的 ONVIF 实现可能有差异
2. 部分设备可能不支持 Recording/Replay 服务
3. 回放协议可能因设备而异（RTSP/HTTP）

## 后续优化

1. 支持录像下载
2. 支持录像删除（如果设备支持）
3. 支持更复杂的搜索条件
