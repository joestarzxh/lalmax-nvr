package gb28181

import (
	"testing"
)

func TestNewTalkManager(t *testing.T) {
	cfg := &Config{
		ID:      "server001",
		MediaIP: "10.0.0.1",
		Port:    5060,
	}

	hub := NewWSHub()
	tm := NewTalkManager(nil, cfg, NewDeviceStore(nil, hub))

	if tm == nil {
		t.Fatal("Expected TalkManager to be created")
	}
	if len(tm.sessions) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(tm.sessions))
	}
}

func TestTalkKey(t *testing.T) {
	key := talkKey("device001", "channel001")
	if key != "device001_channel001" {
		t.Errorf("Expected key device001_channel001, got %s", key)
	}
}
