package api

import (
	"testing"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/model"
)

func TestStreamRingSinceChronological(t *testing.T) {
	r := &streamRing{}
	base := time.Now().Unix()
	for i := 0; i < 5; i++ {
		r.append(model.StreamMetricSample{Timestamp: base + int64(i), InFPS: float64(i)})
	}

	got := r.since(base + 2)
	if len(got) != 3 {
		t.Fatalf("expected 3 samples since cutoff, got %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].Timestamp < got[i-1].Timestamp {
			t.Fatalf("samples not in chronological order: %v", got)
		}
	}
	if got[0].InFPS != 2 {
		t.Fatalf("expected first sample InFPS=2, got %v", got[0].InFPS)
	}
}

func TestStreamRingWrapsAtCapacity(t *testing.T) {
	r := &streamRing{}
	base := time.Now().Unix()
	total := streamMetricCap + 50
	for i := 0; i < total; i++ {
		r.append(model.StreamMetricSample{Timestamp: base + int64(i)})
	}
	if r.count != streamMetricCap {
		t.Fatalf("expected count capped at %d, got %d", streamMetricCap, r.count)
	}
	all := r.since(0)
	if len(all) != streamMetricCap {
		t.Fatalf("expected %d retained samples, got %d", streamMetricCap, len(all))
	}
	// Oldest retained sample should be the (total-cap)th appended, not the very first.
	wantOldest := base + int64(total-streamMetricCap)
	if all[0].Timestamp != wantOldest {
		t.Fatalf("expected oldest ts %d, got %d", wantOldest, all[0].Timestamp)
	}
}

func TestStreamMetricsHistoryEvictIdle(t *testing.T) {
	h := newStreamMetricsHistory()
	now := time.Now().Unix()
	h.record("live", model.StreamMetricSample{Timestamp: now})
	h.record("stale", model.StreamMetricSample{Timestamp: now - 3600})

	h.evictIdle(now - 1800) // evict anything older than 30 min

	if got := h.since("live", 0); len(got) == 0 {
		t.Fatal("live stream should be retained")
	}
	if got := h.since("stale", 0); len(got) != 0 {
		t.Fatalf("stale stream should be evicted, got %d samples", len(got))
	}
}
