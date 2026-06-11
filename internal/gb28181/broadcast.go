package gb28181

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

// BroadcastSession represents a voice broadcast/intercom session.
type BroadcastSession struct {
	DeviceID    string
	ChannelID   string
	Session     *sipgo.DialogClientSession
	RTPConn     *net.UDPConn
	TCPConn     net.Conn
	TCPListener net.Listener
	RTPPort     int
	RTPPeerIP   string
	RTPPeerPort int
	SSRC        string
	CallID      string
	PayloadType uint8
	IsTCP       bool
	TCPPassive  bool
	AudioChan   chan []byte
	ReadyCh     chan struct{}
	ReadyOnce   sync.Once
	StopOnce    sync.Once
	IdleTimer   *time.Timer
	SeqNum      uint16
	Timestamp   uint32
	AudioBuffer []byte
	client      *sipgo.Client
	cfg         *Config
	stopped     bool
	mu          sync.Mutex
}

// BroadcastManager manages broadcast sessions.
type BroadcastManager struct {
	mu       sync.RWMutex
	sessions map[string]*BroadcastSession // key: deviceID_channelID
	client   *sipgo.Client
	cfg      *Config
}

// NewBroadcastManager creates a new broadcast manager.
func NewBroadcastManager(client *sipgo.Client, cfg *Config) *BroadcastManager {
	return &BroadcastManager{
		sessions: make(map[string]*BroadcastSession),
		client:   client,
		cfg:      cfg,
	}
}

func broadcastKey(deviceID, channelID string) string {
	return deviceID + "_" + channelID
}

// StartBroadcast starts a voice broadcast session to a device channel.
func (bm *BroadcastManager) StartBroadcast(deviceID, channelID string, store *DeviceStore) (*BroadcastSession, error) {
	key := broadcastKey(deviceID, channelID)

	bm.mu.Lock()
	if existing, ok := bm.sessions[key]; ok {
		bm.mu.Unlock()
		return existing, nil
	}
	bm.mu.Unlock()

	dev, ok := store.Load(deviceID)
	if !ok || !dev.IsOnline {
		return nil, ErrDeviceOffline
	}

	_, ok = dev.GetChannel(channelID)
	if !ok {
		return nil, ErrChannelNotExist
	}

	bs := &BroadcastSession{
		DeviceID:    deviceID,
		ChannelID:   channelID,
		AudioChan:   make(chan []byte, 100),
		ReadyCh:     make(chan struct{}),
		PayloadType: 8, // PCMA
		client:      bm.client,
		cfg:         bm.cfg,
	}

	// Generate SSRC
	bs.SSRC = fmt.Sprintf("%010d", randInt(1000000000, 9999999999))

	// Prepare UDP listener
	if err := bs.prepareRTPConn(); err != nil {
		return nil, fmt.Errorf("prepare RTP failed: %w", err)
	}

	bm.mu.Lock()
	bm.sessions[key] = bs
	bm.mu.Unlock()

	// Send broadcast notify to device
	if err := bs.sendBroadcastNotify(store); err != nil {
		bm.RemoveSession(key)
		return nil, fmt.Errorf("send broadcast notify failed: %w", err)
	}

	slog.Info("[Broadcast] session started", "device_id", deviceID, "channel_id", channelID, "port", bs.RTPPort)
	return bs, nil
}

// StopBroadcast stops a broadcast session.
func (bm *BroadcastManager) StopBroadcast(deviceID, channelID string) error {
	key := broadcastKey(deviceID, channelID)
	bm.mu.Lock()
	bs, ok := bm.sessions[key]
	if !ok {
		bm.mu.Unlock()
		return nil
	}
	delete(bm.sessions, key)
	bm.mu.Unlock()

	return bs.Stop()
}

// RemoveSession removes a session from the manager.
func (bm *BroadcastManager) RemoveSession(key string) {
	bm.mu.Lock()
	delete(bm.sessions, key)
	bm.mu.Unlock()
}

// GetSession returns a broadcast session by key.
func (bm *BroadcastManager) GetSession(deviceID, channelID string) (*BroadcastSession, bool) {
	key := broadcastKey(deviceID, channelID)
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	bs, ok := bm.sessions[key]
	return bs, ok
}

func (bs *BroadcastSession) prepareRTPConn() error {
	localAddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return err
	}

	bs.RTPConn = conn
	bs.RTPPort = conn.LocalAddr().(*net.UDPAddr).Port
	return nil
}

func (bs *BroadcastSession) sendBroadcastNotify(store *DeviceStore) error {
	dev, ok := store.Load(bs.DeviceID)
	if !ok {
		return ErrDeviceNotExist
	}

	uri := sip.Uri{
		Scheme: "sip",
		User:   bs.ChannelID,
		Host:   getHost(dev.Address),
		Port:   getPort(dev.Address),
	}

	req := sip.NewRequest(sip.MESSAGE, uri)
	req.AppendHeader(sip.NewHeader("Content-Type", "Application/MANSCDP+xml"))

	sourceID := bs.cfg.ID
	if sourceID == "" {
		sourceID = bs.DeviceID
	}

	sn := randInt(100000, 999999)
	body := fmt.Sprintf(`<?xml version="1.0" encoding="GB2312"?>
<Notify>
<CmdType>Broadcast</CmdType>
<SN>%d</SN>
<SourceID>%s</SourceID>
<TargetID>%s</TargetID>
</Notify>`, sn, sourceID, bs.ChannelID)
	req.SetBody([]byte(body))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := bs.client.Do(ctx, req)
	return err
}

// SendAudioData sends audio data to the broadcast session.
func (bs *BroadcastSession) SendAudioData(data []byte) error {
	bs.mu.Lock()
	if bs.stopped {
		bs.mu.Unlock()
		return fmt.Errorf("broadcast session stopped")
	}
	bs.mu.Unlock()

	select {
	case bs.AudioChan <- data:
		return nil
	default:
		return fmt.Errorf("audio channel full")
	}
}

// Stop stops the broadcast session.
func (bs *BroadcastSession) Stop() error {
	var err error
	bs.StopOnce.Do(func() {
		bs.mu.Lock()
		bs.stopped = true
		bs.mu.Unlock()

		if bs.IdleTimer != nil {
			bs.IdleTimer.Stop()
		}

		close(bs.AudioChan)

		if bs.Session != nil {
			_ = bs.Session.Bye(context.Background())
		}

		if bs.RTPConn != nil {
			bs.RTPConn.Close()
		}
		if bs.TCPConn != nil {
			bs.TCPConn.Close()
		}
		if bs.TCPListener != nil {
			bs.TCPListener.Close()
		}

		slog.Info("[Broadcast] session stopped", "device_id", bs.DeviceID, "channel_id", bs.ChannelID)
	})
	return err
}

// OnInvite handles a SIP INVITE from a device for broadcast/intercom.
func (bm *BroadcastManager) OnInvite(req *sip.Request, tx sip.ServerTransaction) {
	from := req.From()
	if from == nil || from.Address.User == "" {
		slog.Error("[Broadcast] OnInvite: invalid from header")
		return
	}

	deviceID := from.Address.User
	sdpBody := string(req.Body())

	slog.Info("[Broadcast] OnInvite received", "device_id", deviceID, "sdp_len", len(sdpBody))

	// Parse SDP to get peer info
	peerIP, peerPort, isTCP, payloadType := parseBroadcastSDP(sdpBody)

	// Find matching session
	bm.mu.RLock()
	var bs *BroadcastSession
	for _, s := range bm.sessions {
		if s.DeviceID == deviceID {
			bs = s
			break
		}
	}
	bm.mu.RUnlock()

	if bs == nil {
		slog.Warn("[Broadcast] OnInvite: no session found", "device_id", deviceID)
		tx.Respond(sip.NewResponseFromRequest(req, 404, "Not Found", nil))
		return
	}

	bs.RTPPeerIP = peerIP
	bs.RTPPeerPort = peerPort
	bs.IsTCP = isTCP
	if payloadType >= 0 {
		bs.PayloadType = uint8(payloadType)
	}

	// Build 200 OK response with SDP
	sdpIP := bs.cfg.MediaIP
	if sdpIP == "" {
		sdpIP = bs.cfg.Host
	}

	mediaPort := bs.RTPPort
	protocol := "RTP/AVP"
	if isTCP {
		protocol = "TCP/RTP/AVP"
	}

	responseSDP := fmt.Sprintf(`v=0
o=%s 0 0 IN IP4 %s
s=Play
c=IN IP4 %s
t=0 0
m=audio %d %s %d
a=sendonly
a=rtpmap:%d PCMA/8000
y=%s
f=v/a/1/8/1/8000
`, bs.cfg.ID, sdpIP, sdpIP, mediaPort, protocol, bs.PayloadType, bs.PayloadType, bs.SSRC)

	okResp := sip.NewResponseFromRequest(req, 200, "OK", nil)
	okResp.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	okResp.SetBody([]byte(responseSDP))

	if err := tx.Respond(okResp); err != nil {
		slog.Error("[Broadcast] OnInvite: send 200 OK failed", "error", err)
		return
	}

	// Start audio sender
	go bs.startAudioSender()

	// Notify ready
	bs.ReadyOnce.Do(func() {
		close(bs.ReadyCh)
	})

	slog.Info("[Broadcast] OnInvite: 200 OK sent", "device_id", deviceID, "port", mediaPort)
}

// OnAck handles a SIP ACK for broadcast.
func (bm *BroadcastManager) OnAck(req *sip.Request, tx sip.ServerTransaction) {
	from := req.From()
	if from == nil {
		return
	}
	deviceID := from.Address.User
	slog.Debug("[Broadcast] OnAck received", "device_id", deviceID)
}

// OnBye handles a SIP BYE for broadcast.
func (bm *BroadcastManager) OnBye(req *sip.Request, tx sip.ServerTransaction) {
	from := req.From()
	if from == nil {
		return
	}
	deviceID := from.Address.User

	bm.mu.Lock()
	for key, bs := range bm.sessions {
		if bs.DeviceID == deviceID {
			delete(bm.sessions, key)
			go bs.Stop()
			break
		}
	}
	bm.mu.Unlock()

	tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
	slog.Info("[Broadcast] OnBye: session stopped", "device_id", deviceID)
}

func (bs *BroadcastSession) startAudioSender() {
	slog.Info("[Broadcast] audio sender started", "device_id", bs.DeviceID)

	<-bs.ReadyCh

	for audioData := range bs.AudioChan {
		if err := bs.sendAudioData(audioData); err != nil {
			slog.Error("[Broadcast] send audio failed", "error", err)
		}
	}

	slog.Info("[Broadcast] audio sender stopped", "device_id", bs.DeviceID)
}

func (bs *BroadcastSession) sendAudioData(data []byte) error {
	if bs.RTPConn == nil || bs.RTPPeerIP == "" || bs.RTPPeerPort == 0 {
		return fmt.Errorf("RTP connection not ready")
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", bs.RTPPeerIP, bs.RTPPeerPort))
	if err != nil {
		return err
	}

	// Simple RTP packet construction
	bs.SeqNum++
	bs.Timestamp += uint32(len(data))

	// RTP header (12 bytes) + payload
	rtpPacket := make([]byte, 12+len(data))
	rtpPacket[0] = 0x80 // V=2
	rtpPacket[1] = bs.PayloadType
	rtpPacket[2] = byte(bs.SeqNum >> 8)
	rtpPacket[3] = byte(bs.SeqNum)
	rtpPacket[4] = byte(bs.Timestamp >> 24)
	rtpPacket[5] = byte(bs.Timestamp >> 16)
	rtpPacket[6] = byte(bs.Timestamp >> 8)
	rtpPacket[7] = byte(bs.Timestamp)
	// SSRC
	ssrc := uint32(randInt(0, 0x7FFFFFFF))
	rtpPacket[8] = byte(ssrc >> 24)
	rtpPacket[9] = byte(ssrc >> 16)
	rtpPacket[10] = byte(ssrc >> 8)
	rtpPacket[11] = byte(ssrc)
	copy(rtpPacket[12:], data)

	_, err = bs.RTPConn.WriteToUDP(rtpPacket, remoteAddr)
	return err
}

// parseBroadcastSDP extracts peer IP, port, transport, and codec from SDP.
func parseBroadcastSDP(sdpBody string) (ip string, port int, isTCP bool, payloadType int) {
	payloadType = 8 // default PCMA
	port = 0
	isTCP = false

	lines := splitLines(sdpBody)
	for _, line := range lines {
		switch {
		case len(line) > 2 && line[:2] == "c=":
			// c=IN IP4 x.x.x.x
			parts := splitFields(line[2:])
			for i, p := range parts {
				if p == "IP4" && i+1 < len(parts) {
					ip = parts[i+1]
				}
			}
		case len(line) > 2 && line[:2] == "m=":
			// m=audio port RTP/AVP 8
			parts := splitFields(line[2:])
			if len(parts) >= 3 {
				fmt.Sscanf(parts[1], "%d", &port)
				if parts[2] == "TCP/RTP/AVP" || parts[2] == "RTP/AVP/TCP" {
					isTCP = true
				}
				if len(parts) >= 4 {
					fmt.Sscanf(parts[3], "%d", &payloadType)
				}
			}
		}
	}
	return
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, current)
			current = ""
		} else if c != '\r' {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func splitFields(s string) []string {
	var fields []string
	current := ""
	for _, c := range s {
		if c == ' ' || c == '\t' {
			if current != "" {
				fields = append(fields, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		fields = append(fields, current)
	}
	return fields
}
