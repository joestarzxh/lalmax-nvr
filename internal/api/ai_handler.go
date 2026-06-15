package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/ai"
	"github.com/lalmax-pro/lalmax-nvr/internal/ai/webhook"
	"github.com/lalmax-pro/lalmax-nvr/internal/model"
)

// --- AI Manager interface for testability ---

// AIManagerInterface abstracts the AI Manager for the handler.
type AIManagerInterface interface {
	Status() ai.Status
	EnableCamera(camID string, hub interface{}) error
	DisableCamera(camID string, hub interface{})
	IsEnabled(camID string) bool
	EnabledCameras() []string
	OnDetection(cb webhook.CallbackFunc) string
	UnregisterCallback(id string) bool
	StopAll()
	GetReceiver() *webhook.Receiver
}

// --- AI status types ---

// aiStatusResponse is the JSON response for GET /api/ai/status.
type aiStatusResponse struct {
	Available bool   `json:"available"`
	Backend   string `json:"backend"`   // "http", "webhook", "disabled"
	Reason    string `json:"reason"`    // human-readable status explanation
}

// aiEnableRequest is the JSON body for POST /api/ai/enable.
type aiEnableRequest struct {
	CameraID string `json:"camera_id"`
}

// aiDisableRequest is the JSON body for POST /api/ai/disable.
type aiDisableRequest struct {
	CameraID string `json:"camera_id"`
}

// --- AI handlers ---

// handleGetAIStatus handles GET /api/ai/status.
func (h *Handler) handleGetAIStatus(w http.ResponseWriter, r *http.Request) {
	if h.aiManager == nil {
		writeJSON(w, http.StatusOK, aiStatusResponse{
			Available: false,
			Backend:   "disabled",
			Reason:    "AI 模块未初始化",
		})
		return
	}

	status := h.aiManager.Status()
	writeJSON(w, http.StatusOK, aiStatusResponse{
		Available: status.Available,
		Backend:   status.Backend,
		Reason:    status.Reason,
	})
}

// handleEnableAI handles POST /api/ai/enable.
func (h *Handler) handleEnableAI(w http.ResponseWriter, r *http.Request) {
	if h.aiManager == nil {
		writeError(w, http.StatusServiceUnavailable, "AI detection not available")
		return
	}

	var req aiEnableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CameraID == "" {
		writeError(w, http.StatusBadRequest, "missing camera_id")
		return
	}

	// Validate camera exists.
	if h.db != nil {
		cam, err := h.db.GetCamera(r.Context(), req.CameraID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get camera")
			return
		}
		if cam == nil {
			writeError(w, http.StatusNotFound, "camera not found")
			return
		}
	}

	// Get the camera's StreamHub.
	if h.camMgr == nil {
		writeError(w, http.StatusServiceUnavailable, "camera manager not available")
		return
	}
	rec := h.camMgr.GetRecorder(req.CameraID)
	if rec == nil {
		writeError(w, http.StatusBadRequest, "camera recorder not running")
		return
	}
	hub := getStreamHub(rec)
	if hub == nil {
		writeError(w, http.StatusServiceUnavailable, "camera stream hub not available")
		return
	}

	if err := h.aiManager.EnableCamera(req.CameraID, hub); err != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("failed to enable AI: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "enabled",
		"camera_id": req.CameraID,
	})
}

// handleDisableAI handles POST /api/ai/disable.
func (h *Handler) handleDisableAI(w http.ResponseWriter, r *http.Request) {
	if h.aiManager == nil {
		writeError(w, http.StatusServiceUnavailable, "AI detection not available")
		return
	}

	var req aiDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CameraID == "" {
		writeError(w, http.StatusBadRequest, "missing camera_id")
		return
	}

	// Get StreamHub for unsubscribe
	var hub *model.StreamHub
	if h.camMgr != nil {
		if rec := h.camMgr.GetRecorder(req.CameraID); rec != nil {
			hub = getStreamHub(rec)
		}
	}

	h.aiManager.DisableCamera(req.CameraID, hub)

	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "disabled",
		"camera_id": req.CameraID,
	})
}

// handleAIEvents handles GET /api/ai/events.
// SSE stream that pushes detection events to the frontend.
func (h *Handler) handleAIEvents(w http.ResponseWriter, r *http.Request) {
	if h.aiManager == nil {
		writeError(w, http.StatusServiceUnavailable, "AI detection not available")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	eventCh := make(chan webhook.DetectionResult, 16)
	var eventClosed atomic.Bool

	// Register detection callback.
	callbackID := h.aiManager.OnDetection(func(result webhook.DetectionResult) {
		if eventClosed.Load() {
			return
		}
		select {
		case eventCh <- result:
		default:
			logger.Warn("AI SSE: dropping detection event, client too slow")
		}
	})

	// Heartbeat ticker.
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	defer func() {
		eventClosed.Store(true)
		close(eventCh)
	}()
	defer h.aiManager.UnregisterCallback(callbackID)

	for {
		select {
		case <-ctx.Done():
			return
		case result := <-eventCh:
			data, err := json.Marshal(result)
			if err != nil {
				logger.Warn("AI SSE: failed to marshal detection", "error", err)
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// handleAIWebhook handles POST /api/ai/webhook.
// Receives detection events from external AI services.
func (h *Handler) handleAIWebhook(w http.ResponseWriter, r *http.Request) {
	if h.aiManager == nil {
		writeError(w, http.StatusServiceUnavailable, "AI detection not available")
		return
	}

	receiver := h.aiManager.GetReceiver()
	if receiver == nil {
		writeError(w, http.StatusServiceUnavailable, "AI webhook not configured")
		return
	}

	receiver.HandleHTTP()(w, r)
}
