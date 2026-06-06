package srt

import (
	"context"
	"sync"
	"testing"

	"github.com/lalmax-pro/lalmax-nvr/internal/media"
	"github.com/stretchr/testify/require"
)

func TestIngestHandlerResolvesStream(t *testing.T) {
	sub := newMockSubscriber()

	var started []string
	handler := NewIngestHandler(sub, func(name string) (string, bool) {
		if name == "mystream" {
			return "cam-1", true
		}
		return "", false
	}, func(camID, stream string) {
		started = append(started, camID)
	}, nil)

	ctx := context.Background()
	err := handler.Start(ctx)
	require.NoError(t, err)

	sub.send(media.SRTEvent{StreamID: "mystream", Protocol: "srt", Type: "pub_start"})
	sub.close()

	handler.Stop()

	require.Equal(t, []string{"cam-1"}, started)
}

func TestIngestHandlerIgnoresUnmapped(t *testing.T) {
	sub := newMockSubscriber()

	var started []string
	handler := NewIngestHandler(sub, func(name string) (string, bool) {
		return "", false
	}, func(camID, stream string) {
		started = append(started, camID)
	}, nil)

	ctx := context.Background()
	err := handler.Start(ctx)
	require.NoError(t, err)

	sub.send(media.SRTEvent{StreamID: "unknown", Protocol: "srt", Type: "pub_start"})
	sub.close()

	handler.Stop()

	require.Empty(t, started)
}

func TestIngestHandlerStop(t *testing.T) {
	sub := newMockSubscriber()

	var stopped []string
	handler := NewIngestHandler(sub, func(name string) (string, bool) {
		return "cam-1", true
	}, nil, func(camID, stream string) {
		stopped = append(stopped, camID)
	})

	ctx := context.Background()
	err := handler.Start(ctx)
	require.NoError(t, err)

	sub.send(media.SRTEvent{StreamID: "mystream", Protocol: "srt", Type: "pub_start"})
	sub.send(media.SRTEvent{StreamID: "mystream", Protocol: "srt", Type: "pub_stop"})
	sub.close()

	handler.Stop()

	require.Equal(t, []string{"cam-1"}, stopped)
}

func TestIngestHandlerIgnoresNonSRT(t *testing.T) {
	sub := newMockSubscriber()

	var started []string
	handler := NewIngestHandler(sub, func(name string) (string, bool) {
		return "cam-1", true
	}, func(camID, stream string) {
		started = append(started, camID)
	}, nil)

	ctx := context.Background()
	err := handler.Start(ctx)
	require.NoError(t, err)

	sub.send(media.SRTEvent{StreamID: "mystream", Protocol: "rtmp", Type: "pub_start"})
	sub.close()

	handler.Stop()

	require.Empty(t, started)
}

type mockSubscriber struct {
	events    chan media.SRTEvent
	closeOnce sync.Once
}

func newMockSubscriber() *mockSubscriber {
	return &mockSubscriber{events: make(chan media.SRTEvent, 16)}
}

func (m *mockSubscriber) SubscribeSRTEvents(ctx context.Context) (<-chan media.SRTEvent, error) {
	go func() {
		<-ctx.Done()
		m.closeOnce.Do(func() { close(m.events) })
	}()
	return m.events, nil
}

func (m *mockSubscriber) send(ev media.SRTEvent) {
	m.events <- ev
}

func (m *mockSubscriber) close() {
	m.closeOnce.Do(func() { close(m.events) })
}
