package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lalmax-pro/lalmax-nvr/internal/onvif"
)

// ONVIFRecordingResponse represents the response for ONVIF recordings.
type ONVIFRecordingResponse struct {
	Recordings []onvif.Recording `json:"recordings"`
}

// ONVIFRecordingSegmentResponse represents the response for ONVIF recording segments.
type ONVIFRecordingSegmentResponse struct {
	Segments []onvif.RecordingSegment `json:"segments"`
}

// ONVIFReplayResponse represents the response for ONVIF replay URI.
type ONVIFReplayResponse struct {
	URI      string `json:"uri"`
	Protocol string `json:"protocol"`
}

// handleGetONVIFRecordings queries recordings from an ONVIF device.
func (h *Handler) handleGetONVIFRecordings(w http.ResponseWriter, r *http.Request) {
	cameraID := chi.URLParam(r, "id")
	if cameraID == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	// Get camera config
	cam := h.camMgr.GetCameraConfig(cameraID)
	if cam == nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	// Check if camera is ONVIF protocol
	if cam.Protocol != "onvif" {
		writeError(w, http.StatusBadRequest, "camera is not an ONVIF device")
		return
	}

	// Parse time range parameters
	var startTime, endTime time.Time
	if st := r.URL.Query().Get("start_time"); st != "" {
		var err error
		startTime, err = time.Parse(time.RFC3339, st)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid start_time format, use RFC3339")
			return
		}
	} else {
		// Default to last 24 hours
		startTime = time.Now().Add(-24 * time.Hour)
	}

	if et := r.URL.Query().Get("end_time"); et != "" {
		var err error
		endTime, err = time.Parse(time.RFC3339, et)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid end_time format, use RFC3339")
			return
		}
	} else {
		endTime = time.Now()
	}

	// Create ONVIF client
	client := onvif.NewClient(cam.URL, cam.Username, cam.Password)
	if err := client.Connect(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to connect to ONVIF device: %v", err))
		return
	}

	// Get recordings
	recordings, err := client.GetRecordings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get recordings: %v", err))
		return
	}

	// Filter by time range if needed
	filtered := make([]onvif.Recording, 0, len(recordings))
	for _, rec := range recordings {
		if (rec.StartTime.IsZero() || !rec.StartTime.Before(startTime)) &&
			(rec.EndTime.IsZero() || !rec.EndTime.After(endTime)) {
			filtered = append(filtered, rec)
		}
	}

	writeJSON(w, http.StatusOK, ONVIFRecordingResponse{
		Recordings: filtered,
	})
}

// handleSearchONVIFRecordings searches for recording segments on an ONVIF device.
func (h *Handler) handleSearchONVIFRecordings(w http.ResponseWriter, r *http.Request) {
	cameraID := chi.URLParam(r, "id")
	if cameraID == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	// Get camera config
	cam := h.camMgr.GetCameraConfig(cameraID)
	if cam == nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	// Check if camera is ONVIF protocol
	if cam.Protocol != "onvif" {
		writeError(w, http.StatusBadRequest, "camera is not an ONVIF device")
		return
	}

	// Parse time range parameters
	startTimeStr := r.URL.Query().Get("start_time")
	if startTimeStr == "" {
		writeError(w, http.StatusBadRequest, "start_time is required")
		return
	}

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_time format, use RFC3339")
		return
	}

	var endTime time.Time
	if et := r.URL.Query().Get("end_time"); et != "" {
		endTime, err = time.Parse(time.RFC3339, et)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid end_time format, use RFC3339")
			return
		}
	} else {
		endTime = time.Now()
	}

	// Parse max results
	maxResults := 100
	if mr := r.URL.Query().Get("max_results"); mr != "" {
		if _, err := fmt.Sscanf(mr, "%d", &maxResults); err != nil {
			maxResults = 100
		}
	}

	// Create ONVIF client
	client := onvif.NewClient(cam.URL, cam.Username, cam.Password)
	if err := client.Connect(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to connect to ONVIF device: %v", err))
		return
	}

	// Search recordings
	searchReq := onvif.SearchRequest{
		StartTime:  startTime,
		EndTime:    endTime,
		MaxResults: maxResults,
	}

	segments, err := client.SearchRecordings(r.Context(), searchReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to search recordings: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, ONVIFRecordingSegmentResponse{
		Segments: segments,
	})
}

// handleGetONVIFReplayURI gets the replay URI for a recording.
func (h *Handler) handleGetONVIFReplayURI(w http.ResponseWriter, r *http.Request) {
	cameraID := chi.URLParam(r, "id")
	if cameraID == "" {
		writeError(w, http.StatusBadRequest, "camera ID is required")
		return
	}

	recordingToken := chi.URLParam(r, "token")
	if recordingToken == "" {
		writeError(w, http.StatusBadRequest, "recording token is required")
		return
	}

	// Get camera config
	cam := h.camMgr.GetCameraConfig(cameraID)
	if cam == nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	// Check if camera is ONVIF protocol
	if cam.Protocol != "onvif" {
		writeError(w, http.StatusBadRequest, "camera is not an ONVIF device")
		return
	}

	// Create ONVIF client
	client := onvif.NewClient(cam.URL, cam.Username, cam.Password)
	if err := client.Connect(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to connect to ONVIF device: %v", err))
		return
	}

	// Get replay URI
	uri, err := client.GetReplayURI(r.Context(), recordingToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get replay URI: %v", err))
		return
	}

	// Determine protocol from URI
	protocol := "rtsp"
	if len(uri) > 4 {
		switch uri[:4] {
		case "http":
			protocol = "http"
		case "rtsp":
			protocol = "rtsp"
		}
	}

	writeJSON(w, http.StatusOK, ONVIFReplayResponse{
		URI:      uri,
		Protocol: protocol,
	})
}
