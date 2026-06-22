package ai

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/config"
	"github.com/lalmax-pro/lalmax-nvr/internal/model"

	httpBackend "github.com/lalmax-pro/lalmax-nvr/internal/ai/http"
	"github.com/lalmax-pro/lalmax-nvr/internal/ai/multimodal"
	"github.com/lalmax-pro/lalmax-nvr/internal/ai/webhook"
)

var logger = slog.Default().With("component", "ai-manager")

// Manager coordinates AI detection across all cameras.
// It supports multiple backend modes:
//   - http: NVR sends frames to an external AI service and receives detections
//   - webhook: External AI services push detections to NVR
//   - multimodal: NVR sends frames to multimodal LLMs (DeepSeek, OpenAI, etc.) for analysis
type Manager struct {
	cfg               config.AIConfig
	detector          *httpBackend.Detector // HTTP mode: sends frames externally
	receiver          *webhook.Receiver     // Webhook mode: receives external pushes
	multimodalManager *multimodal.Manager   // Multimodal mode: sends frames to LLMs
	store             Store

	mu           sync.RWMutex
	enabled      map[string]bool // cameraID -> enabled
	callbacks    map[string]webhook.CallbackFunc
	cbCounter    int
	analysisSem  chan struct{}
	lastAnalysis map[string]time.Time
	frameCounts  map[string]uint64
	unsupported  map[string]bool
	codecs       map[string]model.Format
	decoders     map[string]*realtimeDecoder
	ctx          context.Context
	cancel       context.CancelFunc
}

// SetStore sets the persistence backend for AI history.
func (m *Manager) SetStore(store Store) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = store
}

// NewManager creates a new AI Manager based on configuration.
func NewManager(cfg config.AIConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		cfg:          cfg,
		enabled:      make(map[string]bool),
		callbacks:    make(map[string]webhook.CallbackFunc),
		analysisSem:  make(chan struct{}, 2),
		lastAnalysis: make(map[string]time.Time),
		frameCounts:  make(map[string]uint64),
		unsupported:  make(map[string]bool),
		codecs:       make(map[string]model.Format),
		decoders:     make(map[string]*realtimeDecoder),
		ctx:          ctx,
		cancel:       cancel,
	}

	if !cfg.Enabled {
		logger.Info("AI detection disabled")
		return m
	}

	m.initMultimodal()

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
		m.receiver.OnDetection(func(result webhook.DetectionResult) {
			m.persistDetection(result)
			m.dispatchDetection(result)
		})
		logger.Info("AI webhook backend initialized")

	case "multimodal":
		if m.multimodalManager == nil {
			logger.Warn("AI backend is 'multimodal' but multimodal manager is not initialized")
			return m
		}
		logger.Info("AI multimodal backend initialized", "provider", cfg.Multimodal.Provider)

	case "disabled", "":
		logger.Info("AI detection disabled (no backend)")

	default:
		logger.Warn("unknown AI backend", "backend", cfg.Backend)
	}

	return m
}

// SetCameraCodec records the live stream codec used by the camera's StreamHub.
func (m *Manager) SetCameraCodec(camID string, codec model.Format) {
	if camID == "" || codec == "" {
		return
	}
	m.mu.Lock()
	m.codecs[camID] = codec
	m.mu.Unlock()
}

func (m *Manager) initMultimodal() {
	if m.cfg.Multimodal == nil || !m.cfg.Multimodal.Enabled {
		return
	}
	m.multimodalManager = multimodal.NewManager()
	for name, providerCfg := range m.cfg.Multimodal.Providers {
		analyzer, err := multimodal.CreateAnalyzer(multimodal.ProviderConfig{
			Provider:    providerCfg.Provider,
			APIKey:      providerCfg.APIKey,
			Endpoint:    providerCfg.Endpoint,
			Model:       providerCfg.Model,
			VisionModel: providerCfg.VisionModel,
			MaxTokens:   providerCfg.MaxTokens,
			Temperature: providerCfg.Temperature,
			Timeout:     providerCfg.Timeout,
		})
		if err != nil {
			logger.Error("Failed to create analyzer", "provider", name, "error", err)
			continue
		}
		m.multimodalManager.RegisterAnalyzer(name, analyzer)
	}
	if err := m.multimodalManager.SetProvider(m.cfg.Multimodal.Provider); err != nil {
		logger.Error("Failed to set active provider", "provider", m.cfg.Multimodal.Provider, "error", err)
		return
	}
	logger.Info("AI multimodal analyzer initialized", "provider", m.cfg.Multimodal.Provider)
}

// GetDetector returns the detector for HTTP mode (nil if not configured).
func (m *Manager) GetDetector() *httpBackend.Detector {
	return m.detector
}

// GetReceiver returns the webhook receiver (nil if not configured).
func (m *Manager) GetReceiver() *webhook.Receiver {
	return m.receiver
}

// GetMultimodalManager returns the multimodal manager (nil if not configured).
func (m *Manager) GetMultimodalManager() *multimodal.Manager {
	return m.multimodalManager
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
			if _, err := resolveFFmpegPath(m.cfg.FFmpegPath); err != nil {
				return Status{Backend: "http", Available: true, Reason: "YOLO 服务可用；H264/H265 实时分析需要安装 FFmpeg 或配置 ai.ffmpeg_path"}
			}
			return Status{Backend: "http", Available: true}
		}
		return Status{Backend: "http", Available: false, Reason: "无法连接到远程 AI 服务: " + m.cfg.HTTP.Endpoint}

	case "webhook":
		return Status{Backend: "webhook", Available: true}

	case "multimodal":
		if m.multimodalManager == nil {
			return Status{Backend: "multimodal", Available: false, Reason: "多模态后端未初始化"}
		}
		if m.multimodalManager.IsAvailable() {
			if _, err := resolveFFmpegPath(m.cfg.FFmpegPath); err != nil {
				return Status{Backend: "multimodal", Available: true, Reason: "多模态服务可用；H264/H265 实时分析需要安装 FFmpeg 或配置 ai.ffmpeg_path"}
			}
			return Status{Backend: "multimodal", Available: true}
		}
		return Status{Backend: "multimodal", Available: false, Reason: "多模态服务不可用"}

	default:
		return Status{Backend: "disabled", Available: false, Reason: "未知后端: " + m.cfg.Backend}
	}
}

// EnableCamera enables AI detection for a camera by subscribing to its StreamHub.
func (m *Manager) EnableCamera(camID string, hub *model.StreamHub) error {
	if !m.cfg.Enabled {
		return fmt.Errorf("AI is disabled")
	}
	if m.cfg.Backend != "http" && m.cfg.Backend != "multimodal" {
		return fmt.Errorf("EnableCamera only works with http or multimodal backend, current: %s", m.cfg.Backend)
	}
	if m.cfg.Backend == "http" && m.detector == nil {
		return fmt.Errorf("AI HTTP detector not initialized")
	}
	if m.cfg.Backend == "multimodal" && m.multimodalManager == nil {
		return fmt.Errorf("AI multimodal analyzer not initialized")
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
	delete(m.frameCounts, camID)
	delete(m.unsupported, camID)
	delete(m.codecs, camID)
	decoder := m.decoders[camID]
	delete(m.decoders, camID)
	m.mu.Unlock()
	if decoder != nil {
		decoder.close()
	}

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
	m.frameCounts = make(map[string]uint64)
	m.unsupported = make(map[string]bool)
	decoders := m.decoders
	m.decoders = make(map[string]*realtimeDecoder)
	m.mu.Unlock()
	for _, decoder := range decoders {
		decoder.close()
	}

	if m.detector != nil {
		m.detector.Close()
	}

	if m.multimodalManager != nil {
		m.multimodalManager.Close()
	}

	m.cancel()
	logger.Info("AI manager stopped")
}

// processFrame sends a frame to the appropriate backend and dispatches results.
func (m *Manager) processFrame(camID string, pts int64, au [][]byte) {
	frame, ok := imageFrameFromAU(au)
	if ok {
		m.processRealtimeImage(camID, pts, frame)
		return
	}

	decoder, err := m.decoderForCamera(camID)
	if err != nil {
		m.logUnsupportedFrame(camID, err)
		return
	}
	decoder.enqueue(pts, concatNALUs(au))
}

func (m *Manager) processRealtimeImage(camID string, pts int64, frame []byte) {
	if len(frame) == 0 {
		return
	}
	frameSkip := m.cfg.FrameSkipRate
	if frameSkip <= 0 {
		frameSkip = 3
	}
	if !m.shouldProcessLiveFrame(camID, frameSkip) {
		return
	}

	// Create a context with timeout
	timeout := m.cfg.InferenceTimeoutMs
	if timeout <= 0 {
		timeout = 30000
	}
	ctx, cancel := context.WithTimeout(m.ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	switch m.cfg.Backend {
	case "http":
		m.processFrameHTTP(ctx, camID, pts, frame)
	case "multimodal":
		m.processFrameMultimodal(ctx, camID, pts, frame)
	}
}

func (m *Manager) decoderForCamera(camID string) (*realtimeDecoder, error) {
	m.mu.Lock()
	if decoder := m.decoders[camID]; decoder != nil {
		m.mu.Unlock()
		return decoder, nil
	}
	codec := m.codecs[camID]
	m.mu.Unlock()

	switch codec {
	case model.FormatH264:
		codec = model.FormatH264
	case model.FormatH265:
		codec = model.FormatH265
	default:
		return nil, fmt.Errorf("unsupported or unknown realtime codec %q", codec)
	}

	decoder, err := newRealtimeDecoder(m.ctx, m.cfg.FFmpegPath, camID, codec, func(pts int64, jpeg []byte) {
		m.processRealtimeImage(camID, pts, jpeg)
	})
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	if existing := m.decoders[camID]; existing != nil {
		m.mu.Unlock()
		decoder.close()
		return existing, nil
	}
	m.decoders[camID] = decoder
	m.mu.Unlock()

	logger.Info("AI realtime decoder started", "camera_id", camID, "codec", codec)
	return decoder, nil
}

// processFrameHTTP sends a frame to the HTTP detector.
func (m *Manager) processFrameHTTP(ctx context.Context, camID string, pts int64, frame []byte) {
	if m.detector == nil {
		return
	}

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
		ImageURL:   frameDataURL(frame),
	}

	// Dispatch to callbacks
	m.persistDetection(result)
	m.dispatchDetection(result)

	m.queueSemanticAnalysis(camID, pts, frame, webhookDetections)
}

// processFrameMultimodal sends a frame to the multimodal LLM for analysis.
func (m *Manager) processFrameMultimodal(ctx context.Context, camID string, pts int64, frame []byte) {
	if m.multimodalManager == nil {
		return
	}

	prompt := ""
	if m.cfg.Multimodal != nil {
		prompt = m.cfg.Multimodal.AnalysisPrompt
	}

	result, err := m.multimodalManager.Analyze(ctx, frame, prompt)
	if err != nil {
		logger.Debug("Multimodal analysis failed", "camera_id", camID, "error", err)
		return
	}

	result.CameraID = camID
	result.ImageURL = frameDataURL(frame)
	m.persistAnalysis(*result)

	logger.Info("Multimodal analysis completed",
		"camera_id", camID,
		"provider", result.Metadata["provider"],
		"labels", result.Labels,
		"analysis_length", len(result.Analysis),
	)
	m.multimodalManager.PublishResult(*result)
}

func (m *Manager) queueSemanticAnalysis(camID string, pts int64, frame []byte, detections []webhook.Detection) {
	if m.multimodalManager == nil || len(detections) == 0 {
		return
	}
	if m.analysisCoolingDown(camID) {
		return
	}
	select {
	case m.analysisSem <- struct{}{}:
	default:
		logger.Debug("Skipping semantic analysis, queue is full", "camera_id", camID)
		return
	}

	frameCopy := append([]byte(nil), frame...)
	detectionCopy := append([]webhook.Detection(nil), detections...)
	go func() {
		defer func() { <-m.analysisSem }()

		timeout := m.cfg.InferenceTimeoutMs
		if timeout <= 0 {
			timeout = 30000
		}
		ctx, cancel := context.WithTimeout(m.ctx, time.Duration(timeout)*time.Millisecond)
		defer cancel()

		prompt := m.semanticPrompt(detectionCopy)
		result, err := m.multimodalManager.Analyze(ctx, frameCopy, prompt)
		if err != nil {
			logger.Debug("Semantic analysis failed", "camera_id", camID, "error", err)
			return
		}
		result.CameraID = camID
		result.ImageURL = frameDataURL(frameCopy)
		result.TriggerDetections = toTriggerDetections(detectionCopy)
		if result.Metadata == nil {
			result.Metadata = map[string]string{}
		}
		result.Metadata["trigger"] = "yolo"
		result.Metadata["pts"] = fmt.Sprintf("%d", pts)
		result.Metadata["objects"] = detectionSummary(detectionCopy)
		m.persistAnalysis(*result)
		m.multimodalManager.PublishResult(*result)
	}()
}

func (m *Manager) persistDetection(result webhook.DetectionResult) {
	m.mu.RLock()
	store := m.store
	m.mu.RUnlock()
	if store == nil {
		return
	}
	aiDetections := make([]Detection, 0, len(result.Detections))
	for _, d := range result.Detections {
		aiDetections = append(aiDetections, Detection{
			Label:      d.Label,
			Confidence: d.Confidence,
			Box:        d.Box,
		})
	}
	if err := store.InsertAIDetection(m.ctx, DetectionResult{
		CameraID:   result.CameraID,
		PTS:        result.PTS,
		Timestamp:  result.Timestamp,
		ImageURL:   result.ImageURL,
		Detections: aiDetections,
	}); err != nil {
		logger.Debug("persist AI detection failed", "camera_id", result.CameraID, "error", err)
	}
}

func (m *Manager) persistAnalysis(result multimodal.AnalysisResult) {
	m.mu.RLock()
	store := m.store
	m.mu.RUnlock()
	if store == nil {
		return
	}
	if err := store.InsertAIAnalysis(m.ctx, result); err != nil {
		logger.Debug("persist AI analysis failed", "camera_id", result.CameraID, "error", err)
	}
}

func (m *Manager) analysisCoolingDown(camID string) bool {
	interval := 10 * time.Second
	if m.cfg.Multimodal != nil && m.cfg.Multimodal.AnalysisInterval != "" {
		if parsed, err := time.ParseDuration(m.cfg.Multimodal.AnalysisInterval); err == nil && parsed > 0 {
			interval = parsed
		}
	}

	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	if last, ok := m.lastAnalysis[camID]; ok && now.Sub(last) < interval {
		return true
	}
	m.lastAnalysis[camID] = now
	return false
}

func (m *Manager) semanticPrompt(detections []webhook.Detection) string {
	base := multimodal.DefaultPrompt
	if m.cfg.Multimodal != nil && m.cfg.Multimodal.AnalysisPrompt != "" {
		base = m.cfg.Multimodal.AnalysisPrompt
	}
	return fmt.Sprintf("%s\n\nYOLO 已检测到以下目标：%s。请结合目标检测结果分析这些目标之间的行为语义、风险等级和建议处置。", base, detectionSummary(detections))
}

func detectionSummary(detections []webhook.Detection) string {
	if len(detections) == 0 {
		return "无"
	}
	counts := make(map[string]int)
	for _, d := range detections {
		counts[d.Label]++
	}
	summary := ""
	for label, count := range counts {
		if summary != "" {
			summary += "，"
		}
		summary += fmt.Sprintf("%s x%d", label, count)
	}
	return summary
}

func toTriggerDetections(detections []webhook.Detection) []multimodal.TriggerDetection {
	out := make([]multimodal.TriggerDetection, 0, len(detections))
	for _, d := range detections {
		out = append(out, multimodal.TriggerDetection{
			Label:      d.Label,
			Confidence: d.Confidence,
			Box:        d.Box,
		})
	}
	return out
}

func frameDataURL(frame []byte) string {
	if len(frame) == 0 {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(frame)
}

func (m *Manager) shouldProcessLiveFrame(camID string, frameSkip int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.frameCounts[camID]++
	return m.frameCounts[camID]%uint64(frameSkip) == 0
}

func (m *Manager) logUnsupportedFrame(camID string, err error) {
	m.mu.Lock()
	if m.unsupported[camID] {
		m.mu.Unlock()
		return
	}
	m.unsupported[camID] = true
	m.mu.Unlock()

	logger.Warn("AI realtime decoder unavailable; frame skipped", "camera_id", camID, "error", err)
}

func imageFrameFromAU(au [][]byte) ([]byte, bool) {
	if len(au) == 0 {
		return nil, false
	}
	if len(au) == 1 && isJPEG(au[0]) {
		return au[0], true
	}
	frame := concatNALUs(au)
	if isJPEG(frame) {
		return frame, true
	}
	return nil, false
}

func isJPEG(data []byte) bool {
	return len(data) >= 4 && data[0] == 0xff && data[1] == 0xd8
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
