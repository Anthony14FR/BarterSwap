package main

import (
	"context"
	"errors"
	"testing"
)

func newTestApp() (*App, *memStore) {
	store := newMemStore()
	return NewApp(store), store
}

func mustUser(t *testing.T, a *App, pseudo string) User {
	t.Helper()
	u, err := a.CreateUser(context.Background(), User{Pseudo: pseudo})
	if err != nil {
		t.Fatalf("création de %q: %v", pseudo, err)
	}
	return u
}

func seedProviderService(t *testing.T, a *App, credits int) (User, Service) {
	t.Helper()
	ctx := context.Background()
	provider := mustUser(t, a, "Tom")
	if _, err := a.SetSkills(ctx, provider.ID, provider.ID, []Skill{{Nom: "Jardinage", Niveau: "expert"}}); err != nil {
		t.Fatalf("compétences: %v", err)
	}
	service, err := a.CreateService(ctx, provider.ID, Service{
		Titre: "Tonte de pelouse", Categorie: "Jardinage", DureeMinutes: 60, Credits: credits, Actif: true,
	})
	if err != nil {
		t.Fatalf("service: %v", err)
	}
	return provider, service
}

func TestCreateUserWelcomeCredits(t *testing.T) {
	a, _ := newTestApp()
	u, err := a.CreateUser(context.Background(), User{Pseudo: "Tom"})
	if err != nil {
		t.Fatalf("erreur inattendue: %v", err)
	}
	if u.CreditBalance != welcomeCredits {
		t.Errorf("solde de bienvenue = %d, attendu %d", u.CreditBalance, welcomeCredits)
	}
}

func TestCreateUserInvalidPseudo(t *testing.T) {
	cases := []struct {
		name   string
		pseudo string
	}{
		{"vide", ""},
		{"espaces", "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := newTestApp()
			_, err := a.CreateUser(context.Background(), User{Pseudo: tc.pseudo})
			if !errors.Is(err, ErrValidation) {
				t.Errorf("attendu ErrValidation, obtenu %v", err)
			}
		})
	}
}

func TestCreateServiceRequiresSkill(t *testing.T) {
	a, _ := newTestApp()
	ctx := context.Background()
	provider := mustUser(t, a, "Tom")

	_, err := a.CreateService(ctx, provider.ID, Service{
		Titre: "Cours de piano", Categorie: "Musique", DureeMinutes: 45, Credits: 3,
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("sans compétence: attendu ErrValidation, obtenu %v", err)
	}

	if _, err := a.SetSkills(ctx, provider.ID, provider.ID, []Skill{{Nom: "Musique", Niveau: "expert"}}); err != nil {
		t.Fatalf("compétences: %v", err)
	}
	if _, err := a.CreateService(ctx, provider.ID, Service{
		Titre: "Cours de piano", Categorie: "Musique", DureeMinutes: 45, Credits: 3,
	}); err != nil {
		t.Fatalf("avec compétence: erreur inattendue %v", err)
	}
}

func TestCreateExchangeOwnService(t *testing.T) {
	a, _ := newTestApp()
	provider, service := seedProviderService(t, a, 3)
	_, err := a.CreateExchange(context.Background(), provider.ID, service.ID)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("échange sur son propre service: attendu ErrValidation, obtenu %v", err)
	}
}

func TestCreateExchangeInsufficientCredits(t *testing.T) {
	a, _ := newTestApp()
	_, service := seedProviderService(t, a, 20)
	requester := mustUser(t, a, "Thami")
	_, err := a.CreateExchange(context.Background(), requester.ID, service.ID)
	if !errors.Is(err, ErrInsufficientCredits) {
		t.Errorf("attendu ErrInsufficientCredits, obtenu %v", err)
	}
}

func TestCreateExchangeConflict(t *testing.T) {
	a, _ := newTestApp()
	ctx := context.Background()
	_, service := seedProviderService(t, a, 3)
	r1 := mustUser(t, a, "Thami")
	r2 := mustUser(t, a, "Flo")

	if _, err := a.CreateExchange(ctx, r1.ID, service.ID); err != nil {
		t.Fatalf("premier échange: %v", err)
	}
	_, err := a.CreateExchange(ctx, r2.ID, service.ID)
	if !errors.Is(err, ErrConflict) {
		t.Errorf("service déjà réservé: attendu ErrConflict, obtenu %v", err)
	}
}

func TestExchangeLifecycleCredits(t *testing.T) {
	a, store := newTestApp()
	ctx := context.Background()
	provider, service := seedProviderService(t, a, 4)
	requester := mustUser(t, a, "Thami")

	ex, err := a.CreateExchange(ctx, requester.ID, service.ID)
	if err != nil {
		t.Fatalf("création échange: %v", err)
	}

	if _, err := a.AcceptExchange(ctx, provider.ID, ex.ID); err != nil {
		t.Fatalf("acceptation: %v", err)
	}
	if bal, _ := store.Balance(ctx, requester.ID); bal != welcomeCredits-4 {
		t.Errorf("après blocage, solde demandeur = %d, attendu %d", bal, welcomeCredits-4)
	}
	if bal, _ := store.Balance(ctx, provider.ID); bal != welcomeCredits {
		t.Errorf("après blocage, solde offreur = %d, attendu %d (pas encore crédité)", bal, welcomeCredits)
	}

	if _, err := a.CompleteExchange(ctx, provider.ID, ex.ID); err != nil {
		t.Fatalf("complétion: %v", err)
	}
	if bal, _ := store.Balance(ctx, provider.ID); bal != welcomeCredits+4 {
		t.Errorf("après transfert, solde offreur = %d, attendu %d", bal, welcomeCredits+4)
	}
}

func TestExchangeCancelRefund(t *testing.T) {
	a, store := newTestApp()
	ctx := context.Background()
	provider, service := seedProviderService(t, a, 5)
	requester := mustUser(t, a, "Thami")

	ex, err := a.CreateExchange(ctx, requester.ID, service.ID)
	if err != nil {
		t.Fatalf("création: %v", err)
	}
	if _, err := a.AcceptExchange(ctx, provider.ID, ex.ID); err != nil {
		t.Fatalf("acceptation: %v", err)
	}
	if _, err := a.CancelExchange(ctx, requester.ID, ex.ID); err != nil {
		t.Fatalf("annulation: %v", err)
	}
	if bal, _ := store.Balance(ctx, requester.ID); bal != welcomeCredits {
		t.Errorf("après restitution, solde demandeur = %d, attendu %d", bal, welcomeCredits)
	}
}

func TestReviewRules(t *testing.T) {
	a, _ := newTestApp()
	ctx := context.Background()
	provider, service := seedProviderService(t, a, 3)
	requester := mustUser(t, a, "Thami")
	ex, _ := a.CreateExchange(ctx, requester.ID, service.ID)

	if _, err := a.CreateReview(ctx, requester.ID, ex.ID, Review{Note: 5}); !errors.Is(err, ErrValidation) {
		t.Errorf("note sur échange non terminé: attendu ErrValidation, obtenu %v", err)
	}

	if _, err := a.AcceptExchange(ctx, provider.ID, ex.ID); err != nil {
		t.Fatalf("acceptation: %v", err)
	}
	if _, err := a.CompleteExchange(ctx, provider.ID, ex.ID); err != nil {
		t.Fatalf("complétion: %v", err)
	}

	if _, err := a.CreateReview(ctx, requester.ID, ex.ID, Review{Note: 6}); !errors.Is(err, ErrValidation) {
		t.Errorf("note hors bornes: attendu ErrValidation, obtenu %v", err)
	}
	if _, err := a.CreateReview(ctx, requester.ID, ex.ID, Review{Note: 5}); err != nil {
		t.Fatalf("premier avis: %v", err)
	}
	if _, err := a.CreateReview(ctx, requester.ID, ex.ID, Review{Note: 4}); !errors.Is(err, ErrValidation) {
		t.Errorf("second avis: attendu ErrValidation, obtenu %v", err)
	}
}

func TestUserStats(t *testing.T) {
	a, _ := newTestApp()
	ctx := context.Background()
	provider, service := seedProviderService(t, a, 4)
	requester := mustUser(t, a, "Thami")
	ex, _ := a.CreateExchange(ctx, requester.ID, service.ID)
	if _, err := a.AcceptExchange(ctx, provider.ID, ex.ID); err != nil {
		t.Fatalf("acceptation: %v", err)
	}
	if _, err := a.CompleteExchange(ctx, provider.ID, ex.ID); err != nil {
		t.Fatalf("complétion: %v", err)
	}
	if _, err := a.CreateReview(ctx, requester.ID, ex.ID, Review{Note: 4}); err != nil {
		t.Fatalf("avis: %v", err)
	}

	stats, err := a.UserStats(ctx, provider.ID)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.ServicesActifs != 1 {
		t.Errorf("services actifs = %d, attendu 1", stats.ServicesActifs)
	}
	if stats.EchangesCompletes != 1 {
		t.Errorf("échanges complétés = %d, attendu 1", stats.EchangesCompletes)
	}
	if stats.CreditBalance != welcomeCredits+4 {
		t.Errorf("solde = %d, attendu %d", stats.CreditBalance, welcomeCredits+4)
	}
	if stats.NoteMoyenne != 4 {
		t.Errorf("note moyenne = %v, attendu 4", stats.NoteMoyenne)
	}
	if stats.NbAvis != 1 {
		t.Errorf("nombre d'avis = %d, attendu 1", stats.NbAvis)
	}
}
