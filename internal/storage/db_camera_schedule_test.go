package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrateRecordingMode_AddsColumn(t *testing.T) {
	ctx := context.Background()
	db, err := New(filepath.Join(t.TempDir(), "prev23.db"))
	require.NoError(t, err)
	defer db.Close()

	// Pre-v23 cameras table without recording_mode.
	_, err = db.db.ExecContext(ctx, `CREATE TABLE cameras (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, protocol TEXT NOT NULL,
		url TEXT NOT NULL, archived INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx, `INSERT INTO cameras (id, name, protocol, url) VALUES ('cam1', 'cam1', 'rtsp', 'rtsp://h/s');`)
	require.NoError(t, err)

	require.NoError(t, db.migrateRecordingMode(ctx))

	// Column added with default 'continuous' for existing rows.
	var colExists int
	_ = db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('cameras') WHERE name='recording_mode'`).Scan(&colExists)
	require.Equal(t, 1, colExists)
	var mode string
	require.NoError(t, db.db.QueryRowContext(ctx, `SELECT recording_mode FROM cameras WHERE id='cam1'`).Scan(&mode))
	require.Equal(t, RecordingModeContinuous, mode)

	// Idempotent.
	require.NoError(t, db.migrateRecordingMode(ctx))
}

func TestSetAndGetCameraSchedule(t *testing.T) {
	ctx := context.Background()
	db, err := New(filepath.Join(t.TempDir(), "sched.db"))
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Init(ctx))
	_, err = db.db.ExecContext(ctx, `INSERT INTO cameras (id, name, protocol, url) VALUES ('c', 'c', 'rtsp', 'rtsp://h/s');`)
	require.NoError(t, err)

	ranges := []CameraScheduleRange{
		{DayOfWeek: 1, StartTime: "08:00", EndTime: "12:00"},
		{DayOfWeek: 1, StartTime: "13:00", EndTime: "18:00"},
	}
	require.NoError(t, db.SetCameraSchedule(ctx, "c", ranges))

	got, err := db.GetCameraSchedule(ctx, "c")
	require.NoError(t, err)
	require.Equal(t, ranges, got)

	// Replace semantics: setting fewer ranges drops the rest.
	require.NoError(t, db.SetCameraSchedule(ctx, "c", ranges[:1]))
	got, err = db.GetCameraSchedule(ctx, "c")
	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestGetDesiredRecordingState(t *testing.T) {
	ctx := context.Background()
	db, err := New(filepath.Join(t.TempDir(), "desired.db"))
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Init(ctx))

	insert := func(id, mode string) {
		_, err := db.db.ExecContext(ctx,
			`INSERT INTO cameras (id, name, protocol, url, recording_mode) VALUES (?, ?, 'rtsp', 'rtsp://h/s', ?);`,
			id, id, mode)
		require.NoError(t, err)
	}
	insert("cont", RecordingModeContinuous)
	insert("offcam", RecordingModeOff)
	insert("sched", RecordingModeScheduled)

	desired, err := db.GetDesiredRecordingState(ctx)
	require.NoError(t, err)
	require.True(t, desired["cont"], "continuous should always record")
	require.False(t, desired["offcam"], "off should never record")
	require.False(t, desired["sched"], "scheduled with no windows should not record")
}
