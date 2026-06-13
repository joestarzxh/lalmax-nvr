package gb28181

import (
	"fmt"
	"strconv"
	"strings"
)

// TransportMode 定义对讲传输模式
type TransportMode int

const (
	TransportUDP        TransportMode = 0 // UDP
	TransportTCPPassive TransportMode = 1 // TCP 被动：NVR 监听，设备连接
	TransportTCPActive  TransportMode = 2 // TCP 主动：NVR 连接设备
)

// buildTalkSDP 构建服务器 SDP answer
func buildTalkSDP(serverID, mediaIP string, port int, mode TransportMode, ssrc string) []byte {
	protocol := "RTP/AVP"
	if mode == TransportTCPPassive || mode == TransportTCPActive {
		protocol = "TCP/RTP/AVP"
	}

	sdpLines := []string{
		"v=0",
		fmt.Sprintf("o=%s 0 0 IN IP4 %s", serverID, mediaIP),
		"s=Play",
		fmt.Sprintf("c=IN IP4 %s", mediaIP),
		"t=0 0",
		fmt.Sprintf("m=audio %d %s 8", port, protocol),
		"a=rtpmap:8 PCMA/8000",
		"a=sendonly",
	}

	// TCP 模式添加 setup 和 connection 属性
	if mode == TransportTCPPassive {
		sdpLines = append(sdpLines, "a=setup:passive")
		sdpLines = append(sdpLines, "a=connection:new")
	} else if mode == TransportTCPActive {
		sdpLines = append(sdpLines, "a=setup:active")
		sdpLines = append(sdpLines, "a=connection:new")
	}

	// 添加 GB28181 扩展字段
	sdpLines = append(sdpLines, fmt.Sprintf("y=%s", ssrc))
	sdpLines = append(sdpLines, "f=v/a/1/8/1/8000")

	return []byte(strings.Join(sdpLines, "\r\n") + "\r\n")
}

// parseTalkSDP 解析设备 INVITE SDP
// 返回：peerIP, peerPort, isTCP, setupActive, ssrc, payloadType
func parseTalkSDP(sdpBody string) (string, int, bool, bool, string, int) {
	peerIP := ""
	peerPort := 0
	isTCP := false
	setupActive := false
	ssrc := ""
	payloadType := 8 // 默认 PCMA

	lines := strings.Split(sdpBody, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 2 {
			continue
		}

		switch {
		case strings.HasPrefix(line, "c="):
			// c=IN IP4 x.x.x.x
			parts := strings.Fields(line[2:])
			for i, p := range parts {
				if p == "IP4" && i+1 < len(parts) {
					peerIP = parts[i+1]
				}
			}
		case strings.HasPrefix(line, "m="):
			// m=audio port RTP/AVP 8
			parts := strings.Fields(line[2:])
			if len(parts) >= 3 {
				peerPort, _ = strconv.Atoi(parts[1])
				if parts[2] == "TCP/RTP/AVP" || parts[2] == "RTP/AVP/TCP" {
					isTCP = true
				}
				if len(parts) >= 4 {
					payloadType, _ = strconv.Atoi(parts[3])
				}
			}
		case strings.HasPrefix(line, "a=setup:"):
			// a=setup:active 或 a=setup:passive
			setup := strings.TrimPrefix(line, "a=setup:")
			setupActive = setup == "active"
		case strings.HasPrefix(line, "y="):
			// y=0000000033 (GB28181 SSRC)
			ssrc = strings.TrimPrefix(line, "y=")
		}
	}

	return peerIP, peerPort, isTCP, setupActive, ssrc, payloadType
}
