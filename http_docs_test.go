package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestDocsServed(t *testing.T) {
	api := newAPITest(t)

	rec := api.do(http.MethodGet, "/docs", 0, nil)
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/docs/" {
		t.Errorf("/docs : code %d, Location %q, attendu redirection vers /docs/", rec.Code, rec.Header().Get("Location"))
	}

	rec = api.do(http.MethodGet, "/docs/", 0, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/docs/ : code %d, attendu 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("/docs/ : Content-Type %q, attendu text/html", ct)
	}

	rec = api.do(http.MethodGet, "/docs/openapi.yaml", 0, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/docs/openapi.yaml : code %d, attendu 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "openapi: 3") {
		t.Error("/docs/openapi.yaml : spécification OpenAPI 3 attendue")
	}
}

// TestDocsCoverAllRoutes fails when a route registered in NewServer has no
// matching path in the embedded spec. The list below must mirror the mux:
// Go 1.22 patterns and OpenAPI paths share the same {id} syntax.
func TestDocsCoverAllRoutes(t *testing.T) {
	spec, err := docsFiles.ReadFile("docs/openapi.yaml")
	if err != nil {
		t.Fatalf("lecture openapi.yaml : %v", err)
	}

	routes := []string{
		"/health",
		"/api/users",
		"/api/users/{id}",
		"/api/users/{id}/skills",
		"/api/users/{id}/reviews",
		"/api/users/{id}/stats",
		"/api/services",
		"/api/services/{id}",
		"/api/services/{id}/reviews",
		"/api/exchanges",
		"/api/exchanges/{id}",
		"/api/exchanges/{id}/accept",
		"/api/exchanges/{id}/reject",
		"/api/exchanges/{id}/complete",
		"/api/exchanges/{id}/cancel",
		"/api/exchanges/{id}/review",
	}
	for _, route := range routes {
		if !strings.Contains(string(spec), "\n  "+route+":") {
			t.Errorf("endpoint %s absent de docs/openapi.yaml", route)
		}
	}
}
