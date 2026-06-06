package gb28181

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Streams represents an active GB28181 media stream.
type Streams struct {
	DeviceID  string
	ChannelID string
	SSRC      string
	SessionID string
}

// streamsManager manages active streams.
type streamsManager struct {
	streams sync.Map // map[string]*Streams (key = "play:deviceID:channelID")
	ssrc    uint32
}

var globalStreams = &streamsManager{}

func (m *streamsManager) getSSRC(domain string) string {
	v := atomic.AddUint32(&m.ssrc, 1)
	ssrc := v % 9000
	key := fmt.Sprintf("0%s%04d", domain[3:8], ssrc)
	return key
}

func (m *streamsManager) loadStream(key string) (*Streams, bool) {
	v, ok := m.streams.Load(key)
	if !ok {
		return nil, false
	}
	return v.(*Streams), true
}

func (m *streamsManager) storeStream(key string, stream *Streams) {
	m.streams.Store(key, stream)
}

func (m *streamsManager) deleteStream(key string) (*Streams, bool) {
	v, loaded := m.streams.LoadAndDelete(key)
	if !loaded {
		return nil, false
	}
	return v.(*Streams), true
}
