package httpapi

import (
	"errors"
	"net/http"
)

func (r *Router) enrollments(w http.ResponseWriter, req *http.Request) {
	user, err := r.svc.AdminFromToken(req.Context(), req.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch req.Method {
	case http.MethodGet:
		items, err := r.svc.ListEnrollments(req.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		item, token, err := r.svc.NewEnrollment(req.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"enrollment": item, "token": token})
	case http.MethodDelete:
		ok, err := r.svc.RevokeEnrollment(req.Context(), user.ID, req.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, errors.New("no active enrollment found"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
