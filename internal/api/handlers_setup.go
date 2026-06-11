package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/config"
	"github.com/lalmax-pro/lalmax-nvr/internal/middleware"
	"github.com/lalmax-pro/lalmax-nvr/internal/model"
)

// setupRequest is the JSON body for POST /api/setup.
type setupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Language string `json:"language,omitempty"`
}

func resolveSetupStorageRoot(cfg *config.Config) string {
	if cfg != nil && strings.TrimSpace(cfg.Storage.RootDir) != "" {
		return cfg.Storage.RootDir
	}
	if envDir := os.Getenv("NVR_DATA_DIR"); envDir != "" {
		return envDir
	}
	if info, err := os.Stat("/data"); err == nil && info.IsDir() {
		return "/data"
	}
	return "/var/lib/lalmax-nvr"
}

// handleSetup handles POST /api/setup — first-time initialization.
// Only succeeds when no password_hash is configured (SETUP_REQUIRED state).
func (h *Handler) handleSetup(w http.ResponseWriter, r *http.Request) {
	// Security: reject if auth is already configured
	if strings.TrimSpace(h.config.Auth.PasswordHash) != "" {
		writeError(w, http.StatusConflict, "setup already completed")
		return
	}

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate username
	if strings.TrimSpace(req.Username) == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}

	// Validate password (same rule as CLI: min 8 chars)
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// Hash password with bcrypt
	hash, err := middleware.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to hash password: %v", err))
		return
	}

	// Storage root is determined by config file / env / Docker — not the setup wizard.
	dataDir := resolveSetupStorageRoot(h.config)

	cfg := config.Config{
		Server:  config.ServerConfig{Listen: ":9090"},
		Storage: config.StorageConfig{RootDir: dataDir, SegmentDuration: "30s"},
		Auth:    config.AuthConfig{Username: req.Username, PasswordHash: hash},
		Cameras: []config.CameraConfig{},
		Cleanup: config.CleanupConfig{RetentionDays: 30, CheckInterval: "1h", DiskThresholdPercent: 95},
		FTP:     config.FTPConfig{Port: 2121, PassivePortRange: "2122-2140"},
		WebDAV:  config.WebDAVConfig{PathPrefix: "/dav"},
		Observability: config.ObservabilityConfig{
			LogLevel:  "info",
			LogFormat: "text",
		},
		Version: "1.0",
	}

	// Atomic save
	if err := config.Save(h.configPath, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config: %v", err))
		return
	}

	// Update in-memory config so middleware picks up the new password hash
	h.config.Auth.Username = req.Username
	h.config.Auth.PasswordHash = hash
	h.config.Storage.RootDir = dataDir

	// Create super_admin user in the users table
	now := time.Now().UTC()
	dbUser := &model.User{
		Username:     req.Username,
		PasswordHash: hash,
		Role:         model.RoleSuperAdmin,
		DisplayName:  req.Username,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := h.db.CreateUser(r.Context(), dbUser); err != nil {
		logger.Error("failed to create super_admin user in DB", "error", err)
	}

	// Generate basic auth token for auto-login
	token := base64.StdEncoding.EncodeToString([]byte(req.Username + ":" + req.Password))

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"token":  token,
	})
}
