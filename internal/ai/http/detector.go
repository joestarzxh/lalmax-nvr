package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

var logger = slog.Default().With("component", "ai-http")

// Config holds the HTTP remote AI backend configuration.
type Config struct {
	Endpoint string            `yaml:"endpoint"`
	APIKey   string            `yaml:"api_key"`
	Headers  map[string]string `yaml:"headers"`
	Timeout  int               `yaml:"timeout"` // milliseconds
}

// Detection represents a single object detection result.
type Detection struct {
	Label      string     `json:"label"`
	Confidence float32    `json:"confidence"`
	Box        [4]float32 `json:"box"` // [x, y, width, height] in normalized coordinates
}

// detectRequest is the JSON body sent to the remote AI service.
type detectRequest struct {
	Frame     string `json:"frame"`      // base64-encoded JPEG
	CameraID  string `json:"camera_id"`
	Timestamp int64  `json:"timestamp"`
}

// detectResponse is the JSON body returned by the remote AI service.
type detectResponse struct {
	Detections []Detection `json:"detections"`
	Error      string      `json:"error,omitempty"`
}

// Detector implements ai.Detector by calling a remote HTTP API.
type Detector struct {
	cfg    Config
	client *http.Client
	mu     sync.Mutex
	closed bool
}

// NewDetector creates a new HTTP-based remote detector.
func NewDetector(cfg Config) *Detector {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10000 // 10 seconds default
	}

	return &Detector{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Millisecond,
		},
	}
}

// Detect sends a frame to the remote AI service and returns detections.
func (d *Detector) Detect(ctx context.Context, frame []byte) ([]Detection, error) {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil, fmt.Errorf("detector is closed")
	}
	d.mu.Unlock()

	if d.cfg.Endpoint == "" {
		return nil, fmt.Errorf("HTTP endpoint not configured")
	}

	// Encode frame as base64
	b64 := base64.StdEncoding.EncodeToString(frame)

	reqBody := detectRequest{
		Frame:     b64,
		Timestamp: time.Now().UnixMilli(),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if d.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+d.cfg.APIKey)
	}
	for k, v := range d.cfg.Headers {
		req.Header.Set(k, v)
	}

	// Send request
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var detectResp detectResponse
	if err := json.NewDecoder(resp.Body).Decode(&detectResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if detectResp.Error != "" {
		return nil, fmt.Errorf("remote error: %s", detectResp.Error)
	}

	return detectResp.Detections, nil
}

// Close marks the detector as closed.
func (d *Detector) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	return nil
}

// IsAvailable checks if the remote endpoint is reachable.
func (d *Detector) IsAvailable() bool {
	if d.cfg.Endpoint == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.cfg.Endpoint, nil)
	if err != nil {
		return false
	}

	resp, err := d.client.Do(req)
	if err != nil {
		logger.Warn("HTTP AI endpoint not reachable", "endpoint", d.cfg.Endpoint, "error", err)
		return false
	}
	defer resp.Body.Close()

	// Any response means the server is reachable
	return true
}
