package gb28181

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"log/slog"
	"math"
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

// PTZPositionQueryRequest PTZ位置查询请求
type PTZPositionQueryRequest struct {
	XMLName  xml.Name `xml:"Query"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
}

// PTZPosition PTZ位置信息
type PTZPosition struct {
	// 水平角度 (0-360度)
	HorizontalAngle float64 `json:"horizontal_angle"`
	// 垂直角度 (0-90度)
	VerticalAngle float64 `json:"vertical_angle"`
	// 缩放倍数 (1-100倍)
	ZoomLevel float64 `json:"zoom_level"`
}

const (
	PTZ_BIT_ZOOM_OUT = 0x20
	PTZ_BIT_ZOOM_IN  = 0x10
	PTZ_BIT_UP       = 0x08
	PTZ_BIT_DOWN     = 0x04
	PTZ_BIT_LEFT     = 0x02
	PTZ_BIT_RIGHT    = 0x01

	// GB28181-2022 精准位置控制命令码
	PTZ_CMD_SET_POSITION     = 0x81 // 设置精准位置
	PTZ_CMD_QUERY_POSITION   = 0x82 // 查询位置
	PTZ_CMD_SET_PRESET       = 0x83 // 设置预置位
	PTZ_CMD_CALL_PRESET      = 0x84 // 调用预置位
	PTZ_CMD_DELETE_PRESET    = 0x85 // 删除预置位
	PTZ_CMD_CRUISE_ADD_POINT = 0x86 // 加入巡航点
	PTZ_CMD_CRUISE_DELETE    = 0x87 // 删除巡航点
	PTZ_CMD_CRUISE_SPEED     = 0x88 // 设置巡航速度
	PTZ_CMD_CRUISE_START     = 0x89 // 开始巡航
	PTZ_CMD_CRUISE_STOP      = 0x8A // 停止巡航
	PTZ_CMD_SCAN_SET_LEFT    = 0x8B // 设置扫描左边界
	PTZ_CMD_SCAN_SET_RIGHT   = 0x8C // 设置扫描右边界
	PTZ_CMD_SCAN_SET_SPEED   = 0x8D // 设置扫描速度
	PTZ_CMD_SCAN_START       = 0x8E // 开始扫描
	PTZ_CMD_SCAN_STOP        = 0x8F // 停止扫描
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

// BuildFrontEndCmdString 构建前端控制指令字符串
// cmdCode: 指令码
// parameter1: 数据1
// parameter2: 数据2
// combineCode2: 组合码2
func BuildFrontEndCmdString(cmdCode, parameter1, parameter2, combineCode2 int) string {
	byte1 := byte(0xA5)
	byte2 := byte(0x0F)
	byte3 := byte(0x01)
	byte4 := byte(cmdCode & 0xFF)
	byte5 := byte(parameter1 & 0xFF)
	byte6 := byte(parameter2 & 0xFF)
	byte7 := byte((combineCode2 << 4) & 0xF0)
	checksum := byte1 + byte2 + byte3 + byte4 + byte5 + byte6 + byte7
	cmdBytes := []byte{byte1, byte2, byte3, byte4, byte5, byte6, byte7, checksum}
	return hex.EncodeToString(cmdBytes)
}

// BuildSetPositionCmd 构建精准位置控制命令
// horizontalAngle: 水平角度 (0-360度)
// verticalAngle: 垂直角度 (0-90度)
// zoomLevel: 缩放倍数 (1-100倍)
func BuildSetPositionCmd(horizontalAngle, verticalAngle, zoomLevel float64) string {
	// 将角度转换为GB28181标准的参数值
	// 水平角度: 0-360度 -> 0-255
	// 垂直角度: 0-90度 -> 0-255
	// 缩放倍数: 1-100倍 -> 0-255
	horzParam := int(math.Round(horizontalAngle / 360.0 * 255.0))
	vertParam := int(math.Round(verticalAngle / 90.0 * 255.0))
	zoomParam := int(math.Round((zoomLevel - 1) / 99.0 * 255.0))

	// 确保参数在有效范围内
	if horzParam < 0 {
		horzParam = 0
	}
	if horzParam > 255 {
		horzParam = 255
	}
	if vertParam < 0 {
		vertParam = 0
	}
	if vertParam > 255 {
		vertParam = 255
	}
	if zoomParam < 0 {
		zoomParam = 0
	}
	if zoomParam > 255 {
		zoomParam = 255
	}

	return BuildFrontEndCmdString(PTZ_CMD_SET_POSITION, horzParam, vertParam, zoomParam>>4)
}

// BuildQueryPositionCmd 构建位置查询命令
func BuildQueryPositionCmd() string {
	return BuildFrontEndCmdString(PTZ_CMD_QUERY_POSITION, 0, 0, 0)
}

// BuildSetPresetCmd 构建设置预置位命令
// presetID: 预置位编号 (1-255)
func BuildSetPresetCmd(presetID int) string {
	if presetID < 1 {
		presetID = 1
	}
	if presetID > 255 {
		presetID = 255
	}
	return BuildFrontEndCmdString(PTZ_CMD_SET_PRESET, 1, presetID, 0)
}

// BuildCallPresetCmd 构建调用预置位命令
// presetID: 预置位编号 (1-255)
func BuildCallPresetCmd(presetID int) string {
	if presetID < 1 {
		presetID = 1
	}
	if presetID > 255 {
		presetID = 255
	}
	return BuildFrontEndCmdString(PTZ_CMD_CALL_PRESET, 1, presetID, 0)
}

// BuildDeletePresetCmd 构建删除预置位命令
// presetID: 预置位编号 (1-255)
func BuildDeletePresetCmd(presetID int) string {
	if presetID < 1 {
		presetID = 1
	}
	if presetID > 255 {
		presetID = 255
	}
	return BuildFrontEndCmdString(PTZ_CMD_DELETE_PRESET, 1, presetID, 0)
}

// BuildCruiseAddPointCmd 构建加入巡航点命令
// cruiseID: 巡航组号 (0-255)
// presetID: 预置位编号 (1-255)
func BuildCruiseAddPointCmd(cruiseID, presetID int) string {
	if cruiseID < 0 {
		cruiseID = 0
	}
	if cruiseID > 255 {
		cruiseID = 255
	}
	if presetID < 1 {
		presetID = 1
	}
	if presetID > 255 {
		presetID = 255
	}
	return BuildFrontEndCmdString(PTZ_CMD_CRUISE_ADD_POINT, cruiseID, presetID, 0)
}

// BuildCruiseDeletePointCmd 构建删除巡航点命令
// cruiseID: 巡航组号 (0-255)
// presetID: 预置位编号 (0-255, 为0时删除整个巡航)
func BuildCruiseDeletePointCmd(cruiseID, presetID int) string {
	if cruiseID < 0 {
		cruiseID = 0
	}
	if cruiseID > 255 {
		cruiseID = 255
	}
	if presetID < 0 {
		presetID = 0
	}
	if presetID > 255 {
		presetID = 255
	}
	return BuildFrontEndCmdString(PTZ_CMD_CRUISE_DELETE, cruiseID, presetID, 0)
}

// BuildCruiseSpeedCmd 构建设置巡航速度命令
// cruiseID: 巡航组号 (0-255)
// speed: 巡航速度 (1-4095)
func BuildCruiseSpeedCmd(cruiseID, speed int) string {
	if cruiseID < 0 {
		cruiseID = 0
	}
	if cruiseID > 255 {
		cruiseID = 255
	}
	if speed < 1 {
		speed = 1
	}
	if speed > 4095 {
		speed = 4095
	}
	parameter2 := speed & 0xFF
	combineCode2 := speed >> 8
	return BuildFrontEndCmdString(PTZ_CMD_CRUISE_SPEED, cruiseID, parameter2, combineCode2)
}

// BuildCruiseStartCmd 构建开始巡航命令
// cruiseID: 巡航组号 (0-255)
func BuildCruiseStartCmd(cruiseID int) string {
	if cruiseID < 0 {
		cruiseID = 0
	}
	if cruiseID > 255 {
		cruiseID = 255
	}
	return BuildFrontEndCmdString(PTZ_CMD_CRUISE_START, cruiseID, 0, 0)
}

// BuildCruiseStopCmd 构建停止巡航命令
// cruiseID: 巡航组号 (0-255)
func BuildCruiseStopCmd(cruiseID int) string {
	if cruiseID < 0 {
		cruiseID = 0
	}
	if cruiseID > 255 {
		cruiseID = 255
	}
	return BuildFrontEndCmdString(PTZ_CMD_CRUISE_STOP, cruiseID, 0, 0)
}

// BuildScanSetLeftCmd 构建设置扫描左边界命令
// scanID: 扫描组号 (0-255)
func BuildScanSetLeftCmd(scanID int) string {
	if scanID < 0 {
		scanID = 0
	}
	if scanID > 255 {
		scanID = 255
	}
	return BuildFrontEndCmdString(PTZ_CMD_SCAN_SET_LEFT, scanID, 1, 0)
}

// BuildScanSetRightCmd 构建设置扫描右边界命令
// scanID: 扫描组号 (0-255)
func BuildScanSetRightCmd(scanID int) string {
	if scanID < 0 {
		scanID = 0
	}
	if scanID > 255 {
		scanID = 255
	}
	return BuildFrontEndCmdString(PTZ_CMD_SCAN_SET_RIGHT, scanID, 2, 0)
}

// BuildScanSetSpeedCmd 构建设置扫描速度命令
// scanID: 扫描组号 (0-255)
// speed: 扫描速度 (1-4095)
func BuildScanSetSpeedCmd(scanID, speed int) string {
	if scanID < 0 {
		scanID = 0
	}
	if scanID > 255 {
		scanID = 255
	}
	if speed < 1 {
		speed = 1
	}
	if speed > 4095 {
		speed = 4095
	}
	parameter2 := speed & 0xFF
	combineCode2 := speed >> 8
	return BuildFrontEndCmdString(PTZ_CMD_SCAN_SET_SPEED, scanID, parameter2, combineCode2)
}

// BuildScanStartCmd 构建开始扫描命令
// scanID: 扫描组号 (0-255)
func BuildScanStartCmd(scanID int) string {
	if scanID < 0 {
		scanID = 0
	}
	if scanID > 255 {
		scanID = 255
	}
	return BuildFrontEndCmdString(PTZ_CMD_SCAN_START, scanID, 0, 0)
}

// BuildScanStopCmd 构建停止扫描命令
// scanID: 扫描组号 (0-255)
func BuildScanStopCmd(scanID int) string {
	if scanID < 0 {
		scanID = 0
	}
	if scanID > 255 {
		scanID = 255
	}
	return BuildFrontEndCmdString(PTZ_CMD_SCAN_STOP, scanID, 0, 0)
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

// PTZPositionControl 精准位置控制
// horizontalAngle: 水平角度 (0-360度)
// verticalAngle: 垂直角度 (0-90度)
// zoomLevel: 缩放倍数 (1-100倍)
func (g *GB28181API) PTZPositionControl(deviceID, channelID string, horizontalAngle, verticalAngle, zoomLevel float64) error {
	dev, ok := g.store.Load(deviceID)
	if !ok {
		return fmt.Errorf("device not found: %s", deviceID)
	}
	if !dev.IsOnline {
		return ErrDeviceOffline
	}

	// 验证参数范围
	if horizontalAngle < 0 || horizontalAngle > 360 {
		return fmt.Errorf("horizontal angle must be between 0 and 360, got %f", horizontalAngle)
	}
	if verticalAngle < 0 || verticalAngle > 90 {
		return fmt.Errorf("vertical angle must be between 0 and 90, got %f", verticalAngle)
	}
	if zoomLevel < 1 || zoomLevel > 100 {
		return fmt.Errorf("zoom level must be between 1 and 100, got %f", zoomLevel)
	}

	ptzCmd := BuildSetPositionCmd(horizontalAngle, verticalAngle, zoomLevel)

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
		return fmt.Errorf("send PTZ position control command failed: %w", err)
	}

	slog.Info("PTZ position control command sent",
		"device_id", deviceID,
		"channel_id", channelID,
		"horizontal_angle", horizontalAngle,
		"vertical_angle", verticalAngle,
		"zoom_level", zoomLevel)
	return nil
}

// QueryPTZPosition 查询PTZ位置
func (g *GB28181API) QueryPTZPosition(deviceID, channelID string) error {
	dev, ok := g.store.Load(deviceID)
	if !ok {
		return fmt.Errorf("device not found: %s", deviceID)
	}
	if !dev.IsOnline {
		return ErrDeviceOffline
	}

	queryReq := PTZPositionQueryRequest{
		CmdType:  "PTZPosition",
		SN:       randInt(100000, 999999),
		DeviceID: channelID,
	}

	b, _ := xml.Marshal(queryReq)
	xmlBody := append([]byte(`<?xml version="1.0" encoding="GB2312"?>`+"\n"), b...)

	if err := g.sendMessage(channelID, dev, xmlBody); err != nil {
		return fmt.Errorf("send PTZ position query failed: %w", err)
	}

	slog.Info("PTZ position query sent", "device_id", deviceID, "channel_id", channelID)
	return nil
}

// SetPreset 设置预置位
func (g *GB28181API) SetPreset(deviceID, channelID string, presetID int) error {
	ptzCmd := BuildSetPresetCmd(presetID)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// CallPreset 调用预置位
func (g *GB28181API) CallPreset(deviceID, channelID string, presetID int) error {
	ptzCmd := BuildCallPresetCmd(presetID)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// DeletePreset 删除预置位
func (g *GB28181API) DeletePreset(deviceID, channelID string, presetID int) error {
	ptzCmd := BuildDeletePresetCmd(presetID)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// CruiseAddPoint 加入巡航点
func (g *GB28181API) CruiseAddPoint(deviceID, channelID string, cruiseID, presetID int) error {
	ptzCmd := BuildCruiseAddPointCmd(cruiseID, presetID)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// CruiseDeletePoint 删除巡航点
func (g *GB28181API) CruiseDeletePoint(deviceID, channelID string, cruiseID, presetID int) error {
	ptzCmd := BuildCruiseDeletePointCmd(cruiseID, presetID)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// CruiseSetSpeed 设置巡航速度
func (g *GB28181API) CruiseSetSpeed(deviceID, channelID string, cruiseID, speed int) error {
	ptzCmd := BuildCruiseSpeedCmd(cruiseID, speed)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// CruiseStart 开始巡航
func (g *GB28181API) CruiseStart(deviceID, channelID string, cruiseID int) error {
	ptzCmd := BuildCruiseStartCmd(cruiseID)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// CruiseStop 停止巡航
func (g *GB28181API) CruiseStop(deviceID, channelID string, cruiseID int) error {
	ptzCmd := BuildCruiseStopCmd(cruiseID)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// ScanSetLeft 设置扫描左边界
func (g *GB28181API) ScanSetLeft(deviceID, channelID string, scanID int) error {
	ptzCmd := BuildScanSetLeftCmd(scanID)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// ScanSetRight 设置扫描右边界
func (g *GB28181API) ScanSetRight(deviceID, channelID string, scanID int) error {
	ptzCmd := BuildScanSetRightCmd(scanID)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// ScanSetSpeed 设置扫描速度
func (g *GB28181API) ScanSetSpeed(deviceID, channelID string, scanID, speed int) error {
	ptzCmd := BuildScanSetSpeedCmd(scanID, speed)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// ScanStart 开始扫描
func (g *GB28181API) ScanStart(deviceID, channelID string, scanID int) error {
	ptzCmd := BuildScanStartCmd(scanID)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}

// ScanStop 停止扫描
func (g *GB28181API) ScanStop(deviceID, channelID string, scanID int) error {
	ptzCmd := BuildScanStopCmd(scanID)
	return g.PTZControl(deviceID, channelID, ptzCmd)
}
