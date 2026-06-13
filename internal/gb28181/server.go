package gb28181

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/lalmax-pro/lalmax-nvr/internal/media"
	"github.com/lalmax-pro/lalmax-nvr/internal/storage"
)

// Server is the GB28181 SIP signaling server.
type Server struct {
	ua        *sipgo.UserAgent
	srv       *sipgo.Server
	client    *sipgo.Client
	gb        *GB28181API
	cfg       *Config
	store     *DeviceStore
	cancel    context.CancelFunc
	platforms *PlatformManager
	broadcast *BroadcastManager
	talk      *TalkManager
	alarm     *AlarmManager
	download  *DownloadManager
}

// NewServer creates and starts a GB28181 SIP server.
func NewServer(cfg *Config, mediaEngine media.Engine, db *storage.DB) (*Server, func()) {
	store := NewDeviceStore(db)
	
	// Load devices from database
	if err := store.LoadFromDB(); err != nil {
		slog.Error("failed to load GB28181 devices from database", "error", err)
	}

	sipHost := cfg.Host
	if sipHost == "" {
		sipHost = cfg.MediaIP
	}

	ua, err := sipgo.NewUA(
		sipgo.WithUserAgent("lalmax-nvr"),
		sipgo.WithUserAgentHostname(sipHost),
	)
	if err != nil {
		slog.Error("failed to create SIP UA", "error", err)
		return nil, func() {}
	}

	srv, err := sipgo.NewServer(ua)
	if err != nil {
		slog.Error("failed to create SIP server", "error", err)
		ua.Close()
		return nil, func() {}
	}

	client, err := sipgo.NewClient(ua, sipgo.WithClientHostname(sipHost))
	if err != nil {
		slog.Error("failed to create SIP client", "error", err)
		srv.Close()
		ua.Close()
		return nil, func() {}
	}

	s := &Server{
		ua:     ua,
		srv:    srv,
		client: client,
		cfg:    cfg,
		store:  store,
	}

	api := NewGB28181API(cfg, store, client, mediaEngine)
	api.svr = s
	s.gb = api

	// Initialize managers
	s.platforms = NewPlatformManager(client, cfg.Host, cfg.MediaIP, cfg.ID, cfg.Password, store.GetDB())
	s.broadcast = NewBroadcastManager(client, cfg)
	s.talk = NewTalkManager(client, cfg)
	s.alarm = NewAlarmManager(client, cfg, store.GetDB())
	s.download = NewDownloadManager(client, cfg, mediaEngine, store.GetDB(), "")

	// Register SIP handlers
	srv.OnRegister(api.handlerRegister)
	srv.OnMessage(api.handlerMessage)
	srv.OnNotify(api.handlerNotify)
	srv.OnBye(api.handlerBye)
	srv.OnInvite(s.handlerInvite)
	srv.OnAck(s.handlerAck)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	go func() {
		addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
		slog.Info("GB28181 SIP UDP server starting", "addr", addr)
		if err := srv.ListenAndServe(ctx, "udp", addr); err != nil {
			slog.Error("SIP UDP listener error", "error", err)
		}
	}()
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
		slog.Info("GB28181 SIP TCP server starting", "addr", addr)
		if err := srv.ListenAndServe(ctx, "tcp", addr); err != nil {
			slog.Error("SIP TCP listener error", "error", err)
		} else {
			slog.Info("GB28181 SIP TCP server stopped")
		}
	}()
	go s.startTickerCheck()

	// Load upstream platforms
	go func() {
		if err := s.platforms.LoadPlatforms(); err != nil {
			slog.Error("failed to load GB28181 platforms", "error", err)
		}
	}()

	return s, s.Stop
}

// startTickerCheck periodically checks for offline devices via heartbeat timeout.
func (s *Server) startTickerCheck() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		s.store.RangeDevices(func(key string, dev *Device) bool {
			if !dev.IsOnline {
				return true
			}
			if len(key) < 18 {
				return true
			}

			interval := dev.keepaliveInterval
			if interval == 0 {
				interval = 60
			}
			timeoutCount := dev.keepaliveTimeout
			if timeoutCount == 0 {
				timeoutCount = 3
			}
			timeout := time.Duration(interval) * time.Duration(timeoutCount) * time.Second

			if dev.LastKeepaliveAt.IsZero() {
				if !dev.LastRegisterAt.IsZero() && now.Sub(dev.LastRegisterAt) >= timeout {
					s.gb.logout(key)
				}
				return true
			}

			if sub := now.Sub(dev.LastKeepaliveAt); sub >= timeout {
				slog.Info("device offline detected", "device_id", key, "elapsed", sub)
				s.gb.logout(key)
			}
			return true
		})
	}
}

// QueryCatalog queries the catalog of a device.
func (s *Server) QueryCatalog(deviceID string) error {
	return s.gb.QueryCatalog(deviceID)
}

// Play starts a GB28181 play session.
func (s *Server) Play(in *PlayInput) (string, error) {
	return s.gb.Play(in)
}

// StopPlay stops a GB28181 play session.
func (s *Server) StopPlay(in *StopPlayInput) error {
	return s.gb.StopPlay(in)
}

// PTZControl sends a PTZ control command.
func (s *Server) PTZControl(deviceID, channelID, ptzCmd string) error {
	return s.gb.PTZControl(deviceID, channelID, ptzCmd)
}

// QueryRecordInfo queries a device for its recording list.
func (s *Server) QueryRecordInfo(deviceID, channelID string, startTime, endTime time.Time) (*Records, error) {
	return s.gb.QueryRecordInfo(deviceID, channelID, startTime, endTime)
}

// Playback starts a historical playback session.
func (s *Server) Playback(in *PlaybackInput) (string, error) {
	return s.gb.Playback(in)
}

// IsStreamPlaying reports whether the given stream ID has an active GB28181 play session.
func (s *Server) IsStreamPlaying(streamID string) bool {
	return globalStreams.IsStreamPlaying(streamID)
}

// GetPlatforms returns the platform manager.
func (s *Server) GetPlatforms() *PlatformManager {
	return s.platforms
}

// GetBroadcastManager returns the broadcast manager.
func (s *Server) GetBroadcastManager() *BroadcastManager {
	return s.broadcast
}

// GetTalkManager returns the talk manager.
func (s *Server) GetTalkManager() *TalkManager {
	return s.talk
}

// GetAlarmManager returns the alarm manager.
func (s *Server) GetAlarmManager() *AlarmManager {
	return s.alarm
}

// GetDownloadManager returns the download manager.
func (s *Server) GetDownloadManager() *DownloadManager {
	return s.download
}

// GetDeviceStore returns the device store.
func (s *Server) GetDeviceStore() *DeviceStore {
	return s.store
}

// GetDB returns the database.
func (s *Server) GetDB() *storage.DB {
	return s.store.db
}

// GetConfig returns the GB28181 configuration.
func (s *Server) GetConfig() *Config {
	return s.cfg
}

// handlerInvite handles incoming SIP INVITE requests (from devices for broadcast or from upstream platforms for playback).
func (s *Server) handlerInvite(req *sip.Request, tx sip.ServerTransaction) {
	from := req.From()
	if from == nil || from.Address.User == "" {
		slog.Error("[SIP] INVITE: invalid from header")
		tx.Respond(sip.NewResponseFromRequest(req, 400, "Bad Request", nil))
		return
	}

	fromUser := from.Address.User
	sdpBody := string(req.Body())

	slog.Info("[SIP] INVITE received", "from", fromUser, "sdp_len", len(sdpBody))

	// Check if this is from an upstream platform (by matching ServerGBID)
	isUpstream := false
	s.platforms.mu.RLock()
	for _, p := range s.platforms.platforms {
		if p.Config.ServerGBID == fromUser || p.Config.DeviceGBID == fromUser {
			isUpstream = true
			break
		}
	}
	s.platforms.mu.RUnlock()

	if isUpstream {
		// This is from upstream platform - handle as platform INVITE
		slog.Info("[SIP] INVITE from upstream platform", "from", fromUser)
		// Forward to platform handler
		tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
		return
	}

	// Check for pending talk sessions first
	if from != nil {
		to := req.To()
		if to != nil {
			if _, ok := s.talk.GetSession(from.Address.User, to.Address.User); ok {
				s.talk.OnInvite(req, tx)
				return
			}
		}
	}

	// Fall through to broadcast
	s.broadcast.OnInvite(req, tx)
}

// handlerAck handles incoming SIP ACK requests.
func (s *Server) handlerAck(req *sip.Request, tx sip.ServerTransaction) {
	from := req.From()
	if from == nil {
		return
	}

	fromUser := from.Address.User

	// Check if this is for a broadcast session
	s.broadcast.mu.RLock()
	for _, bs := range s.broadcast.sessions {
		if bs.DeviceID == fromUser {
			s.broadcast.mu.RUnlock()
			s.broadcast.OnAck(req, tx)
			return
		}
	}
	s.broadcast.mu.RUnlock()

	slog.Debug("[SIP] ACK received", "from", fromUser)
}

// ListDevices returns all registered devices with their channels.
func (s *Server) ListDevices() []map[string]interface{} {
	var result []map[string]interface{}
	s.store.RangeDevices(func(deviceID string, dev *Device) bool {
		channels := make([]map[string]interface{}, 0)
		dev.Channels.Range(func(k, v any) bool {
			ch := v.(*Channel)
			channels = append(channels, map[string]interface{}{
				"channel_id":  ch.ChannelID,
				"is_playing":  globalStreams.isPlaying(deviceID, ch.ChannelID),
				"stream_id":   StreamID(deviceID, ch.ChannelID),
			})
			return true
		})
		
		// Get device info from database
		deviceInfo := map[string]interface{}{
			"device_id": deviceID,
			"is_online": dev.IsOnline,
			"address":   dev.Address,
			"channels":  channels,
		}
		
		// Try to get additional info from database
		if dbDev, err := s.store.GetDB().GetGB28181Device(context.Background(), deviceID); err == nil && dbDev != nil {
			deviceInfo["name"] = dbDev.Name
			deviceInfo["manufacturer"] = dbDev.Manufacturer
			deviceInfo["model"] = dbDev.Model
			deviceInfo["firmware"] = dbDev.Firmware
			if dbDev.LastKeepaliveAt != nil {
				deviceInfo["last_keepalive_at"] = dbDev.LastKeepaliveAt
			}
			if dbDev.LastRegisterAt != nil {
				deviceInfo["last_register_at"] = dbDev.LastRegisterAt
			}
		}
		
		result = append(result, deviceInfo)
		return true
	})
	return result
}

// Stop stops the SIP server.
func (s *Server) Stop() {
	slog.Info("[SIP] stopping GB28181 server")
	if s.cancel != nil {
		s.cancel()
	}
	// Stop platform manager
	if s.platforms != nil {
		s.platforms.mu.Lock()
		for _, p := range s.platforms.platforms {
			p.Stop()
		}
		s.platforms.mu.Unlock()
	}
	// Stop talk manager
	if s.talk != nil {
		s.talk.StopAll()
	}
	// Small delay to allow listeners to close
	time.Sleep(100 * time.Millisecond)
	if s.client != nil {
		s.client.Close()
	}
	if s.srv != nil {
		s.srv.Close()
	}
	if s.ua != nil {
		s.ua.Close()
	}
	slog.Info("[SIP] GB28181 server stopped")
}

// resolveHost resolves a hostname to IP address.
func resolveHost(host string) string {
	if host == "" {
		return ""
	}
	if net.ParseIP(host) != nil {
		return host
	}
	addrs, err := net.LookupHost(host)
	if err != nil || len(addrs) == 0 {
		return host
	}
	return addrs[0]
}
