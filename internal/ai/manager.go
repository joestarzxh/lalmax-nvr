package ai

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/config"
	"github.com/lalmax-pro/lalmax-nvr/internal/model"

	httpBackend "github.com/lalmax-pro/lalmax-nvr/internal/ai/http"
	"github.com/lalmax-pro/lalmax-nvr/internal/ai/webhook"
)

var logger = slog.Default().With("component", "ai-manager")

// Manager coordinates AI detection across all cameras.
// It supports two backend modes:
//   - http: NVR sends frames to an external AI service and receives detections
//   - webhook: External AI services push detections to NVR
type Manager struct {
	cfg      config.AIConfig
	detector *httpBackend.Detector // HTTP mode: sends frames externally
	receiver *webhook.Receiver    // Webhook mode: receives external pushes

	mu        sync.RWMutex
	enabled   map[string]bool      // cameraID -> enabled
	callbacks map[string]webhook.CallbackFunc
	cbCounter int
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new AI Manager based on configuration.
func NewManager(cfg config.AIConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		cfg:       cfg,
		enabled:   make(map[string]bool),
		callbacks: make(map[string]webhook.CallbackFunc),
		ctx:       ctx,
		cancel:    cancel,
	}

	if !cfg.Enabled {
		logger.Info("AI detection disabled")
		return m
	}

	switch cfg.Backend {
	case "http":
		if cfg.HTTP == nil {
			logger.Warn("AI backend is 'http' but no http config provided")
			return m
		}
		httpCfg := httpBackend.Config{
			Endpoint: cfg.HTTP.Endpoint,
			APIKey:   cfg.HTTP.APIKey,
			Headers:  cfg.HTTP.Headers,
			Timeout:  cfg.HTTP.Timeout,
		}
		m.detector = httpBackend.NewDetector(httpCfg)
		logger.Info("AI HTTP backend initialized", "endpoint", cfg.HTTP.Endpoint)

	case "webhook":
		m.receiver = webhook.NewReceiver()
		m.receiver.OnDetection(m.dispatchDetection)
		logger.Info("AI webhook backend initialized")

	case "disabled", "":
		logger.Info("AI detection disabled (no backend)")

	default:
		logger.Warn("unknown AI backend", "backend", cfg.Backend)
	}

	return m
}

// GetDetector returns the detector for HTTP mode (nil if not configured).
func (m *Manager) GetDetector() *httpBackend.Detector {
	return m.detector
}

// GetReceiver returns the webhook receiver (nil if not configured).
func (m *Manager) GetReceiver() *webhook.Receiver {
	return m.receiver
}

// Status returns the current AI subsystem status.
func (m *Manager) Status() Status {
	if !m.cfg.Enabled {
		return Status{Backend: "disabled", Available: false, Reason: "AI 已禁用"}
	}

	switch m.cfg.Backend {
	case "http":
		if m.detector == nil {
			return Status{Backend: "http", Available: false, Reason: "HTTP 后端未初始化"}
		}
		if m.detector.IsAvailable() {
			return Status{Backend: "http", Available: true}
		}
		return Status{Backend: "http", Available: false, Reason: "无法连接到远程 AI 服务: " + m.cfg.HTTP.Endpoint}

	case "webhook":
		return Status{Backend: "webhook", Available: true}

	default:
		return Status{Backend: "disabled", Available: false, Reason: "未知后端: " + m.cfg.Backend}
	}
}

// EnableCamera enables AI detection for a camera by subscribing to its StreamHub.
func (m *Manager) EnableCamera(camID string, hub *model.StreamHub) error {
	if !m.cfg.Enabled {
		return fmt.Errorf("AI is disabled")
	}
	if m.cfg.Backend != "http" {
		return fmt.Errorf("EnableCamera only works with http backend, current: %s", m.cfg.Backend)
	}
	if m.detector == nil {
		return fmt.Errorf("AI detector not initialized")
	}

	m.mu.Lock()
	if m.enabled[camID] {
		m.mu.Unlock()
		return fmt.Errorf("AI already enabled for camera %s", camID)
	}
	m.enabled[camID] = true
	m.mu.Unlock()

	// Subscribe to the camera's StreamHub
	subscriberID := "ai-" + camID
	err := hub.Subscribe(subscriberID, func(pts int64, au [][]byte) {
		m.processFrame(camID, pts, au)
	})
	if err != nil {
		m.mu.Lock()
		delete(m.enabled, camID)
		m.mu.Unlock()
		return fmt.Errorf("subscribe to stream hub: %w", err)
	}

	logger.Info("AI detection enabled for camera", "camera_id", camID)
	return nil
}

// DisableCamera disables AI detection for a camera.
func (m *Manager) DisableCamera(camID string, hub *model.StreamHub) {
	m.mu.Lock()
	if !m.enabled[camID] {
		m.mu.Unlock()
		return
	}
	delete(m.enabled, camID)
	m.mu.Unlock()

	if hub != nil {
		hub.Unsubscribe("ai-" + camID)
	}

	logger.Info("AI detection disabled for camera", "camera_id", camID)
}

// IsEnabled returns whether AI detection is enabled for a camera.
func (m *Manager) IsEnabled(camID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled[camID]
}

// EnabledCameras returns a list of camera IDs with AI enabled.
func (m *Manager) EnabledCameras() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.enabled))
	for id := range m.enabled {
		ids = append(ids, id)
	}
	return ids
}

// OnDetection registers a callback for detection events.
func (m *Manager) OnDetection(cb webhook.CallbackFunc) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cbCounter++
	id := fmt.Sprintf("mgr-cb-%d", m.cbCounter)
	m.callbacks[id] = cb
	return id
}

// UnregisterCallback removes a detection callback.
func (m *Manager) UnregisterCallback(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.callbacks[id]
	delete(m.callbacks, id)
	return ok
}

// StopAll disables all cameras and stops the manager.
func (m *Manager) StopAll() {
	m.mu.Lock()
	m.enabled = make(map[string]bool)
	m.mu.Unlock()

	if m.detector != nil {
		m.detector.Close()
	}

	m.cancel()
	logger.Info("AI manager stopped")
}

// processFrame sends a frame to the HTTP detector and dispatches results.
func (m *Manager) processFrame(camID string, pts int64, au [][]byte) {
	if m.detector == nil {
		return
	}

	// Frame skip: only process every Nth frame
	frameSkip := m.cfg.FrameSkipRate
	if frameSkip <= 0 {
		frameSkip = 3
	}
	// Use PTS as a simple frame counter proxy
	if pts%int64(frameSkip) != 0 {
		return
	}

	// Concatenate NALUs into a single frame buffer
	frame := concatNALUs(au)
	if len(frame) == 0 {
		return
	}

	// Create a context with timeout
	timeout := m.cfg.InferenceTimeoutMs
	if timeout <= 0 {
		timeout = 30000
	}
	ctx, cancel := context.WithTimeout(m.ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	// Send to detector
	httpDetections, err := m.detector.Detect(ctx, frame)
	if err != nil {
		logger.Debug("AI detection failed", "camera_id", camID, "error", err)
		return
	}

	// Filter by confidence threshold and convert types
	threshold := m.cfg.ConfidenceThreshold
	if threshold <= 0 {
		threshold = 0.3
	}
	filtered := make([]Detection, 0, len(httpDetections))
	for _, d := range httpDetections {
		if d.Confidence >= float32(threshold) {
			filtered = append(filtered, Detection{
				Label:      d.Label,
				Confidence: d.Confidence,
				Box:        d.Box,
			})
		}
	}

	if len(filtered) == 0 {
		return
	}

	// Build result (convert to webhook type)
	webhookDetections := make([]webhook.Detection, len(filtered))
	for i, d := range filtered {
		webhookDetections[i] = webhook.Detection{
			Label:      d.Label,
			Confidence: d.Confidence,
			Box:        d.Box,
		}
	}
	result := webhook.DetectionResult{
		CameraID:   camID,
		PTS:        pts,
		Timestamp:  time.Now().UnixMilli(),
		Detections: webhookDetections,
	}

	// Dispatch to callbacks
	m.dispatchDetection(result)
}

func (m *Manager) dispatchDetection(result webhook.DetectionResult) {
	m.mu.RLock()
	callbacks := make([]webhook.CallbackFunc, 0, len(m.callbacks))
	for _, cb := range m.callbacks {
		callbacks = append(callbacks, cb)
	}
	m.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(result)
	}
}

// concatNALUs concatenates NALU access units into a single buffer with start codes.
func concatNALUs(au [][]byte) []byte {
	if len(au) == 0 {
		return nil
	}

	totalLen := 0
	for _, nalu := range au {
		totalLen += 4 + len(nalu) // 4 bytes for start code
	}

	buf := make([]byte, 0, totalLen)
	for _, nalu := range au {
		buf = append(buf, 0x00, 0x00, 0x00, 0x01) // Annex B start code
		buf = append(buf, nalu...)
	}
	return buf
}
