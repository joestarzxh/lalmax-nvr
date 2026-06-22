package multimodal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// DeepSeekConfig holds DeepSeek-specific configuration.
type DeepSeekConfig struct {
	ProviderConfig `yaml:",inline"`
	VisionModel    string `yaml:"vision_model" json:"visionModel"` // Vision-specific model (e.g., "deepseek-vl")
}

// deepSeekRequest represents the API request body.
type deepSeekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepSeekMessage `json:"messages"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float32           `json:"temperature,omitempty"`
	Stream      bool              `json:"stream"`
}

// deepSeekMessage represents a single message in the conversation.
type deepSeekMessage struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

// contentPart represents a part of the message content.
type contentPart struct {
	Type     string    `json:"type"`                // "text" or "image_url"
	Text     string    `json:"text,omitempty"`      // For text type
	ImageURL *imageURL `json:"image_url,omitempty"` // For image_url type
}

// imageURL represents an image URL with optional detail level.
type imageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", or "auto"
}

// deepSeekResponse represents the API response.
type deepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// DeepSeekAnalyzer implements the Analyzer interface for DeepSeek's multimodal models.
type DeepSeekAnalyzer struct {
	cfg    DeepSeekConfig
	client *http.Client
	mu     sync.Mutex
	closed bool
}

// NewDeepSeekAnalyzer creates a new DeepSeek analyzer.
func NewDeepSeekAnalyzer(cfg DeepSeekConfig) *DeepSeekAnalyzer {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 // 60 seconds default for LLM calls
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.deepseek.com/v1"
	}
	cfg.Endpoint = endpoint

	return &DeepSeekAnalyzer{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// Analyze sends a frame to DeepSeek's vision model and returns analysis.
func (a *DeepSeekAnalyzer) Analyze(ctx context.Context, frame []byte, prompt string) (*AnalysisResult, error) {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil, fmt.Errorf("analyzer is closed")
	}
	a.mu.Unlock()

	if a.cfg.APIKey == "" {
		return nil, fmt.Errorf("DeepSeek API key not configured")
	}

	// Use default prompt if none provided
	if prompt == "" {
		prompt = DefaultPrompt
	}

	// Encode frame as base64 data URL
	b64 := base64.StdEncoding.EncodeToString(frame)
	dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", b64)

	// Determine model to use
	model := a.cfg.Model
	if model == "" {
		model = "deepseek-chat" // Default model
	}
	if a.cfg.VisionModel != "" {
		model = a.cfg.VisionModel
	}

	// Build request
	maxTokens := a.cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2000
	}

	temperature := a.cfg.Temperature
	if temperature <= 0 {
		temperature = 0.7
	}

	reqBody := deepSeekRequest{
		Model: model,
		Messages: []deepSeekMessage{
			{
				Role: "user",
				Content: []contentPart{
					{
						Type: "text",
						Text: prompt,
					},
					{
						Type: "image_url",
						ImageURL: &imageURL{
							URL:    dataURL,
							Detail: "high",
						},
					},
				},
			},
		},
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build endpoint URL
	endpoint := fmt.Sprintf("%s/chat/completions", a.cfg.Endpoint)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)

	// Send request
	startTime := time.Now()
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek API request failed: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime)

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DeepSeek API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var deepResp deepSeekResponse
	if err := json.NewDecoder(resp.Body).Decode(&deepResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if deepResp.Error != nil {
		return nil, fmt.Errorf("DeepSeek error: %s", deepResp.Error.Message)
	}

	if len(deepResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from DeepSeek")
	}

	analysis := deepResp.Choices[0].Message.Content

	logger.Info("DeepSeek analysis completed",
		"latency", latency.String(),
		"model", model,
		"response_length", len(analysis),
	)

	return &AnalysisResult{
		CameraID:   "", // Will be set by caller
		Timestamp:  time.Now().UnixMilli(),
		Analysis:   analysis,
		Labels:     extractLabels(analysis),
		Confidence: 0.85, // Default confidence for LLM analysis
		Metadata: map[string]string{
			"model":    model,
			"provider": "deepseek",
			"latency":  latency.String(),
		},
	}, nil
}

// IsAvailable checks if the DeepSeek API is reachable.
func (a *DeepSeekAnalyzer) IsAvailable() bool {
	if a.cfg.APIKey == "" {
		return false
	}

	// Simple health check - try to connect to the endpoint
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	endpoint := fmt.Sprintf("%s/models", a.cfg.Endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}

	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)

	resp, err := a.client.Do(req)
	if err != nil {
		logger.Warn("DeepSeek API not reachable", "endpoint", a.cfg.Endpoint, "error", err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Close marks the analyzer as closed.
func (a *DeepSeekAnalyzer) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	return nil
}

// extractLabels extracts potential labels/keywords from the analysis text.
func extractLabels(analysis string) []string {
	// Simple keyword extraction - in production, this could be more sophisticated
	keywords := []string{
		"人员", "车辆", "异常", "安全", "入侵", "徘徊", "聚集",
		"遗留物", "烟火", "打架", "摔倒", "盗窃", "破坏",
	}

	var labels []string
	for _, kw := range keywords {
		if contains(analysis, kw) {
			labels = append(labels, kw)
		}
	}

	return labels
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
