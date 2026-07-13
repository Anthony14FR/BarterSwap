package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
)

func integrationAPI(t *testing.T) *apiTest {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL non défini : test d'intégration PostgreSQL ignoré")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("ouverture base: %v", err)
	}
	store := NewSQLStore(db)
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migration: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`TRUNCATE users, skills, services, exchanges, credit_transactions, reviews RESTART IDENTITY CASCADE`,
	); err != nil {
		t.Fatalf("nettoyage: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return &apiTest{t: t, srv: NewServer(NewApp(store))}
}

func TestIntegrationUserAndServiceLifecycle(t *testing.T) {
	api := integrationAPI(t)

	provider := api.createUser("provider")

	if rec := api.do(http.MethodPut, fmt.Sprintf("/api/users/%d", provider.ID), provider.ID,
		map[string]string{"pseudo": "provider2", "bio": "jardinier", "ville": "Paris"}); rec.Code != http.StatusOK {
		t.Fatalf("modification profil: code %d, corps %s", rec.Code, rec.Body)
	}

	api.do(http.MethodPut, fmt.Sprintf("/api/users/%d/skills", provider.ID), provider.ID,
		[]Skill{{Nom: "Jardinage", Niveau: "expert"}, {Nom: "Bricolage", Niveau: "intermédiaire"}})

	if rec := api.do(http.MethodGet, fmt.Sprintf("/api/users/%d/skills", provider.ID), 0, nil); rec.Code != http.StatusOK {
		t.Fatalf("lecture compétences: code %d", rec.Code)
	}

	rec := api.do(http.MethodPost, "/api/services", provider.ID, map[string]any{
		"titre": "Tonte", "categorie": "Jardinage", "duree_minutes": 60, "credits": 4, "ville": "Paris",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("création service: code %d, corps %s", rec.Code, rec.Body)
	}
	var svc Service
	decodeBody(t, rec, &svc)

	if rec := api.do(http.MethodPut, fmt.Sprintf("/api/services/%d", svc.ID), provider.ID, map[string]any{
		"titre": "Tonte pro", "categorie": "Jardinage", "duree_minutes": 90, "credits": 5, "ville": "Paris",
	}); rec.Code != http.StatusOK {
		t.Fatalf("modification service: code %d, corps %s", rec.Code, rec.Body)
	}

	for _, q := range []string{"?categorie=Jardinage", "?ville=paris", "?search=tonte"} {
		rec := api.do(http.MethodGet, "/api/services"+q, 0, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("liste %s: code %d", q, rec.Code)
		}
		var services []Service
		decodeBody(t, rec, &services)
		if len(services) != 1 {
			t.Errorf("liste %s: %d résultat(s), attendu 1", q, len(services))
		}
	}

	if rec := api.do(http.MethodDelete, fmt.Sprintf("/api/services/%d", svc.ID), provider.ID, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("suppression: code %d", rec.Code)
	}
	if rec := api.do(http.MethodGet, fmt.Sprintf("/api/services/%d", svc.ID), 0, nil); rec.Code != http.StatusNotFound {
		t.Errorf("service supprimé: code %d, attendu 404", rec.Code)
	}
}

func TestIntegrationExchangeHappyPath(t *testing.T) {
	api := integrationAPI(t)
	provider, svc := api.seedService("provider", "Jardinage", 4)
	requester := api.createUser("requester")

	rec := api.do(http.MethodPost, "/api/exchanges", requester.ID, map[string]int{"service_id": svc.ID})
	if rec.Code != http.StatusCreated {
		t.Fatalf("création échange: code %d, corps %s", rec.Code, rec.Body)
	}
	var ex Exchange
	decodeBody(t, rec, &ex)

	if rec := api.do(http.MethodGet, fmt.Sprintf("/api/exchanges/%d", ex.ID), requester.ID, nil); rec.Code != http.StatusOK {
		t.Fatalf("détail échange: code %d", rec.Code)
	}
	if rec := api.do(http.MethodGet, "/api/exchanges?status=pending", requester.ID, nil); rec.Code != http.StatusOK {
		t.Fatalf("liste échanges: code %d", rec.Code)
	}

	api.do(http.MethodPut, fmt.Sprintf("/api/exchanges/%d/accept", ex.ID), provider.ID, nil)
	api.do(http.MethodPut, fmt.Sprintf("/api/exchanges/%d/complete", ex.ID), provider.ID, nil)

	if rec := api.do(http.MethodPost, fmt.Sprintf("/api/exchanges/%d/review", ex.ID), requester.ID,
		map[string]any{"note": 5, "commentaire": "top"}); rec.Code != http.StatusCreated {
		t.Fatalf("avis: code %d, corps %s", rec.Code, rec.Body)
	}
	if rec := api.do(http.MethodPost, fmt.Sprintf("/api/exchanges/%d/review", ex.ID), requester.ID,
		map[string]any{"note": 3}); rec.Code != http.StatusBadRequest {
		t.Errorf("second avis: code %d, attendu 400", rec.Code)
	}

	if rec := api.do(http.MethodGet, fmt.Sprintf("/api/users/%d/reviews", provider.ID), 0, nil); rec.Code != http.StatusOK {
		t.Fatalf("avis utilisateur: code %d", rec.Code)
	}
	if rec := api.do(http.MethodGet, fmt.Sprintf("/api/services/%d/reviews", svc.ID), 0, nil); rec.Code != http.StatusOK {
		t.Fatalf("avis service: code %d", rec.Code)
	}

	rec = api.do(http.MethodGet, fmt.Sprintf("/api/users/%d/stats", provider.ID), 0, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: code %d", rec.Code)
	}
	var stats UserStats
	decodeBody(t, rec, &stats)
	if stats.CreditBalance != welcomeCredits+4 || stats.EchangesCompletes != 1 || stats.TotalGagne != welcomeCredits+4 {
		t.Errorf("stats incohérentes: %+v", stats)
	}
}

func TestIntegrationRejectAndCancelRefund(t *testing.T) {
	api := integrationAPI(t)
	provider, svc := api.seedService("provider", "Jardinage", 5)
	requester := api.createUser("requester")

	// reject
	rec := api.do(http.MethodPost, "/api/exchanges", requester.ID, map[string]int{"service_id": svc.ID})
	var ex Exchange
	decodeBody(t, rec, &ex)
	if rec := api.do(http.MethodPut, fmt.Sprintf("/api/exchanges/%d/reject", ex.ID), provider.ID, nil); rec.Code != http.StatusOK {
		t.Fatalf("refus: code %d", rec.Code)
	}

	// cancel with refund
	rec = api.do(http.MethodPost, "/api/exchanges", requester.ID, map[string]int{"service_id": svc.ID})
	decodeBody(t, rec, &ex)
	api.do(http.MethodPut, fmt.Sprintf("/api/exchanges/%d/accept", ex.ID), provider.ID, nil)
	if rec := api.do(http.MethodPut, fmt.Sprintf("/api/exchanges/%d/cancel", ex.ID), requester.ID, nil); rec.Code != http.StatusOK {
		t.Fatalf("annulation: code %d", rec.Code)
	}

	rec = api.do(http.MethodGet, fmt.Sprintf("/api/users/%d/stats", requester.ID), 0, nil)
	var stats UserStats
	decodeBody(t, rec, &stats)
	if stats.CreditBalance != welcomeCredits {
		t.Errorf("après restitution, solde = %d, attendu %d", stats.CreditBalance, welcomeCredits)
	}
}

func TestIntegrationExchangeConflict(t *testing.T) {
	api := integrationAPI(t)
	_, svc := api.seedService("provider", "Jardinage", 3)
	r1 := api.createUser("r1")
	r2 := api.createUser("r2")

	if rec := api.do(http.MethodPost, "/api/exchanges", r1.ID, map[string]int{"service_id": svc.ID}); rec.Code != http.StatusCreated {
		t.Fatalf("premier échange: code %d", rec.Code)
	}
	if rec := api.do(http.MethodPost, "/api/exchanges", r2.ID, map[string]int{"service_id": svc.ID}); rec.Code != http.StatusConflict {
		t.Errorf("service déjà réservé: code %d, attendu 409", rec.Code)
	}
}

func TestIntegrationConcurrentAcceptInsufficientCredits(t *testing.T) {
	api := integrationAPI(t)
	requester := api.createUser("requester") // solde = welcomeCredits (10)

	const (
		numServices = 5
		serviceCost = 3
		expectedOK  = welcomeCredits / serviceCost // 3
	)

	type pending struct {
		ownerID    int
		exchangeID int
	}
	pairs := make([]pending, numServices)
	for i := 0; i < numServices; i++ {
		provider, svc := api.seedService(fmt.Sprintf("provider%d", i), "Jardinage", serviceCost)
		rec := api.do(http.MethodPost, "/api/exchanges", requester.ID, map[string]int{"service_id": svc.ID})
		if rec.Code != http.StatusCreated {
			t.Fatalf("création échange %d: code %d, corps %s", i, rec.Code, rec.Body)
		}
		var ex Exchange
		decodeBody(t, rec, &ex)
		pairs[i] = pending{ownerID: provider.ID, exchangeID: ex.ID}
	}

	var wg sync.WaitGroup
	codes := make([]int, numServices)
	for i, p := range pairs {
		wg.Add(1)
		go func(i int, p pending) {
			defer wg.Done()
			rec := api.do(http.MethodPut, fmt.Sprintf("/api/exchanges/%d/accept", p.exchangeID), p.ownerID, nil)
			codes[i] = rec.Code
		}(i, p)
	}
	wg.Wait()

	okCount, badCount := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			okCount++
		case http.StatusBadRequest:
			badCount++
		default:
			t.Errorf("code inattendu: %d", c)
		}
	}
	if okCount != expectedOK {
		t.Errorf("acceptations réussies = %d, attendu %d", okCount, expectedOK)
	}
	if badCount != numServices-expectedOK {
		t.Errorf("acceptations refusées = %d, attendu %d", badCount, numServices-expectedOK)
	}

	rec := api.do(http.MethodGet, fmt.Sprintf("/api/users/%d/stats", requester.ID), 0, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: code %d", rec.Code)
	}
	var stats UserStats
	decodeBody(t, rec, &stats)
	if stats.CreditBalance < 0 {
		t.Fatalf("solde négatif détecté: %d", stats.CreditBalance)
	}
	if stats.CreditBalance != welcomeCredits-expectedOK*serviceCost {
		t.Errorf("solde final = %d, attendu %d", stats.CreditBalance, welcomeCredits-expectedOK*serviceCost)
	}
}
