package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/model"
	"github.com/stretchr/testify/require"
)

func TestGetRecordingDays(t *testing.T) {
	ctx := context.Background()
	db, err := New(filepath.Join(t.TempDir(), "days.db"))
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Init(ctx))

	_, err = db.db.ExecContext(ctx, `INSERT INTO cameras (id, name, protocol, url) VALUES ('cam1', 'cam1', 'rtsp', 'rtsp://h/s');`)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx, `INSERT INTO cameras (id, name, protocol, url) VALUES ('cam2', 'cam2', 'rtsp', 'rtsp://h/s');`)
	require.NoError(t, err)

	mk := func(id, camID string, start time.Time) {
		require.NoError(t, db.InsertRecording(ctx, &model.Recording{
			ID: id, CameraID: camID, FilePath: "/x/" + id + ".mp4", Format: "h264",
			StartedAt: start, EndedAt: start.Add(time.Minute),
		}))
	}
	// cam1: two recordings on Jun 3, one on Jun 20, one in July (different month).
	mk("r1", "cam1", time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC))
	mk("r2", "cam1", time.Date(2026, 6, 3, 14, 0, 0, 0, time.UTC))
	mk("r3", "cam1", time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC))
	mk("r4", "cam1", time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC))
	// cam2: should not appear in cam1's results.
	mk("r5", "cam2", time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC))

	days, err := db.GetRecordingDays(ctx, "cam1", "2026-06")
	require.NoError(t, err)
	require.Equal(t, []string{"2026-06-03", "2026-06-20"}, days)

	// Empty month returns empty slice, not nil.
	days, err = db.GetRecordingDays(ctx, "cam1", "2026-01")
	require.NoError(t, err)
	require.NotNil(t, days)
	require.Len(t, days, 0)
}
