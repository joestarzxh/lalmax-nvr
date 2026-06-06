package hls

import (
	"testing"

	lalmaxconfig "github.com/q191201771/lalmax/config"
	"github.com/stretchr/testify/require"
)

func TestHlsServer_OnDemand_LazySession(t *testing.T) {
	svr := NewHlsServer(lalmaxconfig.Fmp4HlsConfig{Enable: true, OnDemand: true})
	require.True(t, svr.OnDemandEnabled())

	svr.NewHlsSessionWithAppName("live", "eager")
	_, ok := svr.getSession("live", "eager")
	require.True(t, ok)

	svr2 := NewHlsServer(lalmaxconfig.Fmp4HlsConfig{Enable: true, OnDemand: true})
	_, ok = svr2.getSession("live", "lazy")
	require.False(t, ok)
	_, ok = svr2.ensureSession("live", "lazy")
	require.True(t, ok)
}

func TestHlsServer_SetEnabled_StopsSessions(t *testing.T) {
	svr := NewHlsServer(lalmaxconfig.Fmp4HlsConfig{Enable: true})
	svr.NewHlsSessionWithAppName("live", "cam-1")
	svr.SetEnabled(false)
	svr.NewHlsSessionWithAppName("live", "cam-2")

	var count int
	svr.sessions.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	require.Equal(t, 0, count)
	require.False(t, svr.conf.Enable)
}
