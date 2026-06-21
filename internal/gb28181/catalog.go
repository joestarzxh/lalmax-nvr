package gb28181

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

type MessageDeviceListResponse struct {
	XMLName  struct{}      `xml:"Response"`
	CmdType  string        `xml:"CmdType"`
	SN       int           `xml:"SN"`
	DeviceID string        `xml:"DeviceID"`
	SumNum   int           `xml:"SumNum"`
	Item     []ChannelsXML `xml:"DeviceList>Item"`
}

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
	for i := range dbChannels {
		dev.Channels.Store(dbChannels[i].ChannelID, &dbChannels[i])
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

	// 广播通道更新事件
	if g.hub != nil {
		g.hub.Broadcast(Event{
			Type: EventChannelUpdate,
			Data: map[string]interface{}{
				"device_id":     deviceID,
				"channel_count": len(dbChannels),
			},
		})
	}

	slog.Info("catalog updated", "device_id", deviceID, "channels", len(msg.Item))
}

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

func (g *GB28181API) QueryCatalog(deviceID string) error {
	slog.Debug("QueryCatalog", "device_id", deviceID)
	dev, ok := g.store.Load(deviceID)
	if !ok || !dev.IsOnline {
		return ErrDeviceOffline
	}

	xmlBody := catalogXML(deviceID)
	return g.sendMessage(deviceID, dev, xmlBody)
}

func catalogXML(deviceID string) []byte {
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="GB2312"?>
<Query>
<CmdType>Catalog</CmdType>
<SN>%d</SN>
<DeviceID>%s</DeviceID>
</Query>`, randInt(100000, 999999), deviceID))
}
