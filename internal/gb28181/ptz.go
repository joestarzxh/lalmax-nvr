package gb28181

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"log/slog"
)

type PTZControlInfo struct {
	ControlPriority int `xml:"ControlPriority"`
}

type PTZControlRequest struct {
	XMLName  xml.Name       `xml:"Control"`
	CmdType  string         `xml:"CmdType"`
	SN       int            `xml:"SN"`
	DeviceID string         `xml:"DeviceID"`
	PTZCmd   string         `xml:"PTZCmd"`
	Info     PTZControlInfo `xml:"Info"`
}

const (
	PTZ_BIT_ZOOM_OUT = 0x20
	PTZ_BIT_ZOOM_IN  = 0x10
	PTZ_BIT_UP       = 0x08
	PTZ_BIT_DOWN     = 0x04
	PTZ_BIT_LEFT     = 0x02
	PTZ_BIT_RIGHT    = 0x01
)

type PTZCmdBuilder struct {
	address   byte
	direction byte
	horzSpeed byte
	vertSpeed byte
	zoomSpeed byte
}

func BuildContinuousMove(direction string, speed float64) string {
	builder := &PTZCmdBuilder{address: 0x01}
	speedByte := byte(0x80)

	switch direction {
	case "up":
		builder.direction = PTZ_BIT_UP
	case "down":
		builder.direction = PTZ_BIT_DOWN
	case "left":
		builder.direction = PTZ_BIT_LEFT
	case "right":
		builder.direction = PTZ_BIT_RIGHT
	case "upleft":
		builder.direction = PTZ_BIT_UP | PTZ_BIT_LEFT
	case "upright":
		builder.direction = PTZ_BIT_UP | PTZ_BIT_RIGHT
	case "downleft":
		builder.direction = PTZ_BIT_DOWN | PTZ_BIT_LEFT
	case "downright":
		builder.direction = PTZ_BIT_DOWN | PTZ_BIT_RIGHT
	case "zoomin":
		builder.direction = PTZ_BIT_ZOOM_IN
		builder.zoomSpeed = speedByte >> 4
	case "zoomout":
		builder.direction = PTZ_BIT_ZOOM_OUT
		builder.zoomSpeed = speedByte >> 4
	default:
		return ""
	}
	builder.horzSpeed = speedByte
	builder.vertSpeed = speedByte
	return builder.build()
}

func BuildStop() string {
	return (&PTZCmdBuilder{address: 0x01}).build()
}

func (p *PTZCmdBuilder) build() string {
	byte1 := byte(0xA5)
	byte2 := byte(0x0F)
	byte3 := byte(0x01)
	byte7 := (p.zoomSpeed << 4) | 0x00
	if byte7 == 0 {
		byte7 = 0x80
	}
	checksum := byte1 + byte2 + byte3 + p.direction + p.horzSpeed + p.vertSpeed + byte7
	cmdBytes := []byte{byte1, byte2, byte3, p.direction, p.horzSpeed, p.vertSpeed, byte7, checksum}
	return hex.EncodeToString(cmdBytes)
}

func (g *GB28181API) PTZControl(deviceID, channelID, ptzCmd string) error {
	dev, ok := g.store.Load(deviceID)
	if !ok {
		return fmt.Errorf("device not found: %s", deviceID)
	}
	if !dev.IsOnline {
		return ErrDeviceOffline
	}

	ptzReq := PTZControlRequest{
		CmdType:  "DeviceControl",
		SN:       randInt(100000, 999999),
		DeviceID: channelID,
		PTZCmd:   ptzCmd,
		Info:     PTZControlInfo{ControlPriority: 5},
	}

	b, _ := xml.Marshal(ptzReq)
	xmlBody := append([]byte(`<?xml version="1.0" encoding="GB2312"?>`+"\n"), b...)

	if err := g.sendMessage(channelID, dev, xmlBody); err != nil {
		return fmt.Errorf("send PTZ command failed: %w", err)
	}

	slog.Info("PTZ command sent", "device_id", deviceID, "channel_id", channelID, "cmd", ptzCmd)
	return nil
}
