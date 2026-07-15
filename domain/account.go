import "errors"

var (
	ErrInvalidAmount     = errors.New("amount must be positive")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrAccountNotActive  = errors.New("account is not active")
)

const (
	AccountStatusActive AccountStatus = "active"
	AccountStatusFrozen AccountStatus = "frozen"
	AccountStatusClosed AccountStatus = "closed"
)

type ChangeTracker struct {
	dirty map[string]struct{}
}

func (c *ChangeTracker) Mark(field string) {}

func (c *ChangeTracker) IsDirty(field string) bool {}

func (c *ChangeTracker) Fields() []string {}

// Any reports anything has changed at all
func (c *ChangeTracker) Any() bool {}

type Account struct {
	id      AccountID
	balance int64 // cents
	status  AccountStatus
	Changes ChangeTracker
}

func NewAccount(id AccountID, balance int64, status AccountStatus) *Account {
	return &Account{id: id, balance: balance, status: status}
}
func (a *Account) ID() AccountID         { return a.id }
func (a *Account) Balance() int64        { return a.balance }
func (a *Account) Status() AccountStatus { return a.status }

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

// You implement
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

