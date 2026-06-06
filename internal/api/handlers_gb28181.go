package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/gb28181"
)

// GB28181Handler provides HTTP API endpoints for GB28181 management.
type GB28181Handler struct {
	svr *gb28181.Server
}

// NewGB28181Handler creates a new GB28181Handler.
func NewGB28181Handler(svr *gb28181.Server) *GB28181Handler {
	return &GB28181Handler{svr: svr}
}

// ListDevices returns all registered GB28181 devices.
func (h *GB28181Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
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
		streamID = req.ChannelID
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ssrc":      ssrc,
		"stream_id": streamID,
	})
}

// StopPlayRequest is the request body for the stop play endpoint.
type StopPlayRequest struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
}

// StopPlay stops a GB28181 play session.
func (h *GB28181Handler) StopPlay(w http.ResponseWriter, r *http.Request) {
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
	var req RecordInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		startTime, err = time.Parse("2006-01-02T15:04:05", req.StartTime)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_time format"})
			return
		}
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		endTime, err = time.Parse("2006-01-02T15:04:05", req.EndTime)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_time format"})
			return
		}
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
	var req PlaybackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.DeviceID == "" || req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and channel_id are required"})
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		startTime, err = time.Parse("2006-01-02T15:04:05", req.StartTime)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_time format"})
			return
		}
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		endTime, err = time.Parse("2006-01-02T15:04:05", req.EndTime)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_time format"})
			return
		}
	}

	streamID := req.StreamID
	if streamID == "" {
		streamID = req.ChannelID + "_pb"
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ssrc":      ssrc,
		"stream_id": streamID,
	})
}
