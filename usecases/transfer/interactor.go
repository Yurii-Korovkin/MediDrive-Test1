package transfer

import (
	"context"
	"errors"
	"money-transfer/contracts"
	"money-transfer/domain"
)

var (
	ErrNilRequest        = errors.New("transfer request is nil")
	ErrSameAccount       = errors.New("source and destination accounts must differ")
	ErrNonPositiveAmount = errors.New("transfer amount must be positive")
)

type TransferRequest struct {
	FromAccountID domain.AccountID
	ToAccountID   domain.AccountID
	Amount        int64 // cents
}

type Interactor struct {
	repo contracts.AccountRepository
}

// NewInteractor wires the usecase to a repository.
func NewInteractor(repo contracts.AccountRepository) *Interactor {
	return &Interactor{repo: repo}
}

func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) (*contracts.Plan, error) {}

func validate(req *TransferRequest) error {}
