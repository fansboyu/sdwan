package httpapi

import (
	"errors"
	"net/http"

	"englishlisten/sdwan/internal/app"
)

func (r *Router) lanVerifications(w http.ResponseWriter, req *http.Request) {
	user, err := r.svc.AdminFromToken(req.Context(), req.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	items, err := r.svc.EmployeeLANVerifications(req.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (r *Router) reportLANVerification(w http.ResponseWriter, req *http.Request) {
	var report app.LANReport
	if err := readJSON(req, &report); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.svc.ReportEmployeeLAN(req.Context(), req.Header.Get("Authorization"), report); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, app.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
