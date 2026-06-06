package media

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lalmax-pro/lalmax-nvr/internal/config"
)

func TestPatchLalmaxHLSConfig_PreservesUnrelatedFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lalmax.conf.json")
	initial := `{
  "lal": {
    "rtmp": {"enable": true, "addr": ":1935"},
    "hls": {"enable": true, "url_pattern": "/hls/", "fragment_duration_ms": 3000, "fragment_num": 6}
  },
  "lalmax": {
    "fmp4_config": {
      "http": {"enable": true},
      "hls": {"enable": true, "segment_count": 7, "segment_duration": 1, "part_duration": 200, "low_latency": true}
    }
  }
}`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o644))

	cfg := EmbeddedLalmaxConfig{
		HLSEnabled:            false,
		LalFragmentDurationMs: 4000,
		LalFragmentNum:        8,
		LalCleanupMode:        2,
		LalUseMemory:          true,
		LalmaxSegmentCount:    9,
		LalmaxSegmentDuration: 2,
		LalmaxPartDuration:    300,
	}
	require.NoError(t, patchLalmaxHLSConfig(path, cfg))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))

	lal := parsed["lal"].(map[string]any)
	rtmp := lal["rtmp"].(map[string]any)
	require.True(t, rtmp["enable"].(bool))
	require.Equal(t, ":1935", rtmp["addr"])

	hls := lal["hls"].(map[string]any)
	require.False(t, hls["enable"].(bool))
	require.Equal(t, "/hls/", hls["url_pattern"])
	require.Equal(t, float64(4000), hls["fragment_duration_ms"])
	require.Equal(t, float64(8), hls["fragment_num"])
	require.Equal(t, float64(2), hls["cleanup_mode"])
	require.True(t, hls["use_memory_as_disk_flag"].(bool))

	lalmax := parsed["lalmax"].(map[string]any)
	fmp4 := lalmax["fmp4_config"].(map[string]any)
	llHls := fmp4["hls"].(map[string]any)
	require.False(t, llHls["enable"].(bool))
	require.Equal(t, float64(9), llHls["segment_count"])
	require.Equal(t, float64(2), llHls["segment_duration"])
	require.Equal(t, float64(300), llHls["part_duration"])
	require.True(t, llHls["low_latency"].(bool))
}

func TestEnsureLalLogConfig_AddsLogWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lalmax.conf.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"lal":{"hls":{"enable":true}},"lalmax":{}}`), 0o644))

	require.NoError(t, ensureLalLogConfig(path, "info"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	lal := parsed["lal"].(map[string]any)
	logCfg := lal["log"].(map[string]any)
	require.Equal(t, float64(2), logCfg["level"])
	require.Equal(t, false, logCfg["is_to_stdout"])
}

func TestEnsureLalLogConfig_SkipsWhenPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lalmax.conf.json")
	initial := `{"lal":{"log":{"level":1,"is_to_stdout":true}}}`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o644))

	require.NoError(t, ensureLalLogConfig(path, "info"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, initial, string(data))
}

func TestEmbeddedLalmax_ApplyHLSConfig_PatchesFileWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lalmax.conf.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"lal":{"hls":{"enable":true}},"lalmax":{"fmp4_config":{"hls":{"enable":true}}}}`), 0o644))

	emb := &EmbeddedLalmax{
		cfg: EmbeddedLalmaxConfig{ConfigPath: path, HLSEnabled: true},
	}
	enabled := false
	require.NoError(t, emb.ApplyHLSConfig(config.HLSConfig{Enabled: &enabled}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), `"enable": false`)
	require.False(t, emb.cfg.HLSEnabled)
}
