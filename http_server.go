package main

import (
	"context"
	"net/http"
)

// Server exposes the BarterSwap REST API on top of an App.
type Server struct {
	app *App
}

// NewServer builds the full http.Handler for the API: routes, then the
// middleware chain (recovery, logging, CORS, auth, timeout).
func NewServer(app *App) http.Handler {
	s := &Server{app: app}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.health)
	registerDocs(mux)

	mux.HandleFunc("POST /api/users", s.createUser)
	mux.HandleFunc("GET /api/users/{id}", s.getUser)
	mux.HandleFunc("PUT /api/users/{id}", s.updateUser)
	mux.HandleFunc("GET /api/users/{id}/skills", s.getUserSkills)
	mux.HandleFunc("PUT /api/users/{id}/skills", s.setUserSkills)
	mux.HandleFunc("GET /api/users/{id}/reviews", s.getUserReviews)
	mux.HandleFunc("GET /api/users/{id}/stats", s.getUserStats)

	mux.HandleFunc("GET /api/services", s.listServices)
	mux.HandleFunc("POST /api/services", s.createService)
	mux.HandleFunc("GET /api/services/{id}", s.getService)
	mux.HandleFunc("PUT /api/services/{id}", s.updateService)
	mux.HandleFunc("DELETE /api/services/{id}", s.deleteService)
	mux.HandleFunc("GET /api/services/{id}/reviews", s.getServiceReviews)

	mux.HandleFunc("POST /api/exchanges", s.createExchange)
	mux.HandleFunc("GET /api/exchanges", s.listExchanges)
	mux.HandleFunc("GET /api/exchanges/{id}", s.getExchange)
	mux.HandleFunc("PUT /api/exchanges/{id}/accept", s.acceptExchange)
	mux.HandleFunc("PUT /api/exchanges/{id}/reject", s.rejectExchange)
	mux.HandleFunc("PUT /api/exchanges/{id}/complete", s.completeExchange)
	mux.HandleFunc("PUT /api/exchanges/{id}/cancel", s.cancelExchange)
	mux.HandleFunc("POST /api/exchanges/{id}/review", s.createReview)

	return chain(mux, recovery, logging, cors, auth, timeout)
}

func (s *Server) currentUser(r *http.Request) (int, error) {
	id := userIDFromContext(r.Context())
	if id <= 0 {
		return 0, newError(ErrUnauthorized, "header X-UserID requis")
	}
	return id, nil
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req userRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	user, err := s.app.CreateUser(r.Context(), req.toUser())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	user, err := s.app.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	actor, err := s.currentUser(r)
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req userRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	user, err := s.app.UpdateUser(r.Context(), actor, id, req.toUser())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) getUserSkills(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	skills, err := s.app.GetSkills(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

func (s *Server) setUserSkills(w http.ResponseWriter, r *http.Request) {
	actor, err := s.currentUser(r)
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var skills []Skill
	if err := decodeJSON(r, &skills); err != nil {
		writeError(w, err)
		return
	}
	saved, err := s.app.SetSkills(r.Context(), actor, id, skills)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) getUserReviews(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	reviews, err := s.app.UserReviews(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (s *Server) getUserStats(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	stats, err := s.app.UserStats(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) listServices(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := ServiceFilter{
		Categorie: q.Get("categorie"),
		Ville:     q.Get("ville"),
		Search:    q.Get("search"),
	}
	services, err := s.app.ListServices(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, services)
}

func (s *Server) createService(w http.ResponseWriter, r *http.Request) {
	actor, err := s.currentUser(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req serviceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	service, err := s.app.CreateService(r.Context(), actor, req.toService())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, service)
}

func (s *Server) getService(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	service, err := s.app.GetService(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, service)
}

func (s *Server) updateService(w http.ResponseWriter, r *http.Request) {
	actor, err := s.currentUser(r)
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req serviceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	service, err := s.app.UpdateService(r.Context(), actor, id, req.toService())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, service)
}

func (s *Server) deleteService(w http.ResponseWriter, r *http.Request) {
	actor, err := s.currentUser(r)
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.app.DeleteService(r.Context(), actor, id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) getServiceReviews(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	reviews, err := s.app.ServiceReviews(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (s *Server) createExchange(w http.ResponseWriter, r *http.Request) {
	actor, err := s.currentUser(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req exchangeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	exchange, err := s.app.CreateExchange(r.Context(), actor, req.ServiceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, exchange)
}

func (s *Server) listExchanges(w http.ResponseWriter, r *http.Request) {
	actor, err := s.currentUser(r)
	if err != nil {
		writeError(w, err)
		return
	}
	exchanges, err := s.app.ListExchanges(r.Context(), actor, r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exchanges)
}

func (s *Server) getExchange(w http.ResponseWriter, r *http.Request) {
	actor, err := s.currentUser(r)
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	exchange, err := s.app.GetExchange(r.Context(), actor, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exchange)
}

func (s *Server) acceptExchange(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, s.app.AcceptExchange)
}

func (s *Server) rejectExchange(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, s.app.RejectExchange)
}

func (s *Server) completeExchange(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, s.app.CompleteExchange)
}

func (s *Server) cancelExchange(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, s.app.CancelExchange)
}

func (s *Server) transition(w http.ResponseWriter, r *http.Request, action func(ctx context.Context, actorID, id int) (Exchange, error)) {
	actor, err := s.currentUser(r)
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	exchange, err := action(r.Context(), actor, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exchange)
}

func (s *Server) createReview(w http.ResponseWriter, r *http.Request) {
	actor, err := s.currentUser(r)
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req reviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	review, err := s.app.CreateReview(r.Context(), actor, id, Review{Note: req.Note, Commentaire: req.Commentaire})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, review)
}

type userRequest struct {
	Pseudo string `json:"pseudo"`
	Bio    string `json:"bio"`
	Ville  string `json:"ville"`
}

func (req userRequest) toUser() User {
	return User{
		Pseudo: req.Pseudo,
		Bio:    req.Bio,
		Ville:  req.Ville,
	}
}

type serviceRequest struct {
	Titre        string `json:"titre"`
	Description  string `json:"description"`
	Categorie    string `json:"categorie"`
	DureeMinutes int    `json:"duree_minutes"`
	Credits      int    `json:"credits"`
	Ville        string `json:"ville"`
	Actif        *bool  `json:"actif"`
}

func (req serviceRequest) toService() Service {
	actif := true
	if req.Actif != nil {
		actif = *req.Actif
	}
	return Service{
		Titre:        req.Titre,
		Description:  req.Description,
		Categorie:    req.Categorie,
		DureeMinutes: req.DureeMinutes,
		Credits:      req.Credits,
		Ville:        req.Ville,
		Actif:        actif,
	}
}

type exchangeRequest struct {
	ServiceID int `json:"service_id"`
}

type reviewRequest struct {
	Note        int    `json:"note"`
	Commentaire string `json:"commentaire"`
}
