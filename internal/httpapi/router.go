package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"englishlisten/sdwan/internal/app"
)

type Router struct {
	svc    *app.Service
	logger *slog.Logger
	mux    *http.ServeMux
}

func NewRouter(svc *app.Service, logger *slog.Logger) http.Handler {
	r := &Router{
		svc:    svc,
		logger: logger,
		mux:    http.NewServeMux(),
	}
	r.routes()
	return r.withMiddleware(r.mux)
}

func (r *Router) routes() {
	r.mux.HandleFunc("GET /healthz", r.healthz)
	r.mux.HandleFunc("GET /readyz", r.healthz)
	r.mux.HandleFunc("GET /api/v1/server/version", r.serverVersion)

	r.mux.HandleFunc("POST /admin/auth/register", r.registerAdmin)
	r.mux.HandleFunc("POST /admin/auth/login", r.loginAdmin)
	r.mux.HandleFunc("GET /admin/auth/me", r.me)
	r.mux.HandleFunc("GET /admin/account", r.account)
	r.mux.HandleFunc("GET /admin/plans", r.plans)
	r.mux.HandleFunc("GET /admin/devices", r.listDevices)
	r.mux.HandleFunc("GET /admin/devices/{deviceID}", r.getDeviceDetail)

	r.mux.HandleFunc("POST /api/v1/devices/register", r.registerDevice)
	r.mux.HandleFunc("POST /api/v1/devices/poll", r.poll)
	r.mux.HandleFunc("GET /api/v1/netmap", r.netmap)
	r.mux.HandleFunc("GET /api/v1/bootstrap/peers", r.bootstrapPeers)
	r.mux.HandleFunc("POST /api/v1/bootstrap/endpoints", r.reportBootstrapEndpoint)
}

func (r *Router) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) serverVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, r.svc.ServerVersion())
}

func (r *Router) registerAdmin(w http.ResponseWriter, req *http.Request) {
	var body app.AuthRequest
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := r.svc.RegisterAdmin(req.Context(), body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (r *Router) loginAdmin(w http.ResponseWriter, req *http.Request) {
	var body app.AuthRequest
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := r.svc.LoginAdmin(req.Context(), body)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) me(w http.ResponseWriter, req *http.Request) {
	user, err := r.svc.AdminFromToken(req.Context(), req.Header.Get("Authorization"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "admin_user": user})
}

func (r *Router) account(w http.ResponseWriter, req *http.Request) {
	user, err := r.svc.AdminFromToken(req.Context(), req.Header.Get("Authorization"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	summary, err := r.svc.AccountSummary(req.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (r *Router) plans(w http.ResponseWriter, req *http.Request) {
	plans, err := r.svc.ListPlans(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans})
}

func (r *Router) listDevices(w http.ResponseWriter, req *http.Request) {
	user, err := r.svc.AdminFromToken(req.Context(), req.Header.Get("Authorization"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	devices, err := r.svc.ListDevices(req.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (r *Router) getDeviceDetail(w http.ResponseWriter, req *http.Request) {
	user, err := r.svc.AdminFromToken(req.Context(), req.Header.Get("Authorization"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	detail, err := r.svc.GetDeviceDetail(req.Context(), user, req.PathValue("deviceID"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (r *Router) registerDevice(w http.ResponseWriter, req *http.Request) {
	var body app.RegisterDeviceRequest
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := r.svc.RegisterDevice(req.Context(), body)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		} else if errors.Is(err, app.ErrUpgradeRequired) {
			status = http.StatusPaymentRequired
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (r *Router) poll(w http.ResponseWriter, req *http.Request) {
	var body app.PollRequest
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := r.svc.Poll(req.Context(), req.Header.Get("Authorization"), body)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) netmap(w http.ResponseWriter, req *http.Request) {
	resp, err := r.svc.Netmap(req.Context(), req.Header.Get("Authorization"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) reportBootstrapEndpoint(w http.ResponseWriter, req *http.Request) {
	var body app.BootstrapEndpointReportRequest
	if err := readJSON(req, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.svc.ReportBootstrapEndpoint(req.Context(), req.Header.Get("Authorization"), body); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) bootstrapPeers(w http.ResponseWriter, req *http.Request) {
	resp, err := r.svc.BootstrapPeers(req.Context(), req.Header.Get("Authorization"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/api/") || strings.HasPrefix(req.URL.Path, "/admin/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			if req.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, req)
	})
}

func readJSON(req *http.Request, target any) error {
	defer req.Body.Close()
	return json.NewDecoder(req.Body).Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
