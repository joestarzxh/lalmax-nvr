package srt

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/media"
)

var logger = slog.Default().With("component", "srt-ingest")

// OnIngestStart is called when an SRT push stream is detected and mapped to a camera.
type OnIngestStart func(cameraID string, streamName string)

// OnIngestStop is called when an SRT push stream disappears.
type OnIngestStop func(cameraID string, streamName string)

// IngestHandler watches lalmax for SRT push streams and maps them to cameras.
type IngestHandler struct {
	subscriber interface {
		SubscribeSRTEvents(ctx context.Context) (<-chan media.SRTEvent, error)
	}
	resolv  func(streamName string) (cameraID string, ok bool)
	onStart OnIngestStart
	onStop  OnIngestStop

	mu     sync.Mutex
	active map[string]string // streamName -> cameraID
	cancel context.CancelFunc
	done   chan struct{}
}

// NewIngestHandler creates a new SRT ingest handler.
func NewIngestHandler(subscriber interface {
	SubscribeSRTEvents(ctx context.Context) (<-chan media.SRTEvent, error)
}, resolv func(string) (string, bool), onStart OnIngestStart, onStop OnIngestStop) *IngestHandler {
	return &IngestHandler{
		subscriber: subscriber,
		resolv:     resolv,
		onStart:    onStart,
		onStop:     onStop,
		active:     make(map[string]string),
	}
}

// Start begins watching for SRT push streams via lalmax SSE events.
func (h *IngestHandler) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	h.done = make(chan struct{})

	go h.subscribeAndRun(ctx)
	logger.Info("srt ingest handler started")
	return nil
}

// Stop stops the ingest handler.
func (h *IngestHandler) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	if h.done != nil {
		<-h.done
	}
}

// ActiveIngests returns the number of active SRT ingest streams.
func (h *IngestHandler) ActiveIngests() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.active)
}

func (h *IngestHandler) subscribeAndRun(ctx context.Context) {
	defer close(h.done)

	for {
		events, err := h.subscriber.SubscribeSRTEvents(ctx)
		if err == nil {
			h.run(ctx, events)
		} else if ctx.Err() == nil {
			logger.Warn("failed to subscribe to SRT events, retrying", "error", fmt.Errorf("subscribe lalmax SRT events: %w", err))
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (h *IngestHandler) run(ctx context.Context, events <-chan media.SRTEvent) {
	for ev := range events {
		h.handleEvent(ctx, ev)
	}
}

func (h *IngestHandler) handleEvent(ctx context.Context, ev media.SRTEvent) {
	if ev.Protocol != "" && ev.Protocol != "srt" {
		return
	}

	streamName := ev.StreamID
	if streamName == "" {
		return
	}

	switch ev.Type {
	case "pub_start", "stream_active":
		h.handleStreamStart(ctx, streamName)
	case "pub_stop", "stream_stopped":
		h.handleStreamStop(streamName)
	}
}

func (h *IngestHandler) handleStreamStart(ctx context.Context, streamName string) {
	h.mu.Lock()
	if _, exists := h.active[streamName]; exists {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()

	cameraID, ok := h.resolv(streamName)
	if !ok {
		logger.Debug("srt push stream not mapped to camera", "stream", streamName)
		return
	}

	h.mu.Lock()
	h.active[streamName] = cameraID
	h.mu.Unlock()

	logger.Info("srt ingest detected", "camera_id", cameraID, "stream", streamName)

	if h.onStart != nil {
		h.onStart(cameraID, streamName)
	}
}

func (h *IngestHandler) handleStreamStop(streamName string) {
	h.mu.Lock()
	cameraID, exists := h.active[streamName]
	if !exists {
		h.mu.Unlock()
		return
	}
	delete(h.active, streamName)
	h.mu.Unlock()

	logger.Info("srt ingest stopped", "camera_id", cameraID, "stream", streamName)

	if h.onStop != nil {
		h.onStop(cameraID, streamName)
	}
}
