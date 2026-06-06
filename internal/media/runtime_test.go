package media

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/lalmax-pro/lalmax-nvr/internal/config"
)

func TestNewRuntime_MediaEngineEnabledDisablesLegacyLiveManagers(t *testing.T) {
	cfg := &config.Config{}
	cfg.ApplyDefaults()
	cfg.Media.Enabled = true

	rt := NewRuntime(cfg, nil)
	require.NotNil(t, rt)
	require.Nil(t, rt.HLS())
	require.Nil(t, rt.FLV())
	require.NotNil(t, rt.WS())
}

func TestNewRuntime_LegacyLiveManagersRemainWhenMediaEngineDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.ApplyDefaults()
	cfg.Media.Enabled = false
	enabled := true
	cfg.HLS.Enabled = &enabled

	rt := NewRuntime(cfg, nil)
	require.NotNil(t, rt)
	require.NotNil(t, rt.HLS())
	require.NotNil(t, rt.WS())
}

func TestNewRuntime_HLSDisabledSkipsLegacyManager(t *testing.T) {
	cfg := &config.Config{}
	cfg.ApplyDefaults()
	cfg.Media.Enabled = false

	rt := NewRuntime(cfg, nil)
	require.NotNil(t, rt)
	require.Nil(t, rt.HLS())
}
