package recorder

import (
	"math/rand"
	"strings"
	"time"
)

// TieredBackoff returns a retry delay based on attempt count, following go2rtc's strategy:
//   - attempts 0–4:  1s  (fast recovery for transient failures)
//   - attempts 5–9:  5s  (short network issues)
//   - attempts 10–19: 10s (persistent problems)
//   - attempts 20+:   60s (long-term unreachable)
func TieredBackoff(attempt int) time.Duration {
	switch {
	case attempt < 5:
		return time.Second
	case attempt < 10:
		return 5 * time.Second
	case attempt < 20:
		return 10 * time.Second
	default:
		return time.Minute
	}
}

// TieredBackoffWithJitter returns TieredBackoff(attempt) with up to 1 second of random jitter
// to avoid thundering herd when multiple cameras reconnect simultaneously.
func TieredBackoffWithJitter(attempt int) time.Duration {
	return TieredBackoff(attempt) + time.Duration(rand.Int63n(int64(time.Second)))
}

// classifyDisconnectReason maps an error string to a short reason tag
// stored in Recording.GapReason for later querying.
func classifyDisconnectReason(err error) string {
	if err == nil {
		return "unknown"
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "watchdog"):
		return "frame_watchdog"
	case strings.Contains(msg, "DESCRIBE"), strings.Contains(msg, "SETUP"), strings.Contains(msg, "PLAY"):
		return "rtsp_negotiation"
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "no route to host"):
		return "connection_refused"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline"):
		return "connection_timeout"
	case strings.Contains(msg, "EOF"), strings.Contains(msg, "closed"):
		return "connection_lost"
	default:
		return "connection_error"
	}
}
