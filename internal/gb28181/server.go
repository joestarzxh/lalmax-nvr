package gb28181

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/emiago/sipgo"
	"github.com/lalmax-pro/lalmax-nvr/internal/media"
)

// Server is the GB28181 SIP signaling server.
type Server struct {
	ua     *sipgo.UserAgent
	srv    *sipgo.Server
	client *sipgo.Client
	gb     *GB28181API
	cfg    *Config
	store  *DeviceStore
	cancel context.CancelFunc
}

// NewServer creates and starts a GB28181 SIP server.
func NewServer(cfg *Config, mediaEngine media.Engine) (*Server, func()) {
	store := NewDeviceStore()

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

	// Register SIP handlers
	srv.OnRegister(api.handlerRegister)
	srv.OnMessage(api.handlerMessage)
	srv.OnNotify(api.handlerNotify)
	srv.OnBye(api.handlerBye)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	go func() {
		addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
		slog.Info("GB28181 SIP server starting", "addr", addr)
		if err := srv.ListenAndServe(ctx, "udp", addr); err != nil {
			slog.Error("SIP UDP listener error", "error", err)
		}
	}()
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
		if err := srv.ListenAndServe(ctx, "tcp", addr); err != nil {
			slog.Error("SIP TCP listener error", "error", err)
		}
	}()
	go s.startTickerCheck()

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

// ListDevices returns all registered devices with their channels.
func (s *Server) ListDevices() []map[string]interface{} {
	var result []map[string]interface{}
	s.store.RangeDevices(func(deviceID string, dev *Device) bool {
		channels := make([]map[string]interface{}, 0)
		dev.Channels.Range(func(k, v any) bool {
			ch := v.(*Channel)
			channels = append(channels, map[string]interface{}{
				"channel_id": ch.ChannelID,
			})
			return true
		})
		result = append(result, map[string]interface{}{
			"device_id": deviceID,
			"is_online": dev.IsOnline,
			"address":   dev.Address,
			"channels":  channels,
		})
		return true
	})
	return result
}

// Stop stops the SIP server.
func (s *Server) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.client != nil {
		s.client.Close()
	}
	if s.srv != nil {
		s.srv.Close()
	}
	if s.ua != nil {
		s.ua.Close()
	}
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
