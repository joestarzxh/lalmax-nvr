package camera

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lalmax-pro/lalmax-nvr/internal/config"
	"github.com/lalmax-pro/lalmax-nvr/internal/storage"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSyncCamerasFromStorage_MigratesYAMLAndStripsFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	configPath := filepath.Join(dir, "lalmax-nvr.yaml")

	db, err := storage.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Init(context.Background()))

	cfg := &config.Config{
		Storage: config.StorageConfig{RootDir: dir},
		Cameras: []config.CameraConfig{{
			ID: "cam-1", Name: "Front", Protocol: "rtsp", Encoding: "h264",
			URL: "rtsp://192.168.1.10/stream", Username: "admin", Password: "secret", Enabled: true,
		}},
	}
	cfg.ApplyDefaults()

	require.NoError(t, SyncCamerasFromStorage(context.Background(), cfg, db, configPath))
	require.Len(t, cfg.Cameras, 1)
	require.Equal(t, "cam-1", cfg.Cameras[0].ID)
	require.Equal(t, "secret", cfg.Cameras[0].Password)

	raw, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var saved config.Config
	require.NoError(t, yaml.Unmarshal(raw, &saved))
	require.Empty(t, saved.Cameras)
}
