package main

// User is a user account and its public profile.
type User struct {
	ID            int     `json:"id"`
	Pseudo        string  `json:"pseudo"`
	Bio           string  `json:"bio,omitempty"`
	Ville         string  `json:"ville,omitempty"`
	Skills        []Skill `json:"skills,omitempty"`
	CreditBalance int     `json:"credit_balance"`
	CreatedAt     string  `json:"created_at"`
}

// Skill is a skill a user has, with its level.
type Skill struct {
	Nom    string `json:"nom"`
	Niveau string `json:"niveau"`
}

// Service is a listing posted by a user to offer help in one of their skills.
type Service struct {
	ID           int    `json:"id"`
	ProviderID   int    `json:"provider_id"`
	Titre        string `json:"titre"`
	Description  string `json:"description,omitempty"`
	Categorie    string `json:"categorie"`
	DureeMinutes int    `json:"duree_minutes"`
	Credits      int    `json:"credits"`
	Ville        string `json:"ville,omitempty"`
	Actif        bool   `json:"actif"`
	CreatedAt    string `json:"created_at"`
}

// Exchange is a request on a service. Requester is who asks for it, Owner is
// who offers the service.
type Exchange struct {
	ID          int    `json:"id"`
	ServiceID   int    `json:"service_id"`
	RequesterID int    `json:"requester_id"`
	OwnerID     int    `json:"owner_id"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// CreditTransaction is one entry in a user's credit log. Montant is positive
// for a credit and negative for a debit. A user's balance is the sum of all entries.
type CreditTransaction struct {
	ID         int    `json:"id"`
	UserID     int    `json:"user_id"`
	ExchangeID int    `json:"exchange_id"`
	Montant    int    `json:"montant"`
	Type       string `json:"type"`
	CreatedAt  string `json:"created_at"`
}

// Review is a rating left by one participant about a finished exchange.
type Review struct {
	ID          int    `json:"id"`
	ExchangeID  int    `json:"exchange_id"`
	AuthorID    int    `json:"author_id"`
	TargetID    int    `json:"target_id"`
	Note        int    `json:"note"`
	Commentaire string `json:"commentaire,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// UserStats is a summary of a user's activity.
type UserStats struct {
	UserID            int     `json:"user_id"`
	ServicesActifs    int     `json:"services_actifs"`
	EchangesCompletes int     `json:"echanges_completes"`
	CreditBalance     int     `json:"credit_balance"`
	NoteMoyenne       float64 `json:"note_moyenne"`
	NbAvis            int     `json:"nb_avis"`
	TotalGagne        int     `json:"total_gagne"`
	TotalDepense      int     `json:"total_depense"`
}

const welcomeCredits = 10

// Status values for the life cycle of an Exchange.
const (
	StatusPending   = "pending"
	StatusAccepted  = "accepted"
	StatusRejected  = "rejected"
	StatusCancelled = "cancelled"
	StatusCompleted = "completed"
)

// Type values for a CreditTransaction entry.
const (
	TxnEarn   = "earn"
	TxnSpend  = "spend"
	TxnRefund = "refund"
)

var categories = map[string]bool{
	"Informatique": true,
	"Jardinage":    true,
	"Bricolage":    true,
	"Cuisine":      true,
	"Musique":      true,
	"Langues":      true,
	"Sport":        true,
	"Tutorat":      true,
	"Déménagement": true,
	"Photographie": true,
	"Animalier":    true,
	"Couture":      true,
	"Autre":        true,
}

var niveaux = map[string]bool{
	"débutant":      true,
	"intermédiaire": true,
	"expert":        true,
}

func isValidCategory(c string) bool { return categories[c] }

func isValidNiveau(n string) bool { return niveaux[n] }

func isValidStatus(s string) bool {
	switch s {
	case StatusPending, StatusAccepted, StatusRejected, StatusCancelled, StatusCompleted:
		return true
	default:
		return false
	}
}
