package middleware

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lalmax-pro/lalmax-nvr/internal/model"
)

// MultiUserProvider supplies the user lookup function from the database.
type MultiUserProvider struct {
	GetUserByUsername func(ctx context.Context, username string) (*model.User, error)
	CountUsers        func(ctx context.Context) (int, error)
	GetLegacyUsername func() string
}

// NewMultiUserAuthMiddleware returns a middleware that authenticates via the users table.
// It injects the authenticated *model.User into the request context.
// When no users exist in the DB, it falls back to the legacy single-user config-based auth
// and injects a synthetic super_admin user for backward compatibility.
func NewMultiUserAuthMiddleware(provider MultiUserProvider, legacyMW func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r.RemoteAddr)

			if v, ok := authFailures.Load(ip); ok {
				entry := v.(rateLimitEntry)
				if time.Since(entry.windowStart) > time.Duration(authWindowMinutes)*time.Minute {
					authFailures.Delete(ip)
				} else if entry.count >= authMaxFailures {
					logger.Info("rate limited request", "ip", ip, "failures", entry.count)
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
			}

			// Check if users exist in DB
			count, err := provider.CountUsers(r.Context())
			if err != nil {
				slog.Error("failed to count users", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// No users in DB yet — fall back to legacy single-user auth
			// and inject a synthetic super_admin user for RBAC middleware
			if count == 0 {
				legacyHandler := legacyMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					legacyUser := &model.User{
						Username: provider.GetLegacyUsername(),
						Role:     model.RoleSuperAdmin,
						Enabled:  true,
					}
					ctx := context.WithValue(r.Context(), UserContextKey, legacyUser)
					next.ServeHTTP(w, r.WithContext(ctx))
				}))
				legacyHandler.ServeHTTP(w, r)
				return
			}

			// Extract credentials from Basic Auth or token query param
			user, pass, ok := r.BasicAuth()
			if !ok {
				if tok := r.URL.Query().Get("token"); tok != "" {
					decoded, err := base64.StdEncoding.DecodeString(tok)
					if err == nil {
						parts := strings.SplitN(string(decoded), ":", 2)
						if len(parts) == 2 {
							user = parts[0]
							pass = parts[1]
							ok = true
						}
					}
				}
			}

			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("WWW-Authenticate", `Basic realm="lalmax-nvr"`)
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}

			dbUser, err := provider.GetUserByUsername(r.Context(), user)
			if err != nil {
				slog.Error("user lookup failed", "error", err, "ip", ip)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if dbUser == nil || !dbUser.Enabled || !CheckPassword(pass, dbUser.PasswordHash) {
				if v, ok := authFailures.Load(ip); ok {
					entry := v.(rateLimitEntry)
					if time.Since(entry.windowStart) > time.Duration(authWindowMinutes)*time.Minute {
						authFailures.Store(ip, rateLimitEntry{count: 1, windowStart: time.Now()})
					} else {
						entry.count++
						authFailures.Store(ip, entry)
					}
				} else {
					authFailures.Store(ip, rateLimitEntry{count: 1, windowStart: time.Now()})
				}
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			authFailures.Delete(ip)

			ctx := context.WithValue(r.Context(), UserContextKey, dbUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
