package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

type errorBody struct {
	Error string `json:"error"`
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, ErrValidation), errors.Is(err, ErrInsufficientCredits):
		return http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encodage réponse JSON: %v", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	status := statusFor(err)
	if status == http.StatusInternalServerError {
		log.Printf("erreur interne: %v", err)
		writeJSON(w, status, errorBody{Error: "erreur interne du serveur"})
		return
	}
	writeJSON(w, status, errorBody{Error: err.Error()})
}

func decodeJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return validationError("corps de requête JSON invalide")
	}
	return nil
}

func parseID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		return 0, validationError("identifiant invalide")
	}
	return id, nil
}
