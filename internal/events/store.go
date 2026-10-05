// Package events is the event store: the single source of truth. It is an
// append-only log of everything that ever happened, held in memory for this
// POC. Nothing ever updates or deletes an event; the current state of any
// aggregate is derived by replaying its events.
package events

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Event types in this POC's bank-account domain.
const (
	TypeAccountOpened  = "AccountOpened"
	TypeMoneyDeposited = "MoneyDeposited"
	TypeMoneyWithdrawn = "MoneyWithdrawn"
)

// Event is one immutable fact: something happened to an aggregate.
type Event struct {
	Seq         int64     `json:"seq"`          // global position in the log
	ID          string    `json:"id"`           // unique event id
	AggregateID string    `json:"aggregate_id"` // which account this belongs to
	Type        string    `json:"type"`
	Amount      int64     `json:"amount,omitempty"` // cents, for deposit/withdraw
	Owner       string    `json:"owner,omitempty"`  // for AccountOpened
	At          time.Time `json:"at"`
}

// Store is the append-only event log.
type Store struct {
	mu     sync.Mutex
	events []Event
	seq    atomic.Int64
}

// NewStore returns an empty event store.
func NewStore() *Store { return &Store{} }

// Append records a new event at the end of the log, assigning its sequence
// number. This is the only write operation the store supports.
func (s *Store) Append(e Event) Event {
	e.Seq = s.seq.Add(1)
	e.ID = fmt.Sprintf("evt-%d", e.Seq)
	e.At = time.Now().UTC()
	s.mu.Lock()
	s.events = append(s.events, e)
	s.mu.Unlock()
	return e
}

// All returns a copy of the full log in order.
func (s *Store) All() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

// For returns the events of one aggregate in order.
func (s *Store) For(aggregateID string) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Event
	for _, e := range s.events {
		if e.AggregateID == aggregateID {
			out = append(out, e)
		}
	}
	return out
}

// Count returns the number of stored events.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}
