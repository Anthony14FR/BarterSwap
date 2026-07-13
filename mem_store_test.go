package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type memStore struct {
	users     map[int]User
	skills    map[int][]Skill
	services  map[int]Service
	exchanges map[int]Exchange
	txns      []CreditTransaction
	reviews   []Review

	seqUser, seqService, seqExchange, seqTxn, seqReview int
	clock                                               int
}

func newMemStore() *memStore {
	return &memStore{
		users:     map[int]User{},
		skills:    map[int][]Skill{},
		services:  map[int]Service{},
		exchanges: map[int]Exchange{},
	}
}

func (m *memStore) tick() string {
	m.clock++
	return fmt.Sprintf("2026-01-01T00:%02d:%02dZ", m.clock/60, m.clock%60)
}

func (m *memStore) Close() error { return nil }

func (m *memStore) CreateUser(_ context.Context, u User, welcome int) (User, error) {
	m.seqUser++
	u.ID = m.seqUser
	u.CreatedAt = m.tick()
	u.Skills = nil
	m.users[u.ID] = u
	m.seqTxn++
	m.txns = append(m.txns, CreditTransaction{ID: m.seqTxn, UserID: u.ID, Montant: welcome, Type: TxnEarn})
	u.CreditBalance = welcome
	return u, nil
}

func (m *memStore) GetUser(ctx context.Context, id int) (User, error) {
	u, ok := m.users[id]
	if !ok {
		return User{}, notFoundError("utilisateur %d introuvable", id)
	}
	u.Skills = append([]Skill(nil), m.skills[id]...)
	u.CreditBalance, _ = m.Balance(ctx, id)
	return u, nil
}

func (m *memStore) UpdateUser(ctx context.Context, u User) (User, error) {
	existing, ok := m.users[u.ID]
	if !ok {
		return User{}, notFoundError("utilisateur %d introuvable", u.ID)
	}
	existing.Pseudo = u.Pseudo
	existing.Bio = u.Bio
	existing.Ville = u.Ville
	m.users[u.ID] = existing
	return m.GetUser(ctx, u.ID)
}

func (m *memStore) UserExists(_ context.Context, id int) (bool, error) {
	_, ok := m.users[id]
	return ok, nil
}

func (m *memStore) GetSkills(_ context.Context, userID int) ([]Skill, error) {
	return append([]Skill(nil), m.skills[userID]...), nil
}

func (m *memStore) SetSkills(_ context.Context, userID int, skills []Skill) error {
	m.skills[userID] = append([]Skill(nil), skills...)
	return nil
}

func (m *memStore) CreateService(_ context.Context, svc Service) (Service, error) {
	m.seqService++
	svc.ID = m.seqService
	svc.CreatedAt = m.tick()
	m.services[svc.ID] = svc
	return svc, nil
}

func (m *memStore) GetService(_ context.Context, id int) (Service, error) {
	svc, ok := m.services[id]
	if !ok {
		return Service{}, notFoundError("service %d introuvable", id)
	}
	return svc, nil
}

func (m *memStore) UpdateService(_ context.Context, svc Service) (Service, error) {
	existing, ok := m.services[svc.ID]
	if !ok {
		return Service{}, notFoundError("service %d introuvable", svc.ID)
	}
	svc.ProviderID = existing.ProviderID
	svc.CreatedAt = existing.CreatedAt
	m.services[svc.ID] = svc
	return svc, nil
}

func (m *memStore) DeleteService(_ context.Context, id int) error {
	if _, ok := m.services[id]; !ok {
		return notFoundError("service %d introuvable", id)
	}
	delete(m.services, id)
	return nil
}

func (m *memStore) ListServices(_ context.Context, f ServiceFilter) ([]Service, error) {
	out := []Service{}
	for _, svc := range m.services {
		if f.Categorie != "" && svc.Categorie != f.Categorie {
			continue
		}
		if f.Ville != "" && !strings.EqualFold(svc.Ville, f.Ville) {
			continue
		}
		if f.Search != "" {
			hay := strings.ToLower(svc.Titre + " " + svc.Description)
			if !strings.Contains(hay, strings.ToLower(f.Search)) {
				continue
			}
		}
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *memStore) hasActiveExchange(serviceID int) bool {
	for _, e := range m.exchanges {
		if e.ServiceID == serviceID && (e.Status == StatusPending || e.Status == StatusAccepted) {
			return true
		}
	}
	return false
}

func (m *memStore) CreateExchange(_ context.Context, e Exchange) (Exchange, error) {
	if m.hasActiveExchange(e.ServiceID) {
		return Exchange{}, conflictError("le service %d a déjà un échange en cours", e.ServiceID)
	}
	m.seqExchange++
	e.ID = m.seqExchange
	e.Status = StatusPending
	e.CreatedAt = m.tick()
	e.UpdatedAt = e.CreatedAt
	m.exchanges[e.ID] = e
	return e, nil
}

func (m *memStore) GetExchange(_ context.Context, id int) (Exchange, error) {
	e, ok := m.exchanges[id]
	if !ok {
		return Exchange{}, notFoundError("échange %d introuvable", id)
	}
	return e, nil
}

func (m *memStore) ListExchanges(_ context.Context, userID int, status string) ([]Exchange, error) {
	out := []Exchange{}
	for _, e := range m.exchanges {
		if e.RequesterID != userID && e.OwnerID != userID {
			continue
		}
		if status != "" && e.Status != status {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *memStore) TransitionExchange(_ context.Context, id int, from, to string, txns []CreditTransaction) (Exchange, error) {
	e, ok := m.exchanges[id]
	if !ok {
		return Exchange{}, notFoundError("échange %d introuvable", id)
	}
	if e.Status != from {
		return Exchange{}, conflictError("l'échange %d n'est pas dans l'état %q", id, from)
	}
	e.Status = to
	e.UpdatedAt = m.tick()
	m.exchanges[id] = e
	for _, t := range txns {
		m.seqTxn++
		t.ID = m.seqTxn
		t.ExchangeID = id
		m.txns = append(m.txns, t)
	}
	return e, nil
}

func (m *memStore) Balance(_ context.Context, userID int) (int, error) {
	total := 0
	for _, t := range m.txns {
		if t.UserID == userID {
			total += t.Montant
		}
	}
	return total, nil
}

func (m *memStore) CreateReview(_ context.Context, r Review) (Review, error) {
	for _, existing := range m.reviews {
		if existing.ExchangeID == r.ExchangeID && existing.AuthorID == r.AuthorID {
			return Review{}, conflictError("un avis existe déjà pour cet échange")
		}
	}
	m.seqReview++
	r.ID = m.seqReview
	r.CreatedAt = m.tick()
	m.reviews = append(m.reviews, r)
	return r, nil
}

func (m *memStore) ReviewExists(_ context.Context, exchangeID, authorID int) (bool, error) {
	for _, r := range m.reviews {
		if r.ExchangeID == exchangeID && r.AuthorID == authorID {
			return true, nil
		}
	}
	return false, nil
}

func (m *memStore) ReviewsByUser(_ context.Context, targetID int) ([]Review, error) {
	out := []Review{}
	for _, r := range m.reviews {
		if r.TargetID == targetID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *memStore) ReviewsByService(_ context.Context, serviceID int) ([]Review, error) {
	out := []Review{}
	for _, r := range m.reviews {
		if e, ok := m.exchanges[r.ExchangeID]; ok && e.ServiceID == serviceID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *memStore) UserStats(ctx context.Context, id int) (UserStats, error) {
	if _, ok := m.users[id]; !ok {
		return UserStats{}, notFoundError("utilisateur %d introuvable", id)
	}
	stats := UserStats{UserID: id}
	for _, svc := range m.services {
		if svc.ProviderID == id && svc.Actif {
			stats.ServicesActifs++
		}
	}
	for _, e := range m.exchanges {
		if (e.RequesterID == id || e.OwnerID == id) && e.Status == StatusCompleted {
			stats.EchangesCompletes++
		}
	}
	sum := 0
	for _, r := range m.reviews {
		if r.TargetID == id {
			sum += r.Note
			stats.NbAvis++
		}
	}
	if stats.NbAvis > 0 {
		stats.NoteMoyenne = float64(sum) / float64(stats.NbAvis)
	}
	for _, t := range m.txns {
		if t.UserID != id {
			continue
		}
		switch t.Type {
		case TxnEarn:
			stats.TotalGagne += t.Montant
		case TxnSpend:
			stats.TotalDepense += -t.Montant
		}
	}
	stats.CreditBalance, _ = m.Balance(ctx, id)
	return stats, nil
}
