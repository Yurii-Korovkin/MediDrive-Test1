import "errors"

var (
	ErrInvalidAmount     = errors.New("amount must be positive")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrAccountNotActive  = errors.New("account is not active")
)

const (
	// AccountStatusActive indicates that the account is active and can be used for transactions.
	AccountStatusActive AccountStatus = "active"
	// AccountStatusInactive indicates that the account is inactive and cannot be used for transactions.
	AccountStatusInactive AccountStatus = "inactive"
)

type Account struct {
	id      AccountID
	balance int64 // cents
	status  AccountStatus
	Changes ChangeTracker
}

func (a *Account) Withdraw(amount int64) error // You implement
func (a *Account) Deposit(amount int64) error

