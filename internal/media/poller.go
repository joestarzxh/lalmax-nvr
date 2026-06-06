package media

import (
	"context"
	"time"
)

type StreamPoller struct {
	engine   Engine
	interval time.Duration
}

func NewStreamPoller(engine Engine, interval time.Duration) *StreamPoller {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &StreamPoller{engine: engine, interval: interval}
}

func (p *StreamPoller) Run(ctx context.Context) <-chan Event {
	out := make(chan Event, 64)
	go func() {
		defer close(out)
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		known := make(map[string]bool)
		p.poll(ctx, known, out)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.poll(ctx, known, out)
			}
		}
	}()
	return out
}

func (p *StreamPoller) poll(ctx context.Context, known map[string]bool, out chan<- Event) {
	streams, err := p.engine.ListStreams(ctx)
	if err != nil {
		return
	}

	current := make(map[string]StreamInfo, len(streams))
	for _, stream := range streams {
		if stream.StreamID == "" {
			continue
		}
		current[stream.AppName+"/"+stream.StreamID] = stream
	}

	for key, stream := range current {
		if known[key] {
			continue
		}
		known[key] = true
		if !stream.Active {
			continue
		}
		p.emit(ctx, out, Event{
			Type:     EventStreamActive,
			StreamID: stream.StreamID,
			AppName:  stream.AppName,
			At:       time.Now(),
		})
	}

	for key := range known {
		if _, ok := current[key]; ok {
			continue
		}
		delete(known, key)
		appName, streamID := splitStreamKey(key)
		p.emit(ctx, out, Event{
			Type:     EventStreamStopped,
			StreamID: streamID,
			AppName:  appName,
			At:       time.Now(),
		})
	}
}

func (p *StreamPoller) emit(ctx context.Context, out chan<- Event, ev Event) {
	select {
	case out <- ev:
	case <-ctx.Done():
	}
}

func splitStreamKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			return key[:i], key[i+1:]
		}
	}
	return "", key
}
