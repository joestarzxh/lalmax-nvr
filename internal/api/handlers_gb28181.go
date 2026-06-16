package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lalmax-pro/lalmax-nvr/internal/camera"
	"github.com/lalmax-pro/lalmax-nvr/internal/gb28181"
	"github.com/lalmax-pro/lalmax-nvr/internal/media"
	"github.com/lalmax-pro/lalmax-nvr/internal/storage"
)

// GB28181Handler provides HTTP API endpoints for GB28181 management.
type GB28181Handler struct {
	svr         *gb28181.Server
	camMgr      *camera.CameraManager
	db          *storage.DB
	mediaEngine media.Engine
}

// NewGB28181Handler creates a new GB28181Handler.
func NewGB28181Handler(svr *gb28181.Server, camMgr *camera.CameraManager, db *storage.DB, mediaEngine media.Engine) *GB28181Handler {
	return &GB28181Handler{
		svr:         svr,
		camMgr:      camMgr,
		db:          db,
		mediaEngine: mediaEngine,
	}
}

// ListDevices returns all registered GB28181 devices.
func (h *GB28181Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"devices": []interface{}{},
		})
		return
	}
	devices := h.svr.ListDevices()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"devices": devices,
	})
}

// PlayRequest is the request body for the play endpoint.
type PlayRequest struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
	StreamID  string `json:"stream_id,omitempty"` // optional internal stream ID
}

// Play starts a GB28181 play session.
func (h *GB28181Handler) Play(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req PlayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	streamID := req.StreamID
	if streamID == "" {
		streamID = gb28181.StreamID(req.DeviceID, req.ChannelID)
	}

	ssrc, err := h.svr.Play(&gb28181.PlayInput{
		DeviceID:   req.DeviceID,
		ChannelID:  req.ChannelID,
		StreamMode: 0, // Default to UDP
		InternalID: streamID,
	})
	if err != nil {
		slog.Error("GB28181 play failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := h.ensureGB28181Camera(r.Context(), req.DeviceID, req.ChannelID, streamID); err != nil {
		slog.Warn("GB28181 play succeeded but camera registration failed",
			"device_id", req.DeviceID,
			"channel_id", req.ChannelID,
			"stream_id", streamID,
			"error", err,
		)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ssrc":      ssrc,
		"stream_id": streamID,
	})
}

func (h *GB28181Handler) ensureGB28181Camera(ctx context.Context, deviceID, channelID, streamID string) error {
	if h.db == nil {
		return fmt.Errorf("database unavailable")
	}

	if h.camMgr != nil {
		if cam := h.camMgr.GetCameraConfig(streamID); cam != nil && cam.Enabled {
			return nil
		}
	}

	existing, err := h.db.GetCamera(ctx, streamID)
	if err != nil {
		return err
	}
	if existing != nil && !existing.Archived {
		return nil
	}

	name := fmt.Sprintf("GB28181 %s", channelID)
	cameraURL := ""
	if h.mediaEngine != nil {
		playURL, err := h.mediaEngine.BuildPlayURL(ctx, media.PlayURLRequest{
			StreamID: streamID,
			AppName:  "live",
			Protocol: "rtsp",
		})
		if err == nil && playURL != nil && playURL.URL != "" {
			cameraURL = playURL.URL
		}
	}
	if cameraURL == "" {
		cameraURL = fmt.Sprintf("rtsp://127.0.0.1:5544/live/%s", streamID)
	}

	if existing != nil && existing.Archived {
		if err := h.db.UnarchiveCameraDB(ctx, streamID); err != nil {
			return err
		}
	}

	// Only persist to DB for stream-management mapping. Do not add to CameraManager
	// config — GB28181 devices are managed via the GB28181 device API, not the
	// ONVIF/RTSP camera list.
	return h.db.UpsertCamera(ctx, streamID, name, "gb28181", "h264", cameraURL, "", "", true, "", "", "", "")
}

// StopPlayRequest is the request body for the stop play endpoint.
type StopPlayRequest struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
}

// StopPlay stops a GB28181 play session.
func (h *GB28181Handler) StopPlay(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req StopPlayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	if err := h.svr.StopPlay(&gb28181.StopPlayInput{
		DeviceID:  req.DeviceID,
		ChannelID: req.ChannelID,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PTZRequest is the request body for the PTZ control endpoint.
type PTZRequest struct {
	DeviceID  string  `json:"device_id"`
	ChannelID string  `json:"channel_id"`
	Direction string  `json:"direction"` // up, down, left, right, upleft, upright, downleft, downright, zoomin, zoomout, stop
	Speed     float64 `json:"speed"`     // 0.0-1.0
}

// PTZControl sends a PTZ control command.
func (h *GB28181Handler) PTZControl(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req PTZRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	var ptzCmd string
	if req.Direction == "stop" {
		ptzCmd = gb28181.BuildStop()
	} else {
		if req.Speed <= 0 {
			req.Speed = 0.5
		}
		ptzCmd = gb28181.BuildContinuousMove(req.Direction, req.Speed)
	}

	if ptzCmd == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid direction"})
		return
	}

	if err := h.svr.PTZControl(req.DeviceID, req.ChannelID, ptzCmd); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// RecordInfoRequest is the request body for the record info endpoint.
type RecordInfoRequest struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
	StartTime string `json:"start_time"` // RFC3339 or "2006-01-02T15:04:05"
	EndTime   string `json:"end_time"`   // RFC3339 or "2006-01-02T15:04:05"
}

// RecordInfo queries a device for its recording list.
func (h *GB28181Handler) RecordInfo(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req RecordInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	startTime, err := parseTime(req.StartTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_time format"})
		return
	}
	endTime, err := parseTime(req.EndTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_time format"})
		return
	}

	records, err := h.svr.QueryRecordInfo(req.DeviceID, req.ChannelID, startTime, endTime)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, records)
}

// PlaybackRequest is the request body for the playback endpoint.
type PlaybackRequest struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
	StreamID  string `json:"stream_id,omitempty"`
	StartTime string `json:"start_time"` // RFC3339 or "2006-01-02T15:04:05"
	EndTime   string `json:"end_time"`   // RFC3339 or "2006-01-02T15:04:05"
}

// Playback starts a historical video playback session.
func (h *GB28181Handler) Playback(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req PlaybackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	startTime, err := parseTime(req.StartTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_time format"})
		return
	}
	endTime, err := parseTime(req.EndTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_time format"})
		return
	}

	streamID := req.StreamID
	if streamID == "" {
		streamID = gb28181.StreamID(req.DeviceID, req.ChannelID) + "_pb"
	}

	ssrc, err := h.svr.Playback(&gb28181.PlaybackInput{
		DeviceID:   req.DeviceID,
		ChannelID:  req.ChannelID,
		StreamMode: 0,
		InternalID: streamID,
		StartTime:  startTime,
		EndTime:    endTime,
	})
	if err != nil {
		slog.Error("GB28181 playback failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Build play URLs for all supported protocols
	playURLs := buildGB28181PlayURLs(r.Context(), h.mediaEngine, streamID, "rtp")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ssrc":      ssrc,
		"stream_id": streamID,
		"urls":      playURLs,
	})
}

// PlaySpeed changes the playback speed.
func (h *GB28181Handler) PlaySpeed(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req struct {
		DeviceID  string  `json:"device_id"`
		ChannelID string  `json:"channel_id"`
		Speed     float32 `json:"speed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	if req.Speed != 0.5 && req.Speed != 1 && req.Speed != 2 && req.Speed != 4 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "speed must be 0.5, 1, 2, or 4"})
		return
	}

	if err := h.svr.PlaySpeed(&gb28181.PlaySpeedInput{
		DeviceID:  req.DeviceID,
		ChannelID: req.ChannelID,
		Speed:     req.Speed,
	}); err != nil {
		slog.Error("GB28181 play speed failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PlaySeek seeks to a position in playback.
func (h *GB28181Handler) PlaySeek(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req struct {
		DeviceID  string `json:"device_id"`
		ChannelID string `json:"channel_id"`
		SeekTime  int64  `json:"seek_time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	if err := h.svr.PlaySeek(&gb28181.PlaySeekInput{
		DeviceID:  req.DeviceID,
		ChannelID: req.ChannelID,
		SeekTime:  req.SeekTime,
	}); err != nil {
		slog.Error("GB28181 play seek failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PlayPause pauses playback.
func (h *GB28181Handler) PlayPause(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req struct {
		DeviceID  string `json:"device_id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	if err := h.svr.PlayPause(&gb28181.PlayPauseInput{
		DeviceID:  req.DeviceID,
		ChannelID: req.ChannelID,
	}); err != nil {
		slog.Error("GB28181 play pause failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PlayResume resumes playback.
func (h *GB28181Handler) PlayResume(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req struct {
		DeviceID  string `json:"device_id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	if err := h.svr.PlayResume(&gb28181.PlayPauseInput{
		DeviceID:  req.DeviceID,
		ChannelID: req.ChannelID,
	}); err != nil {
		slog.Error("GB28181 play resume failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ==================== Platform API ====================

// ListPlatforms returns all upstream platforms.
func (h *GB28181Handler) ListPlatforms(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"platforms": []interface{}{}})
		return
	}
	pm := h.svr.GetPlatforms()
	if pm == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"platforms": []interface{}{}})
		return
	}

	platforms := pm.ListPlatforms()
	result := make([]map[string]interface{}, 0, len(platforms))
	for _, p := range platforms {
		result = append(result, map[string]interface{}{
			"id":           p.Config.ID,
			"name":         p.Config.Name,
			"enable":       p.Config.Enable,
			"server_gb_id": p.Config.ServerGBID,
			"server_ip":    p.Config.ServerIP,
			"server_port":  p.Config.ServerPort,
			"device_gb_id": p.Config.DeviceGBID,
			"device_ip":    p.Config.DeviceIP,
			"device_port":  p.Config.DevicePort,
			"transport":    p.Config.Transport,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"platforms": result})
}

// AddPlatformRequest is the request body for adding a platform.
type AddPlatformRequest struct {
	Name            string `json:"name"`
	Enable          bool   `json:"enable"`
	ServerGBID      string `json:"server_gb_id"`
	ServerGBDomain  string `json:"server_gb_domain"`
	ServerIP        string `json:"server_ip"`
	ServerPort      int    `json:"server_port"`
	DeviceGBID      string `json:"device_gb_id"`
	DeviceGBDomain  string `json:"device_gb_domain"`
	DeviceIP        string `json:"device_ip"`
	DevicePort      int    `json:"device_port"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	Transport       string `json:"transport"`
	CharacterSet    string `json:"character_set"`
	Expires         int    `json:"expires"`
	KeepTimeout     int    `json:"keep_timeout"`
	MaxTimeoutCount int    `json:"max_timeout_count"`
}

// AddPlatform adds a new upstream platform.
func (h *GB28181Handler) AddPlatform(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req AddPlatformRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.ServerGBID == "" || req.ServerIP == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "server_gb_id and server_ip are required"})
		return
	}

	if req.DeviceGBID == "" {
		req.DeviceGBID = h.svr.GetConfig().ID
	}
	if req.DeviceIP == "" {
		req.DeviceIP = h.svr.GetConfig().MediaIP
	}
	if req.DevicePort == 0 {
		req.DevicePort = h.svr.GetConfig().Port
	}
	if req.Expires == 0 {
		req.Expires = 3600
	}
	if req.KeepTimeout == 0 {
		req.KeepTimeout = 60
	}
	if req.MaxTimeoutCount == 0 {
		req.MaxTimeoutCount = 3
	}
	if req.Transport == "" {
		req.Transport = "UDP"
	}

	pm := h.svr.GetPlatforms()
	if pm == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "platform manager not initialized"})
		return
	}

	cfg := &gb28181.PlatformConfig{
		Name:            req.Name,
		Enable:          req.Enable,
		ServerGBID:      req.ServerGBID,
		ServerGBDomain:  req.ServerGBDomain,
		ServerIP:        req.ServerIP,
		ServerPort:      req.ServerPort,
		DeviceGBID:      req.DeviceGBID,
		DeviceGBDomain:  req.DeviceGBDomain,
		DeviceIP:        req.DeviceIP,
		DevicePort:      req.DevicePort,
		Username:        req.Username,
		Password:        req.Password,
		Transport:       req.Transport,
		CharacterSet:    req.CharacterSet,
		Expires:         req.Expires,
		KeepTimeout:     req.KeepTimeout,
		MaxTimeoutCount: req.MaxTimeoutCount,
	}

	if err := pm.AddPlatform(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"id": cfg.ID, "status": "ok"})
}

// DeletePlatform deletes an upstream platform.
func (h *GB28181Handler) DeletePlatform(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	idStr := r.URL.Query().Get("id")
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	pm := h.svr.GetPlatforms()
	if pm == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "platform manager not initialized"})
		return
	}

	if err := pm.RemovePlatform(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ==================== Broadcast API ====================

// StartBroadcast starts a voice broadcast to a device channel.
func (h *GB28181Handler) StartBroadcast(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req struct {
		DeviceID  string `json:"device_id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	bm := h.svr.GetBroadcastManager()
	if bm == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "broadcast manager not initialized"})
		return
	}

	bs, err := bm.StartBroadcast(req.DeviceID, req.ChannelID, h.svr.GetDeviceStore())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": bs.DeviceID + "_" + bs.ChannelID,
		"port":       bs.RTPPort,
		"ssrc":       bs.SSRC,
	})
}

// StopBroadcast stops a voice broadcast.
func (h *GB28181Handler) StopBroadcast(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req struct {
		DeviceID  string `json:"device_id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	bm := h.svr.GetBroadcastManager()
	if bm == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "broadcast manager not initialized"})
		return
	}

	if err := bm.StopBroadcast(req.DeviceID, req.ChannelID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ==================== Platform Events API ====================

// ListPlatformEvents returns platform events history.
func (h *GB28181Handler) ListPlatformEvents(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"events": []interface{}{},
			"total":  0,
		})
		return
	}

	var platformID int64
	fmt.Sscanf(r.URL.Query().Get("platform_id"), "%d", &platformID)
	eventType := r.URL.Query().Get("event_type")
	limit := 50
	offset := 0
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)

	events, total, err := h.db.ListPlatformEvents(r.Context(), platformID, eventType, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"total":  total,
	})
}

// GetPlatformStatus returns the current status of all platforms.
func (h *GB28181Handler) GetPlatformStatus(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"platforms": []interface{}{},
		})
		return
	}

	statuses, err := h.db.GetPlatformStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"platforms": statuses,
	})
}

// ==================== Alarm API ====================

// ListAlarms returns alarm records.
func (h *GB28181Handler) ListAlarms(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"alarms": []interface{}{},
			"total":  0,
		})
		return
	}
	deviceID := r.URL.Query().Get("device_id")
	limit := 50
	offset := 0
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)

	alarms, total, err := h.db.ListAlarms(r.Context(), deviceID, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if alarms == nil {
		alarms = []storage.AlarmRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alarms": alarms,
		"total":  total,
	})
}

// ==================== Download API ====================

// StartDownload starts a recording download from a device.
func (h *GB28181Handler) StartDownload(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req struct {
		DeviceID  string `json:"device_id"`
		ChannelID string `json:"channel_id"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	startTime, err := parseTime(req.StartTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_time format"})
		return
	}
	endTime, err := parseTime(req.EndTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_time format"})
		return
	}

	dm := h.svr.GetDownloadManager()
	if dm == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "download manager not initialized"})
		return
	}

	ds, err := dm.StartDownload(req.DeviceID, req.ChannelID, startTime, endTime, h.svr.GetDeviceStore())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"download_id": ds.ID,
		"file_path":   ds.FilePath,
		"status":      ds.Status,
	})
}

// StopDownload stops a recording download.
func (h *GB28181Handler) StopDownload(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req struct {
		DeviceID   string `json:"device_id"`
		ChannelID  string `json:"channel_id"`
		DownloadID int64  `json:"download_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	dm := h.svr.GetDownloadManager()
	if dm == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "download manager not initialized"})
		return
	}

	if err := dm.StopDownload(req.DeviceID, req.ChannelID, req.DownloadID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// BatchDownloadRequest is the request body for batch downloading.
type BatchDownloadRequest struct {
	DeviceID  string   `json:"device_id"`
	ChannelID string   `json:"channel_id"`
	Segments  []struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	} `json:"segments"`
}

// BatchDownload starts multiple recording downloads from a device.
func (h *GB28181Handler) BatchDownload(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}
	var req BatchDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	if len(req.Segments) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "segments is required"})
		return
	}

	dm := h.svr.GetDownloadManager()
	if dm == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "download manager not initialized"})
		return
	}

	var results []map[string]interface{}
	var errors []string

	for i, seg := range req.Segments {
		startTime, err := parseTime(seg.StartTime)
		if err != nil {
			errors = append(errors, fmt.Sprintf("segment %d: invalid start_time", i))
			continue
		}
		endTime, err := parseTime(seg.EndTime)
		if err != nil {
			errors = append(errors, fmt.Sprintf("segment %d: invalid end_time", i))
			continue
		}

		ds, err := dm.StartDownload(req.DeviceID, req.ChannelID, startTime, endTime, h.svr.GetDeviceStore())
		if err != nil {
			errors = append(errors, fmt.Sprintf("segment %d: %v", i, err))
			continue
		}

		results = append(results, map[string]interface{}{
			"download_id": ds.ID,
			"file_path":   ds.FilePath,
			"status":      ds.Status,
			"start_time":  seg.StartTime,
			"end_time":    seg.EndTime,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"downloads": results,
		"errors":    errors,
		"total":     len(results),
	})
}

// ListDownloads returns download records.
func (h *GB28181Handler) ListDownloads(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"downloads": []interface{}{},
			"total":     0,
		})
		return
	}
	deviceID := r.URL.Query().Get("device_id")
	channelID := r.URL.Query().Get("channel_id")
	limit := 50
	offset := 0
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)

	downloads, total, err := h.db.ListDownloads(r.Context(), deviceID, channelID, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if downloads == nil {
		downloads = []storage.DownloadRecordRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"downloads": downloads,
		"total":     total,
	})
}

// ==================== Talk WebSocket API ====================

// HandleTalkWS handles WebSocket connections for voice talk/intercom.
func (h *GB28181Handler) HandleTalkWS(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}

	deviceID := r.URL.Query().Get("device_id")
	channelID := r.URL.Query().Get("channel_id")
	transportStr := r.URL.Query().Get("transport")

	if deviceID == "" || channelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	// 解析传输模式
	transportMode := gb28181.TransportUDP
	switch transportStr {
	case "1":
		transportMode = gb28181.TransportTCPPassive
	case "2":
		transportMode = gb28181.TransportTCPActive
	}

	// 获取 TalkManager
	tm := h.svr.GetTalkManager()
	if tm == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "talk manager not initialized"})
		return
	}

	// 检查会话是否已存在
	session, exists := tm.GetSession(deviceID, channelID)
	if !exists {
		// 启动新的对讲会话
		var err error
		session, err = tm.StartTalk(deviceID, channelID, transportMode, h.svr.GetDeviceStore())
		if err != nil {
			slog.Error("Failed to start talk", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	// 升级为 WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		_ = tm.StopTalk(deviceID, channelID)
		return
	}
	defer conn.Close()
	defer func() {
		if err := tm.StopTalk(deviceID, channelID); err != nil {
			slog.Warn("Failed to stop talk session", "device_id", deviceID, "channel_id", channelID, "error", err)
		}
	}()

	slog.Info("Talk WebSocket connected", "device_id", deviceID, "channel_id", channelID, "transport", transportMode)

	// 发送状态消息
	statusMsg := map[string]interface{}{
		"status":     "connected",
		"session_id": deviceID + "_" + channelID,
		"port":       session.RTPPort,
		"transport":  transportMode,
	}
	if err := conn.WriteJSON(statusMsg); err != nil {
		slog.Error("Failed to send status message", "error", err)
		return
	}

	// 等待会话就绪
	select {
	case <-session.ReadyCh:
		// 会话就绪
	case <-time.After(30 * time.Second):
		slog.Error("Talk session timeout", "device_id", deviceID)
		conn.WriteJSON(map[string]string{"error": "session timeout"})
		return
	}

	// 读取音频数据并发送
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Error("WebSocket read error", "error", err)
			}
			break
		}

		if messageType == websocket.BinaryMessage {
			if err := session.SendAudioData(message); err != nil {
				slog.Error("Failed to send audio data", "error", err)
			}
		}
	}

	slog.Info("Talk WebSocket disconnected", "device_id", deviceID, "channel_id", channelID)
}

// ==================== WHIP Talk API ====================

// StartTalkWhipRequest is the request body for starting a WHIP talk session.
type StartTalkWhipRequest struct {
	StreamName string `json:"stream_name"`
	DeviceID   string `json:"device_id"`
	ChannelID  string `json:"channel_id"`
	Transport  string `json:"transport,omitempty"` // 0=UDP, 1=TCP passive, 2=TCP active
}

// HandleStartTalkWhip starts a GB28181 talk session for WHIP streaming.
// The frontend should first establish a WHIP connection to the media server,
// then call this API to start the GB28181 talk session.
func (h *GB28181Handler) HandleStartTalkWhip(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}

	var req StartTalkWhipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	// 解析传输模式
	transportMode := gb28181.TransportUDP
	switch req.Transport {
	case "1":
		transportMode = gb28181.TransportTCPPassive
	case "2":
		transportMode = gb28181.TransportTCPActive
	}

	// 获取 TalkManager
	tm := h.svr.GetTalkManager()
	if tm == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "talk manager not initialized"})
		return
	}

	// 检查会话是否已存在
	session, exists := tm.GetSession(req.DeviceID, req.ChannelID)
	if !exists {
		// 启动新的对讲会话
		var err error
		session, err = tm.StartTalk(req.DeviceID, req.ChannelID, transportMode, h.svr.GetDeviceStore())
		if err != nil {
			slog.Error("Failed to start WHIP talk", "error", err, "device_id", req.DeviceID, "channel_id", req.ChannelID)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	slog.Info("WHIP talk session started", 
		"device_id", req.DeviceID, 
		"channel_id", req.ChannelID,
		"stream_name", req.StreamName,
		"port", session.RTPPort)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "started",
		"session_id": req.DeviceID + "_" + req.ChannelID,
		"port":       session.RTPPort,
		"transport":  transportMode,
		"stream_name": req.StreamName,
	})
}

// HandleStopTalkWhip stops a GB28181 talk session for WHIP streaming.
func (h *GB28181Handler) HandleStopTalkWhip(w http.ResponseWriter, r *http.Request) {
	if h.svr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GB28181 not enabled"})
		return
	}

	var req StartTalkWhipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	// 获取 TalkManager
	tm := h.svr.GetTalkManager()
	if tm == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "talk manager not initialized"})
		return
	}

	// 停止对讲会话
	if err := tm.StopTalk(req.DeviceID, req.ChannelID); err != nil {
		slog.Warn("Failed to stop WHIP talk", "error", err, "device_id", req.DeviceID, "channel_id", req.ChannelID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	slog.Info("WHIP talk session stopped", "device_id", req.DeviceID, "channel_id", req.ChannelID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "stopped",
	})
}

// buildGB28181PlayURLs builds play URLs for all supported protocols.
func buildGB28181PlayURLs(ctx context.Context, engine media.Engine, streamID, appName string) []map[string]string {
	if engine == nil {
		return nil
	}
	protocols := []string{"ws-flv", "flv", "hls", "ll-hls", "webrtc", "fmp4", "rtmp", "rtsp"}
	urls := make([]map[string]string, 0, len(protocols))
	for _, protocol := range protocols {
		playURL, err := engine.BuildPlayURL(ctx, media.PlayURLRequest{
			StreamID: streamID,
			AppName:  appName,
			Protocol: protocol,
		})
		if err != nil || playURL == nil || playURL.URL == "" {
			continue
		}
		urls = append(urls, map[string]string{
			"protocol": protocol,
			"url":      playURL.URL,
		})
	}
	return urls
}

// parseTime parses time strings in various formats and converts to local time.
func parseTime(s string) (time.Time, error) {
	// Try RFC3339 first (handles Z suffix for UTC)
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t.In(time.Local), nil
	}
	// Try RFC3339 without Z - treat as local time
	t, err = time.ParseInLocation("2006-01-02T15:04:05", s, time.Local)
	if err == nil {
		return t, nil
	}
	// Try with milliseconds
	t, err = time.ParseInLocation("2006-01-02T15:04:05.000", s, time.Local)
	if err == nil {
		return t, nil
	}
	// Try RFC3339 with milliseconds and Z
	t, err = time.Parse("2006-01-02T15:04:05.000Z", s)
	if err == nil {
		return t.In(time.Local), nil
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}
