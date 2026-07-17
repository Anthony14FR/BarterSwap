package main

import (
	"context"
	"errors"
	"fmt"
	"log"
)

// seed inserts a demo dataset through the App layer, so every row respects
// the business rules. It refuses to run on a non-empty database: user IDs
// are a SERIAL starting at 1 and accounts are never deleted, so user 1
// existing means the base has already been used.
func seed(ctx context.Context, app *App) error {
	if _, err := app.GetUser(ctx, 1); err == nil {
		return errors.New("seed refusé : la base contient déjà des données")
	} else if !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("seed: vérification base vide: %w", err)
	}

	users := []struct {
		user   User
		skills []Skill
	}{
		{User{Pseudo: "Alice", Bio: "Main verte du dimanche", Ville: "Paris"},
			[]Skill{{Nom: "Jardinage", Niveau: "expert"}, {Nom: "Bricolage", Niveau: "intermédiaire"}}},
		{User{Pseudo: "Bruno", Bio: "Dev le jour, guitariste le soir", Ville: "Lyon"},
			[]Skill{{Nom: "Informatique", Niveau: "expert"}, {Nom: "Musique", Niveau: "intermédiaire"}}},
		{User{Pseudo: "Chloé", Bio: "Cheffe amatrice et polyglotte", Ville: "Marseille"},
			[]Skill{{Nom: "Cuisine", Niveau: "expert"}, {Nom: "Langues", Niveau: "expert"}}},
		{User{Pseudo: "David", Bio: "Photographe du week-end", Ville: "Paris"},
			[]Skill{{Nom: "Photographie", Niveau: "intermédiaire"}, {Nom: "Déménagement", Niveau: "débutant"}}},
		{User{Pseudo: "Emma", Bio: "Étudiante en maths", Ville: "Lyon"},
			[]Skill{{Nom: "Tutorat", Niveau: "expert"}, {Nom: "Couture", Niveau: "débutant"}}},
	}
	userIDs := make([]int, len(users))
	for i, su := range users {
		u, err := app.CreateUser(ctx, su.user)
		if err != nil {
			return fmt.Errorf("seed: utilisateur %q: %w", su.user.Pseudo, err)
		}
		if _, err := app.SetSkills(ctx, u.ID, u.ID, su.skills); err != nil {
			return fmt.Errorf("seed: compétences de %q: %w", su.user.Pseudo, err)
		}
		userIDs[i] = u.ID
	}
	alice, bruno, chloe, david, emma := userIDs[0], userIDs[1], userIDs[2], userIDs[3], userIDs[4]

	services := []struct {
		provider int
		svc      Service
	}{
		{alice, Service{Titre: "Tonte de pelouse et taille de haies", Description: "Matériel fourni", Categorie: "Jardinage", DureeMinutes: 60, Credits: 4, Ville: "Paris", Actif: true}},
		{alice, Service{Titre: "Montage de meubles en kit", Description: "Notice ou pas, ça tient", Categorie: "Bricolage", DureeMinutes: 90, Credits: 5, Ville: "Paris", Actif: true}},
		{bruno, Service{Titre: "Dépannage PC et installation de logiciels", Description: "Windows ou Linux", Categorie: "Informatique", DureeMinutes: 60, Credits: 4, Ville: "Lyon", Actif: true}},
		{bruno, Service{Titre: "Initiation à la guitare", Description: "Guitare prêtée pour la séance", Categorie: "Musique", DureeMinutes: 45, Credits: 3, Ville: "Lyon", Actif: true}},
		{chloe, Service{Titre: "Cours de cuisine provençale", Description: "Ratatouille, tapenade, navettes", Categorie: "Cuisine", DureeMinutes: 120, Credits: 6, Ville: "Marseille", Actif: true}},
		{chloe, Service{Titre: "Conversation en anglais", Description: "Autour d'un café", Categorie: "Langues", DureeMinutes: 45, Credits: 2, Ville: "Marseille", Actif: true}},
		{david, Service{Titre: "Séance photo portrait", Description: "En extérieur, retouches incluses", Categorie: "Photographie", DureeMinutes: 90, Credits: 5, Ville: "Paris", Actif: true}},
		{emma, Service{Titre: "Aide aux devoirs en maths (lycée)", Description: "De la seconde à la terminale", Categorie: "Tutorat", DureeMinutes: 60, Credits: 3, Ville: "Lyon", Actif: true}},
	}
	svcIDs := make([]int, len(services))
	for i, ss := range services {
		svc, err := app.CreateService(ctx, ss.provider, ss.svc)
		if err != nil {
			return fmt.Errorf("seed: service %q: %w", ss.svc.Titre, err)
		}
		svcIDs[i] = svc.ID
	}
	tonte, depannage, cuisine, photo, tutorat := svcIDs[0], svcIDs[2], svcIDs[4], svcIDs[6], svcIDs[7]

	// One exchange per status: completed with two reviews, accepted (credits
	// blocked), pending, rejected, and cancelled after accept (refund).
	done, err := app.CreateExchange(ctx, bruno, tonte)
	if err != nil {
		return fmt.Errorf("seed: échange completed: %w", err)
	}
	if _, err := app.AcceptExchange(ctx, alice, done.ID); err != nil {
		return fmt.Errorf("seed: échange completed: %w", err)
	}
	if _, err := app.CompleteExchange(ctx, alice, done.ID); err != nil {
		return fmt.Errorf("seed: échange completed: %w", err)
	}
	if _, err := app.CreateReview(ctx, bruno, done.ID, Review{Note: 5, Commentaire: "Pelouse impeccable, très pro"}); err != nil {
		return fmt.Errorf("seed: avis de Bruno: %w", err)
	}
	if _, err := app.CreateReview(ctx, alice, done.ID, Review{Note: 4, Commentaire: "Ponctuel et sympathique"}); err != nil {
		return fmt.Errorf("seed: avis d'Alice: %w", err)
	}

	blocked, err := app.CreateExchange(ctx, chloe, depannage)
	if err != nil {
		return fmt.Errorf("seed: échange accepted: %w", err)
	}
	if _, err := app.AcceptExchange(ctx, bruno, blocked.ID); err != nil {
		return fmt.Errorf("seed: échange accepted: %w", err)
	}

	if _, err := app.CreateExchange(ctx, david, cuisine); err != nil {
		return fmt.Errorf("seed: échange pending: %w", err)
	}

	refused, err := app.CreateExchange(ctx, emma, photo)
	if err != nil {
		return fmt.Errorf("seed: échange rejected: %w", err)
	}
	if _, err := app.RejectExchange(ctx, david, refused.ID); err != nil {
		return fmt.Errorf("seed: échange rejected: %w", err)
	}

	dropped, err := app.CreateExchange(ctx, alice, tutorat)
	if err != nil {
		return fmt.Errorf("seed: échange cancelled: %w", err)
	}
	if _, err := app.AcceptExchange(ctx, emma, dropped.ID); err != nil {
		return fmt.Errorf("seed: échange cancelled: %w", err)
	}
	if _, err := app.CancelExchange(ctx, alice, dropped.ID); err != nil {
		return fmt.Errorf("seed: échange cancelled: %w", err)
	}

	log.Printf("seed terminé : %d utilisateurs, %d services, 5 échanges (un par statut), 2 avis", len(users), len(services))
	return nil
}
