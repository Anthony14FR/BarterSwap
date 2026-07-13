package main

import "context"

// ServiceFilter holds the optional filters accepted by Store.ListServices.
type ServiceFilter struct {
	Categorie string
	Ville     string
	Search    string
}

// Store gives access to all stored data, independent of the storage engine.
// SQLStore satisfies it with PostgreSQL in production; memStore satisfies it
// in tests.
type Store interface {
	CreateUser(ctx context.Context, u User, welcome int) (User, error)
	GetUser(ctx context.Context, id int) (User, error)
	UpdateUser(ctx context.Context, u User) (User, error)
	UserExists(ctx context.Context, id int) (bool, error)
	GetSkills(ctx context.Context, userID int) ([]Skill, error)
	SetSkills(ctx context.Context, userID int, skills []Skill) error

	CreateService(ctx context.Context, s Service) (Service, error)
	GetService(ctx context.Context, id int) (Service, error)
	UpdateService(ctx context.Context, s Service) (Service, error)
	DeleteService(ctx context.Context, id int) error
	ListServices(ctx context.Context, f ServiceFilter) ([]Service, error)

	CreateExchange(ctx context.Context, e Exchange) (Exchange, error)
	GetExchange(ctx context.Context, id int) (Exchange, error)
	ListExchanges(ctx context.Context, userID int, status string) ([]Exchange, error)
	TransitionExchange(ctx context.Context, id int, from, to string, txns []CreditTransaction) (Exchange, error)

	Balance(ctx context.Context, userID int) (int, error)

	CreateReview(ctx context.Context, r Review) (Review, error)
	ReviewExists(ctx context.Context, exchangeID, authorID int) (bool, error)
	ReviewsByUser(ctx context.Context, targetID int) ([]Review, error)
	ReviewsByService(ctx context.Context, serviceID int) ([]Review, error)

	UserStats(ctx context.Context, id int) (UserStats, error)

	Close() error
}
