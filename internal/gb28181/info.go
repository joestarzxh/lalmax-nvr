package gb28181

import (
	"encoding/xml"
	"fmt"
	"log/slog"
)

type MessageDeviceInfoResponse struct {
	CmdType      string `xml:"CmdType"`
	SN           int    `xml:"SN"`
	DeviceID     string `xml:"DeviceID"`
	DeviceName   string `xml:"DeviceName"`
	Manufacturer string `xml:"Manufacturer"`
	Model        string `xml:"Model"`
	Firmware     string `xml:"Firmware"`
	Result       string `xml:"Result"`
}

func (g *GB28181API) handleDeviceInfoResponse(deviceID string, body []byte) {
	var msg MessageDeviceInfoResponse
	if err := xml.Unmarshal(body, &msg); err != nil {
		slog.Error("device info xml decode error", "device_id", deviceID, "error", err)
		return
	}

	slog.Info("device info received",
		"device_id", deviceID,
		"manufacturer", msg.Manufacturer,
		"model", msg.Model,
		"firmware", msg.Firmware,
		"name", msg.DeviceName,
	)
}

func (g *GB28181API) sendDeviceInfoQuery(deviceID string) {
	dev, ok := g.store.Load(deviceID)
	if !ok || !dev.IsOnline {
		return
	}

	xmlBody := deviceInfoXML(deviceID)
	if err := g.sendMessage(deviceID, dev, xmlBody); err != nil {
		slog.Error("send DeviceInfo query failed", "device_id", deviceID, "error", err)
	}
}

func deviceInfoXML(deviceID string) []byte {
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="GB2312"?>
<Query>
<CmdType>DeviceInfo</CmdType>
<SN>%d</SN>
<DeviceID>%s</DeviceID>
</Query>`, randInt(100000, 999999), deviceID))
}
