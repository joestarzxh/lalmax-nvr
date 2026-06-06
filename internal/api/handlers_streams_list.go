package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lalmax-pro/lalmax-nvr/internal/config"
	"github.com/lalmax-pro/lalmax-nvr/internal/media"
	"github.com/lalmax-pro/lalmax-nvr/internal/storage"
)

const (
	streamListDefaultLimit    = 20
	streamListMaxLimit        = 100
	streamIdleHistoryMaxAge   = 7 * 24 * time.Hour
	streamIdleHistoryMaxItems = 200
)

type streamListResponse struct {
	Streams []streamSummary `json:"streams"`
	Total   int             `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
}

type streamSummary struct {
	Engine         string                `json:"engine"`
	StreamID       string                `json:"stream_id"`
	AppName        string                `json:"app_name,omitempty"`
	Managed        bool                  `json:"managed"`
	ManagementType string                `json:"management_type,omitempty"`
	CameraID       string                `json:"camera_id,omitempty"`
	CameraName     string                `json:"camera_name,omitempty"`
	SourceType     string                `json:"source_type"`
	Active         bool                  `json:"active"`
	Publisher      *cameraSessionStatus  `json:"publisher,omitempty"`
	Subscribers    []cameraSessionStatus `json:"subscribers,omitempty"`
	VideoCodec     string                `json:"video_codec,omitempty"`
	AudioCodec     string                `json:"audio_codec,omitempty"`
	InFPS          float64               `json:"in_fps,omitempty"`
	LastFrameTime  *time.Time            `json:"last_frame_time,omitempty"`
	PlayURLs       []streamPlayURL       `json:"play_urls,omitempty"`
}

type streamPlayURL struct {
	Protocol string `json:"protocol"`
	URL      string `json:"url"`
	Backend  string `json:"backend"`
}

func (h *Handler) handleListStreams(w http.ResponseWriter, r *http.Request) {
	if h.mediaEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "stream listing unavailable")
		return
	}

	streams, err := h.mediaEngine.ListStreams(r.Context())
	if err != nil {
		logger.Error("list streams failed", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list streams")
		return
	}

	cameraRows, err := h.db.ListCameras(r.Context())
	if err != nil {
		logger.Error("list cameras for streams failed", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to load camera mapping")
		return
	}

	cameraByID := make(map[string]string, len(cameraRows))
	for _, cam := range cameraRows {
		cameraByID[cam.ID] = cam.Name
	}

	bindings, err := h.db.ListStreamBindings(r.Context())
	if err != nil {
		logger.Error("list stream bindings failed", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to load stream bindings")
		return
	}
	bindingByStreamID := make(map[string]string, len(bindings))
	for _, binding := range bindings {
		bindingByStreamID[binding.StreamID] = binding.CameraID
	}

	items := h.buildMergedStreamList(r.Context(), streams, cameraRows, bindingByStreamID, cameraByID)

	search, managedFilter, limit, offset := parseStreamListParams(r)
	filtered := filterStreamSummaries(items, search, managedFilter)
	page, total := paginateStreamSummaries(filtered, limit, offset)

	writeJSON(w, http.StatusOK, streamListResponse{
		Streams: page,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	})
}

func parseStreamListParams(r *http.Request) (search string, managedFilter *bool, limit, offset int) {
	search = strings.TrimSpace(r.URL.Query().Get("q"))
	if v := strings.TrimSpace(r.URL.Query().Get("managed")); v != "" {
		managed := v == "true" || v == "1"
		managedFilter = &managed
	}

	limit = streamListDefaultLimit
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > streamListMaxLimit {
		limit = streamListMaxLimit
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return search, managedFilter, limit, offset
}

func streamDisplayName(item streamSummary) string {
	if item.Managed && item.CameraName != "" {
		return item.CameraName
	}
	return item.StreamID
}

func sortStreamSummaries(items []streamSummary) {
	slices.SortFunc(items, func(a, b streamSummary) int {
		if a.Active != b.Active {
			if a.Active {
				return -1
			}
			return 1
		}
		if a.Managed != b.Managed {
			if a.Managed {
				return -1
			}
			return 1
		}
		displayA := streamDisplayName(a)
		displayB := streamDisplayName(b)
		if cmp := strings.Compare(displayA, displayB); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.StreamID, b.StreamID)
	})
}

func (h *Handler) resolveStreamManagement(
	streamID string,
	info *media.StreamInfo,
	bindingByStreamID, cameraByID map[string]string,
) (cameraID, cameraName, managementType string, managed bool) {
	if boundCameraID, ok := bindingByStreamID[streamID]; ok {
		return boundCameraID, cameraByID[boundCameraID], "bound", true
	}
	if promotedCameraName, promoted := cameraByID[streamID]; promoted {
		managementType = "camera"
		if info != nil {
			managementType = inferCameraManagementType(*info)
		}
		return streamID, promotedCameraName, managementType, true
	}
	return "", "", "", false
}

func (h *Handler) streamSummaryFromMediaInfo(
	ctx context.Context,
	info media.StreamInfo,
	bindingByStreamID, cameraByID map[string]string,
) streamSummary {
	cameraID, cameraName, managementType, managed := h.resolveStreamManagement(info.StreamID, &info, bindingByStreamID, cameraByID)
	item := streamSummary{
		Engine:         "lalmax",
		StreamID:       info.StreamID,
		AppName:        info.AppName,
		Managed:        managed,
		ManagementType: managementType,
		SourceType:     inferStreamSourceType(info, managed),
		Active:         info.Active,
		Publisher:      sessionStatusFromInfo(info.Publisher),
		Subscribers:    sessionStatusesFromInfo(info.Subscribers),
		VideoCodec:     info.VideoCodec,
		AudioCodec:     info.AudioCodec,
		InFPS:          info.InFPS,
		LastFrameTime:  timePointer(info.LastFrameTime),
		PlayURLs:       h.buildStreamPlayURLs(ctx, info.StreamID, info.AppName),
	}
	if managed {
		item.CameraID = cameraID
		item.CameraName = cameraName
	}
	return item
}

func (h *Handler) buildMergedStreamList(
	ctx context.Context,
	liveStreams []media.StreamInfo,
	cameraRows []storage.CameraRow,
	bindingByStreamID, cameraByID map[string]string,
) []streamSummary {
	byID := make(map[string]streamSummary, len(liveStreams)+len(cameraRows))

	for _, info := range liveStreams {
		byID[info.StreamID] = h.streamSummaryFromMediaInfo(ctx, info, bindingByStreamID, cameraByID)
	}

	for _, cam := range cameraRows {
		if !cam.Enabled {
			continue
		}
		if _, ok := byID[cam.ID]; ok {
			continue
		}
		byID[cam.ID] = streamSummary{
			Engine:         "lalmax",
			StreamID:       cam.ID,
			AppName:        "live",
			Managed:        true,
			ManagementType: "camera",
			CameraID:       cam.ID,
			CameraName:     cam.Name,
			SourceType:     "camera",
			Active:         false,
			PlayURLs:       h.buildStreamPlayURLs(ctx, cam.ID, "live"),
		}
	}

	for streamID, cameraID := range bindingByStreamID {
		if _, ok := byID[streamID]; ok {
			continue
		}
		cameraName := cameraByID[cameraID]
		if cameraName == "" {
			continue
		}
		byID[streamID] = streamSummary{
			Engine:         "lalmax",
			StreamID:       streamID,
			AppName:        "live",
			Managed:        true,
			ManagementType: "bound",
			CameraID:       cameraID,
			CameraName:     cameraName,
			SourceType:     "camera",
			Active:         false,
			PlayURLs:       h.buildStreamPlayURLs(ctx, streamID, "live"),
		}
	}

	since := time.Now().Add(-streamIdleHistoryMaxAge)
	recent, err := h.db.ListRecentStreamSnapshots(ctx, since, streamIdleHistoryMaxItems)
	if err != nil {
		logger.Error("list recent stream snapshots failed", "err", err)
	} else {
		for _, snap := range recent {
			if _, ok := byID[snap.StreamID]; ok {
				continue
			}
			if _, isCamera := cameraByID[snap.StreamID]; isCamera {
				continue
			}
			if _, isBound := bindingByStreamID[snap.StreamID]; isBound {
				continue
			}
			appName := snap.AppName
			if appName == "" {
				appName = "live"
			}
			lastSeen := snap.StartedAt
			if snap.EndedAt != nil {
				lastSeen = *snap.EndedAt
			}
			byID[snap.StreamID] = streamSummary{
				Engine:        "lalmax",
				StreamID:      snap.StreamID,
				AppName:       appName,
				SourceType:    inferStreamSourceTypeFromProtocol(snap.Protocol),
				Active:        false,
				LastFrameTime: timePointer(lastSeen),
				PlayURLs:      h.buildStreamPlayURLs(ctx, snap.StreamID, appName),
			}
		}
	}

	items := make([]streamSummary, 0, len(byID))
	for _, item := range byID {
		items = append(items, item)
	}
	sortStreamSummaries(items)
	return items
}

func filterStreamSummaries(items []streamSummary, search string, managedFilter *bool) []streamSummary {
	if search == "" && managedFilter == nil {
		return items
	}

	q := strings.ToLower(strings.TrimSpace(search))
	filtered := make([]streamSummary, 0, len(items))
	for _, item := range items {
		if managedFilter != nil && item.Managed != *managedFilter {
			continue
		}
		if q != "" && !streamSummaryMatchesSearch(item, q) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func streamSummaryMatchesSearch(item streamSummary, q string) bool {
	for _, field := range []string{item.StreamID, item.CameraName, item.CameraID, item.AppName} {
		if field != "" && strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	return false
}

func paginateStreamSummaries(items []streamSummary, limit, offset int) ([]streamSummary, int) {
	total := len(items)
	if offset >= total {
		return nil, total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return items[offset:end], total
}

func inferStreamSourceType(info media.StreamInfo, managed bool) string {
	if managed {
		return "camera"
	}
	if info.Publisher == nil {
		return "stream"
	}
	return inferStreamSourceTypeFromProtocol(info.Publisher.Protocol)
}

func inferStreamSourceTypeFromProtocol(protocol string) string {
	switch strings.ToLower(protocol) {
	case "rtmp":
		return "rtmp_push"
	case "srt":
		return "srt_push"
	case "rtsp", "relay_pull", "pull":
		return "relay_pull"
	default:
		return "stream"
	}
}

func inferCameraManagementType(info media.StreamInfo) string {
	if info.Publisher == nil {
		return "camera"
	}
	switch strings.ToLower(info.Publisher.Protocol) {
	case "rtmp", "srt":
		return "promoted"
	default:
		return "camera"
	}
}

func sessionStatusFromInfo(session *media.SessionInfo) *cameraSessionStatus {
	if session == nil {
		return nil
	}
	return &cameraSessionStatus{
		SessionID:         session.SessionID,
		Protocol:          session.Protocol,
		Remote:            session.Remote,
		BitrateKbits:      session.BitrateKbits,
		ReadBitrateKbits:  session.ReadBitrateKbits,
		WriteBitrateKbits: session.WriteBitrateKbits,
	}
}

func sessionStatusesFromInfo(sessions []media.SessionInfo) []cameraSessionStatus {
	if len(sessions) == 0 {
		return nil
	}
	items := make([]cameraSessionStatus, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, cameraSessionStatus{
			SessionID:         session.SessionID,
			Protocol:          session.Protocol,
			Remote:            session.Remote,
			BitrateKbits:      session.BitrateKbits,
			ReadBitrateKbits:  session.ReadBitrateKbits,
			WriteBitrateKbits: session.WriteBitrateKbits,
		})
	}
	return items
}

func timePointer(v time.Time) *time.Time {
	if v.IsZero() {
		return nil
	}
	return &v
}

func (h *Handler) buildStreamPlayURLs(ctx context.Context, streamID, appName string) []streamPlayURL {
	if h.mediaEngine == nil {
		return nil
	}
	if appName == "" {
		appName = "live"
	}
	protocols := []string{"hls", "ll-hls", "flv", "ws-flv", "webrtc", "fmp4", "rtmp", "rtsp"}
	urls := make([]streamPlayURL, 0, len(protocols))
	for _, protocol := range protocols {
		playURL, err := h.mediaEngine.BuildPlayURL(ctx, media.PlayURLRequest{
			StreamID: streamID,
			AppName:  appName,
			Protocol: protocol,
		})
		if err != nil || playURL == nil || playURL.URL == "" {
			continue
		}
		urls = append(urls, streamPlayURL{
			Protocol: protocol,
			URL:      playURL.URL,
			Backend:  "lalmax",
		})
	}
	return urls
}

func (h *Handler) handleGetStream(w http.ResponseWriter, r *http.Request) {
	if h.mediaEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "stream listing unavailable")
		return
	}

	streamID := chi.URLParam(r, "stream_id")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream_id is required")
		return
	}

	info, err := h.mediaEngine.GetStream(r.Context(), streamID)
	if err != nil {
		logger.Error("get stream failed", "stream_id", streamID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get stream")
		return
	}

	if info != nil {
		cameraRows, err := h.db.ListCameras(r.Context())
		if err != nil {
			logger.Error("list cameras for stream failed", "err", err)
			writeError(w, http.StatusInternalServerError, "failed to load camera mapping")
			return
		}
		cameraByID := make(map[string]string, len(cameraRows))
		for _, cam := range cameraRows {
			cameraByID[cam.ID] = cam.Name
		}
		bindings, err := h.db.ListStreamBindings(r.Context())
		if err != nil {
			logger.Error("list stream bindings failed", "err", err)
			writeError(w, http.StatusInternalServerError, "failed to load stream bindings")
			return
		}
		bindingByStreamID := make(map[string]string, len(bindings))
		for _, binding := range bindings {
			bindingByStreamID[binding.StreamID] = binding.CameraID
		}
		writeJSON(w, http.StatusOK, h.streamSummaryFromMediaInfo(r.Context(), *info, bindingByStreamID, cameraByID))
		return
	}

	item, ok := h.buildIdleStreamSummary(r.Context(), streamID)
	if !ok {
		writeError(w, http.StatusNotFound, "stream not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) buildIdleStreamSummary(ctx context.Context, streamID string) (streamSummary, bool) {
	cam, err := h.db.GetCamera(ctx, streamID)
	if err != nil {
		logger.Error("get camera for idle stream failed", "stream_id", streamID, "err", err)
		return streamSummary{}, false
	}
	if cam != nil && cam.Enabled && !cam.Archived {
		return streamSummary{
			Engine:         "lalmax",
			StreamID:       cam.ID,
			AppName:        "live",
			Managed:        true,
			ManagementType: "camera",
			CameraID:       cam.ID,
			CameraName:     cam.Name,
			SourceType:     "camera",
			Active:         false,
			PlayURLs:       h.buildStreamPlayURLs(ctx, cam.ID, "live"),
		}, true
	}

	binding, err := h.db.GetStreamBinding(ctx, streamID)
	if err != nil {
		logger.Error("get stream binding for idle stream failed", "stream_id", streamID, "err", err)
		return streamSummary{}, false
	}
	if binding != nil {
		boundCam, err := h.db.GetCamera(ctx, binding.CameraID)
		if err != nil {
			logger.Error("get bound camera for idle stream failed", "stream_id", streamID, "err", err)
			return streamSummary{}, false
		}
		if boundCam != nil && !boundCam.Archived {
			return streamSummary{
				Engine:         "lalmax",
				StreamID:       streamID,
				AppName:        "live",
				Managed:        true,
				ManagementType: "bound",
				CameraID:       binding.CameraID,
				CameraName:     boundCam.Name,
				SourceType:     "camera",
				Active:         false,
				PlayURLs:       h.buildStreamPlayURLs(ctx, streamID, "live"),
			}, true
		}
	}

	histories, _, err := h.db.ListStreamHistory(ctx, streamID, 1, 0)
	if err != nil {
		logger.Error("list stream history for idle stream failed", "stream_id", streamID, "err", err)
		return streamSummary{}, false
	}
	if len(histories) == 0 {
		return streamSummary{}, false
	}
	latest := histories[0]
	if time.Since(latest.StartedAt) > streamIdleHistoryMaxAge {
		return streamSummary{}, false
	}
	appName := latest.AppName
	if appName == "" {
		appName = "live"
	}
	lastSeen := latest.StartedAt
	if latest.EndedAt != nil {
		lastSeen = *latest.EndedAt
	}
	return streamSummary{
		Engine:        "lalmax",
		StreamID:      latest.StreamID,
		AppName:       appName,
		SourceType:    inferStreamSourceTypeFromProtocol(latest.Protocol),
		Active:        false,
		LastFrameTime: timePointer(lastSeen),
		PlayURLs:      h.buildStreamPlayURLs(ctx, latest.StreamID, appName),
	}, true
}

type bindCameraRequest struct {
	CameraID string `json:"camera_id"`
}

func (h *Handler) handleBindCamera(w http.ResponseWriter, r *http.Request) {
	if h.mediaEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "stream management unavailable")
		return
	}

	streamID := chi.URLParam(r, "stream_id")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream_id is required")
		return
	}

	var req bindCameraRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CameraID == "" {
		writeError(w, http.StatusBadRequest, "camera_id is required")
		return
	}

	info, err := h.mediaEngine.GetStream(r.Context(), streamID)
	if err != nil {
		logger.Error("get stream for bind failed", "stream_id", streamID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get stream")
		return
	}
	if info == nil {
		writeError(w, http.StatusNotFound, "stream not found")
		return
	}

	cam, err := h.db.GetCamera(r.Context(), req.CameraID)
	if err != nil {
		logger.Error("get camera for bind failed", "camera_id", req.CameraID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get camera")
		return
	}
	if cam == nil {
		writeError(w, http.StatusNotFound, "camera not found")
		return
	}

	if err := h.db.BindStreamToCamera(r.Context(), streamID, req.CameraID); err != nil {
		logger.Error("bind stream to camera failed", "stream_id", streamID, "camera_id", req.CameraID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to bind stream to camera")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"stream_id": streamID,
		"camera_id": req.CameraID,
		"status":    "bound",
	})
}

func (h *Handler) handleUnbindCamera(w http.ResponseWriter, r *http.Request) {
	if h.mediaEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "stream management unavailable")
		return
	}

	streamID := chi.URLParam(r, "stream_id")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream_id is required")
		return
	}

	binding, err := h.db.GetStreamBinding(r.Context(), streamID)
	if err != nil {
		logger.Error("get stream binding failed", "stream_id", streamID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get stream binding")
		return
	}
	if binding == nil {
		writeError(w, http.StatusNotFound, "stream binding not found")
		return
	}

	if err := h.db.UnbindStreamFromCamera(r.Context(), streamID); err != nil {
		logger.Error("unbind stream from camera failed", "stream_id", streamID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to unbind stream from camera")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"stream_id": streamID,
		"status":    "unbound",
	})
}

type promoteStreamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
}

func (h *Handler) handlePromoteStream(w http.ResponseWriter, r *http.Request) {
	if h.mediaEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "stream management unavailable")
		return
	}

	streamID := chi.URLParam(r, "stream_id")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream_id is required")
		return
	}

	var req promoteStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	info, err := h.mediaEngine.GetStream(r.Context(), streamID)
	if err != nil {
		logger.Error("get stream for promote failed", "stream_id", streamID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get stream")
		return
	}
	if info == nil {
		writeError(w, http.StatusNotFound, "stream not found")
		return
	}

	existingCam, err := h.db.GetCamera(r.Context(), streamID)
	if err != nil {
		logger.Error("check existing camera failed", "stream_id", streamID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to check existing camera")
		return
	}
	if existingCam != nil && !existingCam.Archived {
		writeError(w, http.StatusConflict, "stream already mapped to a camera")
		return
	}

	sourceType := "rtmp_push"
	if info.Publisher != nil {
		switch strings.ToLower(info.Publisher.Protocol) {
		case "srt":
			sourceType = "srt_push"
		case "rtsp", "relay_pull", "pull":
			sourceType = "relay_pull"
		}
	}

	encoding := ""
	if info.VideoCodec != "" {
		encoding = strings.ToLower(info.VideoCodec)
	}

	// Determine URL based on source type
	var cameraURL string
	if sourceType == "relay_pull" {
		// For relay pull streams, use the original source URL
		if info.Publisher != nil && info.Publisher.Remote != "" {
			cameraURL = info.Publisher.Remote
		}
	} else {
		// For push streams (RTMP/SRT), use lal's RTSP play URL
		// The recorder will pull from lal's RTSP server, not from the push client
		if h.mediaEngine != nil {
			playURL, err := h.mediaEngine.BuildPlayURL(r.Context(), media.PlayURLRequest{
				StreamID: streamID,
				AppName:  "live",
				Protocol: "rtsp",
			})
			if err == nil && playURL != nil && playURL.URL != "" {
				cameraURL = playURL.URL
			}
		}
		// Fallback to constructing URL manually
		if cameraURL == "" {
			cameraURL = fmt.Sprintf("rtsp://127.0.0.1:5544/live/%s", streamID)
		}
	}

	protocol := "rtsp"

	// Create camera config for CameraManager
	cam := config.CameraConfig{
		ID:       streamID,
		Name:     req.Name,
		Protocol: protocol,
		Encoding: encoding,
		URL:      cameraURL,
		Enabled:  true,
	}

	// If camera exists but is archived, unarchive and update it
	if existingCam != nil && existingCam.Archived {
		if err := h.db.UnarchiveCameraDB(r.Context(), streamID); err != nil {
			logger.Error("unarchive camera failed", "stream_id", streamID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to unarchive camera")
			return
		}
		// Update camera fields
		if err := h.db.UpsertCamera(r.Context(), streamID, req.Name, sourceType, encoding, cameraURL, "", "", true, req.Description, req.Location, ""); err != nil {
			logger.Error("update archived camera failed", "stream_id", streamID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to update camera")
			return
		}
		// Re-add to CameraManager if available
		if h.camMgr != nil {
			if _, err := h.camMgr.AddCamera(r.Context(), cam); err != nil {
				logger.Warn("re-add archived camera to manager failed", "stream_id", streamID, "error", err)
			}
		}
	} else {
		// Add camera to CameraManager (this will also insert into DB)
		if h.camMgr != nil {
			if _, err := h.camMgr.AddCamera(r.Context(), cam); err != nil {
				logger.Error("promote stream to camera via CameraManager failed", "stream_id", streamID, "err", err)
				writeError(w, http.StatusInternalServerError, "failed to promote stream to camera")
				return
			}
		} else {
			// Fallback to direct DB insert if CameraManager is not available
			if err := h.db.UpsertCamera(r.Context(), streamID, req.Name, sourceType, encoding, cameraURL, "", "", true, req.Description, req.Location, ""); err != nil {
				logger.Error("promote stream to camera failed", "stream_id", streamID, "err", err)
				writeError(w, http.StatusInternalServerError, "failed to promote stream to camera")
				return
			}
		}
	}

	// Update metadata if provided
	if req.Description != "" || req.Location != "" {
		if err := h.db.UpdateCameraMetadata(r.Context(), streamID, req.Description, req.Location, "", "", "", 0); err != nil {
			logger.Warn("failed to set camera metadata", "camera_id", streamID, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"stream_id":   streamID,
		"camera_id":   streamID,
		"source_type": sourceType,
		"status":      "promoted",
	})
}

func (h *Handler) handleDeleteStream(w http.ResponseWriter, r *http.Request) {
	if h.mediaEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "stream management unavailable")
		return
	}

	streamID := chi.URLParam(r, "stream_id")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream_id is required")
		return
	}

	info, err := h.mediaEngine.GetStream(r.Context(), streamID)
	if err != nil {
		logger.Error("get stream for delete failed", "stream_id", streamID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get stream")
		return
	}
	if info == nil {
		writeError(w, http.StatusNotFound, "stream not found")
		return
	}

	if info.Publisher != nil {
		if err := h.mediaEngine.KickSession(r.Context(), info.Publisher.SessionID); err != nil {
			logger.Error("kick publisher failed", "stream_id", streamID, "session_id", info.Publisher.SessionID, "err", err)
			writeError(w, http.StatusInternalServerError, "failed to kick publisher")
			return
		}
	}

	if err := h.mediaEngine.StopPull(r.Context(), streamID); err != nil {
		logger.Debug("stop pull failed (may not be a pull stream)", "stream_id", streamID, "err", err)
	}

	// Clear stream history
	if err := h.db.DeleteStreamHistory(r.Context(), streamID); err != nil {
		logger.Warn("failed to clear stream history", "stream_id", streamID, "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"stream_id": streamID,
		"status":    "deleted",
	})
}

func (h *Handler) handleKickPublisher(w http.ResponseWriter, r *http.Request) {
	if h.mediaEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "stream management unavailable")
		return
	}

	streamID := chi.URLParam(r, "stream_id")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream_id is required")
		return
	}

	info, err := h.mediaEngine.GetStream(r.Context(), streamID)
	if err != nil {
		logger.Error("get stream for kick publisher failed", "stream_id", streamID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get stream")
		return
	}
	if info == nil {
		writeError(w, http.StatusNotFound, "stream not found")
		return
	}

	if info.Publisher == nil {
		writeError(w, http.StatusNotFound, "no active publisher for this stream")
		return
	}

	if err := h.mediaEngine.KickSession(r.Context(), info.Publisher.SessionID); err != nil {
		logger.Error("kick publisher failed", "stream_id", streamID, "session_id", info.Publisher.SessionID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to kick publisher")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"stream_id":  streamID,
		"session_id": info.Publisher.SessionID,
		"status":     "kicked",
	})
}

func (h *Handler) handleListStreamHistory(w http.ResponseWriter, r *http.Request) {
	streamID := r.URL.Query().Get("stream_id")
	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		fmt.Sscanf(v, "%d", &offset)
	}

	items, total, err := h.db.ListStreamHistory(r.Context(), streamID, limit, offset)
	if err != nil {
		logger.Error("list stream history failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list stream history")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"history": items,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *Handler) handleDeleteStreamHistory(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "stream_id")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream_id is required")
		return
	}

	if err := h.db.DeleteStreamHistory(r.Context(), streamID); err != nil {
		logger.Error("delete stream history failed", "stream_id", streamID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete stream history")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"stream_id": streamID,
		"status":    "deleted",
	})
}

func (h *Handler) handleBanStream(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "stream_id")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream_id is required")
		return
	}

	if h.banMgr == nil {
		writeError(w, http.StatusServiceUnavailable, "ban manager unavailable")
		return
	}

	var req struct {
		Reason    string `json:"reason"`
		ExpiresAt string `json:"expires_at,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err == nil {
			expiresAt = &t
		}
	}

	if err := h.banMgr.Ban(r.Context(), streamID, req.Reason, expiresAt); err != nil {
		logger.Error("ban stream failed", "stream_id", streamID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to ban stream")
		return
	}

	// Kick any currently active publisher
	if h.mediaEngine != nil {
		info, err := h.mediaEngine.GetStream(r.Context(), streamID)
		if err == nil && info != nil && info.Publisher != nil {
			_ = h.mediaEngine.KickSession(r.Context(), info.Publisher.SessionID)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"stream_id": streamID,
		"status":    "banned",
	})
}

func (h *Handler) handleUnbanStream(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "stream_id")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream_id is required")
		return
	}

	if h.banMgr == nil {
		writeError(w, http.StatusServiceUnavailable, "ban manager unavailable")
		return
	}

	if err := h.banMgr.Unban(r.Context(), streamID); err != nil {
		logger.Error("unban stream failed", "stream_id", streamID, "error", err)
		writeError(w, http.StatusNotFound, "stream not banned")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"stream_id": streamID,
		"status":    "unbanned",
	})
}

func (h *Handler) handleListBans(w http.ResponseWriter, r *http.Request) {
	bans, err := h.db.ListStreamBans(r.Context())
	if err != nil {
		logger.Error("list bans failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list bans")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"bans": bans,
	})
}
