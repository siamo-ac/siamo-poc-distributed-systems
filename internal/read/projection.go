// Package read is the CQRS read side: a projection that turns the event log
// into a queryable "current state" view (account balances). It owns no
// business rules and accepts no commands; it only folds events into its
// view. It can be thrown away and rebuilt from the log at any time — that
// is the point of the /admin/replay endpoint.
package read

import (
	"fmt"
	"sync"

	"github.com/siamosystems/siamo-poc-distributed-systems/internal/events"
)

// Balance is the read model's view of one account.
type Balance struct {
	AccountID string `json:"account_id"`
	Owner     string `json:"owner"`
	Balance   int64  `json:"balance_cents"`
	Events    int    `json:"events_applied"`
}

// Projection is the read model: account id -> current balance view.
type Projection struct {
	mu       sync.RWMutex
	balances map[string]*Balance
}

// NewProjection returns an empty read model.
func NewProjection() *Projection {
	return &Projection{balances: map[string]*Balance{}}
}

// Apply folds one event into the projection.
func (p *Projection) Apply(e events.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	b, ok := p.balances[e.AggregateID]
	if !ok {
		b = &Balance{AccountID: e.AggregateID}
		p.balances[e.AggregateID] = b
	}
	switch e.Type {
	case events.TypeAccountOpened:
		b.Owner = e.Owner
	case events.TypeMoneyDeposited:
		b.Balance += e.Amount
	case events.TypeMoneyWithdrawn:
		b.Balance -= e.Amount
	}
	b.Events++
}

// Get returns the projected balance for an account.
func (p *Projection) Get(accountID string) (Balance, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	b, ok := p.balances[accountID]
	if !ok {
		return Balance{}, fmt.Errorf("account %s not found in read model", accountID)
	}
	return *b, nil
}

// Reset wipes the projection. The next Replay rebuilds it from scratch.
func (p *Projection) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.balances = map[string]*Balance{}
}

// Replay rebuilds the entire read model from a full event history.
func (p *Projection) Replay(history []events.Event) int {
	p.Reset()
	for _, e := range history {
		p.Apply(e)
	}
	return len(history)
}
