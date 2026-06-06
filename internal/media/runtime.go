package media

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/config"
	"github.com/lalmax-pro/lalmax-nvr/internal/flv"
	"github.com/lalmax-pro/lalmax-nvr/internal/hls"
	"github.com/lalmax-pro/lalmax-nvr/internal/metrics"
	"github.com/lalmax-pro/lalmax-nvr/internal/model"
	"github.com/lalmax-pro/lalmax-nvr/internal/wsstream"
)

type HLS interface {
	StartStream(cameraID string, sps, pps []byte, maxFPS int) error
	StartStreamH265(cameraID string, vps, sps, pps []byte, maxFPS int) error
	StartStreamWithAudio(cameraID string, sps, pps []byte, maxFPS int, audioCodec string, audioConfig []byte) error
	StartStreamH265WithAudio(cameraID string, vps, sps, pps []byte, maxFPS int, audioCodec string, audioConfig []byte) error
	StartSubStreamReader(cameraID, rtspURL string, isH265 bool, fallback func()) error
	SubscribeToHub(cameraID string, hub *model.StreamHub, isH265 bool) error
	SubscribeAudioToHub(cameraID string, hub *model.StreamHub) error
	WriteH264(cameraID string, pts int64, au [][]byte) error
	WriteH265(cameraID string, pts int64, au [][]byte) error
	WriteAudio(cameraID string, pts int64, au [][]byte)
	IsActive(cameraID string) bool
	Handle(cameraID string, w http.ResponseWriter, r *http.Request) bool
	StopStream(cameraID string)
	StopAll()
}

type FLV interface {
	RegisterStream(cameraID string, codec model.Format, sps, pps, vps []byte, hub *model.StreamHub) error
	SetAudioConfig(camID string, audioCodec model.AudioCodec, audioConfig []byte)
	IsActive(cameraID string) bool
	ServeFLV(cameraID string, w http.ResponseWriter, r *http.Request) error
}

type WS interface {
	RegisterStream(cameraID string, codec model.Format, sps, pps, vps []byte, hub *model.StreamHub) error
	IsActive(cameraID string) bool
	ServeWS(cameraID string, w http.ResponseWriter, r *http.Request) error
	StopAll()
}

type Runtime struct {
	hls HLS
	flv FLV
	ws  WS
}

func NewRuntime(cfg *config.Config, m *metrics.Metrics) *Runtime {
	rt := &Runtime{
		ws: wsstream.NewManager(
			wsstream.WithMaxViewers(cfg.WebSocket.MaxViewers),
			wsstream.WithWriteBufSize(cfg.WebSocket.WriteBufSize),
			wsstream.WithIdleTimeout(cfg.WebSocket.IdleTimeout),
		),
	}

	if !cfg.Media.Enabled && cfg.IsHLSEnabled() {
		hlsDataDir := filepath.Join(cfg.Storage.RootDir, "hls")
		hlsMgr := hls.NewManagerWithOpts(context.Background(), hlsDataDir, cfg.HLS.WriteBufferSize, cfg.HLS.SegmentMaxSizeMB*1024*1024, cfg.HLS.SegmentCount, m)
		if cfg.HLS.LowLatency {
			partDur, _ := time.ParseDuration(cfg.HLS.PartMinDuration)
			hlsMgr.SetLowLatency(true, partDur)
		}
		rt.hls = hlsMgr

		if cfg.Streaming.FLV.Enabled != nil && *cfg.Streaming.FLV.Enabled {
			rt.flv = flv.NewManager(
				flv.WithMaxViewers(cfg.Streaming.FLV.MaxViewers),
				flv.WithMetrics(m),
			)
		}
	}
	return rt
}

func (r *Runtime) HLS() HLS { return r.hls }
func (r *Runtime) FLV() FLV { return r.flv }
func (r *Runtime) WS() WS   { return r.ws }

func (r *Runtime) Start(ctx context.Context) error {
	return nil
}

func (r *Runtime) Stop() error {
	if r.ws != nil {
		r.ws.StopAll()
	}
	if r.hls != nil {
		r.hls.StopAll()
	}
	return nil
}
