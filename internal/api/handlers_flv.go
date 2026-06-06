package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lalmax-pro/lalmax-nvr/internal/flv"
	"github.com/lalmax-pro/lalmax-nvr/internal/model"
)

// --- HTTP-FLV streaming endpoint ---

// handleFLVStream handles GET /api/cameras/{id}/stream.flv
// It streams FLV data via HTTP chunked transfer encoding.
func (h *Handler) handleFLVStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.mediaEngine != nil {
		upstream, err := h.mediaPlayURL(r.Context(), id, "flv")
		if err == nil && upstream != nil {
			if r.URL.RawQuery != "" {
				if upstream.RawQuery == "" {
					upstream.RawQuery = r.URL.RawQuery
				} else {
					upstream.RawQuery = upstream.RawQuery + "&" + r.URL.RawQuery
				}
			}
			if err := h.proxyMediaRequest(w, r, upstream); err != nil {
				logger.Error("failed to proxy FLV request to media engine", "camera_id", id, "error", err)
				writeError(w, http.StatusBadGateway, "failed to proxy FLV stream")
			}
			return
		}
	}

	if h.flvMgr == nil {
		writeError(w, http.StatusServiceUnavailable, "FLV streaming not available")
		return
	}

	// Check camera exists
	cam, err := h.db.GetCamera(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get camera")
		return
	}
	if cam == nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	// On-demand registration: if FLV stream not registered, register it
	if !h.flvMgr.IsActive(id) {
		if h.camMgr == nil {
			writeError(w, http.StatusNotFound, "FLV stream not active")
			return
		}
		rec := h.camMgr.GetRecorder(id)
		if rec == nil {
			writeError(w, http.StatusBadRequest, "camera recorder not running")
			return
		}

		codec, sps, pps, vps := getCodecParams(rec)
		if sps == nil || pps == nil {
			writeError(w, http.StatusServiceUnavailable, "waiting for video stream")
			return
		}

		hub := getStreamHub(rec)
		if err := h.flvMgr.RegisterStream(id, codec, sps, pps, vps, hub); err != nil {
			if !errors.Is(err, flv.ErrStreamExists) {
				logger.Error("failed to register FLV stream", "camera_id", id, "error", err)
				writeError(w, http.StatusInternalServerError, "failed to register FLV stream")
				return
			}
		}

		// Set audio config if available
		if audioCodec, audioConfig := getAudioConfig(rec); audioCodec != "" {
			h.flvMgr.SetAudioConfig(id, model.AudioCodec(audioCodec), audioConfig)
		}
	}

	// Serve FLV stream (blocks until client disconnects)
	if err := h.flvMgr.ServeFLV(id, w, r); err != nil {
		if errors.Is(err, flv.ErrStreamNotActive) {
			writeError(w, http.StatusNotFound, "FLV stream not active")
			return
		}
		if errors.Is(err, flv.ErrMaxViewers) {
			writeError(w, http.StatusServiceUnavailable, "maximum FLV viewers reached")
			return
		}
		// Client disconnect or write error — log at debug level, not an error
		logger.Debug("FLV stream ended", "camera_id", id, "error", err)
	}
}
