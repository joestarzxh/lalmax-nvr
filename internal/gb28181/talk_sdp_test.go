package gb28181

import (
	"strings"
	"testing"
)

func TestBuildTalkSDP_UDP(t *testing.T) {
	sdp := buildTalkSDP("server001", "10.0.0.1", 8000, TransportUDP, "1234567890")
	sdpStr := string(sdp)

	if !strings.Contains(sdpStr, "m=audio 8000 RTP/AVP 8") {
		t.Error("UDP SDP should use RTP/AVP")
	}
	if !strings.Contains(sdpStr, "a=sendonly") {
		t.Error("SDP should have sendonly attribute")
	}
	if !strings.Contains(sdpStr, "y=1234567890") {
		t.Error("SDP should have SSRC")
	}
	if strings.Contains(sdpStr, "a=setup:") {
		t.Error("UDP SDP should not have setup attribute")
	}
}

func TestBuildTalkSDP_TCPPassive(t *testing.T) {
	sdp := buildTalkSDP("server001", "10.0.0.1", 8000, TransportTCPPassive, "1234567890")
	sdpStr := string(sdp)

	if !strings.Contains(sdpStr, "m=audio 8000 TCP/RTP/AVP 8") {
		t.Error("TCP SDP should use TCP/RTP/AVP")
	}
	if !strings.Contains(sdpStr, "a=setup:passive") {
		t.Error("TCP passive SDP should have setup:passive")
	}
	if !strings.Contains(sdpStr, "a=connection:new") {
		t.Error("TCP SDP should have connection:new")
	}
}

func TestBuildTalkSDP_TCPActive(t *testing.T) {
	sdp := buildTalkSDP("server001", "10.0.0.1", 8000, TransportTCPActive, "1234567890")
	sdpStr := string(sdp)

	if !strings.Contains(sdpStr, "a=setup:active") {
		t.Error("TCP active SDP should have setup:active")
	}
}

func TestParseTalkSDP_UDP(t *testing.T) {
	sdp := `v=0
o=32011100491327000001 0 0 IN IP4 10.0.15.209
s=Play
c=IN IP4 10.0.15.209
t=0 0
m=audio 15218 RTP/AVP 8
a=recvonly
a=rtpmap:8 PCMA/8000
y=0000000033
f=v/////a/1/8/1`

	peerIP, peerPort, isTCP, setupActive, ssrc, payloadType, err := parseTalkSDP(sdp)
	if err != nil {
		t.Fatalf("parseTalkSDP failed: %v", err)
	}

	if peerIP != "10.0.15.209" {
		t.Errorf("Expected peerIP 10.0.15.209, got %s", peerIP)
	}
	if peerPort != 15218 {
		t.Errorf("Expected peerPort 15218, got %d", peerPort)
	}
	if isTCP {
		t.Error("Expected UDP mode")
	}
	if setupActive {
		t.Error("Expected passive setup")
	}
	if ssrc != "0000000033" {
		t.Errorf("Expected ssrc 0000000033, got %s", ssrc)
	}
	if payloadType != 8 {
		t.Errorf("Expected payloadType 8, got %d", payloadType)
	}
}

func TestParseTalkSDP_TCPActive(t *testing.T) {
	sdp := `v=0
o=32011100491327000001 0 0 IN IP4 10.0.15.209
s=Play
c=IN IP4 10.0.15.209
t=0 0
m=audio 15218 TCP/RTP/AVP 8
a=recvonly
a=rtpmap:8 PCMA/8000
a=setup:active
y=0000000033`

	_, _, isTCP, setupActive, _, _, err := parseTalkSDP(sdp)
	if err != nil {
		t.Fatalf("parseTalkSDP failed: %v", err)
	}

	if !isTCP {
		t.Error("Expected TCP mode")
	}
	if !setupActive {
		t.Error("Expected active setup")
	}
}
