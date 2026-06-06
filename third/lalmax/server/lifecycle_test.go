package server

import (
	"context"
	"testing"
	"time"

	config "github.com/q191201771/lalmax/config"
)

func TestLalMaxServerLifecycle(t *testing.T) {
	svr, err := NewLalMaxServer(&config.Config{
		HttpConfig: config.HttpConfig{
			ListenAddr: "127.0.0.1:0",
		},
		LalRawContent: []byte(`{
			"conf_version": "v0.4.1",
			"rtmp": {"enable": false},
			"rtsp": {"enable": false},
			"httpflv": {"enable": false},
			"httpts": {"enable": false},
			"hls": {"enable": false},
			"http_api": {"enable": false},
			"pprof": {"enable": false},
			"record": {"enable_flv": false, "enable_mpegts": false},
			"relay_push": {"enable": false, "addr_list": []},
			"static_relay_pull": {"enable": false, "addr": ""},
			"log": {"level": 1, "is_to_stdout": false}
		}`),
	})
	if err != nil {
		t.Fatalf("NewLalMaxServer: %v", err)
	}

	if err := svr.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !svr.Ready() {
		t.Fatal("server should be ready after Start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := svr.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if svr.Ready() {
		t.Fatal("server should not be ready after Shutdown")
	}
}
