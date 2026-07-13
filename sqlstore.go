package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migration: %w", err)
	}
	return nil
}

func (s *SQLStore) Close() error { return s.db.Close() }

func ts(t time.Time) string { return t.Format(time.RFC3339) }

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

func (s *SQLStore) CreateUser(ctx context.Context, u User, welcome int) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	var created time.Time
	err = tx.QueryRowContext(ctx,
		`INSERT INTO users (pseudo, bio, ville) VALUES ($1, $2, $3) RETURNING id, created_at`,
		u.Pseudo, u.Bio, u.Ville,
	).Scan(&u.ID, &created)
	if err != nil {
		return User{}, err
	}

	if _, err = tx.ExecContext(ctx,
		`INSERT INTO credit_transactions (user_id, exchange_id, montant, type) VALUES ($1, NULL, $2, $3)`,
		u.ID, welcome, TxnEarn,
	); err != nil {
		return User{}, err
	}

	if err = tx.Commit(); err != nil {
		return User{}, err
	}

	u.CreditBalance = welcome
	u.CreatedAt = ts(created)
	u.Skills = nil
	return u, nil
}

func (s *SQLStore) GetUser(ctx context.Context, id int) (User, error) {
	var u User
	var created time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT id, pseudo, bio, ville, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Pseudo, &u.Bio, &u.Ville, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, notFoundError("utilisateur %d introuvable", id)
	}
	if err != nil {
		return User{}, err
	}
	u.CreatedAt = ts(created)

	if u.Skills, err = s.GetSkills(ctx, id); err != nil {
		return User{}, err
	}
	if u.CreditBalance, err = s.Balance(ctx, id); err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *SQLStore) UpdateUser(ctx context.Context, u User) (User, error) {
	var created time.Time
	err := s.db.QueryRowContext(ctx,
		`UPDATE users SET pseudo = $1, bio = $2, ville = $3 WHERE id = $4 RETURNING created_at`,
		u.Pseudo, u.Bio, u.Ville, u.ID,
	).Scan(&created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, notFoundError("utilisateur %d introuvable", u.ID)
	}
	if err != nil {
		return User{}, err
	}
	u.CreatedAt = ts(created)
	if u.Skills, err = s.GetSkills(ctx, u.ID); err != nil {
		return User{}, err
	}
	if u.CreditBalance, err = s.Balance(ctx, u.ID); err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *SQLStore) UserExists(ctx context.Context, id int) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

func (s *SQLStore) GetSkills(ctx context.Context, userID int) ([]Skill, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT nom, niveau FROM skills WHERE user_id = $1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var sk Skill
		if err := rows.Scan(&sk.Nom, &sk.Niveau); err != nil {
			return nil, err
		}
		skills = append(skills, sk)
	}
	return skills, rows.Err()
}

func (s *SQLStore) SetSkills(ctx context.Context, userID int, skills []Skill) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `DELETE FROM skills WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, sk := range skills {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO skills (user_id, nom, niveau) VALUES ($1, $2, $3)`,
			userID, sk.Nom, sk.Niveau,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLStore) CreateService(ctx context.Context, svc Service) (Service, error) {
	var created time.Time
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville, actif)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, created_at`,
		svc.ProviderID, svc.Titre, svc.Description, svc.Categorie,
		svc.DureeMinutes, svc.Credits, svc.Ville, svc.Actif,
	).Scan(&svc.ID, &created)
	if err != nil {
		return Service{}, err
	}
	svc.CreatedAt = ts(created)
	return svc, nil
}

func (s *SQLStore) GetService(ctx context.Context, id int) (Service, error) {
	var svc Service
	var created time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT id, provider_id, titre, description, categorie, duree_minutes, credits, ville, actif, created_at
		 FROM services WHERE id = $1`, id,
	).Scan(&svc.ID, &svc.ProviderID, &svc.Titre, &svc.Description, &svc.Categorie,
		&svc.DureeMinutes, &svc.Credits, &svc.Ville, &svc.Actif, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Service{}, notFoundError("service %d introuvable", id)
	}
	if err != nil {
		return Service{}, err
	}
	svc.CreatedAt = ts(created)
	return svc, nil
}

func (s *SQLStore) UpdateService(ctx context.Context, svc Service) (Service, error) {
	var created time.Time
	err := s.db.QueryRowContext(ctx,
		`UPDATE services SET titre = $1, description = $2, categorie = $3, duree_minutes = $4,
		 credits = $5, ville = $6, actif = $7 WHERE id = $8 RETURNING provider_id, created_at`,
		svc.Titre, svc.Description, svc.Categorie, svc.DureeMinutes,
		svc.Credits, svc.Ville, svc.Actif, svc.ID,
	).Scan(&svc.ProviderID, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Service{}, notFoundError("service %d introuvable", svc.ID)
	}
	if err != nil {
		return Service{}, err
	}
	svc.CreatedAt = ts(created)
	return svc, nil
}

func (s *SQLStore) DeleteService(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM services WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFoundError("service %d introuvable", id)
	}
	return nil
}

func (s *SQLStore) ListServices(ctx context.Context, f ServiceFilter) ([]Service, error) {
	var conds []string
	var args []any
	i := 1

	if f.Categorie != "" {
		conds = append(conds, fmt.Sprintf("categorie = $%d", i))
		args = append(args, f.Categorie)
		i++
	}
	if f.Ville != "" {
		conds = append(conds, fmt.Sprintf("ville ILIKE $%d", i))
		args = append(args, f.Ville)
		i++
	}
	if f.Search != "" {
		conds = append(conds, fmt.Sprintf("(titre ILIKE $%d OR description ILIKE $%d)", i, i))
		args = append(args, "%"+f.Search+"%")
		i++
	}

	query := `SELECT id, provider_id, titre, description, categorie, duree_minutes, credits, ville, actif, created_at FROM services`
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY created_at DESC, id DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	services := []Service{}
	for rows.Next() {
		var svc Service
		var created time.Time
		if err := rows.Scan(&svc.ID, &svc.ProviderID, &svc.Titre, &svc.Description, &svc.Categorie,
			&svc.DureeMinutes, &svc.Credits, &svc.Ville, &svc.Actif, &created); err != nil {
			return nil, err
		}
		svc.CreatedAt = ts(created)
		services = append(services, svc)
	}
	return services, rows.Err()
}
