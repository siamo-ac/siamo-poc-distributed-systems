// Package write is the CQRS write side: it validates commands against the
// aggregate (rebuilt from the event store) and, when valid, appends new
// events. It never writes a "current balance" anywhere — the event log is
// the only thing it persists.
package write

import (
	"fmt"
	"sync/atomic"

	"github.com/siamosystems/siamo-poc-distributed-systems/internal/domain"
	"github.com/siamosystems/siamo-poc-distributed-systems/internal/events"
)

// Service handles commands. onAppended is called with each newly stored
// event so the read side can update its projection (in a real distributed
// system this would be a message bus; here it is a direct callback, and the
// README is honest about that simplification).
type Service struct {
	store      *events.Store
	onAppended func(events.Event)
	seq        atomic.Int64
}

// NewService wires the write side to the store and the read-side callback.
func NewService(store *events.Store, onAppended func(events.Event)) *Service {
	return &Service{store: store, onAppended: onAppended}
}

// OpenAccount validates and records an account opening.
func (s *Service) OpenAccount(owner string) (string, error) {
	if owner == "" {
		return "", fmt.Errorf("owner is required")
	}
	id := fmt.Sprintf("acct-%d", s.seq.Add(1))
	e := s.store.Append(events.Event{
		AggregateID: id,
		Type:        events.TypeAccountOpened,
		Owner:       owner,
	})
	s.onAppended(e)
	return id, nil
}

// Deposit validates and records a deposit.
func (s *Service) Deposit(accountID string, cents int64) error {
	return s.applyChecked(accountID, events.Event{
		AggregateID: accountID,
		Type:        events.TypeMoneyDeposited,
		Amount:      cents,
	})
}

// Withdraw validates and records a withdrawal.
func (s *Service) Withdraw(accountID string, cents int64) error {
	return s.applyChecked(accountID, events.Event{
		AggregateID: accountID,
		Type:        events.TypeMoneyWithdrawn,
		Amount:      cents,
	})
}

// applyChecked rebuilds the aggregate from its events, applies the new event
// to check the business rules, and only then appends it to the log.
func (s *Service) applyChecked(accountID string, e events.Event) error {
	history := s.store.For(accountID)
	if len(history) == 0 {
		return fmt.Errorf("account %s not found", accountID)
	}
	acct, err := domain.Rebuild(accountID, history)
	if err != nil {
		return err
	}
	if err := acct.Apply(e); err != nil {
		return fmt.Errorf("command rejected: %w", err)
	}
	stored := s.store.Append(e)
	s.onAppended(stored)
	return nil
}
