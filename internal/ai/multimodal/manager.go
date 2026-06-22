package multimodal

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

var logger = slog.Default().With("component", "ai-multimodal-manager")

// Manager coordinates multimodal analysis across different providers.
type Manager struct {
	mu        sync.RWMutex
	analyzers map[string]Analyzer
	provider  string // Current active provider
	callbacks map[string]func(AnalysisResult)
	cbCounter int
}

// NewManager creates a new multimodal analysis manager.
func NewManager() *Manager {
	return &Manager{
		analyzers: make(map[string]Analyzer),
		callbacks: make(map[string]func(AnalysisResult)),
	}
}

// RegisterAnalyzer registers an analyzer for a specific provider.
func (m *Manager) RegisterAnalyzer(provider string, analyzer Analyzer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.analyzers[provider] = analyzer
	logger.Info("Registered multimodal analyzer", "provider", provider)
}

// SetProvider sets the active provider for analysis.
func (m *Manager) SetProvider(provider string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.analyzers[provider]; !ok {
		return fmt.Errorf("provider %s not registered", provider)
	}

	m.provider = provider
	logger.Info("Set active multimodal provider", "provider", provider)
	return nil
}

// GetProvider returns the current active provider.
func (m *Manager) GetProvider() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.provider
}

// Analyze sends a frame to the active provider for analysis.
// The caller is responsible for enriching and publishing the result.
func (m *Manager) Analyze(ctx context.Context, frame []byte, prompt string) (*AnalysisResult, error) {
	m.mu.RLock()
	provider := m.provider
	analyzer, ok := m.analyzers[provider]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no active provider set")
	}

	if !analyzer.IsAvailable() {
		return nil, fmt.Errorf("provider %s is not available", provider)
	}

	result, err := analyzer.Analyze(ctx, frame, prompt)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// AnalyzeWithProvider sends a frame to a specific provider for analysis.
// The caller is responsible for enriching and publishing the result.
func (m *Manager) AnalyzeWithProvider(ctx context.Context, provider string, frame []byte, prompt string) (*AnalysisResult, error) {
	m.mu.RLock()
	analyzer, ok := m.analyzers[provider]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider %s not registered", provider)
	}

	if !analyzer.IsAvailable() {
		return nil, fmt.Errorf("provider %s is not available", provider)
	}

	result, err := analyzer.Analyze(ctx, frame, prompt)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// PublishResult dispatches an enriched analysis result to all registered callbacks.
func (m *Manager) PublishResult(result AnalysisResult) {
	m.dispatchResult(result)
}

// OnResult registers a callback for analysis results.
func (m *Manager) OnResult(cb func(AnalysisResult)) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cbCounter++
	id := fmt.Sprintf("multimodal-cb-%d", m.cbCounter)
	m.callbacks[id] = cb
	return id
}

// UnregisterCallback removes a callback.
func (m *Manager) UnregisterCallback(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.callbacks[id]
	delete(m.callbacks, id)
	return ok
}

// IsAvailable checks if the current provider is available.
func (m *Manager) IsAvailable() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.provider == "" {
		return false
	}

	analyzer, ok := m.analyzers[m.provider]
	if !ok {
		return false
	}

	return analyzer.IsAvailable()
}

// Status returns the current status of the multimodal manager.
func (m *Manager) Status() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]interface{}{
		"active_provider": m.provider,
		"providers":       make(map[string]bool),
	}

	providers := status["providers"].(map[string]bool)
	for name, analyzer := range m.analyzers {
		providers[name] = analyzer.IsAvailable()
	}

	return status
}

// Close closes all analyzers.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, analyzer := range m.analyzers {
		if err := analyzer.Close(); err != nil {
			logger.Error("Failed to close analyzer", "provider", name, "error", err)
		}
	}

	m.analyzers = make(map[string]Analyzer)
	logger.Info("Multimodal manager closed")
}

// dispatchResult sends the result to all registered callbacks.
func (m *Manager) dispatchResult(result AnalysisResult) {
	m.mu.RLock()
	callbacks := make([]func(AnalysisResult), 0, len(m.callbacks))
	for _, cb := range m.callbacks {
		callbacks = append(callbacks, cb)
	}
	m.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(result)
	}
}

// CreateAnalyzer creates an analyzer based on the provider configuration.
func CreateAnalyzer(cfg ProviderConfig) (Analyzer, error) {
	switch cfg.Provider {
	case "deepseek":
		return NewDeepSeekAnalyzer(DeepSeekConfig{
			ProviderConfig: cfg,
		}), nil
	case "openai", "qwen", "custom", "openai-compatible":
		return NewOpenAICompatibleAnalyzer(cfg), nil
	default:
		if cfg.Endpoint != "" && cfg.Model != "" {
			return NewOpenAICompatibleAnalyzer(cfg), nil
		}
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}
