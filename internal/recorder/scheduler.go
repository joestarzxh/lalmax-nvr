package recorder

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/storage"
)

var schedLogger = slog.Default().With("component", "recording-scheduler")

// RecordingScheduler periodically checks recording plans and pauses/resumes
// cameras based on whether the current time falls within a plan's time range.
type RecordingScheduler struct {
	db     *storage.DB
	mu     sync.Mutex
	stopCh chan struct{}
	done   chan struct{}
}

func NewRecordingScheduler(db *storage.DB) *RecordingScheduler {
	return &RecordingScheduler{
		db:     db,
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start begins the scheduler loop. It checks every 30 seconds.
func (s *RecordingScheduler) Start(ctx context.Context, pauseFn, resumeFn func(ctx context.Context, cameraID string) error) {
	go s.run(ctx, pauseFn, resumeFn)
}

func (s *RecordingScheduler) Stop() {
	close(s.stopCh)
	<-s.done
}

func (s *RecordingScheduler) run(ctx context.Context, pauseFn, resumeFn func(ctx context.Context, cameraID string) error) {
	defer close(s.done)

	schedLogger.Info("recording scheduler started")
	s.check(ctx, pauseFn, resumeFn)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			schedLogger.Info("recording scheduler stopped")
			return
		case <-ctx.Done():
			schedLogger.Info("recording scheduler stopped (context cancelled)")
			return
		case <-ticker.C:
			s.check(ctx, pauseFn, resumeFn)
		}
	}
}

func (s *RecordingScheduler) check(ctx context.Context, pauseFn, resumeFn func(ctx context.Context, cameraID string) error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get cameras that should be active NOW based on plans
	activeCameraIDs, err := s.db.GetActiveCameraIDs(ctx)
	if err != nil {
		schedLogger.Error("failed to get active camera IDs", "error", err)
		return
	}

	// Get cameras that are associated with ANY enabled plan
	camerasWithPlan, err := s.db.GetCamerasWithPlan(ctx)
	if err != nil {
		schedLogger.Error("failed to get cameras with plan", "error", err)
		return
	}

	// For cameras in a plan but NOT in the active time range → pause
	for cameraID := range camerasWithPlan {
		if !activeCameraIDs[cameraID] {
			if err := pauseFn(ctx, cameraID); err != nil {
				schedLogger.Debug("pause failed (camera may already be paused)", "camera_id", cameraID, "error", err)
			}
		}
	}

	// For cameras in the active time range → resume
	for cameraID := range activeCameraIDs {
		if err := resumeFn(ctx, cameraID); err != nil {
			schedLogger.Debug("resume failed (camera may already be recording)", "camera_id", cameraID, "error", err)
		}
	}
}
