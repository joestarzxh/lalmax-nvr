package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/lalmax-pro/lalmax-nvr/internal/storage"
)

func (h *Handler) handleListRecordingPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.db.ListRecordingPlans(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list recording plans")
		return
	}
	if plans == nil {
		plans = []storage.RecordingPlanWithDetails{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"plans": plans})
}

func (h *Handler) handleGetRecordingPlan(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan id")
		return
	}
	plan, err := h.db.GetRecordingPlan(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"plan": plan})
}

type CreateRecordingPlanRequest struct {
	Name       string                        `json:"name"`
	Enabled    bool                          `json:"enabled"`
	TimeRanges []storage.RecordingPlanTimeRange `json:"time_ranges"`
	Channels   []storage.RecordingPlanChannel   `json:"channels"`
}

func (h *Handler) handleCreateRecordingPlan(w http.ResponseWriter, r *http.Request) {
	var req CreateRecordingPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	plan := &storage.RecordingPlanWithDetails{
		RecordingPlan: storage.RecordingPlan{
			Name:    req.Name,
			Enabled: req.Enabled,
		},
		TimeRanges: req.TimeRanges,
		Channels:   req.Channels,
	}

	id, err := h.db.CreateRecordingPlan(r.Context(), plan)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create recording plan")
		return
	}

	plan.ID = id
	writeJSON(w, http.StatusCreated, map[string]interface{}{"plan": plan})
}

type UpdateRecordingPlanRequest struct {
	Name       string                        `json:"name"`
	Enabled    bool                          `json:"enabled"`
	TimeRanges []storage.RecordingPlanTimeRange `json:"time_ranges"`
}

func (h *Handler) handleUpdateRecordingPlan(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan id")
		return
	}

	existing, err := h.db.GetRecordingPlan(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}

	var req UpdateRecordingPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	existing.Enabled = req.Enabled
	if req.TimeRanges != nil {
		existing.TimeRanges = req.TimeRanges
	}

	if err := h.db.UpdateRecordingPlan(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update recording plan")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"plan": existing})
}

func (h *Handler) handleDeleteRecordingPlan(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan id")
		return
	}
	if err := h.db.DeleteRecordingPlan(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete recording plan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type SetPlanChannelsRequest struct {
	CameraIDs []string `json:"camera_ids"`
}

func (h *Handler) handleSetPlanChannels(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan id")
		return
	}

	var req SetPlanChannelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.db.SetPlanChannels(r.Context(), id, req.CameraIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set plan channels")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type AddPlanChannelRequest struct {
	CameraID string `json:"camera_id"`
}

func (h *Handler) handleAddPlanChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan id")
		return
	}

	var req AddPlanChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CameraID == "" {
		writeError(w, http.StatusBadRequest, "camera_id is required")
		return
	}

	if err := h.db.AddPlanChannel(r.Context(), id, req.CameraID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add channel to plan")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleRemovePlanChannel(w http.ResponseWriter, r *http.Request) {
	planID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan id")
		return
	}
	cameraID := chi.URLParam(r, "camera_id")
	if cameraID == "" {
		writeError(w, http.StatusBadRequest, "camera_id is required")
		return
	}

	if err := h.db.RemovePlanChannel(r.Context(), planID, cameraID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove channel from plan")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
