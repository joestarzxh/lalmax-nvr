# GB28181 目录查询功能改进设计文档

**日期**: 2026-06-22
**状态**: 已批准
**方案**: 方案 A - 最小改动

---

## 1. 背景

当前 lalmax-nvr 项目的 GB28181 目录查询功能存在以下问题：

| 问题 | 现状 | 风险 |
|------|------|------|
| 内存通道追加 | `dev.Channels.Store` 是追加模式，旧通道不删除 | 设备删除通道后，内存中仍有旧数据 |
| 无单调性保护 | 直接覆盖，无时间戳比较 | 心跳乱序可能导致状态回退 |
| 无通道级失踪扫描 | 只检查设备在线，不检查通道 | 设备删除通道后，数据库中仍有旧数据 |
| 无实时通知 | 前端需要手动刷新 | 用户体验差 |

---

## 2. 目标

借鉴 voaglar 项目的优秀设计，改进 lalmax-nvr 的 GB28181 目录查询功能：

1. **批量 Upsert 优化** - 将先删后插改为增量更新
2. **单调性保护** - 添加时间戳比较，防止旧数据覆盖新数据
3. **通道级失踪扫描** - 检测连续 N 次目录响应未出现的通道并标记离线
4. **WebSocket 实时通知** - 设备/通道变更时推送到前端

---

## 3. 详细设计

### 3.1 批量 Upsert 优化

#### 现状分析

**内存层**：
- `dev.Channels` 使用 `sync.Map` 存储通道
- 当前是追加模式，旧通道不会被删除

**数据库层**：
- `ReplaceGB28181Channels` 使用先删后插策略
- 虽然事务保证原子性，但删除和插入之间有时间窗口

#### 改进方案

**内存层**：改为全量替换

```go
// handleCatalogResponse 中
// 清除旧通道
dev.Channels = sync.Map{}

// 存入新通道
for _, ch := range channels {
    dev.Channels.Store(ch.ChannelID, ch)
}
```

**数据库层**：改为增量更新

```go
// BatchUpsertChannels 实现
func (d *DB) BatchUpsertChannels(ctx context.Context, deviceID string, channels []GB28181ChannelRow) error {
    tx, err := d.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 1. 查询现有通道
    existing, err := d.listChannelIDs(ctx, tx, deviceID)
    if err != nil {
        return err
    }

    // 2. 计算差异
    newSet := make(map[string]bool)
    for _, ch := range channels {
        newSet[ch.ChannelID] = true
    }

    // 3. 删除不存在的通道
    for _, chID := range existing {
        if !newSet[chID] {
            if _, err := tx.ExecContext(ctx,
                "DELETE FROM gb28181_channels WHERE device_id = ? AND channel_id = ?;",
                deviceID, chID); err != nil {
                return err
            }
        }
    }

    // 4. 批量插入/更新通道
    for _, ch := range channels {
        if _, err := tx.ExecContext(ctx, `
            INSERT INTO gb28181_channels (device_id, channel_id, name, last_seen_at, status)
            VALUES (?, ?, ?, CURRENT_TIMESTAMP, 'online')
            ON CONFLICT(device_id, channel_id) DO UPDATE SET
                name = COALESCE(NULLIF(excluded.name, ''), name),
                last_seen_at = CURRENT_TIMESTAMP,
                status = 'online',
                missing_count = 0;`,
            ch.DeviceID, ch.ChannelID, ch.Name); err != nil {
            return err
        }
    }

    return tx.Commit()
}
```

---

### 3.2 单调性保护

#### Schema 变更

```sql
ALTER TABLE gb28181_channels ADD COLUMN last_seen_at DATETIME;
ALTER TABLE gb28181_channels ADD COLUMN status TEXT DEFAULT '';
ALTER TABLE gb28181_channels ADD COLUMN missing_count INTEGER DEFAULT 0;
```

#### 更新逻辑

```go
// 更新时检查时间戳
func (d *DB) UpsertChannelWithMonotonicity(ctx context.Context, channel *GB28181ChannelRow) error {
    _, err := d.db.ExecContext(ctx, `
        INSERT INTO gb28181_channels (device_id, channel_id, name, last_seen_at, status, missing_count)
        VALUES (?, ?, ?, ?, 'online', 0)
        ON CONFLICT(device_id, channel_id) DO UPDATE SET
            name = COALESCE(NULLIF(excluded.name, ''), name),
            last_seen_at = CASE
                WHEN excluded.last_seen_at > gb28181_channels.last_seen_at
                THEN excluded.last_seen_at
                ELSE gb28181_channels.last_seen_at
            END,
            status = CASE
                WHEN excluded.last_seen_at > gb28181_channels.last_seen_at
                THEN 'online'
                ELSE gb28181_channels.status
            END,
            missing_count = CASE
                WHEN excluded.last_seen_at > gb28181_channels.last_seen_at
                THEN 0
                ELSE gb28181_channels.missing_count
            END;`,
        channel.DeviceID, channel.ChannelID, channel.Name, channel.LastSeenAt)
    return err
}
```

---

### 3.3 通道级失踪扫描

#### 目录响应处理

```go
// handleCatalogResponse 中
func (g *GB28181API) handleCatalogResponse(deviceID string, body []byte) {
    // ... 解析响应 ...

    // 获取现有通道
    existingChannels := g.getExistingChannels(deviceID)

    // 构建新通道集合
    newChannelSet := make(map[string]bool)
    for _, ch := range channels {
        newChannelSet[ch.ChannelID] = true
    }

    // 更新失踪计数
    for _, chID := range existingChannels {
        if !newChannelSet[chID] {
            // 通道未出现在本次响应中，增加失踪计数
            g.incrementMissingCount(deviceID, chID)
        }
    }

    // ... 保存通道 ...
}
```

#### 定时扫描任务

```go
// server.go 中
func (g *GB28181API) startChannelMissingScan() {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        g.scanMissingChannels()
    }
}

func (g *GB28181API) scanMissingChannels() {
    // 查询 missing_count >= 3 的通道
    channels, err := g.store.GetDB().ListMissingChannels(context.Background(), 3)
    if err != nil {
        slog.Error("failed to list missing channels", "error", err)
        return
    }

    for _, ch := range channels {
        // 标记为离线
        if err := g.store.GetDB().UpdateChannelStatus(context.Background(),
            ch.DeviceID, ch.ChannelID, "offline"); err != nil {
            slog.Error("failed to update channel status",
                "device_id", ch.DeviceID,
                "channel_id", ch.ChannelID,
                "error", err)
        }

        // 广播事件
        g.hub.Broadcast(Event{
            Type: EventChannelOffline,
            Data: map[string]interface{}{
                "device_id":  ch.DeviceID,
                "channel_id": ch.ChannelID,
            },
        })
    }
}
```

---

### 3.4 WebSocket 实时通知

#### WebSocket Hub

```go
// ws_hub.go
package gb28181

import (
    "encoding/json"
    "log/slog"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

type EventType string

const (
    EventDeviceOnline    EventType = "device.online"
    EventDeviceOffline   EventType = "device.offline"
    EventChannelUpdate   EventType = "channel.update"
    EventChannelDelete   EventType = "channel.delete"
    EventChannelOffline  EventType = "channel.offline"
)

type Event struct {
    Type      EventType              `json:"type"`
    Data      map[string]interface{} `json:"data"`
    Timestamp int64                  `json:"timestamp"`
}

type Client struct {
    hub  *WSHub
    conn *websocket.Conn
    send chan []byte
}

type WSHub struct {
    clients    map[*Client]bool
    broadcast  chan Event
    register   chan *Client
    unregister chan *Client
    mu         sync.RWMutex
}

func NewWSHub() *WSHub {
    return &WSHub{
        clients:    make(map[*Client]bool),
        broadcast:  make(chan Event, 256),
        register:   make(chan *Client),
        unregister: make(chan *Client),
    }
}

func (h *WSHub) Run() {
    for {
        select {
        case client := <-h.register:
            h.mu.Lock()
            h.clients[client] = true
            h.mu.Unlock()

        case client := <-h.unregister:
            h.mu.Lock()
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.send)
            }
            h.mu.Unlock()

        case event := <-h.broadcast:
            data, err := json.Marshal(event)
            if err != nil {
                slog.Error("failed to marshal event", "error", err)
                continue
            }

            h.mu.RLock()
            for client := range h.clients {
                select {
                case client.send <- data:
                default:
                    close(client.send)
                    delete(h.clients, client)
                }
            }
            h.mu.RUnlock()
        }
    }
}

func (h *WSHub) Broadcast(event Event) {
    event.Timestamp = time.Now().UnixMilli()
    h.broadcast <- event
}

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

func (h *WSHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        slog.Error("failed to upgrade websocket", "error", err)
        return
    }

    client := &Client{
        hub:  h,
        conn: conn,
        send: make(chan []byte, 256),
    }

    h.register <- client

    go client.writePump()
    go client.readPump()
}

func (c *Client) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()

    c.conn.SetReadLimit(512)
    c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        return nil
    })

    for {
        _, _, err := c.conn.ReadMessage()
        if err != nil {
            break
        }
    }
}

func (c *Client) writePump() {
    ticker := time.NewTicker(30 * time.Second)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()

    for {
        select {
        case message, ok := <-c.send:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }

            w, err := c.conn.NextWriter(websocket.TextMessage)
            if err != nil {
                return
            }
            w.Write(message)

            if err := w.Close(); err != nil {
                return
            }

        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
```

#### 集成事件广播

```go
// devices.go 中
func (s *DeviceStore) UpdateDeviceStatus(deviceID string, isOnline bool, address string) error {
    ctx := context.Background()
    if err := s.db.UpdateGB28181DeviceStatus(ctx, deviceID, isOnline, address); err != nil {
        return err
    }

    // 广播事件
    eventType := EventDeviceOffline
    if isOnline {
        eventType = EventDeviceOnline
    }
    s.hub.Broadcast(Event{
        Type: eventType,
        Data: map[string]interface{}{
            "device_id": deviceID,
            "is_online": isOnline,
        },
    })

    return nil
}
```

---

## 4. 改动文件清单

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/storage/db_gb28181.go` | 修改 | Schema 变更 + 新增方法 |
| `internal/gb28181/catalog.go` | 修改 | 内存全量替换 + 失踪计数 |
| `internal/gb28181/devices.go` | 修改 | 集成事件广播 |
| `internal/gb28181/server.go` | 修改 | 添加定时扫描任务 |
| `internal/gb28181/ws_hub.go` | 新增 | WebSocket Hub |
| `web/src/routes/gb28181/GB28181Devices.svelte` | 修改 | 前端 WebSocket 连接 |

---

## 5. 测试策略

### 单元测试

1. `BatchUpsertChannels` - 测试增量更新逻辑
2. `UpsertChannelWithMonotonicity` - 测试单调性保护
3. `scanMissingChannels` - 测试失踪扫描逻辑
4. `WSHub` - 测试事件广播

### 集成测试

1. 目录查询完整流程
2. WebSocket 连接和事件接收
3. 失踪扫描和通道状态更新

---

## 6. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| Schema 变更导致数据丢失 | 高 | 使用 ALTER TABLE ADD COLUMN，不删除现有数据 |
| WebSocket 连接过多 | 中 | 限制最大连接数，添加心跳检测 |
| 失踪扫描误判 | 中 | 调整 missing_count 阈值，添加手动恢复机制 |

---

## 7. 参考

- voaglar 项目 `DeviceChannelManager.batchUpsertWithStatus` 实现
- voaglar 项目 `Gb28181ProtocolHandler.handleCatalog` 实现
- voaglar 项目 `RedisBackedSseEventBus` 设计
