# WebDAV Integration

lalmax-nvr provides a built-in WebDAV server for accessing camera recordings and snapshots via standard WebDAV protocol. Supports read-only and read-write modes for easy third-party integration.

## Overview

- **Protocol**: WebDAV (Web-based Distributed Authoring and Versioning)
- **Access**: Standard WebDAV clients
- **Path prefix**: `/dav` (configurable)
- **Modes**: Read-only or read-write
- **Authentication**: HTTP Basic (uses global `auth` config)
- **Compatibility**: Standard WebDAV clients supported

## Configuration

### Basic Setup

```yaml
webdav:
  enabled: true
  path_prefix: "/dav"
  read_write: false
```

> **Authentication**: WebDAV uses the global `auth` config for HTTP Basic authentication — no separate WebDAV-specific credentials needed.
>
> ```yaml
> auth:
>   username: "admin"
>   password: "your_password"
> ```

### Configuration Options

| Field | Required | Type | Default | Description |
|-------|----------|------|---------|-------------|
| `enabled` | No | boolean | true | Enable/disable WebDAV server |
| `path_prefix` | No | string | "/dav" | WebDAV URL prefix |
| `read_write` | No | boolean | false | Allow write operations |

### Full Configuration Example

```yaml
auth:
  username: "admin"
  password: "secure_password"

webdav:
  enabled: true
  path_prefix: "/video"
  read_write: false
```

## Usage

### 访问 WebDAV 资源

WebDAV 服务器可通过 `http://localhost:9090/dav` 访问，文件结构如下：

```
/dav/
├── recordings/
│   ├── h264/
│   │   ├── camera_1/
│   │   │   ├── 1704123456789012345.mp4
│   │   │   └── 1704123456789012346.mp4
│   │   └── camera_2/
│   │       ├── 1704123456789012347.mp4
│   │       └── 1704123456789012348.mp4
│   └── h265/
│       └── camera_1/
│           └── 1704123456789012349.mp4
└── snapshots/
    ├── camera_1/
    │   ├── 1704123456789012350.jpg
    │   └── 1704123456789012351.jpg
    └── camera_2/
        ├── 1704123456789012352.jpg
        └── 1704123456789012353.jpg
```

### WebDAV 操作

#### 只读模式

**基本访问**：
```bash
# 列出根目录
curl -u admin:password http://localhost:9090/dav/

# 列出录像目录
curl -u admin:password http://localhost:9090/dav/recordings/h264/

# 下载录像文件
curl -u admin:password -O http://localhost:9090/dav/recordings/h264/camera_1/1704123456789012345.mp4

# 下载快照
curl -u admin:password -O http://localhost:9090/dav/snapshots/camera_1/1704123456789012350.jpg
```

**文件浏览器**：
- Windows: 打开文件管理器，输入 `http://localhost:9090/dav`
- macOS: 连接到服务器，地址 `http://localhost:9090/dav`
- Linux: 文件管理器 → 连接到服务器 → 地址 `http://localhost:9090/dav`

#### 读写模式

启用读写模式后，可以上传文件和创建目录：

```yaml
webdav:
  enabled: true
  path_prefix: "/dav"
  read_write: true
```

**上传文件**：
```bash
# 创建目录
mkdir -p /tmp/test_upload
echo "test content" > /tmp/test_upload/test.txt

# 上传文件
curl -u upload_user:upload_password -T /tmp/test_upload/test.txt \
  http://localhost:9090/dav/recordings/h264/uploaded_files/

# 上传目录
curl -u upload_user:upload_password -T /tmp/test_upload/ \
  http://localhost:9090/dav/recordings/h264/uploaded_directory/
```

## 客户端集成

### Windows

1. 打开文件资源管理器
2. 在地址栏输入 `http://localhost:9090/dav`
3. 输入用户名和密码
4. 访问录像和快照文件

### macOS

1. 打开"访达"
2. 选择"前往" → "连接服务器"
3. 输入地址 `http://localhost:9090/dav`
4. 输入用户名和密码

### Linux

1. 打开文件管理器（Nautilus, Dolphin, Thunar 等）
2. 选择"连接到服务器"
3. 输入地址 `http://localhost:9090/dav`
4. 输入用户名和密码

### 第三方应用

#### VLC 媒体播放器

```bash
# 直接播放录像
vlc http://localhost:9090/dav/recordings/h264/camera_1/1704123456789012345.mp4

# 访问快照库
vlc http://localhost:9090/dav/snapshots/camera_1/
```

#### PotPlayer

```
# 播放录像
http://localhost:9090/dav/recordings/h264/camera_1/1704123456789012345.mp4

# 浏览快照
http://localhost:9090/dav/snapshots/camera_1/
```

#### Adobe Premiere Pro

1. 文件 → 打开
2. 输入 URL：`http://localhost:9090/dav/recordings/h264/`
3. 选择要导入的录像文件

## 云存储集成

### 网盘备份

使用 rclone 将 WebDAV 资源同步到云存储：

```bash
# 配置 rclone remote
rclone config

# 设置名为 backup 的远程
# 选择 WebDAV
# 地址: http://localhost:9090/dav
# 用户名: admin
# 密码: password
# 重命名为 backup

# 执行同步
rclone sync /mnt/disk/nvr backup:video/recordings/

# 定期备份脚本
#!/bin/bash
rclone sync /var/lib/lalmax-nvr/recordings/ backup:recordings/
rclone sync /var/lib/lalmax-nvr/snapshots/ backup:snapshots/
```

### Windows 备份工具

使用 Robocopy 备份：

```batch
@echo off
set SOURCE=D:\lalmax-nvr\recordings\
set DEST=\\192.168.1.100\dav\recordings\
set USER=admin
set PASS=password

net use %DEST% %PASS% /user:%USER%
robocopy %SOURCE% %DEST% /E /R:2 /W:5
net use %DEST% /delete
```

## 安全配置

### HTTPS 支持

建议通过反向代理启用 HTTPS：

```nginx
# Nginx 配置示例
server {
    listen 443 ssl;
    server_name nvr.example.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location /dav/ {
        proxy_pass http://localhost:9090/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### Network Restrictions

使用防火墙限制访问：

```bash
# 允许特定 IP 访问 WebDAV
iptables -A INPUT -p tcp --dport 9090 -s 192.168.1.0/24 -j ACCEPT
iptables -A INPUT -p tcp --dport 9090 -j DROP

# 或者使用 UFW
ufw allow from 192.168.1.0/24 to any port 9090
ufw deny to any port 9090
```

## 故障排除

### 常见问题

**连接被拒绝**
```
curl: (7) Failed to connect to localhost port 9090: Connection refused
```
**解决方案**：确保 WebDAV 已启用并正在运行

**认证失败**
```
HTTP/1.1 401 Unauthorized
```
**解决方案**：检查用户名和密码是否正确

**权限错误**
```
HTTP/1.1 403 Forbidden
```
**解决方案**：检查读写模式设置，确认目录权限

**超时错误**
```
curl: (28) Operation timed out after 30001 milliseconds
```
**解决方案**：网络问题，检查连接或增加超时时间

### 调试方法

**启用调试日志**
```yaml
observability:
  log_level: "debug"
```

**测试 WebDAV 连接**
```bash
# 测试基本连接
curl -v http://localhost:9090/dav/

# 测试认证
curl -v -u admin:password http://localhost:9090/dav/

# 测试文件上传（仅读写模式）
curl -v -u admin:password -T /etc/hosts http://localhost:9090/dav/test.txt
```

**检查系统日志**
```bash
# 查看详细日志
journalctl -u lalmax-nvr -f | grep webdav

# Docker 容器
docker logs -f lalmax-nvr | grep webdav
```

### 性能优化

#### 大文件上传

```yaml
# 调整服务器配置
server:
  max_upload_size: "1GB"
  read_timeout: "300s"
  write_timeout: "300s"
```

#### 网络优化

```bash
# 调整 TCP 设置（用于大文件传输）
sysctl -w net.core.rmem_max=134217728
sysctl -w net.core.wmem_max=134217728
sysctl -w net.ipv4.tcp_rmem="4096 87380 134217728"
sysctl -w net.ipv4.tcp_wmem="4096 65536 134217728"
```

## 集成示例

### Apple Time Machine

通过 WebDAV 实现 Time Machine 备份：

```bash
# 创建备份任务
defaults write com.apple.TimeMachine DoNotOfferNewDisks false
defaults write com.apple.TimeMachine CustomBackupDisk "http://localhost:9090/dav/time_machine"

# 或使用手动设置
# 打开 Time Machine → 选择磁盘 → 输入 WebDAV 地址
```

### rsync 备份

```bash
# 使用 rsync 备份到 WebDAV
rsync -avz /var/lib/lalmax-nvr/recordings/ \
  admin@localhost:/var/lib/lalmax-nvr/recordings/backup/

# 排除特定文件
rsync -avz --exclude='*.tmp' /var/lib/lalmax-nvr/recordings/ \
  admin@localhost:/var/lib/lalmax-nvr/recordings/backup/
```

### Python 自动化脚本

```python
#!/usr/bin/env python3
import requests
import os
from datetime import datetime, timedelta

def download_snapshots(camera_id, output_dir):
    """下载指定摄像头的快照"""
    base_url = "http://localhost:9090/dav/snapshots"
    auth = ("admin", "password")
    
    url = f"{base_url}/{camera_id}/"
    response = requests.get(url, auth=auth)
    
    if response.status_code == 200:
        # 创建输出目录
        os.makedirs(output_dir, exist_ok=True)
        
        # 这里需要解析 WebDAV 列表响应
        # 实际实现需要处理 WebDAV XML 响应
        print(f"下载摄像头 {camera_id} 的快照到 {output_dir}")

def list_recordings(camera_id=None, encoding=None):
    """列出录像文件"""
    base_url = "http://localhost:9090/dav/recordings"
    auth = ("admin", "password")
    
    if camera_id and encoding:
        url = f"{base_url}/{encoding}/{camera_id}/"
    elif encoding:
        url = f"{base_url}/{encoding}/"
    else:
        url = f"{base_url}/"
    
    response = requests.get(url, auth=auth)
    
    if response.status_code == 200:
        print("可用文件:")
        print(response.text)

if __name__ == "__main__":
    # 示例用法
    list_recordings(camera_id="camera_1", encoding="h264")
    download_snapshots("camera_1", "/tmp/snapshots")
```

### Node.js 脚本

```javascript
const fs = require('fs');
const axios = require('axios');

class WebDAVClient {
  constructor(baseUrl, username, password) {
    this.baseUrl = baseUrl;
    this.auth = {
      username: username,
      password: password
    };
  }

  async listDirectory(path) {
    const url = `${this.baseUrl}${path}`;
    const response = await axios.get(url, {
      auth: this.auth,
      headers: {
        Depth: 1
      }
    });
    return response.data;
  }

  async downloadFile(remotePath, localPath) {
    const url = `${this.baseUrl}${remotePath}`;
    const response = await axios.get(url, {
      auth: this.auth,
      responseType: 'stream'
    });
    
    const writer = fs.createWriteStream(localPath);
    response.data.pipe(writer);
    
    return new Promise((resolve, reject) => {
      writer.on('finish', resolve);
      writer.on('error', reject);
    });
  }

  async uploadFile(localPath, remotePath) {
    const url = `${this.baseUrl}${remotePath}`;
    const fileData = fs.readFileSync(localPath);
    
    await axios.put(url, fileData, {
      auth: this.auth,
      headers: {
        'Content-Type': 'application/octet-stream'
      }
    });
  }
}

// 使用示例
const client = new WebDAVClient('http://localhost:9090/dav', 'admin', 'password');

// 列出目录
client.listDirectory('/recordings/h264/').then(console.log);

// 下载文件
client.downloadFile('/recordings/h264/camera_1/latest.mp4', '/tmp/latest.mp4');

// 上传文件
client.uploadFile('/tmp/upload.mp4', '/recordings/h264/uploaded/video.mp4');
```

## 最佳实践

1. **定期清理**：设置自动清理策略，避免存储空间耗尽
2. **备份策略**：定期将重要录像备份到其他位置
3. **权限控制**：为不同用户设置合适的访问权限
4. **监控**：监控 WebDAV 服务器的使用情况和性能
5. **日志记录**：启用详细日志以便问题诊断
6. **网络安全**：使用 HTTPS 和强密码保护数据安全

### 监控脚本示例

```bash
#!/bin/bash
# 监控 WebDAV 使用情况

WEBDAV_URL="http://localhost:9090/dav"
USERNAME="admin"
PASSWORD="password"

# 获取磁盘使用情况
DISK_USAGE=$(curl -s -u $USERNAME:$PASSWORD $WEBDAV_URL/ | grep -o '[0-9]*%' | head -1)

# 获取文件数量
FILE_COUNT=$(curl -s -u $USERNAME:$PASSWORD $WEBDAV_URL/recordings/ | grep -o '<[^>]*>' | wc -l)

echo "磁盘使用: $DISK_USAGE"
echo "文件数量: $FILE_COUNT"

# 如果使用率超过 90%，发送警告
if [[ ${DISK_USAGE%?} -gt 90 ]]; then
    echo "警告：磁盘使用率超过 90%"
    # 可以发送邮件或通知
fi
```

通过 WebDAV 集成，lalmax-nvr 可以轻松与各种第三方系统集成，实现自动备份、远程访问和文件管理等功能。