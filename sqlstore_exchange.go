package main

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *SQLStore) CreateExchange(ctx context.Context, e Exchange) (Exchange, error) {
	var created, updated time.Time
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO exchanges (service_id, requester_id, owner_id, status)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`,
		e.ServiceID, e.RequesterID, e.OwnerID, StatusPending,
	).Scan(&e.ID, &created, &updated)
	if isUniqueViolation(err) {
		return Exchange{}, conflictError("le service %d a déjà un échange en cours", e.ServiceID)
	}
	if err != nil {
		return Exchange{}, err
	}
	e.Status = StatusPending
	e.CreatedAt = ts(created)
	e.UpdatedAt = ts(updated)
	return e, nil
}

func (s *SQLStore) GetExchange(ctx context.Context, id int) (Exchange, error) {
	e, err := scanExchange(s.db.QueryRowContext(ctx,
		`SELECT id, service_id, requester_id, owner_id, status, created_at, updated_at
		 FROM exchanges WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Exchange{}, notFoundError("échange %d introuvable", id)
	}
	return e, err
}

func (s *SQLStore) ListExchanges(ctx context.Context, userID int, status string) ([]Exchange, error) {
	query := `SELECT id, service_id, requester_id, owner_id, status, created_at, updated_at
	          FROM exchanges WHERE (requester_id = $1 OR owner_id = $1)`
	args := []any{userID}
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC, id DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exchanges := []Exchange{}
	for rows.Next() {
		e, err := scanExchange(rows)
		if err != nil {
			return nil, err
		}
		exchanges = append(exchanges, e)
	}
	return exchanges, rows.Err()
}

func (s *SQLStore) TransitionExchange(ctx context.Context, id int, from, to string, txns []CreditTransaction) (Exchange, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Exchange{}, err
	}
	defer tx.Rollback()

	e, err := scanExchange(tx.QueryRowContext(ctx,
		`UPDATE exchanges SET status = $1, updated_at = now()
		 WHERE id = $2 AND status = $3
		 RETURNING id, service_id, requester_id, owner_id, status, created_at, updated_at`,
		to, id, from))
	if errors.Is(err, sql.ErrNoRows) {
		return Exchange{}, s.transitionConflict(ctx, id, from)
	}
	if err != nil {
		return Exchange{}, err
	}

	for _, t := range txns {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO credit_transactions (user_id, exchange_id, montant, type) VALUES ($1, $2, $3, $4)`,
			t.UserID, id, t.Montant, t.Type,
		); err != nil {
			return Exchange{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return Exchange{}, err
	}
	return e, nil
}

func (s *SQLStore) transitionConflict(ctx context.Context, id int, from string) error {
	exists, err := s.exchangeExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return notFoundError("échange %d introuvable", id)
	}
	return conflictError("l'échange %d n'est pas dans l'état %q", id, from)
}

func (s *SQLStore) exchangeExists(ctx context.Context, id int) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM exchanges WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

func (s *SQLStore) Balance(ctx context.Context, userID int) (int, error) {
	var balance int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(montant), 0) FROM credit_transactions WHERE user_id = $1`, userID,
	).Scan(&balance)
	return balance, err
}

func (s *SQLStore) CreateReview(ctx context.Context, r Review) (Review, error) {
	var created time.Time
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO reviews (exchange_id, author_id, target_id, note, commentaire)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		r.ExchangeID, r.AuthorID, r.TargetID, r.Note, r.Commentaire,
	).Scan(&r.ID, &created)
	if isUniqueViolation(err) {
		return Review{}, conflictError("un avis existe déjà pour cet échange")
	}
	if err != nil {
		return Review{}, err
	}
	r.CreatedAt = ts(created)
	return r, nil
}

func (s *SQLStore) ReviewExists(ctx context.Context, exchangeID, authorID int) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM reviews WHERE exchange_id = $1 AND author_id = $2)`,
		exchangeID, authorID,
	).Scan(&exists)
	return exists, err
}

func (s *SQLStore) ReviewsByUser(ctx context.Context, targetID int) ([]Review, error) {
	return s.queryReviews(ctx,
		`SELECT id, exchange_id, author_id, target_id, note, commentaire, created_at
		 FROM reviews WHERE target_id = $1 ORDER BY created_at DESC, id DESC`, targetID)
}

func (s *SQLStore) ReviewsByService(ctx context.Context, serviceID int) ([]Review, error) {
	return s.queryReviews(ctx,
		`SELECT r.id, r.exchange_id, r.author_id, r.target_id, r.note, r.commentaire, r.created_at
		 FROM reviews r JOIN exchanges e ON e.id = r.exchange_id
		 WHERE e.service_id = $1 ORDER BY r.created_at DESC, r.id DESC`, serviceID)
}

func (s *SQLStore) queryReviews(ctx context.Context, query string, args ...any) ([]Review, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reviews := []Review{}
	for rows.Next() {
		var r Review
		var created time.Time
		if err := rows.Scan(&r.ID, &r.ExchangeID, &r.AuthorID, &r.TargetID, &r.Note, &r.Commentaire, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = ts(created)
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}

func (s *SQLStore) UserStats(ctx context.Context, id int) (UserStats, error) {
	exists, err := s.UserExists(ctx, id)
	if err != nil {
		return UserStats{}, err
	}
	if !exists {
		return UserStats{}, notFoundError("utilisateur %d introuvable", id)
	}

	stats := UserStats{UserID: id}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM services WHERE provider_id = $1 AND actif = TRUE`, id,
	).Scan(&stats.ServicesActifs); err != nil {
		return UserStats{}, err
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exchanges WHERE (requester_id = $1 OR owner_id = $1) AND status = 'completed'`, id,
	).Scan(&stats.EchangesCompletes); err != nil {
		return UserStats{}, err
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(AVG(note), 0), COUNT(*) FROM reviews WHERE target_id = $1`, id,
	).Scan(&stats.NoteMoyenne, &stats.NbAvis); err != nil {
		return UserStats{}, err
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT
		   COALESCE(SUM(montant) FILTER (WHERE type = 'earn'), 0),
		   COALESCE(SUM(-montant) FILTER (WHERE type = 'spend'), 0),
		   COALESCE(SUM(montant), 0)
		 FROM credit_transactions WHERE user_id = $1`, id,
	).Scan(&stats.TotalGagne, &stats.TotalDepense, &stats.CreditBalance); err != nil {
		return UserStats{}, err
	}

	return stats, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanExchange(row rowScanner) (Exchange, error) {
	var e Exchange
	var created, updated time.Time
	if err := row.Scan(&e.ID, &e.ServiceID, &e.RequesterID, &e.OwnerID, &e.Status, &created, &updated); err != nil {
		return Exchange{}, err
	}
	e.CreatedAt = ts(created)
	e.UpdatedAt = ts(updated)
	return e, nil
}
