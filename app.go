package main

import (
	"context"
	"fmt"
	"strings"
)

type App struct {
	store Store
}

func NewApp(store Store) *App { return &App{store: store} }

func isParticipant(e Exchange, userID int) bool {
	return userID == e.RequesterID || userID == e.OwnerID
}

func (a *App) CreateUser(ctx context.Context, in User) (User, error) {
	in.Pseudo = strings.TrimSpace(in.Pseudo)
	if in.Pseudo == "" {
		return User{}, validationError("le pseudo est obligatoire")
	}
	return a.store.CreateUser(ctx, in, welcomeCredits)
}

func (a *App) GetUser(ctx context.Context, id int) (User, error) {
	return a.store.GetUser(ctx, id)
}

func (a *App) UpdateUser(ctx context.Context, actorID, id int, in User) (User, error) {
	if actorID != id {
		return User{}, forbiddenError("vous ne pouvez modifier que votre propre profil")
	}
	in.Pseudo = strings.TrimSpace(in.Pseudo)
	if in.Pseudo == "" {
		return User{}, validationError("le pseudo est obligatoire")
	}
	in.ID = id
	return a.store.UpdateUser(ctx, in)
}

func (a *App) GetSkills(ctx context.Context, id int) ([]Skill, error) {
	if err := a.mustExistUser(ctx, id); err != nil {
		return nil, err
	}
	return a.store.GetSkills(ctx, id)
}

func (a *App) SetSkills(ctx context.Context, actorID, id int, skills []Skill) ([]Skill, error) {
	if actorID != id {
		return nil, forbiddenError("vous ne pouvez modifier que vos propres compétences")
	}
	if err := a.mustExistUser(ctx, id); err != nil {
		return nil, err
	}
	for i, sk := range skills {
		skills[i].Nom = strings.TrimSpace(sk.Nom)
		if skills[i].Nom == "" {
			return nil, validationError("le nom de la compétence #%d est obligatoire", i+1)
		}
		if !isValidNiveau(sk.Niveau) {
			return nil, validationError("niveau %q invalide (débutant, intermédiaire, expert)", sk.Niveau)
		}
	}
	if err := a.store.SetSkills(ctx, id, skills); err != nil {
		return nil, err
	}
	return a.store.GetSkills(ctx, id)
}

func (a *App) ListServices(ctx context.Context, f ServiceFilter) ([]Service, error) {
	return a.store.ListServices(ctx, f)
}

func (a *App) CreateService(ctx context.Context, providerID int, in Service) (Service, error) {
	if err := a.validateService(ctx, providerID, in); err != nil {
		return Service{}, err
	}
	in.ProviderID = providerID
	return a.store.CreateService(ctx, in)
}

func (a *App) GetService(ctx context.Context, id int) (Service, error) {
	return a.store.GetService(ctx, id)
}

func (a *App) UpdateService(ctx context.Context, actorID, id int, in Service) (Service, error) {
	existing, err := a.store.GetService(ctx, id)
	if err != nil {
		return Service{}, err
	}
	if existing.ProviderID != actorID {
		return Service{}, forbiddenError("vous ne pouvez modifier que vos propres annonces")
	}
	if err := a.validateService(ctx, actorID, in); err != nil {
		return Service{}, err
	}
	in.ID = id
	in.ProviderID = existing.ProviderID
	return a.store.UpdateService(ctx, in)
}

func (a *App) DeleteService(ctx context.Context, actorID, id int) error {
	existing, err := a.store.GetService(ctx, id)
	if err != nil {
		return err
	}
	if existing.ProviderID != actorID {
		return forbiddenError("vous ne pouvez supprimer que vos propres annonces")
	}
	return a.store.DeleteService(ctx, id)
}

func (a *App) validateService(ctx context.Context, providerID int, in Service) error {
	if strings.TrimSpace(in.Titre) == "" {
		return validationError("le titre est obligatoire")
	}
	if !isValidCategory(in.Categorie) {
		return validationError("catégorie %q invalide", in.Categorie)
	}
	if in.DureeMinutes <= 0 {
		return validationError("la durée doit être positive")
	}
	if in.Credits <= 0 {
		return validationError("le coût en crédits doit être positif")
	}
	skills, err := a.store.GetSkills(ctx, providerID)
	if err != nil {
		return err
	}
	for _, sk := range skills {
		if strings.EqualFold(sk.Nom, in.Categorie) {
			return nil
		}
	}
	return validationError("vous ne possédez pas la compétence %q", in.Categorie)
}

func (a *App) CreateExchange(ctx context.Context, requesterID, serviceID int) (Exchange, error) {
	service, err := a.store.GetService(ctx, serviceID)
	if err != nil {
		return Exchange{}, err
	}
	if service.ProviderID == requesterID {
		return Exchange{}, validationError("vous ne pouvez pas demander votre propre service")
	}
	balance, err := a.store.Balance(ctx, requesterID)
	if err != nil {
		return Exchange{}, err
	}
	if balance < service.Credits {
		return Exchange{}, fmt.Errorf("solde %d, coût %d: %w", balance, service.Credits, ErrInsufficientCredits)
	}
	return a.store.CreateExchange(ctx, Exchange{
		ServiceID:   serviceID,
		RequesterID: requesterID,
		OwnerID:     service.ProviderID,
	})
}

func (a *App) GetExchange(ctx context.Context, actorID, id int) (Exchange, error) {
	e, err := a.store.GetExchange(ctx, id)
	if err != nil {
		return Exchange{}, err
	}
	if !isParticipant(e, actorID) {
		return Exchange{}, forbiddenError("vous n'êtes pas partie à cet échange")
	}
	return e, nil
}

func (a *App) ListExchanges(ctx context.Context, userID int, status string) ([]Exchange, error) {
	if status != "" && !isValidStatus(status) {
		return nil, validationError("statut %q invalide", status)
	}
	return a.store.ListExchanges(ctx, userID, status)
}

func (a *App) AcceptExchange(ctx context.Context, actorID, id int) (Exchange, error) {
	e, err := a.loadTransition(ctx, id, actorID, StatusPending, roleOwner)
	if err != nil {
		return Exchange{}, err
	}
	service, err := a.store.GetService(ctx, e.ServiceID)
	if err != nil {
		return Exchange{}, err
	}
	balance, err := a.store.Balance(ctx, e.RequesterID)
	if err != nil {
		return Exchange{}, err
	}
	if balance < service.Credits {
		return Exchange{}, fmt.Errorf("solde %d, coût %d: %w", balance, service.Credits, ErrInsufficientCredits)
	}
	txn := CreditTransaction{UserID: e.RequesterID, Montant: -service.Credits, Type: TxnSpend}
	return a.store.TransitionExchange(ctx, id, StatusPending, StatusAccepted, []CreditTransaction{txn})
}

func (a *App) RejectExchange(ctx context.Context, actorID, id int) (Exchange, error) {
	if _, err := a.loadTransition(ctx, id, actorID, StatusPending, roleOwner); err != nil {
		return Exchange{}, err
	}
	return a.store.TransitionExchange(ctx, id, StatusPending, StatusRejected, nil)
}

func (a *App) CompleteExchange(ctx context.Context, actorID, id int) (Exchange, error) {
	e, err := a.loadTransition(ctx, id, actorID, StatusAccepted, roleParticipant)
	if err != nil {
		return Exchange{}, err
	}
	service, err := a.store.GetService(ctx, e.ServiceID)
	if err != nil {
		return Exchange{}, err
	}
	txn := CreditTransaction{UserID: e.OwnerID, Montant: service.Credits, Type: TxnEarn}
	return a.store.TransitionExchange(ctx, id, StatusAccepted, StatusCompleted, []CreditTransaction{txn})
}

func (a *App) CancelExchange(ctx context.Context, actorID, id int) (Exchange, error) {
	e, err := a.store.GetExchange(ctx, id)
	if err != nil {
		return Exchange{}, err
	}
	if !isParticipant(e, actorID) {
		return Exchange{}, forbiddenError("vous n'êtes pas partie à cet échange")
	}
	if e.Status != StatusPending && e.Status != StatusAccepted {
		return Exchange{}, conflictError("un échange %q ne peut pas être annulé", e.Status)
	}
	var txns []CreditTransaction
	if e.Status == StatusAccepted {
		service, err := a.store.GetService(ctx, e.ServiceID)
		if err != nil {
			return Exchange{}, err
		}
		txns = []CreditTransaction{{UserID: e.RequesterID, Montant: service.Credits, Type: TxnRefund}}
	}
	return a.store.TransitionExchange(ctx, id, e.Status, StatusCancelled, txns)
}

type role int

const (
	roleOwner role = iota
	roleParticipant
)

func (a *App) loadTransition(ctx context.Context, id, actorID int, required string, r role) (Exchange, error) {
	e, err := a.store.GetExchange(ctx, id)
	if err != nil {
		return Exchange{}, err
	}
	switch r {
	case roleOwner:
		if actorID != e.OwnerID {
			return Exchange{}, forbiddenError("seul l'offreur peut effectuer cette action")
		}
	case roleParticipant:
		if !isParticipant(e, actorID) {
			return Exchange{}, forbiddenError("vous n'êtes pas partie à cet échange")
		}
	}
	if e.Status != required {
		return Exchange{}, conflictError("l'échange est %q, attendu %q", e.Status, required)
	}
	return e, nil
}

func (a *App) CreateReview(ctx context.Context, authorID, exchangeID int, in Review) (Review, error) {
	e, err := a.store.GetExchange(ctx, exchangeID)
	if err != nil {
		return Review{}, err
	}
	if e.Status != StatusCompleted {
		return Review{}, validationError("seul un échange terminé peut être noté")
	}
	if !isParticipant(e, authorID) {
		return Review{}, forbiddenError("vous n'êtes pas partie à cet échange")
	}
	if in.Note < 1 || in.Note > 5 {
		return Review{}, validationError("la note doit être comprise entre 1 et 5")
	}
	exists, err := a.store.ReviewExists(ctx, exchangeID, authorID)
	if err != nil {
		return Review{}, err
	}
	if exists {
		return Review{}, validationError("vous avez déjà noté cet échange")
	}
	target := e.OwnerID
	if authorID == e.OwnerID {
		target = e.RequesterID
	}
	return a.store.CreateReview(ctx, Review{
		ExchangeID:  exchangeID,
		AuthorID:    authorID,
		TargetID:    target,
		Note:        in.Note,
		Commentaire: strings.TrimSpace(in.Commentaire),
	})
}

func (a *App) UserReviews(ctx context.Context, id int) ([]Review, error) {
	if err := a.mustExistUser(ctx, id); err != nil {
		return nil, err
	}
	return a.store.ReviewsByUser(ctx, id)
}

func (a *App) ServiceReviews(ctx context.Context, id int) ([]Review, error) {
	if _, err := a.store.GetService(ctx, id); err != nil {
		return nil, err
	}
	return a.store.ReviewsByService(ctx, id)
}

func (a *App) UserStats(ctx context.Context, id int) (UserStats, error) {
	return a.store.UserStats(ctx, id)
}

func (a *App) mustExistUser(ctx context.Context, id int) error {
	exists, err := a.store.UserExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return notFoundError("utilisateur %d introuvable", id)
	}
	return nil
}
