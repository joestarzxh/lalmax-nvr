# GB28181 目录查询功能改进实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 改进 lalmax-nvr 的 GB28181 目录查询功能，实现批量 Upsert 优化、单调性保护、通道级失踪扫描和 WebSocket 实时通知。

**Architecture:** 在现有架构上增量改进，不改变整体结构。使用 SQLite 存储，添加新字段支持单调性保护和失踪扫描。引入 WebSocket Hub 实现实时通知。

**Tech Stack:** Go, SQLite, gorilla/websocket, sync.Map

---

## 文件结构

| 文件 | 职责 |
|------|------|
| `internal/storage/db_gb28181.go` | 数据库 Schema 和 CRUD 操作 |
| `internal/storage/db_gb28181_test.go` | 数据库操作测试 |
| `internal/gb28181/catalog.go` | 目录查询命令发送和响应处理 |
| `internal/gb28181/catalog_test.go` | 目录查询测试 |
| `internal/gb28181/devices.go` | 设备和通道管理 |
| `internal/gb28181/devices_test.go` | 设备管理测试 |
| `internal/gb28181/server.go` | GB28181 服务器主逻辑 |
| `internal/gb28181/ws_hub.go` | WebSocket Hub |
| `internal/gb28181/ws_hub_test.go` | WebSocket Hub 测试 |
| `web/src/routes/gb28181/GB28181Devices.svelte` | 前端设备管理页面 |

---

## Task 1: 数据库 Schema 变更

**Files:**
- Modify: `internal/storage/db_gb28181.go:48-55`
- Test: `internal/storage/db_gb28181_test.go`

- [ ] **Step 1: 添加新字段到 GB28181ChannelRow 结构体**

```go
// GB28181ChannelRow represents a channel of a GB28181 device.
type GB28181ChannelRow struct {
	DeviceID     string     `json:"device_id"`
	ChannelID    string     `json:"channel_id"`
	Name         string     `json:"name"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
	Status       string     `json:"status"`
	MissingCount int        `json:"missing_count"`
	CreatedAt    time.Time  `json:"created_at"`
}
```

- [ ] **Step 2: 修改 createGB28181Tables 添加新列**

```go
channelSQL := `CREATE TABLE IF NOT EXISTS gb28181_channels (
	device_id TEXT NOT NULL,
	channel_id TEXT NOT NULL,
	name TEXT DEFAULT '',
	last_seen_at DATETIME,
	status TEXT DEFAULT '',
	missing_count INTEGER DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (device_id, channel_id),
	FOREIGN KEY (device_id) REFERENCES gb28181_devices(device_id) ON DELETE CASCADE
);`
```

- [ ] **Step 3: 添加 Schema 迁移逻辑**

在 `Init()` 方法中添加迁移逻辑，为现有表添加新列：

```go
// 迁移：添加新列
migrations := []string{
	`ALTER TABLE gb28181_channels ADD COLUMN last_seen_at DATETIME;`,
	`ALTER TABLE gb28181_channels ADD COLUMN status TEXT DEFAULT '';`,
	`ALTER TABLE gb28181_channels ADD COLUMN missing_count INTEGER DEFAULT 0;`,
}

for _, migration := range migrations {
	_, err := d.db.ExecContext(ctx, migration)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
}
```

- [ ] **Step 4: 修改 ListGB28181Channels 查询新字段**

```go
func (d *DB) ListGB28181Channels(ctx context.Context, deviceID string) ([]GB28181ChannelRow, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT device_id, channel_id, name, last_seen_at, status, missing_count, created_at
		FROM gb28181_channels 
		WHERE device_id = ?
		ORDER BY channel_id;`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var channels []GB28181ChannelRow
	for rows.Next() {
		var ch GB28181ChannelRow
		var lastSeenAt sql.NullString
		if err := rows.Scan(&ch.DeviceID, &ch.ChannelID, &ch.Name, &lastSeenAt, &ch.Status, &ch.MissingCount, &ch.CreatedAt); err != nil {
			return nil, err
		}
		if lastSeenAt.Valid {
			t, _ := parseTime(lastSeenAt.String)
			ch.LastSeenAt = &t
		}
		channels = append(channels, ch)
	}
	return channels, nil
}
```

- [ ] **Step 5: 运行测试验证 Schema 变更**

Run: `go test ./internal/storage/... -v -run TestGB28181`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/storage/db_gb28181.go
git commit -m "feat(gb28181): add last_seen_at, status, missing_count to channels table"
```

---

## Task 2: 批量 Upsert 优化

**Files:**
- Modify: `internal/storage/db_gb28181.go`
- Test: `internal/storage/db_gb28181_test.go`

- [ ] **Step 1: 添加 BatchUpsertChannels 方法**

```go
// BatchUpsertChannels performs incremental update of channels for a device.
// It deletes channels that no longer exist and inserts/updates new channels.
func (d *DB) BatchUpsertChannels(ctx context.Context, deviceID string, channels []GB28181ChannelRow) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 查询现有通道
	existingRows, err := tx.QueryContext(ctx, 
		"SELECT channel_id FROM gb28181_channels WHERE device_id = ?;", deviceID)
	if err != nil {
		return err
	}
	
	existing := make(map[string]bool)
	for existingRows.Next() {
		var chID string
		if err := existingRows.Scan(&chID); err != nil {
			existingRows.Close()
			return err
		}
		existing[chID] = true
	}
	existingRows.Close()

	// 2. 构建新通道集合
	newSet := make(map[string]bool)
	for _, ch := range channels {
		newSet[ch.ChannelID] = true
	}

	// 3. 删除不存在的通道
	for chID := range existing {
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
			INSERT INTO gb28181_channels (device_id, channel_id, name, last_seen_at, status, missing_count)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP, 'online', 0)
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

- [ ] **Step 2: 添加 ListMissingChannels 方法**

```go
// ListMissingChannels returns channels with missing_count >= threshold.
func (d *DB) ListMissingChannels(ctx context.Context, threshold int) ([]GB28181ChannelRow, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT device_id, channel_id, name, last_seen_at, status, missing_count, created_at
		FROM gb28181_channels 
		WHERE missing_count >= ?
		ORDER BY device_id, channel_id;`, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var channels []GB28181ChannelRow
	for rows.Next() {
		var ch GB28181ChannelRow
		var lastSeenAt sql.NullString
		if err := rows.Scan(&ch.DeviceID, &ch.ChannelID, &ch.Name, &lastSeenAt, &ch.Status, &ch.MissingCount, &ch.CreatedAt); err != nil {
			return nil, err
		}
		if lastSeenAt.Valid {
			t, _ := parseTime(lastSeenAt.String)
			ch.LastSeenAt = &t
		}
		channels = append(channels, ch)
	}
	return channels, nil
}
```

- [ ] **Step 3: 添加 UpdateChannelStatus 方法**

```go
// UpdateChannelStatus updates the status of a channel.
func (d *DB) UpdateChannelStatus(ctx context.Context, deviceID, channelID, status string) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE gb28181_channels 
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE device_id = ? AND channel_id = ?;`, status, deviceID, channelID)
	return err
}
```

- [ ] **Step 4: 添加 IncrementMissingCount 方法**

```go
// IncrementMissingCount increments the missing_count for a channel.
func (d *DB) IncrementMissingCount(ctx context.Context, deviceID, channelID string) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE gb28181_channels 
		SET missing_count = missing_count + 1
		WHERE device_id = ? AND channel_id = ?;`, deviceID, channelID)
	return err
}
```

- [ ] **Step 5: 编写测试**

```go
func TestBatchUpsertChannels(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// 创建测试设备
	err := db.UpsertGB28181Device(ctx, &GB28181DeviceRow{
		DeviceID: "test_device",
		IsOnline: true,
	})
	require.NoError(t, err)

	// 测试批量插入
	channels := []GB28181ChannelRow{
		{DeviceID: "test_device", ChannelID: "channel_1", Name: "Channel 1"},
		{DeviceID: "test_device", ChannelID: "channel_2", Name: "Channel 2"},
	}
	err = db.BatchUpsertChannels(ctx, "test_device", channels)
	require.NoError(t, err)

	// 验证插入结果
	saved, err := db.ListGB28181Channels(ctx, "test_device")
	require.NoError(t, err)
	assert.Len(t, saved, 2)

	// 测试增量更新（删除 channel_2，添加 channel_3）
	channels = []GB28181ChannelRow{
		{DeviceID: "test_device", ChannelID: "channel_1", Name: "Channel 1 Updated"},
		{DeviceID: "test_device", ChannelID: "channel_3", Name: "Channel 3"},
	}
	err = db.BatchUpsertChannels(ctx, "test_device", channels)
	require.NoError(t, err)

	// 验证更新结果
	saved, err = db.ListGB28181Channels(ctx, "test_device")
	require.NoError(t, err)
	assert.Len(t, saved, 2)

	// 验证 channel_2 已删除
	for _, ch := range saved {
		assert.NotEqual(t, "channel_2", ch.ChannelID)
	}
}
```

- [ ] **Step 6: 运行测试**

Run: `go test ./internal/storage/... -v -run TestBatchUpsertChannels`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/storage/db_gb28181.go internal/storage/db_gb28181_test.go
git commit -m "feat(gb28181): implement BatchUpsertChannels for incremental update"
```

---

## Task 3: 修改目录响应处理

**Files:**
- Modify: `internal/gb28181/catalog.go`
- Modify: `internal/gb28181/devices.go`
- Test: `internal/gb28181/catalog_test.go`

- [ ] **Step 1: 修改 SaveChannels 方法使用 BatchUpsertChannels**

```go
// SaveChannels persists channels to database using incremental update.
func (s *DeviceStore) SaveChannels(deviceID string, channels []Channel) error {
	ctx := context.Background()
	dbChannels := make([]storage.GB28181ChannelRow, len(channels))
	for i, ch := range channels {
		dbChannels[i] = storage.GB28181ChannelRow{
			DeviceID:  deviceID,
			ChannelID: ch.ChannelID,
			Name:      ch.Name,
		}
	}
	return s.db.BatchUpsertChannels(ctx, deviceID, dbChannels)
}
```

- [ ] **Step 2: 修改 handleCatalogResponse 实现内存全量替换**

```go
func (g *GB28181API) handleCatalogResponse(deviceID string, body []byte) {
	var msg MessageDeviceListResponse
	if err := xmlUnmarshal(body, &msg); err != nil {
		slog.Error("catalog xml decode error", "device_id", deviceID, "error", err)
		return
	}
	if msg.SumNum <= 0 {
		return
	}

	dev, ok := g.store.Load(deviceID)
	if !ok {
		return
	}
	domain := dev.region
	if domain == "" {
		domain = g.cfg.GetDomain()
	}

	// Ensure device row exists before saving channels
	if err := g.store.SaveDevice(deviceID, dev); err != nil {
		slog.Error("failed to save device before catalog", "device_id", deviceID, "error", err)
	}

	// Build channel list for database
	var dbChannels []Channel
	newChannelMap := make(map[string]bool)
	
	for _, ch := range msg.Item {
		ch.ChannelID = ch.DeviceID
		ch.DeviceID = deviceID
		channel := &Channel{
			ChannelID: ch.ChannelID,
			Name:      ch.Name,
			device:    dev,
		}
		channel.init(domain)
		newChannelMap[ch.ChannelID] = true
		dbChannels = append(dbChannels, *channel)
	}

	// 内存全量替换
	dev.Channels = sync.Map{}
	for _, ch := range dbChannels {
		dev.Channels.Store(ch.ChannelID, &ch)
	}

	// 获取现有通道用于失踪计数
	existingChannels := g.getExistingChannelIDs(deviceID)

	// Persist channels to database
	if err := g.store.SaveChannels(deviceID, dbChannels); err != nil {
		slog.Error("failed to save channels to DB", "device_id", deviceID, "error", err)
	}

	// 更新失踪计数
	for _, chID := range existingChannels {
		if !newChannelMap[chID] {
			if err := g.store.GetDB().IncrementMissingCount(context.Background(), deviceID, chID); err != nil {
				slog.Error("failed to increment missing count",
					"device_id", deviceID,
					"channel_id", chID,
					"error", err)
			}
		}
	}

	slog.Info("catalog updated", "device_id", deviceID, "channels", len(msg.Item))
}
```

- [ ] **Step 3: 添加 getExistingChannelIDs 辅助方法**

```go
// getExistingChannelIDs returns existing channel IDs for a device.
func (g *GB28181API) getExistingChannelIDs(deviceID string) []string {
	ctx := context.Background()
	channels, err := g.store.GetDB().ListGB28181Channels(ctx, deviceID)
	if err != nil {
		slog.Error("failed to list existing channels", "device_id", deviceID, "error", err)
		return nil
	}
	
	var channelIDs []string
	for _, ch := range channels {
		channelIDs = append(channelIDs, ch.ChannelID)
	}
	return channelIDs
}
```

- [ ] **Step 4: 编写测试**

```go
func TestHandleCatalogResponse(t *testing.T) {
	// 设置测试环境
	store := setupTestStore(t)
	api := setupTestAPI(t, store)

	// 模拟设备注册
	deviceID := "34020000001320000001"
	dev := &Device{
		DeviceID: deviceID,
		IsOnline: true,
		Address:  "192.168.1.100",
	}
	store.Store(deviceID, dev)

	// 模拟目录响应
	catalogXML := `<?xml version="1.0" encoding="GB2312"?>
<Response>
<CmdType>Catalog</CmdType>
<SN>123456</SN>
<DeviceID>34020000001320000001</DeviceID>
<SumNum>2</SumNum>
<DeviceList>
<Item>
<DeviceID>34020000001320000002</DeviceID>
<Name>Camera 1</Name>
</Item>
<Item>
<DeviceID>34020000001320000003</DeviceID>
<Name>Camera 2</Name>
</Item>
</DeviceList>
</Response>`

	// 处理目录响应
	api.handleCatalogResponse(deviceID, []byte(catalogXML))

	// 验证内存中的通道
	ch1, ok := dev.GetChannel("34020000001320000002")
	assert.True(t, ok)
	assert.Equal(t, "Camera 1", ch1.Name)

	ch2, ok := dev.GetChannel("34020000001320000003")
	assert.True(t, ok)
	assert.Equal(t, "Camera 2", ch2.Name)

	// 验证数据库中的通道
	ctx := context.Background()
	dbChannels, err := store.GetDB().ListGB28181Channels(ctx, deviceID)
	require.NoError(t, err)
	assert.Len(t, dbChannels, 2)
}
```

- [ ] **Step 5: 运行测试**

Run: `go test ./internal/gb28181/... -v -run TestHandleCatalogResponse`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/gb28181/catalog.go internal/gb28181/devices.go
git commit -m "feat(gb28181): implement incremental channel update with missing count"
```

---

## Task 4: 通道级失踪扫描

**Files:**
- Modify: `internal/gb28181/server.go`
- Test: `internal/gb28181/server_test.go`

- [ ] **Step 1: 添加 startChannelMissingScan 方法**

```go
// startChannelMissingScan starts a goroutine that periodically scans for missing channels.
func (g *GB28181API) startChannelMissingScan() {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			g.scanMissingChannels()
		}
	}()
}
```

- [ ] **Step 2: 添加 scanMissingChannels 方法**

```go
// scanMissingChannels checks for channels with high missing_count and marks them offline.
func (g *GB28181API) scanMissingChannels() {
	ctx := context.Background()
	
	// 查询 missing_count >= 3 的通道
	channels, err := g.store.GetDB().ListMissingChannels(ctx, 3)
	if err != nil {
		slog.Error("failed to list missing channels", "error", err)
		return
	}

	for _, ch := range channels {
		// 标记为离线
		if err := g.store.GetDB().UpdateChannelStatus(ctx,
			ch.DeviceID, ch.ChannelID, "offline"); err != nil {
			slog.Error("failed to update channel status",
				"device_id", ch.DeviceID,
				"channel_id", ch.ChannelID,
				"error", err)
			continue
		}

		// 从内存中删除
		if dev, ok := g.store.Load(ch.DeviceID); ok {
			dev.Channels.Delete(ch.ChannelID)
		}

		// 广播事件
		g.hub.Broadcast(Event{
			Type: EventChannelOffline,
			Data: map[string]interface{}{
				"device_id":  ch.DeviceID,
				"channel_id": ch.ChannelID,
			},
		})

		slog.Info("channel marked offline due to missing",
			"device_id", ch.DeviceID,
			"channel_id", ch.ChannelID,
			"missing_count", ch.MissingCount)
	}
}
```

- [ ] **Step 3: 在 Start 方法中启动失踪扫描**

在 `Start()` 方法中添加：

```go
// 启动通道失踪扫描
g.startChannelMissingScan()
```

- [ ] **Step 4: 编写测试**

```go
func TestScanMissingChannels(t *testing.T) {
	// 设置测试环境
	store := setupTestStore(t)
	api := setupTestAPI(t, store)

	// 创建测试设备和通道
	deviceID := "test_device"
	ctx := context.Background()
	
	err := store.GetDB().UpsertGB28181Device(ctx, &storage.GB28181DeviceRow{
		DeviceID: deviceID,
		IsOnline: true,
	})
	require.NoError(t, err)

	// 创建通道并设置高 missing_count
	channels := []storage.GB28181ChannelRow{
		{DeviceID: deviceID, ChannelID: "channel_1", Name: "Channel 1"},
	}
	err = store.GetDB().BatchUpsertChannels(ctx, deviceID, channels)
	require.NoError(t, err)

	// 手动设置 missing_count
	_, err = store.GetDB().ExecContext(ctx, 
		"UPDATE gb28181_channels SET missing_count = 3 WHERE device_id = ? AND channel_id = ?;",
		deviceID, "channel_1")
	require.NoError(t, err)

	// 执行扫描
	api.scanMissingChannels()

	// 验证通道状态
	ch, err := store.GetDB().GetGB28181Channel(ctx, deviceID, "channel_1")
	require.NoError(t, err)
	assert.Equal(t, "offline", ch.Status)
}
```

- [ ] **Step 5: 运行测试**

Run: `go test ./internal/gb28181/... -v -run TestScanMissingChannels`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/gb28181/server.go
git commit -m "feat(gb28181): implement channel missing scan"
```

---

## Task 5: WebSocket Hub 实现

**Files:**
- Create: `internal/gb28181/ws_hub.go`
- Test: `internal/gb28181/ws_hub_test.go`

- [ ] **Step 1: 创建 ws_hub.go 文件**

```go
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
	EventDeviceOnline   EventType = "device.online"
	EventDeviceOffline  EventType = "device.offline"
	EventChannelUpdate  EventType = "channel.update"
	EventChannelDelete  EventType = "channel.delete"
	EventChannelOffline EventType = "channel.offline"
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

- [ ] **Step 2: 编写测试**

```go
func TestWSHub(t *testing.T) {
	hub := NewWSHub()
	go hub.Run()

	// 等待 Hub 启动
	time.Sleep(100 * time.Millisecond)

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	// 连接 WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// 等待连接注册
	time.Sleep(100 * time.Millisecond)

	// 广播事件
	hub.Broadcast(Event{
		Type: EventDeviceOnline,
		Data: map[string]interface{}{
			"device_id": "test_device",
		},
	})

	// 接收消息
	_, message, err := conn.ReadMessage()
	require.NoError(t, err)

	var event Event
	err = json.Unmarshal(message, &event)
	require.NoError(t, err)

	assert.Equal(t, EventDeviceOnline, event.Type)
	assert.Equal(t, "test_device", event.Data["device_id"])
}
```

- [ ] **Step 3: 运行测试**

Run: `go test ./internal/gb28181/... -v -run TestWSHub`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/gb28181/ws_hub.go internal/gb28181/ws_hub_test.go
git commit -m "feat(gb28181): implement WebSocket Hub for real-time notifications"
```

---

## Task 6: 集成事件广播

**Files:**
- Modify: `internal/gb28181/devices.go`
- Modify: `internal/gb28181/server.go`
- Test: `internal/gb28181/devices_test.go`

- [ ] **Step 1: 修改 DeviceStore 添加 hub 字段**

```go
type DeviceStore struct {
	devices sync.Map
	db      *storage.DB
	hub     *WSHub
}

func NewDeviceStore(db *storage.DB, hub *WSHub) *DeviceStore {
	return &DeviceStore{
		db:  db,
		hub: hub,
	}
}
```

- [ ] **Step 2: 修改 UpdateDeviceStatus 添加事件广播**

```go
func (s *DeviceStore) UpdateDeviceStatus(deviceID string, isOnline bool, address string) error {
	ctx := context.Background()
	if err := s.db.UpdateGB28181DeviceStatus(ctx, deviceID, isOnline, address); err != nil {
		return err
	}

	// 广播事件
	if s.hub != nil {
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
	}

	return nil
}
```

- [ ] **Step 3: 修改 GB28181API 添加 hub 字段**

```go
type GB28181API struct {
	store  *DeviceStore
	cfg    *Config
	hub    *WSHub
	// ... 其他字段
}
```

- [ ] **Step 4: 修改 NewGB28181API 初始化 hub**

```go
func NewGB28181API(cfg *Config, db *storage.DB) *GB28181API {
	hub := NewWSHub()
	store := NewDeviceStore(db, hub)
	
	api := &GB28181API{
		store: store,
		cfg:   cfg,
		hub:   hub,
		// ... 其他初始化
	}
	
	// 启动 WebSocket Hub
	go hub.Run()
	
	return api
}
```

- [ ] **Step 5: 添加 WebSocket 路由**

在 `Start()` 方法中添加：

```go
// WebSocket 路由
http.HandleFunc("/api/gb28181/ws", g.hub.HandleWebSocket)
```

- [ ] **Step 6: 运行测试**

Run: `go test ./internal/gb28181/... -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/gb28181/devices.go internal/gb28181/server.go
git commit -m "feat(gb28181): integrate WebSocket Hub with device store"
```

---

## Task 7: 前端 WebSocket 连接

**Files:**
- Modify: `web/src/routes/gb28181/GB28181Devices.svelte`

- [ ] **Step 1: 添加 WebSocket 连接逻辑**

```svelte
<script>
  import { onMount, onDestroy } from 'svelte';
  
  let ws = null;
  let wsConnected = false;
  
  function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsURL = `${protocol}//${window.location.host}/api/gb28181/ws`;
    
    ws = new WebSocket(wsURL);
    
    ws.onopen = () => {
      wsConnected = true;
      console.log('WebSocket connected');
    };
    
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      handleWebSocketEvent(data);
    };
    
    ws.onclose = () => {
      wsConnected = false;
      console.log('WebSocket disconnected');
      // 重连
      setTimeout(connectWebSocket, 3000);
    };
    
    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }
  
  function handleWebSocketEvent(event) {
    switch (event.type) {
      case 'device.online':
      case 'device.offline':
        // 更新设备状态
        updateDeviceStatus(event.data.device_id, event.data.is_online);
        break;
      case 'channel.update':
      case 'channel.delete':
      case 'channel.offline':
        // 刷新通道列表
        refreshChannels(event.data.device_id);
        break;
    }
  }
  
  function updateDeviceStatus(deviceID, isOnline) {
    devices = devices.map(d => {
      if (d.device_id === deviceID) {
        return { ...d, is_online: isOnline };
      }
      return d;
    });
  }
  
  async function refreshChannels(deviceID) {
    // 重新加载通道列表
    if (selectedDevice === deviceID) {
      await loadChannels(deviceID);
    }
  }
  
  onMount(() => {
    connectWebSocket();
  });
  
  onDestroy(() => {
    if (ws) {
      ws.close();
    }
  });
</script>
```

- [ ] **Step 2: 添加连接状态指示器**

```svelte
<div class="ws-status" class:connected={wsConnected}>
  {wsConnected ? '实时更新已连接' : '实时更新已断开'}
</div>

<style>
  .ws-status {
    padding: 4px 8px;
    border-radius: 4px;
    font-size: 12px;
    background-color: #fee2e2;
    color: #dc2626;
  }
  
  .ws-status.connected {
    background-color: #dcfce7;
    color: #16a34a;
  }
</style>
```

- [ ] **Step 3: 运行前端测试**

Run: `cd web && npm test`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/src/routes/gb28181/GB28181Devices.svelte
git commit -m "feat(gb28181): add WebSocket real-time updates to frontend"
```

---

## Task 8: 集成测试

**Files:**
- Test: `internal/gb28181/integration_test.go`

- [ ] **Step 1: 编写完整流程集成测试**

```go
func TestCatalogIntegration(t *testing.T) {
	// 设置测试环境
	db := setupTestDB(t)
	hub := NewWSHub()
	store := NewDeviceStore(db, hub)
	api := setupTestAPI(t, store, hub)

	// 启动 WebSocket Hub
	go hub.Run()

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/gb28181/ws" {
			hub.HandleWebSocket(w, r)
		}
	}))
	defer server.Close()

	// 连接 WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer wsConn.Close()

	// 模拟设备注册
	deviceID := "34020000001320000001"
	dev := &Device{
		DeviceID: deviceID,
		IsOnline: true,
		Address:  "192.168.1.100",
	}
	store.Store(deviceID, dev)

	// 模拟目录响应
	catalogXML := `<?xml version="1.0" encoding="GB2312"?>
<Response>
<CmdType>Catalog</CmdType>
<SN>123456</SN>
<DeviceID>34020000001320000001</DeviceID>
<SumNum>2</SumNum>
<DeviceList>
<Item>
<DeviceID>34020000001320000002</DeviceID>
<Name>Camera 1</Name>
</Item>
<Item>
<DeviceID>34020000001320000003</DeviceID>
<Name>Camera 2</Name>
</Item>
</DeviceList>
</Response>`

	// 处理目录响应
	api.handleCatalogResponse(deviceID, []byte(catalogXML))

	// 验证内存中的通道
	ch1, ok := dev.GetChannel("34020000001320000002")
	assert.True(t, ok)
	assert.Equal(t, "Camera 1", ch1.Name)

	// 验证数据库中的通道
	ctx := context.Background()
	dbChannels, err := db.ListGB28181Channels(ctx, deviceID)
	require.NoError(t, err)
	assert.Len(t, dbChannels, 2)

	// 模拟第二次目录响应（删除一个通道）
	catalogXML2 := `<?xml version="1.0" encoding="GB2312"?>
<Response>
<CmdType>Catalog</CmdType>
<SN>123457</SN>
<DeviceID>34020000001320000001</DeviceID>
<SumNum>1</SumNum>
<DeviceList>
<Item>
<DeviceID>34020000001320000002</DeviceID>
<Name>Camera 1</Name>
</Item>
</DeviceList>
</Response>`

	// 处理第二次目录响应
	api.handleCatalogResponse(deviceID, []byte(catalogXML2))

	// 验证 channel_2 的 missing_count 增加
	ch2, err := db.GetGB28181Channel(ctx, deviceID, "34020000001320000003")
	require.NoError(t, err)
	assert.Equal(t, 1, ch2.MissingCount)

	// 执行失踪扫描
	api.scanMissingChannels()

	// 验证 channel_2 仍然存在（missing_count < 3）
	ch2, err = db.GetGB28181Channel(ctx, deviceID, "34020000001320000003")
	require.NoError(t, err)
	assert.NotEqual(t, "offline", ch2.Status)
}
```

- [ ] **Step 2: 运行集成测试**

Run: `go test ./internal/gb28181/... -v -run TestCatalogIntegration`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/gb28181/integration_test.go
git commit -m "test(gb28181): add integration test for catalog improvements"
```

---

## 验证清单

- [ ] 所有单元测试通过
- [ ] 集成测试通过
- [ ] 前端 WebSocket 连接正常
- [ ] 目录查询功能正常
- [ ] 失踪扫描功能正常
- [ ] 实时通知功能正常
- [ ] 无回归问题

---

## 执行选项

**Plan complete and saved to `docs/superpowers/plans/2026-06-22-gb28181-catalog-improvements.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - 我为每个任务分发一个新的子任务代理，任务之间进行审查，快速迭代

**2. Inline Execution** - 在当前会话中执行任务，批量执行并设置检查点

**你选择哪种方式？**
