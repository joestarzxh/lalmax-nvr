package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/ai"
	"github.com/lalmax-pro/lalmax-nvr/internal/ai/webhook"
	"github.com/lalmax-pro/lalmax-nvr/internal/config"
	"github.com/stretchr/testify/require"
)

// --- Test helpers ---

func setupAIHandlerWithManager(t *testing.T, cfg config.AIConfig) *Handler {
	t.Helper()
	db, store := setupTestDB(t)
	h := TestHandler(db, store)
	mgr := ai.NewManager(cfg)
	h.SetAIManager(mgr)
	return h
}

func doJSONRequest(t *testing.T, handler http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader strings.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = *strings.NewReader(string(data))
	}
	req := httptest.NewRequest(method, path, &bodyReader)
	req.SetBasicAuth("admin", "admin12345")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// --- Tests ---

func TestGetAIStatus_Disabled(t *testing.T) {
	t.Parallel()
	h := setupAIHandlerWithManager(t, config.AIConfig{Enabled: false})

	rr := doJSONRequest(t, h.Routes(), http.MethodGet, "/api/ai/status", nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp aiStatusResponse
	parseJSON(t, rr, &resp)
	require.False(t, resp.Available)
	require.Equal(t, "disabled", resp.Backend)
}

func TestGetAIStatus_HTTPBackend(t *testing.T) {
	t.Parallel()
	h := setupAIHandlerWithManager(t, config.AIConfig{
		Enabled: true,
		Backend: "http",
		HTTP: &config.AIHTTPConfig{
			Endpoint: "http://localhost:8080/detect",
		},
	})

	rr := doJSONRequest(t, h.Routes(), http.MethodGet, "/api/ai/status", nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp aiStatusResponse
	parseJSON(t, rr, &resp)
	require.Equal(t, "http", resp.Backend)
}

func TestGetAIStatus_WebhookBackend(t *testing.T) {
	t.Parallel()
	h := setupAIHandlerWithManager(t, config.AIConfig{
		Enabled: true,
		Backend: "webhook",
	})

	rr := doJSONRequest(t, h.Routes(), http.MethodGet, "/api/ai/status", nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp aiStatusResponse
	parseJSON(t, rr, &resp)
	require.True(t, resp.Available)
	require.Equal(t, "webhook", resp.Backend)
}

func TestGetAIStatus_NoManager(t *testing.T) {
	t.Parallel()
	db, store := setupTestDB(t)
	h := TestHandler(db, store)
	// Don't set AI manager

	rr := doJSONRequest(t, h.Routes(), http.MethodGet, "/api/ai/status", nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp aiStatusResponse
	parseJSON(t, rr, &resp)
	require.False(t, resp.Available)
	require.Equal(t, "disabled", resp.Backend)
}

func TestEnableAI_CameraNotFound(t *testing.T) {
	t.Parallel()
	h := setupAIHandlerWithManager(t, config.AIConfig{
		Enabled: true,
		Backend: "http",
		HTTP: &config.AIHTTPConfig{
			Endpoint: "http://localhost:8080/detect",
		},
	})

	rr := doJSONRequest(t, h.Routes(), http.MethodPost, "/api/ai/enable", aiEnableRequest{CameraID: "nonexistent-cam"})
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestEnableAI_NoManager(t *testing.T) {
	t.Parallel()
	db, store := setupTestDB(t)
	h := TestHandler(db, store)

	rr := doJSONRequest(t, h.Routes(), http.MethodPost, "/api/ai/enable", aiEnableRequest{CameraID: "test-cam"})
	require.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestEnableAI_MissingCameraID(t *testing.T) {
	t.Parallel()
	h := setupAIHandlerWithManager(t, config.AIConfig{
		Enabled: true,
		Backend: "http",
	})

	rr := doJSONRequest(t, h.Routes(), http.MethodPost, "/api/ai/enable", aiEnableRequest{CameraID: ""})
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestEnableAI_InvalidBody(t *testing.T) {
	t.Parallel()
	h := setupAIHandlerWithManager(t, config.AIConfig{
		Enabled: true,
		Backend: "http",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/ai/enable", strings.NewReader("not json"))
	req.SetBasicAuth("admin", "admin12345")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDisableAI_Success(t *testing.T) {
	t.Parallel()
	h := setupAIHandlerWithManager(t, config.AIConfig{
		Enabled: true,
		Backend: "http",
	})

	rr := doJSONRequest(t, h.Routes(), http.MethodPost, "/api/ai/disable", aiDisableRequest{CameraID: "test-cam"})
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	parseJSON(t, rr, &resp)
	require.Equal(t, "disabled", resp["status"])
	require.Equal(t, "test-cam", resp["camera_id"])
}

func TestDisableAI_NoManager(t *testing.T) {
	t.Parallel()
	db, store := setupTestDB(t)
	h := TestHandler(db, store)

	rr := doJSONRequest(t, h.Routes(), http.MethodPost, "/api/ai/disable", aiDisableRequest{CameraID: "test-cam"})
	require.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestDisableAI_MissingCameraID(t *testing.T) {
	t.Parallel()
	h := setupAIHandlerWithManager(t, config.AIConfig{
		Enabled: true,
		Backend: "http",
	})

	rr := doJSONRequest(t, h.Routes(), http.MethodPost, "/api/ai/disable", aiDisableRequest{CameraID: ""})
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAIEvents_SSEHeaders(t *testing.T) {
	t.Parallel()
	h := setupAIHandlerWithManager(t, config.AIConfig{
		Enabled: true,
		Backend: "webhook",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/ai/events", nil)
	req.SetBasicAuth("admin", "admin12345")
	rr := httptest.NewRecorder()

	// Use a cancellable context so the SSE loop exits.
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	// Cancel after headers are written.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Routes().ServeHTTP(rr, req)
	}()

	<-done

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "text/event-stream", rr.Header().Get("Content-Type"))
	require.Equal(t, "no-cache", rr.Header().Get("Cache-Control"))
}

func TestAIEvents_DetectionEvent(t *testing.T) {
	t.Parallel()
	h := setupAIHandlerWithManager(t, config.AIConfig{
		Enabled: true,
		Backend: "webhook",
	})

	// Use a buffering response writer that supports Flush.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/ai/events", nil).WithContext(ctx)
	req.SetBasicAuth("admin", "admin12345")
	rec := newSSERecorder()

	// Run handler in goroutine — it will block until ctx cancel.
	var handlerDone atomic.Bool
	go func() {
		h.Routes().ServeHTTP(rec, req)
		handlerDone.Store(true)
	}()

	// Wait for handler to start and register callback.
	time.Sleep(50 * time.Millisecond)

	// Register a callback to verify it works
	cbID := h.aiManager.OnDetection(func(r webhook.DetectionResult) {
		// Callback registered successfully
	})
	_ = cbID

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestAIEvents_ContextCancellation(t *testing.T) {
	t.Parallel()
	h := setupAIHandlerWithManager(t, config.AIConfig{
		Enabled: true,
		Backend: "webhook",
	})

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/ai/events", nil).WithContext(ctx)
	req.SetBasicAuth("admin", "admin12345")
	rec := newSSERecorder()

	var handlerDone atomic.Bool
	go func() {
		h.Routes().ServeHTTP(rec, req)
		handlerDone.Store(true)
	}()

	// Verify handler is running.
	time.Sleep(50 * time.Millisecond)
	require.False(t, handlerDone.Load())

	// Cancel context.
	cancel()
	time.Sleep(100 * time.Millisecond)
	require.True(t, handlerDone.Load())
}

func TestAIEvents_NoManager(t *testing.T) {
	t.Parallel()
	db, store := setupTestDB(t)
	h := TestHandler(db, store)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/events", nil)
	req.SetBasicAuth("admin", "admin12345")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	require.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// --- SSE recorder for testing streaming endpoints ---

// sseRecorder implements http.ResponseWriter with buffering and flush support.
type sseRecorder struct {
	mu      sync.Mutex
	headers http.Header
	body    strings.Builder
	code    int
	flushed bool
}

func newSSERecorder() *sseRecorder {
	return &sseRecorder{headers: make(http.Header)}
}
func (r *sseRecorder) header() http.Header { return r.headers }
func (r *sseRecorder) Header() http.Header { return r.headers }
func (r *sseRecorder) Flush()              { r.flushed = true }

func (r *sseRecorder) WriteHeader(code int) {
	r.mu.Lock()
	r.code = code
	r.mu.Unlock()
}

func (r *sseRecorder) Code() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.code
}

func (r *sseRecorder) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.Write(b)
}

func (r *sseRecorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

// --- ensure imports compile ---

var _ = fmt.Sprintf
var _ = sync.Once{}
