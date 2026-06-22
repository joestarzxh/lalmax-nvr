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

// OpenAICompatibleAnalyzer calls providers that implement OpenAI's chat/completions
// vision message shape, including OpenAI, Qwen-compatible gateways, and custom
// OpenAI-compatible base URLs.
type OpenAICompatibleAnalyzer struct {
	cfg    ProviderConfig
	client *http.Client
	mu     sync.Mutex
	closed bool
}

func NewOpenAICompatibleAnalyzer(cfg ProviderConfig) *OpenAICompatibleAnalyzer {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60
	}
	if cfg.Endpoint == "" && cfg.Provider == "openai" {
		cfg.Endpoint = "https://api.openai.com/v1"
	}
	return &OpenAICompatibleAnalyzer{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

func (a *OpenAICompatibleAnalyzer) Analyze(ctx context.Context, frame []byte, prompt string) (*AnalysisResult, error) {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil, fmt.Errorf("analyzer is closed")
	}
	a.mu.Unlock()

	if a.cfg.APIKey == "" {
		return nil, fmt.Errorf("%s API key not configured", a.providerName())
	}
	if a.cfg.Endpoint == "" {
		return nil, fmt.Errorf("%s endpoint not configured", a.providerName())
	}
	if prompt == "" {
		prompt = DefaultPrompt
	}

	model := a.cfg.VisionModel
	if model == "" {
		model = a.cfg.Model
	}
	if model == "" {
		return nil, fmt.Errorf("%s model not configured", a.providerName())
	}

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
					{Type: "text", Text: prompt},
					{
						Type: "image_url",
						ImageURL: &imageURL{
							URL:    "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(frame),
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.Endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)

	start := time.Now()
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s API request failed: %w", a.providerName(), err)
	}
	defer resp.Body.Close()

	latency := time.Since(start)
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s API error (HTTP %d): %s", a.providerName(), resp.StatusCode, string(respBody))
	}

	var parsed deepSeekResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("%s error: %s", a.providerName(), parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("no response from %s", a.providerName())
	}

	analysis := parsed.Choices[0].Message.Content
	return &AnalysisResult{
		Timestamp:  time.Now().UnixMilli(),
		Analysis:   analysis,
		Labels:     extractLabels(analysis),
		Confidence: 0.85,
		Metadata: map[string]string{
			"model":    model,
			"provider": a.providerName(),
			"latency":  latency.String(),
		},
	}, nil
}

func (a *OpenAICompatibleAnalyzer) IsAvailable() bool {
	return a.cfg.APIKey != "" && a.cfg.Endpoint != "" && (a.cfg.Model != "" || a.cfg.VisionModel != "")
}

func (a *OpenAICompatibleAnalyzer) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	return nil
}

func (a *OpenAICompatibleAnalyzer) providerName() string {
	if a.cfg.Provider == "" {
		return "openai-compatible"
	}
	return a.cfg.Provider
}
