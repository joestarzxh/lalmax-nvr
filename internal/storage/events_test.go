package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/model"
	"github.com/stretchr/testify/require"
)

func setupEventsTest(t *testing.T) (*DB, context.Context) {
	t.Helper()
	db, err := New(filepath.Join(t.TempDir(), "events_test.db"))
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, db.Init(ctx))
	return db, ctx
}

func TestEventsCRUD(t *testing.T) {
	db, ctx := setupEventsTest(t)
	defer db.Close()

	now := time.Now().UTC()
	id, err := db.InsertEvent(ctx, model.Event{
		CameraID:    "front-door",
		Source:      model.EventSourceRecorder,
		Type:        "recorder_reconnected",
		Severity:    model.EventSeverityWarning,
		Status:      model.EventStatusOpen,
		Message:     "Recorder reconnected",
		Metadata:    `{"retry_count":2}`,
		RecordingID: "rec-1",
		StartedAt:   now,
		CreatedAt:   now,
	})
	require.NoError(t, err)
	require.NotZero(t, id)

	events, total, err := db.ListEvents(ctx, EventsFilter{CameraID: "front-door", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, events, 1)
	require.Equal(t, "recorder_reconnected", events[0].Type)
	require.Equal(t, "rec-1", events[0].RecordingID)

	got, err := db.GetEvent(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, model.EventStatusOpen, got.Status)

	require.NoError(t, db.AcknowledgeEvent(ctx, id, now.Add(time.Minute)))
	got, err = db.GetEvent(ctx, id)
	require.NoError(t, err)
	require.Equal(t, model.EventStatusAcknowledged, got.Status)
	require.NotNil(t, got.AcknowledgedAt)

	require.NoError(t, db.DeleteEvent(ctx, id))
	got, err = db.GetEvent(ctx, id)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestInsertHealthEventMirrorsUnifiedEvent(t *testing.T) {
	db, ctx := setupEventsTest(t)
	defer db.Close()

	err := db.InsertHealthEvent(ctx, model.HealthEvent{
		CameraID:  "cam-1",
		EventType: string(model.HealthEventConnectionLost),
		Status:    string(model.HealthStatusError),
		Message:   "connection lost",
		Metadata:  `{"reason":"timeout"}`,
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	events, total, err := db.ListEvents(ctx, EventsFilter{Source: model.EventSourceHealth})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, events, 1)
	require.Equal(t, "cam-1", events[0].CameraID)
	require.Equal(t, string(model.HealthEventConnectionLost), events[0].Type)
	require.Equal(t, model.EventSeverityCritical, events[0].Severity)
}
