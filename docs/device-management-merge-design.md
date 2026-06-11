# 设备管理页面合并设计方案（最终版）

## 目标
将摄像头页面（Cameras.svelte）的功能整合到设备管理页面（Devices.svelte）中，提供统一的设备管理体验。

## 功能对比矩阵

### 合并后的设备管理页面

| 功能 | ONVIF | GB28181 | 小米 | 推流 |
|------|-------|---------|------|------|
| **基本信息** | ✅ | ✅ | ✅ | ✅ |
| **状态显示** | ✅ | ✅ | ✅ | ✅ |
| **快照预览** | ✅ | ✅ (有摄像头时) | ✅ | ✅ |
| **录制控制** | ✅ | ✅ | ✅ | ✅ |
| **暂停/恢复录制** | ✅ | ✅ | ✅ | ✅ |
| **编辑配置** | ✅ | ✅ | ✅ | ✅ |
| **实时预览** | ✅ | ✅ | ✅ | ✅ |
| **查看事件** | ✅ (快捷链接) | ✅ (快捷链接) | ✅ (快捷链接) | ✅ (快捷链接) |
| **查看录像** | ✅ (快捷链接) | ✅ (快捷链接) | ✅ (快捷链接) | ✅ (快捷链接) |
| **归档** | ✅ | ✅ | ✅ | ✅ |
| **永久删除** | ✅ | ✅ | ✅ | ❌ |
| **健康监控** | ✅ | ❌ | ✅ | ❌ |
| **设备发现** | ✅ | ❌ | ✅ | ❌ |
| **语音广播** | ❌ | ✅ | ❌ | ❌ |
| **设备端录像查询** | ✅ | ✅ | ❌ | ❌ |
| **设备端录像回放** | ✅ | ✅ | ❌ | ❌ |
| **设备端报警记录** | ❌ | ✅ | ❌ | ❌ |

## 实现细节

### 1. ONVIF 设备录像查询

#### 技术实现

**后端 ONVIF Recording/Replay 客户端**：
```go
// internal/onvif/recording.go
type Recording struct {
    Token       string
    Name        string
    Description string
    Source      RecordingSource
    StartTime   time.Time
    EndTime     time.Time
    Status      string
}

type RecordingSegment struct {
    Token          string
    RecordingToken string
    StartTime      time.Time
    EndTime        time.Time
    FilePath       string
    Duration       int64
    Size           int64
}

func (c *Client) GetRecordings(ctx context.Context) ([]Recording, error)
func (c *Client) SearchRecordings(ctx context.Context, req SearchRequest) ([]RecordingSegment, error)
func (c *Client) GetReplayURI(ctx context.Context, recordingToken string) (string, error)
```

**后端 API 接口**：
```go
// internal/api/handlers_onvif_recording.go
GET /api/cameras/:id/onvif/recordings          // 查询设备录像
GET /api/cameras/:id/onvif/recordings/search    // 搜索录像片段
GET /api/cameras/:id/onvif/replay/:token        // 获取回放地址
```

**前端 API**：
```typescript
// web/src/lib/api/onvif-recording.ts
export async function getONVIFRecordings(cameraId: string, params?): Promise<ONVIFRecordingsResponse>
export async function searchONVIFRecordings(cameraId: string, params): Promise<ONVIFRecordingSegmentsResponse>
export async function getONVIFReplayURI(cameraId: string, recordingToken: string): Promise<ONVIFReplayResponse>
```

**前端组件**：
```svelte
<!-- web/src/lib/components/ONVIFRecordingQuery.svelte -->
- 设备选择
- 时间范围选择
- 查询模式（录像列表/片段搜索）
- 录像/片段列表
- 回放播放器
```

#### 子标签页

ONVIF 设备标签页新增「设备录像」子标签：
```
[设备列表] [设备录像]
```

### 2. GB28181 设备录像查询

#### 子标签页
```
[设备列表] [设备录像] [报警记录]
```

#### 功能
- 设备录像查询和回放
- 报警记录查看

### 3. 小米设备

通过 DiscoveryPanel 登录小米账号后发现并添加，使用 CameraCard 组件提供完整管理功能。

### 4. 推流设备

为已升级为摄像头的推流设备提供完整管理功能，包括暂停/恢复录制。

## API 依赖

### ONVIF Recording 相关 API
- `getONVIFRecordings()` - 查询设备录像
- `searchONVIFRecordings()` - 搜索录像片段
- `getONVIFReplayURI()` - 获取回放地址

### GB28181 相关 API
- `queryDeviceRecords()` - 查询设备录像
- `startDevicePlayback()` - 开始录像回放
- `listGB28181Alarms()` - 获取报警记录
- `startBroadcast()` / `stopBroadcast()` - 语音广播

### 摄像头管理 API（复用）
- `listCameras()` - 获取摄像头列表
- `startCamera()` / `stopCamera()` - 启动/停止摄像头
- `pauseRecording()` / `resumeRecording()` - 暂停/恢复录制
- `updateCamera()` - 更新摄像头配置
- `deleteCamera()` / `permanentlyDeleteCamera()` - 删除摄像头
- `getSnapshotUrl()` - 获取快照 URL
- `getHealthStatus()` - 获取健康状态

## 组件依赖

1. **CameraCard** - 摄像头卡片组件
2. **CameraForm** - 摄像头表单组件
3. **ConfirmDialog** - 确认对话框组件
4. **ArchiveConfirmDialog** - 归档确认对话框组件
5. **DiscoveryPanel** - 设备发现面板
6. **FlvPlayer** - FLV 播放器组件
7. **ONVIFRecordingQuery** - ONVIF 录像查询组件（新增）

## 测试要点

### ONVIF 设备录像测试

1. **录像查询**
   - 查询设备上的录像列表
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

### GB28181 设备测试

1. **设备录像查询**
   - 选择设备和通道
   - 选择时间范围
   - 查询录像列表
   - 播放录像

2. **报警记录**
   - 查看报警列表
   - 分页功能

3. **语音广播**
   - 开始广播
   - 停止广播

### 推流设备测试

1. **录制控制**
   - 启动录制
   - 暂停录制
   - 恢复录制
   - 停止录制

## 总结

本方案实现了设备管理页面的完整功能合并：

1. **ONVIF 设备** - 完整的摄像头管理和设备录像查询功能
2. **GB28181 设备** - 增强通道管理，支持摄像头管理、设备录像查询、报警记录、语音广播
3. **小米设备** - 通过 DiscoveryPanel 添加后，使用 CameraCard 提供完整管理功能
4. **推流设备** - 为升级的摄像头提供完整管理功能，包括暂停/恢复录制

所有设备类型现在都具有一致的管理体验，用户可以在一个页面中管理所有类型的视频设备。

### 关键特性

- **统一管理**：所有设备类型在同一个页面管理
- **功能完整**：暂停/恢复录制功能已扩展到所有支持的设备类型
- **设备端录像**：ONVIF 和 GB28181 设备支持查询和回放设备端录像
- **代码复用**：最大化复用现有组件，减少代码重复
- **用户体验**：一致的操作体验和状态显示
