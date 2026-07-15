package domain

import "errors"

// AccountID uniquely identifies an account.
type AccountID string

// AccountStatus represents the lifecycle state of an account.
type AccountStatus string

const (
	AccountStatusActive AccountStatus = "active"
	AccountStatusFrozen AccountStatus = "frozen"
	AccountStatusClosed AccountStatus = "closed"
)

// Domain errors are returned as-is by usecases - never wrapped - so callers
// (services, HTTP handlers, etc.) can compare them with errors.Is and react
// accordingly (e.g. map ErrInsufficientFunds to HTTP 402).
var (
	ErrInvalidAmount     = errors.New("amount must be positive")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrAccountNotActive  = errors.New("account is not active")
)

// ChangeTracker records which fields of an entity were mutated since it was
// loaded, so a repository can build a Mutation that touches only the
// columns that actually changed instead of overwriting the whole row.
type ChangeTracker struct {
	dirty map[string]struct{}
}

// Mark flags a field as dirty.
func (c *ChangeTracker) Mark(field string) {
	if c.dirty == nil {
		c.dirty = make(map[string]struct{})
	}
	c.dirty[field] = struct{}{}
}

// IsDirty reports whether a specific field was marked as changed.
func (c *ChangeTracker) IsDirty(field string) bool {
	_, ok := c.dirty[field]
	return ok
}

// Fields returns the names of all fields marked dirty so far.
func (c *ChangeTracker) Fields() []string {
	fields := make([]string, 0, len(c.dirty))
	for f := range c.dirty {
		fields = append(fields, f)
	}
	return fields
}

// Any reports whether anything has changed at all.
func (c *ChangeTracker) Any() bool {
	return len(c.dirty) > 0
}

// Account is the money-transfer domain entity. All state changes go
// through exported behaviour methods (Withdraw/Deposit) - never through
// direct field assignment from outside the package - so business
// invariants can never be bypassed by callers.
type Account struct {
	id      AccountID
	balance int64 // cents
	status  AccountStatus

	// Changes is exported so repositories (a different package) can
	// inspect which fields changed in order to build a dirty-fields-only
	// Mutation. It is intentionally NOT something outside code can use to
	// mutate state - only Mark() flips a field, and only Withdraw/Deposit
	// call Mark().
	Changes ChangeTracker
}

// NewAccount constructs an Account from already-persisted state (e.g. a
// repository hydrating a row). It does NOT mark anything dirty - nothing
// has changed yet, this simply reflects what's already stored.
func NewAccount(id AccountID, balance int64, status AccountStatus) *Account {
	return &Account{id: id, balance: balance, status: status}
}

func (a *Account) ID() AccountID         { return a.id }
func (a *Account) Balance() int64        { return a.balance }
func (a *Account) Status() AccountStatus { return a.status }

// Withdraw removes amount cents from the account, enforcing:
//   - amount must be positive
//   - account must be active
//   - sufficient funds must be available
func (a *Account) Withdraw(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if a.status != AccountStatusActive {
		return ErrAccountNotActive
	}
	if a.balance < amount {
		return ErrInsufficientFunds
	}
	a.balance -= amount
	a.Changes.Mark("balance")
	return nil
}

// Deposit adds amount cents to the account, enforcing:
//   - amount must be positive
//   - account must be active (frozen/closed accounts can't receive funds)
func (a *Account) Deposit(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if a.status != AccountStatusActive {
		return ErrAccountNotActive
	}
	a.balance += amount
	a.Changes.Mark("balance")
	return nil
}
