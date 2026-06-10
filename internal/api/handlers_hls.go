package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lalmax-pro/lalmax-nvr/internal/hls"
	"github.com/lalmax-pro/lalmax-nvr/internal/media"
	"github.com/lalmax-pro/lalmax-nvr/internal/model"
	"github.com/lalmax-pro/lalmax-nvr/internal/recorder"
)

// --- HLS streaming endpoints ---

// subscribeHLS registers an HLS consumer on the recorder's StreamHub.
// It uses Hub.Subscribe with consumer ID "hls" so that the HLS manager
// receives frames via the fan-out architecture.
// It first unsubscribes any stale "hls" consumer left over from a previous
// session (e.g. after idle eviction), then subscribes with the new callback.
func subscribeHLS(hub *model.StreamHub, cameraID string, hlsMgr media.HLS, isH265 bool) error {
	if hub == nil {
		return nil // no hub, no subscription (shouldn't happen in practice)
	}
	hub.Unsubscribe("hls") // clean up stale consumer from previous session
	return hlsMgr.SubscribeToHub(cameraID, hub, isH265)
}

func (h *Handler) handleHLSStream(w http.ResponseWriter, r *http.Request) {
	id := getCameraID(r)
	if h.config != nil && !h.config.IsHLSEnabled() {
		writeError(w, http.StatusServiceUnavailable, "HLS is disabled")
		return
	}
	if h.mediaEngine != nil {
		tail := chi.URLParam(r, "*")
		upstream, err := h.mediaHLSResourceURL(r.Context(), id, tail, r.URL.RawQuery)
		if err == nil && upstream != nil {
			if err := h.proxyMediaRequest(w, r, upstream); err != nil {
				logger.Error("failed to proxy HLS request to media engine", "camera_id", id, "tail", tail, "error", err)
				writeError(w, http.StatusBadGateway, "failed to proxy HLS stream")
			}
			return
		}
	}

	if h.hlsMgr == nil || h.camMgr == nil {
		writeError(w, http.StatusInternalServerError, "HLS not available")
		return
	}

	// Get camera to check protocol
	cam, err := h.db.GetCamera(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get camera")
		return
	}
	if cam == nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	// If stream not active, start it
	if !h.hlsMgr.IsActive(id) {
		rec := h.camMgr.GetRecorder(id)
		if rec == nil {
			writeError(w, http.StatusBadRequest, "camera recorder not running")
			return
		}

		// Get camera config for HLS options
		camCfg := h.camMgr.GetCameraConfig(id)
		hlsMaxFPS := 0
		if camCfg != nil {
			hlsMaxFPS = camCfg.HLSMaxFPS
		}

		// Try H264 recorder first
		if h264Rec, ok := rec.(*recorder.H264Recorder); ok {
			sps := h264Rec.SPS()
			pps := h264Rec.PPS()
			if sps == nil || pps == nil {
				writeError(w, http.StatusServiceUnavailable, "SPS/PPS not available yet, waiting for video stream")
				return
			}

			audioCodec, audioConfig := getAudioConfig(rec)
			var err error
			if audioCodec == "aac" && len(audioConfig) > 0 {
				err = h.hlsMgr.StartStreamWithAudio(id, sps, pps, hlsMaxFPS, audioCodec, audioConfig)
			} else {
				err = h.hlsMgr.StartStream(id, sps, pps, hlsMaxFPS)
			}
			if err != nil {
				if errors.Is(err, hls.ErrMaxStreamsReached) {
					writeAPIError(w, http.StatusServiceUnavailable, &model.HLSMaxStreamsError{})
				} else {
					logger.Error("failed to start HLS stream", "camera_id", id, "error", err)
					writeError(w, http.StatusInternalServerError, "failed to start HLS stream")
				}
				return
			}

			// Check if sub-stream URL is configured
			if camCfg != nil && camCfg.SubStreamURL != "" {
				fallback := func() {
					_ = subscribeHLS(h264Rec.Hub, id, h.hlsMgr, false)
					if audioCodec == "aac" {
						_ = h.hlsMgr.SubscribeAudioToHub(id, h264Rec.Hub)
					}
				}
				if subErr := h.hlsMgr.StartSubStreamReader(id, camCfg.SubStreamURL, false, fallback); subErr != nil {
					logger.Warn("failed to start HLS sub-stream reader, falling back to main stream", "camera_id", id, "error", subErr)
					fallback()
				}
				// Sub-stream reader is running — do NOT subscribe hub on recorder
			} else {
				_ = subscribeHLS(h264Rec.Hub, id, h.hlsMgr, false)
				if audioCodec == "aac" {
					_ = h.hlsMgr.SubscribeAudioToHub(id, h264Rec.Hub)
				}
			}
		} else if h265Rec, ok := rec.(*recorder.H265Recorder); ok {
			vps := h265Rec.VPS()
			sps := h265Rec.SPS()
			pps := h265Rec.PPS()
			if vps == nil || sps == nil || pps == nil {
				writeError(w, http.StatusServiceUnavailable, "VPS/SPS/PPS not available yet, waiting for video stream")
				return
			}

			audioCodec, audioConfig := getAudioConfig(rec)
			var err error
			if audioCodec == "aac" && len(audioConfig) > 0 {
				err = h.hlsMgr.StartStreamH265WithAudio(id, vps, sps, pps, hlsMaxFPS, audioCodec, audioConfig)
			} else {
				err = h.hlsMgr.StartStreamH265(id, vps, sps, pps, hlsMaxFPS)
			}
			if err != nil {
				if errors.Is(err, hls.ErrMaxStreamsReached) {
					writeAPIError(w, http.StatusServiceUnavailable, &model.HLSMaxStreamsError{})
				} else {
					logger.Error("failed to start HLS H265 stream", "camera_id", id, "error", err)
					writeError(w, http.StatusInternalServerError, "failed to start HLS stream")
				}
				return
			}

			// Check if sub-stream URL is configured
			if camCfg != nil && camCfg.SubStreamURL != "" {
				fallback := func() {
					_ = subscribeHLS(h265Rec.Hub, id, h.hlsMgr, true)
					if audioCodec == "aac" {
						_ = h.hlsMgr.SubscribeAudioToHub(id, h265Rec.Hub)
					}
				}
				if subErr := h.hlsMgr.StartSubStreamReader(id, camCfg.SubStreamURL, true, fallback); subErr != nil {
					logger.Warn("failed to start HLS sub-stream reader, falling back to main stream", "camera_id", id, "error", subErr)
					fallback()
				}
			} else {
				_ = subscribeHLS(h265Rec.Hub, id, h.hlsMgr, true)
				if audioCodec == "aac" {
					_ = h.hlsMgr.SubscribeAudioToHub(id, h265Rec.Hub)
				}
			}
		} else if onvifRec, ok := rec.(*recorder.ONVIFRecorder); ok {
			// ONVIF recorder delegates to H264/H265 internally
			delegate := onvifRec.Delegate()
			if delegate == nil {
				writeError(w, http.StatusServiceUnavailable, "ONVIF recorder delegate not available yet")
				return
			}
			// Unwrap the delegate and handle as H264/H265
			if h264Rec, ok := delegate.(*recorder.H264Recorder); ok {
				sps := h264Rec.SPS()
				pps := h264Rec.PPS()
				if sps == nil || pps == nil {
					writeError(w, http.StatusServiceUnavailable, "SPS/PPS not available yet, waiting for video stream")
					return
				}
				audioCodec, audioConfig := getAudioConfig(rec)
				var err error
				if audioCodec == "aac" && len(audioConfig) > 0 {
					err = h.hlsMgr.StartStreamWithAudio(id, sps, pps, hlsMaxFPS, audioCodec, audioConfig)
				} else {
					err = h.hlsMgr.StartStream(id, sps, pps, hlsMaxFPS)
				}
				if err != nil {
					if errors.Is(err, hls.ErrMaxStreamsReached) {
						writeAPIError(w, http.StatusServiceUnavailable, &model.HLSMaxStreamsError{})
					} else {
						writeError(w, http.StatusInternalServerError, "failed to start HLS stream")
					}
					return
				}
				_ = subscribeHLS(h264Rec.Hub, id, h.hlsMgr, false)
				if audioCodec == "aac" {
					_ = h.hlsMgr.SubscribeAudioToHub(id, h264Rec.Hub)
				}
			} else if h265Rec, ok := delegate.(*recorder.H265Recorder); ok {
				vps := h265Rec.VPS()
				sps := h265Rec.SPS()
				pps := h265Rec.PPS()
				if vps == nil || sps == nil || pps == nil {
					writeError(w, http.StatusServiceUnavailable, "VPS/SPS/PPS not available yet, waiting for video stream")
					return
				}
				audioCodec, audioConfig := getAudioConfig(rec)
				var err error
				if audioCodec == "aac" && len(audioConfig) > 0 {
					err = h.hlsMgr.StartStreamH265WithAudio(id, vps, sps, pps, hlsMaxFPS, audioCodec, audioConfig)
				} else {
					err = h.hlsMgr.StartStreamH265(id, vps, sps, pps, hlsMaxFPS)
				}
				if err != nil {
					if errors.Is(err, hls.ErrMaxStreamsReached) {
						writeAPIError(w, http.StatusServiceUnavailable, &model.HLSMaxStreamsError{})
					} else {
						writeError(w, http.StatusInternalServerError, "failed to start HLS stream")
					}
					return
				}
				_ = subscribeHLS(h265Rec.Hub, id, h.hlsMgr, true)
				if audioCodec == "aac" {
					_ = h.hlsMgr.SubscribeAudioToHub(id, h265Rec.Hub)
				}
			} else {
				writeAPIError(w, http.StatusBadRequest, &model.HLSSupportedCodecError{CameraID: id})
				return
			}
		} else if provider, ok := rec.(model.HLSProvider); ok {
			codec, sps, pps, vps := provider.CodecParams()
			if sps == nil || pps == nil {
				writeError(w, http.StatusServiceUnavailable, "codec params not ready yet, waiting for video stream")
				return
			}
			switch codec {
			case model.FormatH264:
				err := h.hlsMgr.StartStream(id, sps, pps, hlsMaxFPS)
				if err != nil {
					if errors.Is(err, hls.ErrMaxStreamsReached) {
						writeAPIError(w, http.StatusServiceUnavailable, &model.HLSMaxStreamsError{})
					} else {
						logger.Error("failed to start HLS stream", "camera_id", id, "error", err)
						writeError(w, http.StatusInternalServerError, "failed to start HLS stream")
					}
					return
				}
				provider.SetOnHLSFrame(func(pts int64, au [][]byte) {
					_ = h.hlsMgr.WriteH264(id, pts, au)
				})
			case model.FormatH265:
				if vps == nil {
					writeError(w, http.StatusServiceUnavailable, "VPS not ready yet, waiting for video stream")
					return
				}
				err := h.hlsMgr.StartStreamH265(id, vps, sps, pps, hlsMaxFPS)
				if err != nil {
					if errors.Is(err, hls.ErrMaxStreamsReached) {
						writeAPIError(w, http.StatusServiceUnavailable, &model.HLSMaxStreamsError{})
					} else {
						logger.Error("failed to start HLS H265 stream", "camera_id", id, "error", err)
						writeError(w, http.StatusInternalServerError, "failed to start HLS stream")
					}
					return
				}
				provider.SetOnHLSFrame(func(pts int64, au [][]byte) {
					_ = h.hlsMgr.WriteH265(id, pts, au)
				})
			default:
				writeAPIError(w, http.StatusBadRequest, &model.HLSSupportedCodecError{CameraID: id})
				return
			}
		} else {
			writeAPIError(w, http.StatusBadRequest, &model.HLSSupportedCodecError{CameraID: id})
			return
		}
	}
	// Proxy to muxer handler
	if !h.hlsMgr.Handle(id, w, r) {
		writeError(w, http.StatusServiceUnavailable, "HLS stream not available")
		return
	}
}

func (h *Handler) handleStopHLSStream(w http.ResponseWriter, r *http.Request) {
	id := getCameraID(r)

	if h.hlsMgr == nil {
		writeError(w, http.StatusInternalServerError, "HLS not available")
		return
	}

	if !h.hlsMgr.IsActive(id) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "not active"})
		return
	}

	// Unsubscribe HLS consumer from StreamHub before stopping the stream
	if h.camMgr != nil {
		if rec := h.camMgr.GetRecorder(id); rec != nil {
			hub := getRecorderHub(rec)
			if hub != nil {
				hub.Unsubscribe("hls")
			}
		}
	}

	h.hlsMgr.StopStream(id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// getRecorderHub extracts the StreamHub from any recorder type.
// Returns nil if the recorder doesn't have a Hub.
func getRecorderHub(rec model.Recorder) *model.StreamHub {
	switch r := rec.(type) {
	case *recorder.H264Recorder:
		return r.Hub
	case *recorder.H265Recorder:
		return r.Hub
	case *recorder.ONVIFRecorder:
		// ONVIF passes Hub to delegate, but we unsubscribe from the delegate's hub
		if delegate := r.Delegate(); delegate != nil {
			return getRecorderHub(delegate)
		}
		return r.Hub
	case *recorder.MJPEGRecorder:
		return r.Hub
	case *recorder.HTTPJPEGRecorder:
		return r.Hub
	default:
		return nil
	}
}

// --- Snapshot endpoint ---

func (h *Handler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	cameraID := getCameraID(r)
	if cameraID == "" {
		writeError(w, http.StatusBadRequest, "camera id is required")
		return
	}
	h.serveCameraSnapshot(w, r, cameraID)
}
