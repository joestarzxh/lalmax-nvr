package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/model"
)

// --- Per-stream live metrics history ---

const (
	// streamMetricInterval is how often each active stream is sampled.
	streamMetricInterval = 5 * time.Second
	// streamMetricCap is the per-stream ring buffer size: 30 min at 5s intervals.
	streamMetricCap = 360
	// streamMetricTTL is how long an idle (no-longer-listed) stream's history is
	// retained before eviction. A stream not seen for this long is dropped.
	streamMetricTTL = 30 * time.Minute
)

// streamRing is a fixed-capacity ring buffer of StreamMetricSample for one stream.
type streamRing struct {
	samples  [streamMetricCap]model.StreamMetricSample
	head     int   // next write position
	count    int   // number of valid entries (≤ cap)
	lastSeen int64 // Unix seconds of most recent sample (for TTL eviction)
}

func (r *streamRing) append(s model.StreamMetricSample) {
	r.samples[r.head] = s
	r.head = (r.head + 1) % streamMetricCap
	if r.count < streamMetricCap {
		r.count++
	}
	r.lastSeen = s.Timestamp
}

// since returns all samples newer than cutoff, in chronological order.
func (r *streamRing) since(cutoff int64) []model.StreamMetricSample {
	if r.count == 0 {
		return []model.StreamMetricSample{}
	}
	start := (r.head - r.count + streamMetricCap) % streamMetricCap
	result := make([]model.StreamMetricSample, 0, r.count)
	for i := 0; i < r.count; i++ {
		pos := (start + i) % streamMetricCap
		if r.samples[pos].Timestamp >= cutoff {
			result = append(result, r.samples[pos])
		}
	}
	return result
}

// streamMetricsHistory holds per-stream ring buffers of live metric samples.
type streamMetricsHistory struct {
	mu      sync.RWMutex
	streams map[string]*streamRing
}

func newStreamMetricsHistory() *streamMetricsHistory {
	return &streamMetricsHistory{streams: make(map[string]*streamRing)}
}

// record appends a sample for the given stream, creating its ring on first sight.
func (h *streamMetricsHistory) record(streamID string, s model.StreamMetricSample) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ring := h.streams[streamID]
	if ring == nil {
		ring = &streamRing{}
		h.streams[streamID] = ring
	}
	ring.append(s)
}

// evictIdle drops streams whose newest sample is older than the TTL cutoff.
func (h *streamMetricsHistory) evictIdle(cutoff int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, ring := range h.streams {
		if ring.lastSeen < cutoff {
			delete(h.streams, id)
		}
	}
}

// since returns a stream's samples newer than cutoff, in chronological order.
func (h *streamMetricsHistory) since(streamID string, cutoff int64) []model.StreamMetricSample {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ring := h.streams[streamID]
	if ring == nil {
		return []model.StreamMetricSample{}
	}
	return ring.since(cutoff)
}

// startStreamMetricsSampler launches a background goroutine that samples every
// active stream's live metrics on a fixed interval and evicts idle streams.
func (h *Handler) startStreamMetricsSampler(ctx context.Context) {
	if h.streamMetrics == nil || h.mediaEngine == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(streamMetricInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.sampleStreamMetrics(ctx)
			}
		}
	}()
}

// sampleStreamMetrics records one data point for every active stream.
func (h *Handler) sampleStreamMetrics(ctx context.Context) {
	sampleCtx, cancel := context.WithTimeout(ctx, streamMetricInterval)
	defer cancel()

	streams, err := h.mediaEngine.ListStreams(sampleCtx)
	if err != nil {
		return
	}

	now := time.Now().Unix()
	for _, s := range streams {
		if !s.Active {
			continue
		}
		bitrate := 0
		if s.Publisher != nil {
			bitrate = s.Publisher.BitrateKbits
		}
		h.streamMetrics.record(s.StreamID, model.StreamMetricSample{
			Timestamp:    now,
			InFPS:        s.InFPS,
			BitrateKbits: bitrate,
			Subscribers:  len(s.Subscribers),
		})
	}

	h.streamMetrics.evictIdle(time.Now().Add(-streamMetricTTL).Unix())
}

// handleStreamMetricsHistory returns a time-series of a stream's live metrics.
// Query param: period=5m|15m|30m (default 15m). Window is capped at 30 min of
// retained history regardless of period.
func (h *Handler) handleStreamMetricsHistory(w http.ResponseWriter, r *http.Request) {
	streamID := streamIDFromRequest(r)
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream_id is required")
		return
	}
	if h.streamMetrics == nil {
		writeJSON(w, http.StatusOK, []model.StreamMetricSample{})
		return
	}

	var cutoff int64
	switch r.URL.Query().Get("period") {
	case "5m":
		cutoff = time.Now().Add(-5 * time.Minute).Unix()
	case "30m":
		cutoff = time.Now().Add(-30 * time.Minute).Unix()
	default: // "15m"
		cutoff = time.Now().Add(-15 * time.Minute).Unix()
	}
	writeJSON(w, http.StatusOK, h.streamMetrics.since(streamID, cutoff))
}
