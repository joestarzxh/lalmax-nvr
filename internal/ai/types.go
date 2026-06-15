package ai

import "context"

// Detection represents a single object detection result.
type Detection struct {
	Label      string     `json:"label"`
	Confidence float32    `json:"confidence"`
	Box        [4]float32 `json:"box"` // [x, y, width, height] in normalized coordinates
}

// DetectionResult is a complete detection event for a camera frame.
type DetectionResult struct {
	CameraID   string      `json:"camera_id"`
	PTS        int64       `json:"pts"`
	Timestamp  int64       `json:"timestamp"`
	Detections []Detection `json:"detections"`
}

// Detector performs object detection on a single video frame.
// Implementations must be safe for concurrent use.
type Detector interface {
	Detect(ctx context.Context, frame []byte) ([]Detection, error)
	Close() error
}

// Status represents the current state of the AI subsystem.
type Status struct {
	Backend   string `json:"backend"`   // "http", "webhook", "disabled"
	Available bool   `json:"available"` // backend is ready
	Reason    string `json:"reason"`    // human-readable status explanation
}

// CallbackFunc is invoked when a detection event occurs.
type CallbackFunc func(result DetectionResult)
