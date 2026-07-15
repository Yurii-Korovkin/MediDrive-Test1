// Package transfer implements the TransferMoney usecase.
package transfer

import (
	"context"
	"errors"

	"money-transfer/contracts"
	"money-transfer/domain"
)

// Usecase-level validation errors. These are distinct from domain errors
// (which come from Account.Withdraw/Deposit): they represent a malformed
// request rather than a business-rule violation against real account state.
var (
	ErrNilRequest        = errors.New("transfer request is nil")
	ErrSameAccount       = errors.New("source and destination accounts must differ")
	ErrNonPositiveAmount = errors.New("transfer amount must be positive")
)

// TransferRequest describes a request to move funds between two accounts.
type TransferRequest struct {
	FromAccountID domain.AccountID
	ToAccountID   domain.AccountID
	Amount        int64 // cents
}

// Interactor implements the TransferMoney usecase. It depends only on the
// contracts.AccountRepository interface, never on a concrete
// implementation, so it can be tested with a fake and swapped between
// storage backends without any change to this file.
type Interactor struct {
	repo contracts.AccountRepository
}

// NewInteractor wires the usecase to a repository.
func NewInteractor(repo contracts.AccountRepository) *Interactor {
	return &Interactor{repo: repo}
}

// Execute runs the transfer:
//  1. validates the request
//  2. retrieves both accounts
//  3. calls domain methods (Withdraw/Deposit) - never touches balance or
//     status fields directly
//  4. asks the repository for mutations describing what changed
//  5. returns the resulting Plan without applying it - applying is the
//     service layer's job, via a Committer
//
// If validation fails, retrieval fails, or either domain call fails, the
// domain/validation error is returned unwrapped and the plan is nil: no
// mutation was ever built, so there is nothing that could be partially
// applied.
func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) (*contracts.Plan, error) {
	if err := validate(req); err != nil {
		return nil, err
	}

	source, err := uc.repo.Retrieve(ctx, req.FromAccountID)
	if err != nil {
		return nil, err
	}

	dest, err := uc.repo.Retrieve(ctx, req.ToAccountID)
	if err != nil {
		return nil, err
	}

	if err := source.Withdraw(req.Amount); err != nil {
		return nil, err
	}
	if err := dest.Deposit(req.Amount); err != nil {
		return nil, err
	}

	plan := contracts.NewPlan()
	plan.Add(uc.repo.UpdateMut(source))
	plan.Add(uc.repo.UpdateMut(dest))

	return plan, nil
}

func validate(req *TransferRequest) error {
	if req == nil {
		return ErrNilRequest
	}
	if req.FromAccountID == req.ToAccountID {
		return ErrSameAccount
	}
	if req.Amount <= 0 {
		return ErrNonPositiveAmount
	}
	return nil
}
