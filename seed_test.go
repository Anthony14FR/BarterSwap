package main

import (
	"context"
	"testing"
)

func TestSeed(t *testing.T) {
	a, store := newTestApp()
	ctx := context.Background()

	if err := seed(ctx, a); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if got := len(store.users); got != 5 {
		t.Errorf("utilisateurs = %d, attendu 5", got)
	}
	if got := len(store.services); got != 8 {
		t.Errorf("services = %d, attendu 8", got)
	}
	if got := len(store.reviews); got != 2 {
		t.Errorf("avis = %d, attendu 2", got)
	}

	byStatus := map[string]int{}
	for _, e := range store.exchanges {
		byStatus[e.Status]++
	}
	for _, status := range []string{StatusPending, StatusAccepted, StatusRejected, StatusCancelled, StatusCompleted} {
		if byStatus[status] != 1 {
			t.Errorf("échanges %q = %d, attendu 1", status, byStatus[status])
		}
	}

	// Alice earned 4 and got her 3 refunded, Bruno spent 4, Chloé has 4 blocked.
	wantBalances := map[string]int{
		"Alice": welcomeCredits + 4,
		"Bruno": welcomeCredits - 4,
		"Chloé": welcomeCredits - 4,
		"David": welcomeCredits,
		"Emma":  welcomeCredits,
	}
	for _, u := range store.users {
		bal, err := store.Balance(ctx, u.ID)
		if err != nil {
			t.Fatalf("solde de %q: %v", u.Pseudo, err)
		}
		if want, ok := wantBalances[u.Pseudo]; !ok || bal != want {
			t.Errorf("solde de %q = %d, attendu %d", u.Pseudo, bal, want)
		}
	}

	if err := seed(ctx, a); err == nil {
		t.Fatal("second seed: erreur attendue sur une base non vide")
	}
	if got := len(store.users); got != 5 {
		t.Errorf("après refus, utilisateurs = %d, attendu 5 (aucune écriture)", got)
	}
}
