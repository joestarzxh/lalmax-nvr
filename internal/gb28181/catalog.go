package gb28181

import (
	"fmt"
	"log/slog"
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
	for _, ch := range msg.Item {
		ch.ChannelID = ch.DeviceID
		ch.DeviceID = deviceID
		channel := &Channel{
			ChannelID: ch.ChannelID,
			device:    dev,
		}
		channel.init(domain)
		dev.Channels.Store(ch.ChannelID, channel)
		dbChannels = append(dbChannels, *channel)
	}

	// Persist channels to database
	if err := g.store.SaveChannels(deviceID, dbChannels); err != nil {
		slog.Error("failed to save channels to DB", "device_id", deviceID, "error", err)
	}

	slog.Info("catalog updated", "device_id", deviceID, "channels", len(msg.Item))
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
