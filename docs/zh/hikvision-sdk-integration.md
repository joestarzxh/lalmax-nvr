# 海康设备SDK集成方案

## 1. 项目背景

lalmax-nvr 是一个基于 lalmax 媒体引擎的轻量级网络视频录像机，目前支持 RTSP、ONVIF、GB28181 等协议接入摄像头。为了支持更多海康威视设备的接入方式，需要集成海康设备网络SDK和ISUP Ehome协议。

### 1.1 参考项目分析

参考项目 `royi-qos-nvr` 是一个基于 Java 的微服务架构 NVR，通过 JNA 调用海康 SDK，支持：
- 海康设备网络SDK（HCNetSDK）
- ISUP Ehome 主动注册模式
- 大华设备SDK

### 1.2 技术选型

考虑到以下因素，采用**独立C++服务**方案：

| 因素 | CGO直接调用 | 独立C++服务 |
|------|------------|-------------|
| SDK兼容性 | 需要处理CGO绑定 | 原生支持 |
| 跨平台编译 | 复杂 | 简单 |
| 部署灵活性 | 单二进制 | 独立部署/升级 |
| 性能 | 优秀 | 优秀 |
| 维护成本 | 中等 | 低 |

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                      lalmax-nvr (Go)                         │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐       │
│  │ Camera  │  │ Recorder│  │  API    │  │  Web UI │       │
│  └────┬────┘  └────┬────┘  └────┬────┘  └─────────┘       │
│       │            │            │                            │
│       └────────────┼────────────┘                            │
│                    │                                         │
│              ┌─────▼─────┐                                   │
│              │  lalmax   │                                   │
│              │  (媒体引擎) │                                   │
│              └─────┬─────┘                                   │
└────────────────────┼────────────────────────────────────────┘
                     │ RTSP/RTMP/RTP
                     │
┌────────────────────┼────────────────────────────────────────┐
│              ┌─────▼─────┐                                   │
│              │  Gateway  │  (Go - 协议转换层)                 │
│              └─────┬─────┘                                   │
└────────────────────┼────────────────────────────────────────┘
                     │ gRPC/Unix Socket
                     │
┌────────────────────┼────────────────────────────────────────┐
│  ┌─────────────────▼─────────────────┐                      │
│  │     Hikvision SDK Service (C++)   │                      │
│  │  ┌─────────┐  ┌─────────┐        │                      │
│  │  │ HCNetSDK│  │ ISUP    │        │                      │
│  │  │ (设备SDK)│  │ (Ehome) │        │                      │
│  │  └─────────┘  └─────────┘        │                      │
│  └───────────────────────────────────┘                      │
│                     独立C++服务                               │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 数据流

```
摄像头 ──→ 海康SDK(C++) ──→ RTP封装 ──→ RTSP/RTMP推流 ──→ lalmax ──→ 录像/直播
   │                           │
   └──→ 云台控制 ──→ gRPC ────┘
```

### 2.3 组件职责

| 组件 | 职责 | 技术栈 |
|------|------|--------|
| Hikvision SDK Service | SDK调用、设备管理、流处理 | C++17 |
| Gateway | 协议转换、服务发现 | Go |
| lalmax-nvr | 录像、直播、Web UI | Go |
| lalmax | 媒体转发、协议转换 | Go |

## 3. 项目结构

### 3.1 C++服务项目结构

```
hikvision-sdk-service/
├── CMakeLists.txt                    # CMake构建配置
├── README.md
├── LICENSE
│
├── include/                          # 头文件
│   ├── hcnet/
│   │   ├── HCNetSDK.h               # 海康设备SDK头文件
│   │   ├── HCNetSDKType.h           # 类型定义
│   │   └── plaympeg4.h              # 播放库头文件
│   │
│   ├── isup/
│   │   ├── EHomeCMS.h               # ISUP CMS头文件
│   │   ├── EHomeDAS.h               # ISUP DAS头文件
│   │   └── EHomeSMS.h               # ISUP SMS头文件
│   │
│   ├── core/
│   │   ├── sdk_manager.h            # SDK管理器
│   │   ├── device_manager.h         # 设备管理器
│   │   └── stream_manager.h         # 流管理器
│   │
│   ├── protocol/
│   │   ├── rtp_muxer.h              # RTP封装器
│   │   ├── rtsp_pusher.h            # RTSP推流器
│   │   └── rtmp_pusher.h            # RTMP推流器
│   │
│   └── rpc/
│       ├── grpc_server.h            # gRPC服务
│       └── service_impl.h           # 服务实现
│
├── src/                              # 源文件
│   ├── main.cpp                     # 程序入口
│   │
│   ├── core/
│   │   ├── sdk_manager.cpp
│   │   ├── device_manager.cpp
│   │   └── stream_manager.cpp
│   │
│   ├── hcnet/
│   │   ├── hcnet_client.cpp         # 设备SDK客户端
│   │   ├── hcnet_callback.cpp       # 回调处理
│   │   ├── hcnet_ptz.cpp            # 云台控制
│   │   └── hcnet_playback.cpp       # 录像回放
│   │
│   ├── isup/
│   │   ├── isup_server.cpp          # ISUP服务端
│   │   ├── isup_device.cpp          # 设备注册
│   │   └── isup_stream.cpp          # 流处理
│   │
│   ├── protocol/
│   │   ├── rtp_muxer.cpp
│   │   ├── rtsp_pusher.cpp
│   │   └── rtmp_pusher.cpp
│   │
│   └── rpc/
│       ├── grpc_server.cpp
│       └── service_impl.cpp
│
├── proto/                            # Protobuf定义
│   └── hikvision.proto
│
├── lib/                              # 海康SDK库文件
│   ├── linux64/
│   │   ├── libhcnetsdk.so
│   │   ├── libhpr.so
│   │   ├── libHCCore.so
│   │   └── libPlayCtrl.so
│   │
│   ├── linux32/
│   │   └── ...
│   │
│   └── windows/
│       ├── HCNetSDK.dll
│       ├── HCCore.dll
│       └── PlayCtrl.dll
│
├── third_party/                      # 第三方库
│   ├── grpc/
│   ├── protobuf/
│   ├── spdlog/                      # 日志库
│   └── yaml-cpp/                    # 配置解析
│
├── config/                           # 配置文件示例
│   └── config.yaml.example
│
├── tests/                            # 测试代码
│   ├── unit/
│   │   ├── test_rtp_muxer.cpp
│   │   └── test_device_manager.cpp
│   │
│   └── integration/
│       └── test_sdk_service.cpp
│
├── scripts/                          # 脚本
│   ├── build.sh
│   ├── install.sh
│   └── systemd/
│       └── hikvision-sdk.service
│
└── docker/                           # Docker支持
    ├── Dockerfile
    └── docker-compose.yml
```

### 3.2 lalmax-nvr 扩展结构

```
lalmax-nvr/
├── internal/
│   ├── haikang/                      # 海康SDK集成模块
│   │   ├── gateway.go               # gRPC网关
│   │   ├── client.go                # 客户端封装
│   │   ├── config.go                # 配置定义
│   │   └── recorder.go              # 录制器
│   │
│   └── camera/
│       └── manager.go               # 扩展摄像头管理器
│
├── proto/
│   └── haikang/
│       └── hikvision.proto          # 从C++项目同步
│
└── config/
    └── config.yaml                  # 扩展配置
```

## 4. 核心接口设计

### 4.1 gRPC接口定义

```protobuf
// proto/hikvision.proto
syntax = "proto3";

package hikvision;

option go_package = "lalmax-nvr/proto/haikang";

// 海康设备SDK服务
service HikvisionService {
  // 设备管理
  rpc Login(DeviceLoginRequest) returns (DeviceLoginResponse);
  rpc Logout(LogoutRequest) returns (LogoutResponse);
  rpc GetDeviceInfo(DeviceInfoRequest) returns (DeviceInfoResponse);
  
  // 实时预览
  rpc StartPreview(PreviewRequest) returns (PreviewResponse);
  rpc StopPreview(StopPreviewRequest) returns (StopPreviewResponse);
  
  // 云台控制
  rpc PTZControl(PTZRequest) returns (PTZResponse);
  rpc PTZPreset(PTZPresetRequest) returns (PTZPresetResponse);
  
  // 录像回放
  rpc QueryRecord(RecordQueryRequest) returns (RecordQueryResponse);
  rpc StartPlayback(PlaybackRequest) returns (PlaybackResponse);
  rpc StopPlayback(StopPlaybackRequest) returns (StopPlaybackResponse);
  rpc PlaybackControl(PlaybackControlRequest) returns (PlaybackControlResponse);
  
  // 设备发现 (ISUP)
  rpc RegisterISUPDevice(RegisterISUPDeviceRequest) returns (RegisterISUPDeviceResponse);
  rpc UnregisterISUPDevice(UnregisterISUPDeviceRequest) returns (UnregisterISUPDeviceResponse);
  
  // 服务状态
  rpc GetServiceStatus(ServiceStatusRequest) returns (ServiceStatusResponse);
}

// ==================== 设备管理 ====================

message DeviceLoginRequest {
  string ip = 1;
  int32 port = 2;
  string username = 3;
  string password = 4;
  DeviceProtocol protocol = 5;
  int32 timeout = 6;  // 超时时间(秒)
}

enum DeviceProtocol {
  HCNET = 0;    // 海康设备网络SDK
  ISUP = 1;     // ISUP Ehome
}

message DeviceLoginResponse {
  bool success = 1;
  int32 user_id = 2;
  string error_message = 3;
  int32 error_code = 4;
  DeviceInfo device_info = 5;
}

message LogoutRequest {
  int32 user_id = 1;
}

message LogoutResponse {
  bool success = 1;
}

message DeviceInfoRequest {
  int32 user_id = 1;
}

message DeviceInfoResponse {
  DeviceInfo info = 1;
}

message DeviceInfo {
  int32 user_id = 1;
  string device_name = 2;
  string serial_number = 3;
  string device_type = 4;
  int32 channel_count = 5;
  int32 analog_channel_count = 6;
  int32 ip_channel_count = 7;
  string software_version = 8;
  string hardware_version = 9;
}

// ==================== 实时预览 ====================

message PreviewRequest {
  int32 user_id = 1;
  int32 channel = 2;
  StreamType stream_type = 3;
  string output_url = 4;          // 输出流地址 (RTSP/RTMP)
  TransportProtocol transport = 5; // 传输协议
}

enum StreamType {
  MAIN_STREAM = 0;    // 主码流
  SUB_STREAM = 1;     // 子码流
  THIRD_STREAM = 2;   // 第三码流
}

enum TransportProtocol {
  TCP = 0;
  UDP = 1;
}

message PreviewResponse {
  bool success = 1;
  int32 preview_handle = 2;
  string stream_url = 3;    // 生成的流地址
  string error_message = 4;
}

message StopPreviewRequest {
  int32 preview_handle = 1;
}

message StopPreviewResponse {
  bool success = 1;
}

// ==================== 云台控制 ====================

message PTZRequest {
  int32 user_id = 1;
  int32 channel = 2;
  PTZCommand command = 3;
  int32 speed = 4;        // 速度 1-7
  bool stop = 5;          // true:停止 false:开始
}

enum PTZCommand {
  PTZ_UP = 0;
  PTZ_DOWN = 1;
  PTZ_LEFT = 2;
  PTZ_RIGHT = 3;
  PTZ_UP_LEFT = 4;
  PTZ_UP_RIGHT = 5;
  PTZ_DOWN_LEFT = 6;
  PTZ_DOWN_RIGHT = 7;
  PTZ_ZOOM_IN = 8;
  PTZ_ZOOM_OUT = 9;
  PTZ_FOCUS_NEAR = 10;
  PTZ_FOCUS_FAR = 11;
  PTZ_IRIS_OPEN = 12;
  PTZ_IRIS_CLOSE = 13;
  PTZ_AUTO_SCAN = 14;
}

message PTZResponse {
  bool success = 1;
  string error_message = 2;
}

message PTZPresetRequest {
  int32 user_id = 1;
  int32 channel = 2;
  PTZPresetCommand command = 3;
  int32 preset_index = 4;
}

enum PTZPresetCommand {
  SET_PRESET = 0;
  CLE_PRESET = 1;
  GOTO_PRESET = 2;
}

message PTZPresetResponse {
  bool success = 1;
}

// ==================== 录像回放 ====================

message RecordQueryRequest {
  int32 user_id = 1;
  int32 channel = 2;
  int32 file_type = 3;    // 0:定时录像 1:移动侦测 2:报警 3:全部
  string start_time = 4;  // 格式: 2024-01-01 00:00:00
  string end_time = 5;
}

message RecordQueryResponse {
  bool success = 1;
  repeated RecordFile files = 2;
  string error_message = 3;
}

message RecordFile {
  string file_name = 1;
  string start_time = 2;
  string end_time = 3;
  int64 file_size = 4;
}

message PlaybackRequest {
  int32 user_id = 1;
  int32 channel = 2;
  string start_time = 3;
  string end_time = 4;
  string output_url = 5;
}

message PlaybackResponse {
  bool success = 1;
  int32 playback_handle = 2;
  string stream_url = 3;
  string error_message = 4;
}

message StopPlaybackRequest {
  int32 playback_handle = 1;
}

message StopPlaybackResponse {
  bool success = 1;
}

message PlaybackControlRequest {
  int32 playback_handle = 1;
  PlaybackControlCommand command = 2;
  int32 value = 3;
}

enum PlaybackControlCommand {
  PLAY_START = 0;
  PLAY_PAUSE = 1;
  PLAY_RESUME = 2;
  PLAY_FAST = 3;
  PLAY_SLOW = 4;
  PLAY_NORMAL = 5;
  PLAY_SETPOS = 6;
}

message PlaybackControlResponse {
  bool success = 1;
}

// ==================== ISUP Ehome ====================

message RegisterISUPDeviceRequest {
  string device_serial = 1;
  string verify_code = 2;
}

message RegisterISUPDeviceResponse {
  bool success = 1;
  int32 device_id = 2;
  string error_message = 3;
}

message UnregisterISUPDeviceRequest {
  int32 device_id = 1;
}

message UnregisterISUPDeviceResponse {
  bool success = 1;
}

// ==================== 服务状态 ====================

message ServiceStatusRequest {}

message ServiceStatusResponse {
  bool running = 1;
  int32 device_count = 2;
  int32 stream_count = 3;
  string sdk_version = 4;
  int64 uptime_seconds = 5;
}
```

### 4.2 C++核心类设计

#### SDK管理器

```cpp
// include/core/sdk_manager.h
#pragma once

#include <string>
#include <unordered_map>
#include <mutex>
#include <memory>

class SDKManager {
public:
    static SDKManager& Instance();
    
    // 禁止拷贝
    SDKManager(const SDKManager&) = delete;
    SDKManager& operator=(const SDKManager&) = delete;
    
    // 初始化和清理
    bool Initialize(const std::string& sdk_path);
    void Cleanup();
    
    // 设备管理
    int Login(const std::string& ip, int port, 
              const std::string& username, const std::string& password);
    bool Logout(int user_id);
    
    // 流管理
    int StartPreview(int user_id, int channel, int stream_type,
                     const std::string& output_url, int transport);
    bool StopPreview(int preview_handle);
    
    // 云台控制
    bool PTZControl(int user_id, int channel, int command, 
                    int speed, bool stop);
    bool PTZPreset(int user_id, int channel, int command, int preset);
    
    // 录像回放
    int StartPlayback(int user_id, int channel,
                      const std::string& start_time,
                      const std::string& end_time,
                      const std::string& output_url);
    bool StopPlayback(int playback_handle);
    bool PlaybackControl(int playback_handle, int command, int value);
    
    // 状态查询
    int GetDeviceCount() const;
    int GetStreamCount() const;
    std::string GetSDKVersion() const;

private:
    SDKManager();
    ~SDKManager();
    
    // 设备会话管理
    struct DeviceSession {
        int user_id;
        std::string ip;
        int port;
        std::string username;
        void* device_info;
        std::chrono::steady_clock::time_point login_time;
    };
    
    // 流会话管理
    struct StreamSession {
        int handle;
        int user_id;
        int channel;
        std::string output_url;
        void* muxer;
        std::atomic<bool> running;
        std::thread thread;
    };
    
    std::unordered_map<int, std::shared_ptr<DeviceSession>> devices_;
    std::unordered_map<int, std::shared_ptr<StreamSession>> streams_;
    mutable std::mutex devices_mutex_;
    mutable std::mutex streams_mutex_;
    
    int next_user_id_ = 1000;
    int next_handle_ = 1;
    bool initialized_ = false;
    std::string sdk_path_;
    
    // 回调函数
    static void __stdcall RealDataCallback(
        int lRealHandle, unsigned int dwDataType,
        unsigned char* pBuffer, unsigned int dwBufSize,
        void* pUser);
};
```

#### RTP封装器

```cpp
// include/protocol/rtp_muxer.h
#pragma once

#include <string>
#include <cstdint>
#include <functional>

class RtpMuxer {
public:
    enum class Transport {
        UDP,
        TCP
    };
    
    enum class PayloadType {
        H264 = 96,
        H265 = 97,
        PCMU = 0,   // G.711 u-law
        PCMA = 8,   // G.711 a-law
    };
    
    RtpMuxer(const std::string& target_url, Transport transport);
    ~RtpMuxer();
    
    // 禁止拷贝
    RtpMuxer(const RtpMuxer&) = delete;
    RtpMuxer& operator=(const RtpMuxer&) = delete;
    
    // 初始化和清理
    bool Open();
    void Close();
    
    // 写入数据
    bool WriteVideoFrame(const uint8_t* data, size_t size,
                         bool is_key_frame, uint32_t timestamp);
    bool WriteAudioFrame(const uint8_t* data, size_t size,
                         uint32_t timestamp, PayloadType type);
    
    // 状态
    bool IsOpen() const { return is_open_; }
    uint32_t GetSSRC() const { return ssrc_; }

private:
    std::string target_url_;
    Transport transport_;
    bool is_open_ = false;
    
    // RTP参数
    uint16_t sequence_number_ = 0;
    uint32_t ssrc_;
    uint32_t timestamp_ = 0;
    uint32_t clock_rate_ = 90000;  // 视频时钟频率
    
    // 网络
    int socket_ = -1;
    struct sockaddr_in dest_addr_;
    
    // H264/H265解析
    struct NALU {
        const uint8_t* data;
        size_t size;
        int type;
        bool is_key_frame;
    };
    
    // 内部方法
    bool ParseURL(const std::string& url);
    bool CreateSocket();
    bool SendRtpPacket(const uint8_t* payload, size_t size,
                       bool marker, uint32_t timestamp);
    bool SendRtpOverTCP(const uint8_t* packet, size_t size);
    bool SendRtpOverUDP(const uint8_t* packet, size_t size);
    
    // NALU解析
    std::vector<NALU> ParseH264NALUs(const uint8_t* data, size_t size);
    std::vector<NALU> ParseH265NALUs(const uint8_t* data, size_t size);
    
    // RTP打包
    bool SendSingleNALU(const NALU& nalu, uint32_t timestamp);
    bool SendFUANALU(const NALU& nalu, uint32_t timestamp, bool is_h265);
    
    // 辅助函数
    void WriteRtpHeader(uint8_t* buffer, bool marker,
                        uint16_t seq, uint32_t ts, uint32_t ssrc);
};
```

#### gRPC服务实现

```cpp
// include/rpc/service_impl.h
#pragma once

#include <grpcpp/grpcpp.h>
#include "hikvision.grpc.pb.h"

class HikvisionServiceImpl final 
    : public hikvision::HikvisionService::Service {
public:
    HikvisionServiceImpl();
    ~HikvisionServiceImpl() override;
    
    // 设备管理
    grpc::Status Login(
        grpc::ServerContext* context,
        const hikvision::DeviceLoginRequest* request,
        hikvision::DeviceLoginResponse* response) override;
    
    grpc::Status Logout(
        grpc::ServerContext* context,
        const hikvision::LogoutRequest* request,
        hikvision::LogoutResponse* response) override;
    
    grpc::Status GetDeviceInfo(
        grpc::ServerContext* context,
        const hikvision::DeviceInfoRequest* request,
        hikvision::DeviceInfoResponse* response) override;
    
    // 实时预览
    grpc::Status StartPreview(
        grpc::ServerContext* context,
        const hikvision::PreviewRequest* request,
        hikvision::PreviewResponse* response) override;
    
    grpc::Status StopPreview(
        grpc::ServerContext* context,
        const hikvision::StopPreviewRequest* request,
        hikvision::StopPreviewResponse* response) override;
    
    // 云台控制
    grpc::Status PTZControl(
        grpc::ServerContext* context,
        const hikvision::PTZRequest* request,
        hikvision::PTZResponse* response) override;
    
    grpc::Status PTZPreset(
        grpc::ServerContext* context,
        const hikvision::PTZPresetRequest* request,
        hikvision::PTZPresetResponse* response) override;
    
    // 录像回放
    grpc::Status QueryRecord(
        grpc::ServerContext* context,
        const hikvision::RecordQueryRequest* request,
        hikvision::RecordQueryResponse* response) override;
    
    grpc::Status StartPlayback(
        grpc::ServerContext* context,
        const hikvision::PlaybackRequest* request,
        hikvision::PlaybackResponse* response) override;
    
    grpc::Status StopPlayback(
        grpc::ServerContext* context,
        const hikvision::StopPlaybackRequest* request,
        hikvision::StopPlaybackResponse* response) override;
    
    grpc::Status PlaybackControl(
        grpc::ServerContext* context,
        const hikvision::PlaybackControlRequest* request,
        hikvision::PlaybackControlResponse* response) override;
    
    // ISUP Ehome
    grpc::Status RegisterISUPDevice(
        grpc::ServerContext* context,
        const hikvision::RegisterISUPDeviceRequest* request,
        hikvision::RegisterISUPDeviceResponse* response) override;
    
    grpc::Status UnregisterISUPDevice(
        grpc::ServerContext* context,
        const hikvision::UnregisterISUPDeviceRequest* request,
        hikvision::UnregisterISUPDeviceResponse* response) override;
    
    // 服务状态
    grpc::Status GetServiceStatus(
        grpc::ServerContext* context,
        const hikvision::ServiceStatusRequest* request,
        hikvision::ServiceStatusResponse* response) override;

private:
    std::chrono::steady_clock::time_point start_time_;
};
```

### 4.3 Go网关层设计

```go
// internal/haikang/gateway.go
package haikang

import (
    "context"
    "fmt"
    "sync"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    pb "lalmax-nvr/proto/haikang"
)

// Gateway 海康SDK服务网关
type Gateway struct {
    addr   string
    conn   *grpc.ClientConn
    client pb.HikvisionServiceClient
    mu     sync.RWMutex
}

// NewGateway 创建新的网关实例
func NewGateway(addr string) (*Gateway, error) {
    g := &Gateway{
        addr: addr,
    }
    
    if err := g.connect(); err != nil {
        return nil, fmt.Errorf("failed to connect to hikvision service: %w", err)
    }
    
    return g, nil
}

func (g *Gateway) connect() error {
    var err error
    g.conn, err = grpc.Dial(
        g.addr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithBlock(),
        grpc.WithTimeout(5*time.Second),
    )
    if err != nil {
        return err
    }
    
    g.client = pb.NewHikvisionServiceClient(g.conn)
    return nil
}

// Close 关闭连接
func (g *Gateway) Close() error {
    if g.conn != nil {
        return g.conn.Close()
    }
    return nil
}

// Login 设备登录
func (g *Gateway) Login(ctx context.Context, req *DeviceLoginRequest) (*DeviceLoginResponse, error) {
    g.mu.RLock()
    defer g.mu.RUnlock()
    
    resp, err := g.client.Login(ctx, &pb.DeviceLoginRequest{
        Ip:       req.IP,
        Port:     int32(req.Port),
        Username: req.Username,
        Password: req.Password,
        Protocol: pb.DeviceProtocol(req.Protocol),
        Timeout:  int32(req.Timeout),
    })
    if err != nil {
        return nil, err
    }
    
    return &DeviceLoginResponse{
        Success:      resp.Success,
        UserID:       int(resp.UserId),
        ErrorMessage: resp.ErrorMessage,
        ErrorCode:    int(resp.ErrorCode),
        DeviceInfo:   convertDeviceInfo(resp.DeviceInfo),
    }, nil
}

// Logout 设备登出
func (g *Gateway) Logout(ctx context.Context, userID int) error {
    g.mu.RLock()
    defer g.mu.RUnlock()
    
    _, err := g.client.Logout(ctx, &pb.LogoutRequest{
        UserId: int32(userID),
    })
    return err
}

// StartPreview 启动实时预览
func (g *Gateway) StartPreview(ctx context.Context, req *PreviewRequest) (*PreviewResponse, error) {
    g.mu.RLock()
    defer g.mu.RUnlock()
    
    resp, err := g.client.StartPreview(ctx, &pb.PreviewRequest{
        UserId:     int32(req.UserID),
        Channel:    int32(req.Channel),
        StreamType: pb.StreamType(req.StreamType),
        OutputUrl:  req.OutputURL,
        Transport:  pb.TransportProtocol(req.Transport),
    })
    if err != nil {
        return nil, err
    }
    
    return &PreviewResponse{
        Success:       resp.Success,
        PreviewHandle: int(resp.PreviewHandle),
        StreamURL:     resp.StreamUrl,
        ErrorMessage:  resp.ErrorMessage,
    }, nil
}

// StopPreview 停止实时预览
func (g *Gateway) StopPreview(ctx context.Context, handle int) error {
    g.mu.RLock()
    defer g.mu.RUnlock()
    
    _, err := g.client.StopPreview(ctx, &pb.StopPreviewRequest{
        PreviewHandle: int32(handle),
    })
    return err
}

// PTZControl 云台控制
func (g *Gateway) PTZControl(ctx context.Context, req *PTZRequest) error {
    g.mu.RLock()
    defer g.mu.RUnlock()
    
    _, err := g.client.PTZControl(ctx, &pb.PTZRequest{
        UserId:  int32(req.UserID),
        Channel: int32(req.Channel),
        Command: pb.PTZCommand(req.Command),
        Speed:   int32(req.Speed),
        Stop:    req.Stop,
    })
    return err
}

// QueryRecord 查询录像
func (g *Gateway) QueryRecord(ctx context.Context, req *RecordQueryRequest) (*RecordQueryResponse, error) {
    g.mu.RLock()
    defer g.mu.RUnlock()
    
    resp, err := g.client.QueryRecord(ctx, &pb.RecordQueryRequest{
        UserId:    int32(req.UserID),
        Channel:   int32(req.Channel),
        FileType:  int32(req.FileType),
        StartTime: req.StartTime,
        EndTime:   req.EndTime,
    })
    if err != nil {
        return nil, err
    }
    
    files := make([]RecordFile, len(resp.Files))
    for i, f := range resp.Files {
        files[i] = RecordFile{
            FileName:  f.FileName,
            StartTime: f.StartTime,
            EndTime:   f.EndTime,
            FileSize:  f.FileSize,
        }
    }
    
    return &RecordQueryResponse{
        Success:      resp.Success,
        Files:        files,
        ErrorMessage: resp.ErrorMessage,
    }, nil
}

// GetServiceStatus 获取服务状态
func (g *Gateway) GetServiceStatus(ctx context.Context) (*ServiceStatusResponse, error) {
    g.mu.RLock()
    defer g.mu.RUnlock()
    
    resp, err := g.client.GetServiceStatus(ctx, &pb.ServiceStatusRequest{})
    if err != nil {
        return nil, err
    }
    
    return &ServiceStatusResponse{
        Running:        resp.Running,
        DeviceCount:    int(resp.DeviceCount),
        StreamCount:    int(resp.StreamCount),
        SDKVersion:     resp.SdkVersion,
        UptimeSeconds:  resp.UptimeSeconds,
    }, nil
}

// 数据类型定义

type DeviceLoginRequest struct {
    IP       string
    Port     int
    Username string
    Password string
    Protocol int
    Timeout  int
}

type DeviceLoginResponse struct {
    Success      bool
    UserID       int
    ErrorMessage string
    ErrorCode    int
    DeviceInfo   *DeviceInfo
}

type DeviceInfo struct {
    UserID             int
    DeviceName         string
    SerialNumber       string
    DeviceType         string
    ChannelCount       int
    AnalogChannelCount int
    IPChannelCount     int
    SoftwareVersion    string
    HardwareVersion    string
}

type PreviewRequest struct {
    UserID     int
    Channel    int
    StreamType int
    OutputURL  string
    Transport  int
}

type PreviewResponse struct {
    Success       bool
    PreviewHandle int
    StreamURL     string
    ErrorMessage  string
}

type PTZRequest struct {
    UserID  int
    Channel int
    Command int
    Speed   int
    Stop    bool
}

type RecordQueryRequest struct {
    UserID    int
    Channel   int
    FileType  int
    StartTime string
    EndTime   string
}

type RecordQueryResponse struct {
    Success      bool
    Files        []RecordFile
    ErrorMessage string
}

type RecordFile struct {
    FileName  string
    StartTime string
    EndTime   string
    FileSize  int64
}

type ServiceStatusResponse struct {
    Running       bool
    DeviceCount   int
    StreamCount   int
    SDKVersion    string
    UptimeSeconds int64
}

// 辅助函数

func convertDeviceInfo(info *pb.DeviceInfo) *DeviceInfo {
    if info == nil {
        return nil
    }
    return &DeviceInfo{
        UserID:             int(info.UserId),
        DeviceName:         info.DeviceName,
        SerialNumber:       info.SerialNumber,
        DeviceType:         info.DeviceType,
        ChannelCount:       int(info.ChannelCount),
        AnalogChannelCount: int(info.AnalogChannelCount),
        IPChannelCount:     int(info.IpChannelCount),
        SoftwareVersion:    info.SoftwareVersion,
        HardwareVersion:    info.HardwareVersion,
    }
}
```

### 4.4 摄像头管理器扩展

```go
// internal/camera/manager.go 扩展部分

// createRecorder 扩展支持海康SDK协议
func (cm *CameraManager) createRecorder(cam config.CameraConfig, segDur time.Duration) model.Recorder {
    switch cam.Protocol {
    case "hikvision-sdk":
        return cm.createHikvisionRecorder(cam, segDur)
    case "hikvision-isup":
        return cm.createHikvisionISUPRecorder(cam, segDur)
    // ... 其他协议保持不变
    default:
        return nil
    }
}

// createHikvisionRecorder 创建海康SDK录制器
func (cm *CameraManager) createHikvisionRecorder(cam config.CameraConfig, segDur time.Duration) model.Recorder {
    gateway := cm.getHikvisionGateway()
    if gateway == nil {
        logger.Error("hikvision gateway not available", "camera_id", cam.ID)
        return nil
    }
    
    ctx := context.Background()
    
    // 登录设备
    loginResp, err := gateway.Login(ctx, &haikang.DeviceLoginRequest{
        IP:       cam.URL,
        Port:     cam.Port,
        Username: cam.Username,
        Password: cam.Password,
        Protocol: 0, // HCNET
        Timeout:  10,
    })
    if err != nil {
        logger.Error("hikvision login failed", "camera_id", cam.ID, "error", err)
        return nil
    }
    
    if !loginResp.Success {
        logger.Error("hikvision login failed", 
            "camera_id", cam.ID, 
            "error", loginResp.ErrorMessage,
            "code", loginResp.ErrorCode)
        return nil
    }
    
    // 构建RTSP输出URL
    streamID := fmt.Sprintf("hik_%s", cam.ID)
    outputURL := cm.mediaEngine.BuildLocalRTSPURL(streamID)
    
    // 启动预览
    previewResp, err := gateway.StartPreview(ctx, &haikang.PreviewRequest{
        UserID:     loginResp.UserID,
        Channel:    cam.Channel,
        StreamType: 0, // 主码流
        OutputURL:  outputURL,
        Transport:  0, // TCP
    })
    if err != nil {
        logger.Error("hikvision start preview failed", "camera_id", cam.ID, "error", err)
        gateway.Logout(ctx, loginResp.UserID)
        return nil
    }
    
    if !previewResp.Success {
        logger.Error("hikvision start preview failed",
            "camera_id", cam.ID,
            "error", previewResp.ErrorMessage)
        gateway.Logout(ctx, loginResp.UserID)
        return nil
    }
    
    // 使用标准RTSP录制器
    h264Cfg := recorder.H264Config{
        CameraID:   cam.ID,
        RTSPURL:    previewResp.StreamURL,
        SegmentDur: segDur,
        DB:         cm.db,
        EventBus:   cm.eventBus,
    }
    
    rec := recorder.NewH264Recorder(h264Cfg, cm.store, cm.metrics)
    
    // 保存会话信息用于清理
    cm.hikSessions[cam.ID] = &hikSession{
        userID:        loginResp.UserID,
        previewHandle: previewResp.PreviewHandle,
        gateway:       gateway,
    }
    
    return rec
}

// createHikvisionISUPRecorder 创建ISUP录制器
func (cm *CameraManager) createHikvisionISUPRecorder(cam config.CameraConfig, segDur time.Duration) model.Recorder {
    gateway := cm.getHikvisionGateway()
    if gateway == nil {
        logger.Error("hikvision gateway not available", "camera_id", cam.ID)
        return nil
    }
    
    ctx := context.Background()
    
    // 注册ISUP设备
    resp, err := gateway.RegisterISUPDevice(ctx, &haikang.RegisterISUPDeviceRequest{
        DeviceSerial: cam.SerialNumber,
        VerifyCode:   cam.VerifyCode,
    })
    if err != nil {
        logger.Error("hikvision isup register failed", "camera_id", cam.ID, "error", err)
        return nil
    }
    
    if !resp.Success {
        logger.Error("hikvision isup register failed",
            "camera_id", cam.ID,
            "error", resp.ErrorMessage)
        return nil
    }
    
    // ISUP设备注册后，SDK会自动开始推流
    // 使用生成的流地址创建录制器
    streamID := fmt.Sprintf("isup_%s", cam.ID)
    streamURL := cm.mediaEngine.BuildLocalRTSPURL(streamID)
    
    h264Cfg := recorder.H264Config{
        CameraID:   cam.ID,
        RTSPURL:    streamURL,
        SegmentDur: segDur,
        DB:         cm.db,
        EventBus:   cm.eventBus,
    }
    
    return recorder.NewH264Recorder(h264Cfg, cm.store, cm.metrics)
}

// hikSession 海康SDK会话
type hikSession struct {
    userID        int
    previewHandle int
    gateway       *haikang.Gateway
}

// getHikvisionGateway 获取海康SDK网关
func (cm *CameraManager) getHikvisionGateway() *haikang.Gateway {
    if cm.hikGateway != nil {
        return cm.hikGateway
    }
    
    // 从配置创建网关
    addr := cm.cfg.Hikvision.SDKService.Address
    if addr == "" {
        addr = "unix:///var/run/hikvision-sdk.sock"
    }
    
    gateway, err := haikang.NewGateway(addr)
    if err != nil {
        logger.Error("failed to create hikvision gateway", "error", err)
        return nil
    }
    
    cm.hikGateway = gateway
    return gateway
}
```

## 5. 配置文件设计

### 5.1 海康SDK服务配置

```yaml
# hikvision-sdk-service/config.yaml

# 服务配置
server:
  # gRPC服务监听地址
  grpc_address: "0.0.0.0:50051"
  # Unix Socket路径 (优先级高于TCP)
  unix_socket: "/var/run/hikvision-sdk.sock"
  # 日志级别: debug, info, warn, error
  log_level: "info"
  # 日志文件路径
  log_file: "/var/log/hikvision-sdk/service.log"

# 海康SDK配置
sdk:
  # SDK库文件路径
  lib_path: "/opt/hikvision-sdk/lib"
  # 连接超时(毫秒)
  connect_timeout: 5000
  # 重连间隔(秒)
  reconnect_interval: 30
  # 心跳间隔(秒)
  heartbeat_interval: 30
  # 最大连接数
  max_connections: 100

# 流媒体配置
stream:
  # RTP打包大小(字节)
  rtp_packet_size: 1400
  # 发送缓冲区大小
  send_buffer_size: 1048576
  # 接收缓冲区大小
  recv_buffer_size: 1048576
  # 默认传输协议: tcp, udp
  default_transport: "tcp"

# ISUP Ehome配置
isup:
  # 是否启用ISUP
  enabled: false
  # CMS服务器配置
  cms:
    ip: "0.0.0.0"
    port: 7660
    listen_ip: "0.0.0.0"
    listen_port: 7660
  # DAS服务器配置
  das:
    ip: "0.0.0.0"
    port: 7700
  # SMS服务器配置
  sms:
    ip: "0.0.0.0"
    port: 7701
    listen_ip: "0.0.0.0"
    listen_port: 7701
  # 报警服务器配置
  alarm:
    ip: "0.0.0.0"
    tcp_port: 7702
    udp_port: 7702
  # ISUP密钥
  key: "your_isup_key_here"
```

### 5.2 lalmax-nvr 配置扩展

```yaml
# lalmax-nvr/config.yaml

# ... 现有配置 ...

# 海康SDK集成配置
hikvision:
  # 是否启用海康SDK支持
  enabled: true
  
  # SDK服务连接配置
  sdk_service:
    # 服务地址 (Unix Socket或TCP)
    address: "unix:///var/run/hikvision-sdk.sock"
    # 连接超时(秒)
    connect_timeout: 5
    # 是否自动重连
    auto_reconnect: true
    # 重连间隔(秒)
    reconnect_interval: 30
  
  # 默认设备参数
  defaults:
    # 默认协议: hcnet, isup
    protocol: "hcnet"
    # 默认码流: 0-主码流, 1-子码流
    stream_type: 0
    # 默认传输协议: 0-TCP, 1-UDP
    transport: 0
    # 连接超时(秒)
    connect_timeout: 10
  
  # 海康设备列表
  devices:
    - id: "hk_001"
      name: "前门摄像头"
      ip: "192.168.1.100"
      port: 8000
      username: "admin"
      password: "12345"
      protocol: "hcnet"
      channel: 1
      stream_type: 0
      encoding: "h264"
      enabled: true
    
    - id: "hk_002"
      name: "后门摄像头"
      ip: "192.168.1.101"
      port: 8000
      username: "admin"
      password: "12345"
      protocol: "isup"
      serial_number: "DS-2CD2T47WD-IS20230101AACH123456789"
      verify_code: "ABCDEF"
      channel: 1
      encoding: "h264"
      enabled: true
```

## 6. 部署方案

### 6.1 同机部署

```bash
#!/bin/bash
# start.sh

# 启动海康SDK服务
echo "Starting Hikvision SDK Service..."
cd /opt/hikvision-sdk-service
./hikvision-sdk-service --config config.yaml &
HIK_PID=$!
echo "Hikvision SDK Service started with PID: $HIK_PID"

# 等待服务启动
sleep 2

# 启动lalmax-nvr
echo "Starting lalmax-nvr..."
cd /opt/lalmax-nvr
./lalmax-nvr --config config.yaml &
NVR_PID=$!
echo "lalmax-nvr started with PID: $NVR_PID"

# 等待任意子进程退出
wait $HIK_PID $NVR_PID
```

### 6.2 Systemd服务配置

```ini
# /etc/systemd/system/hikvision-sdk.service
[Unit]
Description=Hikvision SDK Service
After=network.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/opt/hikvision-sdk-service
ExecStart=/opt/hikvision-sdk-service/hikvision-sdk-service --config /etc/hikvision-sdk/config.yaml
ExecStop=/bin/kill -TERM $MAINPID
Restart=always
RestartSec=5

# 环境变量
Environment=LD_LIBRARY_PATH=/opt/hikvision-sdk/lib

# 资源限制
LimitNOFILE=65536
LimitNPROC=65536

[Install]
WantedBy=multi-user.target
```

```ini
# /etc/systemd/system/lalmax-nvr.service
[Unit]
Description=lalmax NVR
After=network.target hikvision-sdk.service
Requires=hikvision-sdk.service

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/opt/lalmax-nvr
ExecStart=/opt/lalmax-nvr/lalmax-nvr --config /etc/lalmax-nvr/config.yaml
ExecStop=/bin/kill -TERM $MAINPID
Restart=always
RestartSec=5

# 资源限制
LimitNOFILE=65536
LimitNPROC=65536

[Install]
WantedBy=multi-user.target
```

### 6.3 Docker部署

```dockerfile
# hikvision-sdk-service/docker/Dockerfile
FROM ubuntu:20.04

# 安装依赖
RUN apt-get update && apt-get install -y \
    libssl-dev \
    libgrpc-dev \
    libprotobuf-dev \
    protobuf-compiler-grpc \
    && rm -rf /var/lib/apt/lists/*

# 创建目录
RUN mkdir -p /opt/hikvision-sdk-service

# 复制文件
COPY build/hikvision-sdk-service /opt/hikvision-sdk-service/
COPY lib/ /opt/hikvision-sdk-service/lib/
COPY config/config.yaml /etc/hikvision-sdk/config.yaml

# 设置环境变量
ENV LD_LIBRARY_PATH=/opt/hikvision-sdk-service/lib

# 暴露端口
EXPOSE 50051

# 启动服务
WORKDIR /opt/hikvision-sdk-service
CMD ["./hikvision-sdk-service", "--config", "/etc/hikvision-sdk/config.yaml"]
```

```yaml
# docker-compose.yml
version: '3.8'

services:
  hikvision-sdk:
    build:
      context: ./hikvision-sdk-service
      dockerfile: docker/Dockerfile
    container_name: hikvision-sdk
    restart: always
    volumes:
      # 挂载配置文件
      - ./config/hikvision:/etc/hikvision-sdk
      # 挂载Unix Socket
      - /var/run:/var/run
      # 如果需要访问物理设备
      - /dev:/dev
    network_mode: host
    privileged: true
    environment:
      - LD_LIBRARY_PATH=/opt/hikvision-sdk-service/lib

  lalmax-nvr:
    build:
      context: ./lalmax-nvr
    container_name: lalmax-nvr
    restart: always
    depends_on:
      - hikvision-sdk
    volumes:
      # 挂载配置文件
      - ./config/lalmax:/etc/lalmax-nvr
      # 挂载录像目录
      - ./recordings:/recordings
      # 挂载Unix Socket
      - /var/run:/var/run
    ports:
      # Web UI
      - "9090:9090"
      # RTSP
      - "554:554"
      # RTMP
      - "1935:1935"
    environment:
      - HIKVISION_SDK_ADDRESS=unix:///var/run/hikvision-sdk.sock
```

### 6.4 Kubernetes部署

```yaml
# k8s/hikvision-sdk-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hikvision-sdk
  labels:
    app: hikvision-sdk
spec:
  replicas: 1
  selector:
    matchLabels:
      app: hikvision-sdk
  template:
    metadata:
      labels:
        app: hikvision-sdk
    spec:
      containers:
        - name: hikvision-sdk
          image: hikvision-sdk:latest
          ports:
            - containerPort: 50051
              name: grpc
          volumeMounts:
            - name: config
              mountPath: /etc/hikvision-sdk
            - name: socket
              mountPath: /var/run
          env:
            - name: LD_LIBRARY_PATH
              value: /opt/hikvision-sdk-service/lib
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
      volumes:
        - name: config
          configMap:
            name: hikvision-sdk-config
        - name: socket
          hostPath:
            path: /var/run
            type: DirectoryOrCreate
---
apiVersion: v1
kind: Service
metadata:
  name: hikvision-sdk
spec:
  selector:
    app: hikvision-sdk
  ports:
    - port: 50051
      targetPort: 50051
      name: grpc
  type: ClusterIP
```

## 7. 编译和构建

### 7.1 C++服务构建

```cmake
# CMakeLists.txt
cmake_minimum_required(VERSION 3.16)
project(hikvision-sdk-service VERSION 1.0.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# 选项
option(BUILD_TESTS "Build tests" OFF)
option(BUILD_SHARED_LIBS "Build shared libraries" OFF)

# 查找依赖
find_package(Threads REQUIRED)
find_package(Protobuf REQUIRED)
find_package(gRPC REQUIRED)

# 海康SDK路径
set(HIKVISION_SDK_DIR "${CMAKE_SOURCE_DIR}/lib/linux64" CACHE PATH "Hikvision SDK directory")
set(HIKVISION_INCLUDE_DIR "${CMAKE_SOURCE_DIR}/include" CACHE PATH "Hikvision SDK include directory")

# 包含目录
include_directories(
    ${CMAKE_SOURCE_DIR}/include
    ${HIKVISION_INCLUDE_DIR}
    ${CMAKE_BINARY_DIR}/proto
)

# 生成protobuf和grpc代码
set(PROTO_FILES proto/hikvision.proto)
protobuf_generate_cpp(PROTO_SRCS PROTO_HDRS ${PROTO_FILES})
grpc_generate_cpp(GRPC_SRCS GRPC_HDRS ${PROTO_FILES})

# 源文件
set(SOURCES
    src/main.cpp
    src/core/sdk_manager.cpp
    src/core/device_manager.cpp
    src/core/stream_manager.cpp
    src/hcnet/hcnet_client.cpp
    src/hcnet/hcnet_callback.cpp
    src/hcnet/hcnet_ptz.cpp
    src/hcnet/hcnet_playback.cpp
    src/isup/isup_server.cpp
    src/isup/isup_device.cpp
    src/isup/isup_stream.cpp
    src/protocol/rtp_muxer.cpp
    src/protocol/rtsp_pusher.cpp
    src/protocol/rtmp_pusher.cpp
    src/rpc/grpc_server.cpp
    src/rpc/service_impl.cpp
)

# 添加可执行文件
add_executable(${PROJECT_NAME}
    ${SOURCES}
    ${PROTO_SRCS}
    ${PROTO_HDRS}
    ${GRPC_SRCS}
    ${GRPC_HDRS}
)

# 链接库
target_link_libraries(${PROJECT_NAME}
    PRIVATE
    Threads::Threads
    protobuf::libprotobuf
    gRPC::grpc++
    ${HIKVISION_SDK_DIR}/libhcnetsdk.so
    ${HIKVISION_SDK_DIR}/libhpr.so
    ${HIKVISION_SDK_DIR}/libHCCore.so
    ${HIKVISION_SDK_DIR}/libPlayCtrl.so
)

# 安装
install(TARGETS ${PROJECT_NAME} RUNTIME DESTINATION bin)
install(DIRECTORY lib/ DESTINATION lib)
install(DIRECTORY config/ DESTINATION etc)

# 测试
if(BUILD_TESTS)
    enable_testing()
    add_subdirectory(tests)
endif()
```

```bash
#!/bin/bash
# scripts/build.sh

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}Building Hikvision SDK Service...${NC}"

# 创建构建目录
mkdir -p build
cd build

# 配置CMake
echo -e "${YELLOW}Configuring CMake...${NC}"
cmake .. \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_TESTS=ON \
    -DCMAKE_INSTALL_PREFIX=/opt/hikvision-sdk-service

# 编译
echo -e "${YELLOW}Building...${NC}"
make -j$(nproc)

# 运行测试
echo -e "${YELLOW}Running tests...${NC}"
ctest --output-on-failure

# 安装
echo -e "${YELLOW}Installing...${NC}"
sudo make install

echo -e "${GREEN}Build completed successfully!${NC}"
```

### 7.2 Go网关构建

```bash
#!/bin/bash
# build_gateway.sh

set -e

echo "Building lalmax-nvr with Hikvision support..."

# 生成protobuf代码
echo "Generating protobuf code..."
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/hikvision.proto

# 编译
echo "Building lalmax-nvr..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o bin/lalmax-nvr \
    cmd/lalmax-nvr/main.go

echo "Build completed successfully!"
```

## 8. 测试方案

### 8.1 单元测试

```cpp
// tests/unit/test_rtp_muxer.cpp
#include <gtest/gtest.h>
#include "protocol/rtp_muxer.h"

class RtpMuxerTest : public ::testing::Test {
protected:
    void SetUp() override {
        muxer = std::make_unique<RtpMuxer>(
            "rtsp://localhost:8554/test",
            RtpMuxer::Transport::TCP
        );
    }
    
    std::unique_ptr<RtpMuxer> muxer;
};

TEST_F(RtpMuxerTest, OpenClose) {
    // 注意: 需要实际的RTSP服务器才能测试
    // 这里只测试基本的状态检查
    EXPECT_FALSE(muxer->IsOpen());
}

TEST_F(RtpMuxerTest, InvalidURL) {
    auto invalid_muxer = std::make_unique<RtpMuxer>(
        "invalid://url",
        RtpMuxer::Transport::TCP
    );
    EXPECT_FALSE(invalid_muxer->Open());
}
```

```go
// internal/haikang/gateway_test.go
package haikang

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

type MockHikvisionServiceClient struct {
    mock.Mock
}

func (m *MockHikvisionServiceClient) Login(ctx context.Context, req *pb.DeviceLoginRequest) (*pb.DeviceLoginResponse, error) {
    args := m.Called(ctx, req)
    return args.Get(0).(*pb.DeviceLoginResponse), args.Error(1)
}

func TestGateway_Login(t *testing.T) {
    mockClient := new(MockHikvisionServiceClient)
    
    expectedResp := &pb.DeviceLoginResponse{
        Success: true,
        UserId:  1001,
    }
    
    mockClient.On("Login", mock.Anything, mock.Anything).Return(expectedResp, nil)
    
    gateway := &Gateway{
        client: mockClient,
    }
    
    resp, err := gateway.Login(context.Background(), &DeviceLoginRequest{
        IP:       "192.168.1.100",
        Port:     8000,
        Username: "admin",
        Password: "12345",
    })
    
    assert.NoError(t, err)
    assert.True(t, resp.Success)
    assert.Equal(t, 1001, resp.UserID)
}
```

### 8.2 集成测试

```bash
#!/bin/bash
# tests/integration/test_integration.sh

set -e

echo "Running integration tests..."

# 启动海康SDK服务
echo "Starting Hikvision SDK Service..."
./hikvision-sdk-service --config test_config.yaml &
SDK_PID=$!
sleep 2

# 运行集成测试
echo "Running tests..."
go test -v -tags=integration ./tests/integration/...

# 清理
kill $SDK_PID 2>/dev/null || true

echo "Integration tests completed!"
```

### 8.3 性能测试

```go
// tests/performance/benchmark_test.go
package performance

import (
    "testing"
    "time"
    
    "lalmax-nvr/internal/haikang"
)

func BenchmarkGateway_Login(b *testing.B) {
    gateway, err := haikang.NewGateway("localhost:50051")
    if err != nil {
        b.Fatal(err)
    }
    defer gateway.Close()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = gateway.Login(context.Background(), &haikang.DeviceLoginRequest{
            IP:       "192.168.1.100",
            Port:     8000,
            Username: "admin",
            Password: "12345",
        })
    }
}

func BenchmarkGateway_PTZControl(b *testing.B) {
    gateway, err := haikang.NewGateway("localhost:50051")
    if err != nil {
        b.Fatal(err)
    }
    defer gateway.Close()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = gateway.PTZControl(context.Background(), &haikang.PTZRequest{
            UserID:  1001,
            Channel: 1,
            Command: 0, // UP
            Speed:   3,
            Stop:    false,
        })
    }
}
```

## 9. 故障排查

### 9.1 常见问题

| 问题 | 可能原因 | 解决方案 |
|------|---------|---------|
| SDK初始化失败 | 库文件路径错误 | 检查LD_LIBRARY_PATH和lib_path配置 |
| 设备登录失败 | 用户名密码错误 | 检查设备凭证 |
| 设备登录失败 | 网络不通 | 检查网络连通性 |
| 预览无画面 | 通道号错误 | 确认设备通道号 |
| 预览无画面 | 码流类型不支持 | 尝试切换主/子码流 |
| gRPC连接失败 | 服务未启动 | 检查hikvision-sdk-service状态 |
| gRPC连接失败 | Socket权限问题 | 检查/var/run目录权限 |

### 9.2 日志分析

```bash
# 查看海康SDK服务日志
tail -f /var/log/hikvision-sdk/service.log

# 查看lalmax-nvr日志
tail -f /var/log/lalmax-nvr/nvr.log

# 查看系统日志
journalctl -u hikvision-sdk.service -f
journalctl -u lalmax-nvr.service -f
```

### 9.3 调试模式

```yaml
# 开启调试模式
server:
  log_level: "debug"

sdk:
  # 开启SDK调试日志
  debug: true
  # 保存原始数据包
  save_packets: true
  packet_dir: "/tmp/hikvision-packets"
```

## 10. 安全考虑

### 10.1 认证和授权

```yaml
# 启用gRPC认证
server:
  tls:
    enabled: true
    cert_file: "/etc/hikvision-sdk/certs/server.crt"
    key_file: "/etc/hikvision-sdk/certs/server.key"
    ca_file: "/etc/hikvision-sdk/certs/ca.crt"
  
  auth:
    enabled: true
    # API密钥
    api_key: "your_api_key_here"
```

### 10.2 网络安全

- 使用Unix Socket进行本地通信
- 如果使用TCP，启用TLS加密
- 限制访问IP白名单
- 定期更新SDK版本

### 10.3 密码安全

```yaml
# 使用环境变量存储敏感信息
devices:
  - id: "hk_001"
    ip: "192.168.1.100"
    username: "${HIK_USERNAME}"
    password: "${HIK_PASSWORD}"
```

## 11. 性能优化

### 11.1 连接池

```cpp
// 使用连接池管理设备连接
class ConnectionPool {
public:
    ConnectionPool(size_t max_size) : max_size_(max_size) {}
    
    std::shared_ptr<Connection> Acquire(const std::string& ip, int port);
    void Release(std::shared_ptr<Connection> conn);
    
private:
    size_t max_size_;
    std::unordered_map<std::string, std::queue<std::shared_ptr<Connection>>> pool_;
    std::mutex mutex_;
};
```

### 11.2 零拷贝传输

```cpp
// 使用零拷贝技术减少数据复制
class ZeroCopyBuffer {
public:
    ZeroCopyBuffer(size_t size);
    
    uint8_t* data() { return data_; }
    size_t size() const { return size_; }
    
    // 转移所有权
    std::unique_ptr<ZeroCopyBuffer> Slice(size_t offset, size_t length);
    
private:
    uint8_t* data_;
    size_t size_;
    std::shared_ptr<ZeroCopyBuffer> parent_;
};
```

### 11.3 异步处理

```cpp
// 使用异步IO提高吞吐量
class AsyncStreamProcessor {
public:
    using Callback = std::function<void(const uint8_t* data, size_t size)>;
    
    void Start(int handle, Callback callback);
    void Stop(int handle);
    
private:
    std::unordered_map<int, std::thread> workers_;
    std::unordered_map<int, moodycamel::ConcurrentQueue<Frame>> queues_;
};
```

## 12. 未来扩展

### 12.1 计划功能

- [ ] 支持大华设备SDK
- [ ] 支持宇视设备SDK
- [ ] 支持ONVIF Profile T
- [ ] 支持WebRTC输出
- [ ] 支持AI智能分析
- [ ] 支持边缘存储

### 12.2 API版本管理

```
/api/v1/devices
/api/v1/streams
/api/v1/playback

/api/v2/devices
/api/v2/streams
```

### 12.3 插件化架构

```cpp
// 插件接口
class IDevicePlugin {
public:
    virtual ~IDevicePlugin() = default;
    
    virtual std::string GetName() const = 0;
    virtual std::string GetVersion() const = 0;
    
    virtual bool Initialize(const PluginConfig& config) = 0;
    virtual void Cleanup() = 0;
    
    virtual int Login(const DeviceCredentials& creds) = 0;
    virtual bool Logout(int user_id) = 0;
    
    virtual int StartPreview(int user_id, int channel, 
                            const StreamConfig& config) = 0;
    virtual bool StopPreview(int handle) = 0;
};

// 插件管理器
class PluginManager {
public:
    void RegisterPlugin(std::unique_ptr<IDevicePlugin> plugin);
    IDevicePlugin* GetPlugin(const std::string& name);
    
private:
    std::unordered_map<std::string, std::unique_ptr<IDevicePlugin>> plugins_;
};
```

## 附录

### A. 参考资料

- [海康威视开放平台](https://open.hikvision.com/)
- [海康设备网络SDK开发手册](https://www.hikvision.com/cn/)
- [ISUP Ehome协议文档](https://www.hikvision.com/cn/)
- [gRPC官方文档](https://grpc.io/docs/)
- [lalmax项目](https://github.com/q191201771/lalmax)

### B. 版本历史

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0.0 | 2024-01-01 | 初始版本 |

### C. 许可证

MIT License

### D. 联系方式

- 项目地址: https://github.com/lalmax-pro/lalmax-nvr
- 问题反馈: https://github.com/lalmax-pro/lalmax-nvr/issues
