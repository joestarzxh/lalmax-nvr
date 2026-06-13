package gb28181

import (
	"testing"
)

func TestNewTalkSession(t *testing.T) {
	cfg := &Config{
		ID:      "server001",
		MediaIP: "10.0.0.1",
		Port:    5060,
	}

	session := NewTalkSession("device001", "channel001", TransportUDP, cfg, nil)

	if session.DeviceID != "device001" {
		t.Errorf("Expected DeviceID device001, got %s", session.DeviceID)
	}
	if session.ChannelID != "channel001" {
		t.Errorf("Expected ChannelID channel001, got %s", session.ChannelID)
	}
	if session.TransportMode != TransportUDP {
		t.Errorf("Expected TransportMode UDP, got %d", session.TransportMode)
	}
	if session.PayloadType != 8 {
		t.Errorf("Expected PayloadType 8, got %d", session.PayloadType)
	}
}

func TestBuildRTPPacket(t *testing.T) {
	session := &TalkSession{
		PayloadType: 8,
		SSRC:        "1234567890",
	}

	data := []byte{0x01, 0x02, 0x03, 0x04}
	packet, err := session.buildRTPPacket(data)
	if err != nil {
		t.Fatalf("buildRTPPacket failed: %v", err)
	}

	// 检查 RTP 头
	if packet[0] != 0x80 {
		t.Errorf("Expected version 2, got %d", packet[0]>>6)
	}
	if packet[1] != 8 {
		t.Errorf("Expected payload type 8, got %d", packet[1])
	}

	// 检查序列号
	seqNum := uint16(packet[2])<<8 | uint16(packet[3])
	if seqNum != 1 {
		t.Errorf("Expected seqNum 1, got %d", seqNum)
	}

	// 检查时间戳
	timestamp := uint32(packet[4])<<24 | uint32(packet[5])<<16 | uint32(packet[6])<<8 | uint32(packet[7])
	if timestamp != 160 {
		t.Errorf("Expected timestamp 160, got %d", timestamp)
	}

	// 检查 payload
	if len(packet) != 12+4 {
		t.Errorf("Expected packet length 16, got %d", len(packet))
	}
}

func TestParseSSRC(t *testing.T) {
	ssrc, err := parseSSRC("1234567890")
	if err != nil {
		t.Fatalf("parseSSRC failed: %v", err)
	}
	if ssrc != 1234567890 {
		t.Errorf("Expected ssrc 1234567890, got %d", ssrc)
	}
}

func TestParseSSRCError(t *testing.T) {
	_, err := parseSSRC("not_a_number")
	if err == nil {
		t.Error("Expected error for invalid SSRC, got nil")
	}
}
