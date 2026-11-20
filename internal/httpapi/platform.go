package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"englishlisten/sdwan/internal/app"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func (r *Router) platformRoutes() {
	email, hash := r.svc.PlatformCredentials()
	var mu sync.Mutex
	sessions := map[[32]byte]time.Time{}
	var nextLogin time.Time
	r.mux.HandleFunc("POST /api/platform/login", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if email == "" || hash == "" {
			http.Error(w, "platform login is disabled", http.StatusServiceUnavailable)
			return
		}
		req.Body = http.MaxBytesReader(w, req.Body, 4096)
		var body app.AuthRequest
		if err := readJSON(req, &body); err != nil {
			http.Error(w, "invalid login request", http.StatusBadRequest)
			return
		}
		now := time.Now()
		mu.Lock()
		if now.Before(nextLogin) {
			mu.Unlock()
			w.Header().Set("Retry-After", "1")
			http.Error(w, "try again in one second", http.StatusTooManyRequests)
			return
		}
		nextLogin = now.Add(time.Second)
		mu.Unlock()
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)); err != nil ||
			!strings.EqualFold(strings.TrimSpace(body.Email), email) {
			http.Error(w, "invalid platform credentials", http.StatusUnauthorized)
			return
		}
		var b [32]byte
		if _, err := rand.Read(b[:]); err != nil {
			http.Error(w, "session unavailable", http.StatusInternalServerError)
			return
		}
		token := "sdwan_platform_" + hex.EncodeToString(b[:])
		mu.Lock()
		for key, expiry := range sessions {
			if !now.Before(expiry) {
				delete(sessions, key)
			}
		}
		if len(sessions) >= 32 {
			mu.Unlock()
			http.Error(w, "too many platform sessions", http.StatusTooManyRequests)
			return
		}
		sessions[sha256.Sum256([]byte(token))] = now.Add(8 * time.Hour)
		mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	})
	authorized := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			token, ok := strings.CutPrefix(req.Header.Get("Authorization"), "Bearer sdwan_platform_")
			key := sha256.Sum256([]byte("sdwan_platform_" + token))
			mu.Lock()
			valid := ok && time.Now().Before(sessions[key])
			mu.Unlock()
			if !valid {
				http.Error(w, "platform session required", http.StatusUnauthorized)
				return
			}
			next(w, req)
		}
	}
	r.mux.HandleFunc("POST /api/platform/logout", authorized(func(w http.ResponseWriter, req *http.Request) {
		token := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
		mu.Lock()
		delete(sessions, sha256.Sum256([]byte(token)))
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	r.mux.HandleFunc("GET /api/platform/tenants", authorized(func(w http.ResponseWriter, req *http.Request) {
		items, err := r.svc.PlatformTenants(req.Context())
		if err != nil {
			http.Error(w, "tenant data unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, http.StatusOK, items)
	}))
	r.mux.HandleFunc("GET /api/platform/tenants/{id}", authorized(func(w http.ResponseWriter, req *http.Request) {
		item, err := r.svc.PlatformTenantDetail(req.Context(), req.PathValue("id"))
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, req)
			return
		}
		if err != nil {
			http.Error(w, "tenant data unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, http.StatusOK, item)
	}))
	r.mux.HandleFunc("GET /api/platform/health", authorized(func(w http.ResponseWriter, req *http.Request) {
		dbReady := r.svc.Ready(req.Context()) == nil
		writeJSON(w, http.StatusOK, map[string]any{"controller": true, "postgres": dbReady, "checked_at": time.Now().UTC()})
	}))
}
