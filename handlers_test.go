package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

type apiTest struct {
	t   *testing.T
	srv http.Handler
}

func newAPITest(t *testing.T) *apiTest {
	return &apiTest{t: t, srv: NewServer(NewApp(newMemStore()))}
}

func (a *apiTest) do(method, path string, userID int, body any) *httptest.ResponseRecorder {
	a.t.Helper()
	var r io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			a.t.Fatalf("marshal: %v", err)
		}
		r = bytes.NewReader(buf)
	}
	req := httptest.NewRequest(method, path, r)
	if userID > 0 {
		req.Header.Set("X-UserID", strconv.Itoa(userID))
	}
	rec := httptest.NewRecorder()
	a.srv.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(dst); err != nil {
		t.Fatalf("décodage réponse: %v", err)
	}
}

func (a *apiTest) createUser(pseudo string) User {
	a.t.Helper()
	rec := a.do(http.MethodPost, "/api/users", 0, map[string]string{"pseudo": pseudo})
	if rec.Code != http.StatusCreated {
		a.t.Fatalf("createUser %q: code %d, corps %s", pseudo, rec.Code, rec.Body)
	}
	var u User
	decodeBody(a.t, rec, &u)
	return u
}

func (a *apiTest) seedService(pseudo, categorie string, credits int) (User, Service) {
	a.t.Helper()
	provider := a.createUser(pseudo)
	skills := []Skill{{Nom: categorie, Niveau: "expert"}}
	if rec := a.do(http.MethodPut, fmt.Sprintf("/api/users/%d/skills", provider.ID), provider.ID, skills); rec.Code != http.StatusOK {
		a.t.Fatalf("skills: code %d, corps %s", rec.Code, rec.Body)
	}
	rec := a.do(http.MethodPost, "/api/services", provider.ID, map[string]any{
		"titre": "Service " + categorie, "categorie": categorie, "duree_minutes": 60, "credits": credits,
	})
	if rec.Code != http.StatusCreated {
		a.t.Fatalf("service: code %d, corps %s", rec.Code, rec.Body)
	}
	var svc Service
	decodeBody(a.t, rec, &svc)
	return provider, svc
}

func TestAPICreateUser(t *testing.T) {
	cases := []struct {
		name string
		body any
		code int
	}{
		{"succès", map[string]string{"pseudo": "Tom"}, http.StatusCreated},
		{"pseudo vide", map[string]string{"pseudo": ""}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newAPITest(t)
			rec := api.do(http.MethodPost, "/api/users", 0, tc.body)
			if rec.Code != tc.code {
				t.Fatalf("code = %d, attendu %d (corps %s)", rec.Code, tc.code, rec.Body)
			}
		})
	}
}

func TestAPIGetUser(t *testing.T) {
	api := newAPITest(t)
	u := api.createUser("Tom")

	if rec := api.do(http.MethodGet, fmt.Sprintf("/api/users/%d", u.ID), 0, nil); rec.Code != http.StatusOK {
		t.Errorf("profil existant: code %d", rec.Code)
	}
	if rec := api.do(http.MethodGet, "/api/users/9999", 0, nil); rec.Code != http.StatusNotFound {
		t.Errorf("profil inexistant: code %d, attendu 404", rec.Code)
	}
}

func TestAPICreateServiceAuthAndSkill(t *testing.T) {
	api := newAPITest(t)
	provider := api.createUser("Tom")
	body := map[string]any{"titre": "Tonte", "categorie": "Jardinage", "duree_minutes": 60, "credits": 3}

	if rec := api.do(http.MethodPost, "/api/services", 0, body); rec.Code != http.StatusUnauthorized {
		t.Errorf("sans X-UserID: code %d, attendu 401", rec.Code)
	}
	if rec := api.do(http.MethodPost, "/api/services", provider.ID, body); rec.Code != http.StatusBadRequest {
		t.Errorf("sans compétence: code %d, attendu 400", rec.Code)
	}
	api.do(http.MethodPut, fmt.Sprintf("/api/users/%d/skills", provider.ID), provider.ID, []Skill{{Nom: "Jardinage", Niveau: "expert"}})
	if rec := api.do(http.MethodPost, "/api/services", provider.ID, body); rec.Code != http.StatusCreated {
		t.Errorf("avec compétence: code %d, attendu 201 (corps %s)", rec.Code, rec.Body)
	}
}

func TestAPIExchangeHappyPath(t *testing.T) {
	api := newAPITest(t)
	provider, svc := api.seedService("Tom", "Jardinage", 4)
	requester := api.createUser("Thami")

	if rec := api.do(http.MethodPost, "/api/exchanges", provider.ID, map[string]int{"service_id": svc.ID}); rec.Code != http.StatusBadRequest {
		t.Errorf("échange sur son propre service: code %d, attendu 400", rec.Code)
	}

	rec := api.do(http.MethodPost, "/api/exchanges", requester.ID, map[string]int{"service_id": svc.ID})
	if rec.Code != http.StatusCreated {
		t.Fatalf("création échange: code %d, corps %s", rec.Code, rec.Body)
	}
	var ex Exchange
	decodeBody(t, rec, &ex)

	if rec := api.do(http.MethodPut, fmt.Sprintf("/api/exchanges/%d/accept", ex.ID), provider.ID, nil); rec.Code != http.StatusOK {
		t.Fatalf("acceptation: code %d, corps %s", rec.Code, rec.Body)
	}
	if rec := api.do(http.MethodPut, fmt.Sprintf("/api/exchanges/%d/complete", ex.ID), provider.ID, nil); rec.Code != http.StatusOK {
		t.Fatalf("complétion: code %d, corps %s", rec.Code, rec.Body)
	}
	if rec := api.do(http.MethodPost, fmt.Sprintf("/api/exchanges/%d/review", ex.ID), requester.ID, map[string]any{"note": 5, "commentaire": "parfait"}); rec.Code != http.StatusCreated {
		t.Fatalf("avis: code %d, corps %s", rec.Code, rec.Body)
	}

	rec = api.do(http.MethodGet, fmt.Sprintf("/api/users/%d/stats", provider.ID), 0, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: code %d", rec.Code)
	}
	var stats UserStats
	decodeBody(t, rec, &stats)
	if stats.CreditBalance != welcomeCredits+4 || stats.EchangesCompletes != 1 || stats.NbAvis != 1 {
		t.Errorf("stats incohérentes: %+v", stats)
	}
}

func TestAPIExchangeConflict(t *testing.T) {
	api := newAPITest(t)
	_, svc := api.seedService("Tom", "Jardinage", 3)
	r1 := api.createUser("Thami")
	r2 := api.createUser("Flo")

	if rec := api.do(http.MethodPost, "/api/exchanges", r1.ID, map[string]int{"service_id": svc.ID}); rec.Code != http.StatusCreated {
		t.Fatalf("premier échange: code %d", rec.Code)
	}
	if rec := api.do(http.MethodPost, "/api/exchanges", r2.ID, map[string]int{"service_id": svc.ID}); rec.Code != http.StatusConflict {
		t.Errorf("service déjà réservé: code %d, attendu 409", rec.Code)
	}
}

func TestAPIListServicesFilter(t *testing.T) {
	api := newAPITest(t)
	api.seedService("Tom", "Jardinage", 3)
	api.seedService("Thami", "Informatique", 5)

	rec := api.do(http.MethodGet, "/api/services?categorie=Informatique", 0, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("liste: code %d", rec.Code)
	}
	var services []Service
	decodeBody(t, rec, &services)
	if len(services) != 1 || services[0].Categorie != "Informatique" {
		t.Errorf("filtre catégorie: %d résultat(s), attendu 1 en Informatique", len(services))
	}
}

func TestAPIUpdateServiceForbiddenForNonOwner(t *testing.T) {
	api := newAPITest(t)
	_, svc := api.seedService("Tom", "Jardinage", 3)
	other := api.createUser("Thami")
	rec := api.do(http.MethodPut, fmt.Sprintf("/api/services/%d", svc.ID), other.ID,
		map[string]any{"titre": "hack", "categorie": "Jardinage", "duree_minutes": 10, "credits": 1})
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, attendu 403", rec.Code)
	}
}

func TestAPIDeleteServiceForbiddenForNonOwner(t *testing.T) {
	api := newAPITest(t)
	_, svc := api.seedService("Tom", "Jardinage", 3)
	other := api.createUser("Thami")
	rec := api.do(http.MethodDelete, fmt.Sprintf("/api/services/%d", svc.ID), other.ID, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, attendu 403", rec.Code)
	}
}

func TestAPIGetExchangeForbiddenForNonParticipant(t *testing.T) {
	api := newAPITest(t)
	_, svc := api.seedService("Tom", "Jardinage", 3)
	requester := api.createUser("Thami")
	outsider := api.createUser("Flo")

	rec := api.do(http.MethodPost, "/api/exchanges", requester.ID, map[string]int{"service_id": svc.ID})
	var ex Exchange
	decodeBody(t, rec, &ex)

	rec = api.do(http.MethodGet, fmt.Sprintf("/api/exchanges/%d", ex.ID), outsider.ID, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, attendu 403", rec.Code)
	}
}

func TestAPIUpdateUser(t *testing.T) {
	api := newAPITest(t)
	u := api.createUser("Tom")

	rec := api.do(http.MethodPut, fmt.Sprintf("/api/users/%d", u.ID), u.ID,
		map[string]string{"pseudo": "Thami", "bio": "marseillais fan de jul", "ville": "Marseille"})
	if rec.Code != http.StatusOK {
		t.Fatalf("modification: code %d, corps %s", rec.Code, rec.Body)
	}
	var updated User
	decodeBody(t, rec, &updated)
	if updated.Pseudo != "Thami" || updated.Bio != "marseillais fan de jul" || updated.Ville != "Marseille" {
		t.Errorf("utilisateur modifié = %+v", updated)
	}

	rec = api.do(http.MethodGet, fmt.Sprintf("/api/users/%d", u.ID), 0, nil)
	var fetched User
	decodeBody(t, rec, &fetched)
	if fetched.Pseudo != "Thami" || fetched.Bio != "marseillais fan de jul" || fetched.Ville != "Marseille" {
		t.Errorf("utilisateur relu = %+v", fetched)
	}
}

func TestAPIUpdateService(t *testing.T) {
	api := newAPITest(t)
	provider, svc := api.seedService("Tom", "Jardinage", 3)

	rec := api.do(http.MethodPut, fmt.Sprintf("/api/services/%d", svc.ID), provider.ID,
		map[string]any{"titre": "Tonte pro", "categorie": "Jardinage", "duree_minutes": 90, "credits": 6, "ville": "Montcuq"})
	if rec.Code != http.StatusOK {
		t.Fatalf("modification service: code %d, corps %s", rec.Code, rec.Body)
	}
	var updated Service
	decodeBody(t, rec, &updated)
	if updated.Titre != "Tonte pro" || updated.Credits != 6 || updated.DureeMinutes != 90 || updated.Ville != "Montcuq" {
		t.Errorf("service modifié = %+v", updated)
	}
}

func TestAPIGetUserSkills(t *testing.T) {
	api := newAPITest(t)
	u := api.createUser("Tom")
	skills := []Skill{{Nom: "Jardinage", Niveau: "expert"}, {Nom: "Cuisine", Niveau: "débutant"}}
	if rec := api.do(http.MethodPut, fmt.Sprintf("/api/users/%d/skills", u.ID), u.ID, skills); rec.Code != http.StatusOK {
		t.Fatalf("écriture compétences: code %d", rec.Code)
	}
	rec := api.do(http.MethodGet, fmt.Sprintf("/api/users/%d/skills", u.ID), 0, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("lecture compétences: code %d", rec.Code)
	}
	var got []Skill
	decodeBody(t, rec, &got)
	if len(got) != 2 {
		t.Fatalf("compétences = %v, attendu 2", got)
	}
}

func TestAPIListServicesVilleAndSearchFilters(t *testing.T) {
	api := newAPITest(t)
	provider := api.createUser("Tom")
	api.do(http.MethodPut, fmt.Sprintf("/api/users/%d/skills", provider.ID), provider.ID,
		[]Skill{{Nom: "Jardinage", Niveau: "expert"}, {Nom: "Cuisine", Niveau: "expert"}})

	api.do(http.MethodPost, "/api/services", provider.ID, map[string]any{
		"titre": "Tonte de pelouse", "categorie": "Jardinage", "duree_minutes": 60, "credits": 3, "ville": "Paris",
	})
	api.do(http.MethodPost, "/api/services", provider.ID, map[string]any{
		"titre": "Cours de cuisine", "categorie": "Cuisine", "duree_minutes": 90, "credits": 5, "ville": "Montcuq",
	})

	rec := api.do(http.MethodGet, "/api/services?ville=paris", 0, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("filtre ville: code %d", rec.Code)
	}
	var byVille []Service
	decodeBody(t, rec, &byVille)
	if len(byVille) != 1 || byVille[0].Ville != "Paris" {
		t.Errorf("filtre ville: %d résultat(s), attendu 1", len(byVille))
	}

	rec = api.do(http.MethodGet, "/api/services?search=cuisine", 0, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("filtre recherche: code %d", rec.Code)
	}
	var bySearch []Service
	decodeBody(t, rec, &bySearch)
	if len(bySearch) != 1 || bySearch[0].Titre != "Cours de cuisine" {
		t.Errorf("filtre recherche: %d résultat(s), attendu 1", len(bySearch))
	}
}
