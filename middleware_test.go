package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecoveryMiddlewareReturns500(t *testing.T) {
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	h := recovery(panicking)

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, attendu 500", rec.Code)
	}
	var body errorBody
	decodeBody(t, rec, &body)
	if body.Error != "erreur interne du serveur" {
		t.Errorf("message = %q, attendu %q", body.Error, "erreur interne du serveur")
	}
}

func TestCORSMiddlewareOptions(t *testing.T) {
	api := newAPITest(t)
	rec := api.do(http.MethodOptions, "/api/users", 0, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d, attendu 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin = %q, attendu \"*\"", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Errorf("Allow-Methods manquant")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Errorf("Allow-Headers manquant")
	}
}

func TestAuthInvalidUserIDHeaderTreatedAsAnonymous(t *testing.T) {
	api := newAPITest(t)
	_, svc := api.seedService("provider", "Jardinage", 3)
	body, _ := json.Marshal(map[string]any{
		"titre": "Nouveau titre", "categorie": "Jardinage", "duree_minutes": 30, "credits": 2,
	})

	for _, raw := range []string{"abc", "-1", "0"} {
		t.Run(raw, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/services/%d", svc.ID), bytes.NewReader(body))
			req.Header.Set("X-UserID", raw)
			rec := httptest.NewRecorder()
			api.srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("X-UserID=%q: code = %d, attendu 401", raw, rec.Code)
			}
		})
	}
}
