package main

import "context"

// ServiceFilter holds the optional filters accepted by Store.ListServices.
type ServiceFilter struct {
	Categorie string
	Ville     string
	Search    string
}

// UserStore gives access to user accounts, their skills and their stats.
type UserStore interface {
	CreateUser(ctx context.Context, u User, welcome int) (User, error)
	GetUser(ctx context.Context, id int) (User, error)
	UpdateUser(ctx context.Context, u User) (User, error)
	UserExists(ctx context.Context, id int) (bool, error)
	GetSkills(ctx context.Context, userID int) ([]Skill, error)
	SetSkills(ctx context.Context, userID int, skills []Skill) error
	UserStats(ctx context.Context, id int) (UserStats, error)
}

// ServiceStore gives access to service listings.
type ServiceStore interface {
	CreateService(ctx context.Context, s Service) (Service, error)
	GetService(ctx context.Context, id int) (Service, error)
	UpdateService(ctx context.Context, s Service) (Service, error)
	DeleteService(ctx context.Context, id int) error
	ListServices(ctx context.Context, f ServiceFilter) ([]Service, error)
}

// ExchangeStore gives access to exchanges and to the credit log.
// TransitionExchange moves an exchange from one status to another and records
// the matching credit entries atomically: either both happen, or neither does.
type ExchangeStore interface {
	CreateExchange(ctx context.Context, e Exchange) (Exchange, error)
	GetExchange(ctx context.Context, id int) (Exchange, error)
	ListExchanges(ctx context.Context, userID int, status string) ([]Exchange, error)
	TransitionExchange(ctx context.Context, id int, from, to string, txns []CreditTransaction) (Exchange, error)
	Balance(ctx context.Context, userID int) (int, error)
}

// ReviewStore gives access to the reviews left on finished exchanges.
type ReviewStore interface {
	CreateReview(ctx context.Context, r Review) (Review, error)
	ReviewExists(ctx context.Context, exchangeID, authorID int) (bool, error)
	ReviewsByUser(ctx context.Context, targetID int) ([]Review, error)
	ReviewsByService(ctx context.Context, serviceID int) ([]Review, error)
}

// Store gives access to all stored data, independent of the storage engine.
// It composes one smaller interface per business area, so a caller that only
// needs reviews can depend on ReviewStore alone.
// SQLStore satisfies it with PostgreSQL in production; memStore satisfies it
// in tests.
type Store interface {
	UserStore
	ServiceStore
	ExchangeStore
	ReviewStore
}
