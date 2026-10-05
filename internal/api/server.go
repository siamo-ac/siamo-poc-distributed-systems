// Package api is the HTTP driving adapter. It exposes the CQRS split
// concretely: /commands/* hit the write side, /balance/* reads the
// projection, /admin/* inspects the log and rebuilds the read model.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/siamosystems/siamo-poc-distributed-systems/internal/events"
	"github.com/siamosystems/siamo-poc-distributed-systems/internal/read"
	"github.com/siamosystems/siamo-poc-distributed-systems/internal/write"
)

// Server wires HTTP routes to the write and read sides.
type Server struct {
	writes *write.Service
	reads  *read.Projection
	store  *events.Store
	mux    *http.ServeMux
}

// NewServer builds the HTTP adapter.
func NewServer(w *write.Service, r *read.Projection, s *events.Store) *Server {
	srv := &Server{writes: w, reads: r, store: s, mux: http.NewServeMux()}
	srv.mux.HandleFunc("GET /health", srv.health)
	srv.mux.HandleFunc("POST /commands/open-account", srv.openAccount)
	srv.mux.HandleFunc("POST /commands/deposit", srv.deposit)
	srv.mux.HandleFunc("POST /commands/withdraw", srv.withdraw)
	srv.mux.HandleFunc("GET /balance/{id}", srv.balance)
	srv.mux.HandleFunc("GET /admin/events", srv.eventLog)
	srv.mux.HandleFunc("POST /admin/replay", srv.replay)
	return srv
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "event-sourced-ledger", "status": "ok", "events": s.store.Count(),
	})
}

type openReq struct {
	Owner string `json:"owner"`
}

func (s *Server) openAccount(w http.ResponseWriter, r *http.Request) {
	var req openReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	id, err := s.writes.OpenAccount(req.Owner)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"account_id": id})
}

type moneyReq struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount_cents"`
}

func (s *Server) deposit(w http.ResponseWriter, r *http.Request) {
	var req moneyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := s.writes.Deposit(req.AccountID, req.Amount); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deposited"})
}

func (s *Server) withdraw(w http.ResponseWriter, r *http.Request) {
	var req moneyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := s.writes.Withdraw(req.AccountID, req.Amount); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "withdrawn"})
}

func (s *Server) balance(w http.ResponseWriter, r *http.Request) {
	b, err := s.reads.Get(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) eventLog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.All())
}

func (s *Server) replay(w http.ResponseWriter, _ *http.Request) {
	n := s.reads.Replay(s.store.All())
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "read model rebuilt from event log", "events_replayed": n,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
