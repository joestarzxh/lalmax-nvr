package event

// Event represents a single event published to the bus.
type Event struct {
	Topic string
	Data  interface{}
}

// SegmentCompleted is published when a recording segment finishes writing.
type SegmentCompleted struct {
	CameraID    string
	FilePath    string
	Format      string
	StartedAt   string // RFC3339Nano or DB timestamp format
	EndedAt     string
	FileSize    int64
	RecordingID string
}

// RecorderReconnected is published when a recorder recovers from a disconnection
// and writes its first segment after reconnection.
type RecorderReconnected struct {
	CameraID       string
	DisconnectedAt string // RFC3339Nano — when the connection was lost
	ReconnectedAt  string // RFC3339Nano — when the connection was restored
	Downtime       string // human-readable duration (e.g. "2m30s")
	RetryCount     int    // number of reconnect attempts before success
	GapReason      string // e.g. "connection_lost", "frame_watchdog", "muxer_error"
	RecordingID    string // ID of the first recording segment after reconnect
}
