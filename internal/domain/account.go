// Package domain holds the bank-account aggregate: the write model's view
// of one account, rebuilt by applying that account's events. It carries the
// business rules (no negative deposits, no overdrafts).
package domain

import (
	"fmt"

	"github.com/siamosystems/siamo-poc-distributed-systems/internal/events"
)

// Account is the write-model aggregate.
type Account struct {
	ID      string
	Owner   string
	Balance int64 // cents
	Open    bool
}

// Apply folds one event into the account. This is the only way an account
// changes: there are no direct field mutations from command handlers.
func (a *Account) Apply(e events.Event) error {
	switch e.Type {
	case events.TypeAccountOpened:
		if a.Open {
			return fmt.Errorf("account %s already opened", a.ID)
		}
		a.Open = true
		a.Owner = e.Owner
	case events.TypeMoneyDeposited:
		if e.Amount <= 0 {
			return fmt.Errorf("deposit must be positive")
		}
		a.Balance += e.Amount
	case events.TypeMoneyWithdrawn:
		if e.Amount <= 0 {
			return fmt.Errorf("withdrawal must be positive")
		}
		if e.Amount > a.Balance {
			return fmt.Errorf("insufficient funds: balance %d, tried %d", a.Balance, e.Amount)
		}
		a.Balance -= e.Amount
	default:
		return fmt.Errorf("unknown event type %q", e.Type)
	}
	return nil
}

// Rebuild replays a full event history into a fresh account.
func Rebuild(id string, history []events.Event) (*Account, error) {
	a := &Account{ID: id}
	for _, e := range history {
		if err := a.Apply(e); err != nil {
			return nil, err
		}
	}
	return a, nil
}
