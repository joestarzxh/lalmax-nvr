package rtmp

import (
	"context"
	"sync"
	"testing"

	"github.com/lalmax-pro/lalmax-nvr/internal/media"
	"github.com/stretchr/testify/require"
)

func TestBuildReverseMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: map[string]string{},
		},
		{
			name:     "empty input",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "single mapping",
			input: map[string]string{
				"cam-1": "mystream",
			},
			expected: map[string]string{
				"mystream": "cam-1",
			},
		},
		{
			name: "multiple mappings",
			input: map[string]string{
				"cam-1": "stream1",
				"cam-2": "stream2",
			},
			expected: map[string]string{
				"stream1": "cam-1",
				"stream2": "cam-2",
			},
		},
		{
			name: "empty stream key skipped",
			input: map[string]string{
				"cam-1": "stream1",
				"cam-2": "",
			},
			expected: map[string]string{
				"stream1": "cam-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildReverseMap(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

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

	sub.send(media.RTMPEvent{StreamID: "mystream", Protocol: "rtmp", Type: "pub_start"})
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

	sub.send(media.RTMPEvent{StreamID: "unknown", Protocol: "rtmp", Type: "pub_start"})
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

	sub.send(media.RTMPEvent{StreamID: "mystream", Protocol: "rtmp", Type: "pub_start"})
	sub.send(media.RTMPEvent{StreamID: "mystream", Protocol: "rtmp", Type: "pub_stop"})
	sub.close()

	handler.Stop()

	require.Equal(t, []string{"cam-1"}, stopped)
}

func TestIngestHandlerIgnoresNonRTMP(t *testing.T) {
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

	sub.send(media.RTMPEvent{StreamID: "mystream", Protocol: "rtsp", Type: "pub_start"})
	sub.close()

	handler.Stop()

	require.Empty(t, started)
}

type mockSubscriber struct {
	events    chan media.RTMPEvent
	closeOnce sync.Once
}

func newMockSubscriber() *mockSubscriber {
	return &mockSubscriber{events: make(chan media.RTMPEvent, 16)}
}

func (m *mockSubscriber) SubscribeRTMPEvents(ctx context.Context) (<-chan media.RTMPEvent, error) {
	go func() {
		<-ctx.Done()
		m.closeOnce.Do(func() { close(m.events) })
	}()
	return m.events, nil
}

func (m *mockSubscriber) send(ev media.RTMPEvent) {
	m.events <- ev
}

func (m *mockSubscriber) close() {
	m.closeOnce.Do(func() { close(m.events) })
}
