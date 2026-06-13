package gb28181

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
)

func TestParseBroadcastSDPFromDeviceInvite(t *testing.T) {
	sdp := "v=0\r\n" +
		"o=32011100491377000002 0 0 IN IP4 10.0.15.209\r\n" +
		"s=Play\r\n" +
		"c=IN IP4 10.0.15.209\r\n" +
		"t=0 0\r\n" +
		"m=audio 15218 TCP/RTP/AVP 8\r\n" +
		"a=recvonly\r\n" +
		"a=rtpmap:8 PCMA/8000\r\n" +
		"a=setup:active\r\n" +
		"y=0000000033\r\n" +
		"f=v/////a/1/8/1\r\n"

	offer := parseBroadcastSDP(sdp)
	if offer.ChannelID != "32011100491377000002" {
		t.Fatalf("channel id = %q", offer.ChannelID)
	}
	if offer.IP != "10.0.15.209" {
		t.Fatalf("ip = %q", offer.IP)
	}
	if offer.Port != 15218 {
		t.Fatalf("port = %d", offer.Port)
	}
	if !offer.IsTCP {
		t.Fatal("expected TCP offer")
	}
	if !offer.TCPActive {
		t.Fatal("expected active TCP setup")
	}
	if offer.PayloadType != 8 {
		t.Fatalf("payload type = %d", offer.PayloadType)
	}
	if offer.SSRC != 33 {
		t.Fatalf("ssrc = %d", offer.SSRC)
	}
}

func TestParseBroadcastSDPWithPassiveTCPSetup(t *testing.T) {
	sdp := "v=0\r\n" +
		"o=34020000001320000001 0 0 IN IP4 192.0.2.10\r\n" +
		"s=Play\r\n" +
		"c=IN IP4 192.0.2.10\r\n" +
		"t=0 0\r\n" +
		"m=audio 30000 TCP/RTP/AVP 8\r\n" +
		"a=recvonly\r\n" +
		"a=rtpmap:8 PCMA/8000\r\n" +
		"a=setup:passive\r\n" +
		"y=0000000042\r\n"

	offer := parseBroadcastSDP(sdp)
	if !offer.IsTCP {
		t.Fatal("expected TCP offer")
	}
	if offer.TCPActive {
		t.Fatal("expected passive TCP setup")
	}
	if offer.Port != 30000 {
		t.Fatalf("port = %d", offer.Port)
	}
	if offer.SSRC != 42 {
		t.Fatalf("ssrc = %d", offer.SSRC)
	}
}

func TestNormalizeMediaIPFallsBackToSIPSource(t *testing.T) {
	if got := normalizeMediaIP("10.0.15.209", "192.0.2.10:5060"); got != "10.0.15.209" {
		t.Fatalf("valid media ip fallback = %q", got)
	}
	if got := normalizeMediaIP("0.0.0.0", "192.0.2.10:5060"); got != "192.0.2.10" {
		t.Fatalf("zero media ip fallback = %q", got)
	}
	if got := normalizeMediaIP("", "192.0.2.10:5060"); got != "192.0.2.10" {
		t.Fatalf("empty media ip fallback = %q", got)
	}
	if got := normalizeMediaIP("", "192.0.2.10"); got != "192.0.2.10" {
		t.Fatalf("raw source fallback = %q", got)
	}
}

func TestAdvertisedMediaIPRejectsWildcardAddress(t *testing.T) {
	if got := advertisedMediaIP(&Config{MediaIP: "192.0.2.20"}); got != "192.0.2.20" {
		t.Fatalf("advertised media ip = %q", got)
	}
	if got := advertisedMediaIP(&Config{MediaIP: "0.0.0.0", Host: "192.0.2.30"}); got != "192.0.2.30" {
		t.Fatalf("host fallback media ip = %q", got)
	}
	got := advertisedMediaIP(&Config{MediaIP: "0.0.0.0", Host: "0.0.0.0"})
	if got == "" || got == "0.0.0.0" {
		t.Fatalf("wildcard advertised media ip = %q", got)
	}
}

func TestSendRTPPayloadOverTCPUsesLengthPrefixedPacket(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	bs := &BroadcastSession{
		TCPConn:     client,
		IsTCP:       true,
		PayloadType: 8,
		RTPSSRC:     33,
	}
	payload := make([]byte, 160)

	errCh := make(chan error, 1)
	go func() {
		errCh <- bs.sendRTPPayload(nil, payload)
	}()

	lengthBuf := make([]byte, 2)
	if _, err := io.ReadFull(server, lengthBuf); err != nil {
		t.Fatal(err)
	}
	packetLen := int(binary.BigEndian.Uint16(lengthBuf))
	if packetLen != 12+len(payload) {
		t.Fatalf("packet length = %d", packetLen)
	}

	packet := make([]byte, packetLen)
	if _, err := io.ReadFull(server, packet); err != nil {
		t.Fatal(err)
	}
	if packet[0] != 0x80 || packet[1] != 8 {
		t.Fatalf("unexpected RTP header prefix: % x", packet[:2])
	}
	gotSSRC := binary.BigEndian.Uint32(packet[8:12])
	if gotSSRC != 33 {
		t.Fatalf("ssrc = %d", gotSSRC)
	}

	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}
