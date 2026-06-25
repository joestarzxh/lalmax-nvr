package media

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/lalmax-pro/lalmax-nvr/internal/config"
)

func TestNewRuntime_AlwaysInitializesWS(t *testing.T) {
	cfg := &config.Config{}
	cfg.ApplyDefaults()

	rt := NewRuntime(cfg, nil)
	require.NotNil(t, rt)
	require.NotNil(t, rt.WS())
}
