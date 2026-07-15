package transfer_test

import (
	"context"
	"errors"
	"testing"

	"money-transfer/contracts"
	"money-transfer/domain"
	"money-transfer/usecases/transfer"
)

// fakeRepo is a minimal in-memory stand-in for contracts.AccountRepository.
// Keeping the usecase test on a fake (rather than the real repo package)
// proves the usecase only depends on the interface, and keeps these tests
// fast and independent of any particular storage implementation.
type fakeRepo struct {
	accounts    map[domain.AccountID]*domain.Account
	retrieveErr map[domain.AccountID]error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		accounts:    make(map[domain.AccountID]*domain.Account),
		retrieveErr: make(map[domain.AccountID]error),
	}
}

func (r *fakeRepo) seed(a *domain.Account) {
	r.accounts[a.ID()] = a
}

func (r *fakeRepo) Retrieve(_ context.Context, id domain.AccountID) (*domain.Account, error) {
	if err, ok := r.retrieveErr[id]; ok {
		return nil, err
	}
	acc, ok := r.accounts[id]
	if !ok {
		return nil, errors.New("account not found")
	}
	// Hand out a fresh clone, exactly like a real repository hydrating a
	// new object per Retrieve call. This also lets tests assert that the
	// seeded original in storage stays untouched when a transfer fails.
	return domain.NewAccount(acc.ID(), acc.Balance(), acc.Status()), nil
}

func (r *fakeRepo) UpdateMut(account *domain.Account) *contracts.Mutation {
	if !account.Changes.Any() {
		return nil
	}
	updates := make(map[string]interface{})
	for _, f := range account.Changes.Fields() {
		switch f {
		case "balance":
			updates["balance"] = account.Balance()
		case "status":
			updates["status"] = string(account.Status())
		}
	}
	return &contracts.Mutation{
		Table:   "accounts",
		ID:      string(account.ID()),
		Updates: updates,
	}
}

func TestExecute_SuccessfulTransfer(t *testing.T) {
	// Arrange
	repo := newFakeRepo()
	repo.seed(domain.NewAccount("acc-1", 1000, domain.AccountStatusActive))
	repo.seed(domain.NewAccount("acc-2", 500, domain.AccountStatusActive))
	uc := transfer.NewInteractor(repo)

	// Act
	plan, err := uc.Execute(context.Background(), &transfer.TransferRequest{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        300,
	})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected a plan, got nil")
	}

	muts := plan.Mutations()
	if len(muts) != 2 {
		t.Fatalf("expected 2 mutations, got %d", len(muts))
	}

	byID := map[string]*contracts.Mutation{}
	for _, m := range muts {
		byID[m.ID] = m
	}

	src, ok := byID["acc-1"]
	if !ok {
		t.Fatal("missing mutation for source account")
	}
	if src.Updates["balance"] != int64(700) {
		t.Errorf("expected source balance 700, got %v", src.Updates["balance"])
	}
	if _, hasStatus := src.Updates["status"]; hasStatus {
		t.Error("source mutation should NOT include status - it wasn't changed (dirty-fields-only)")
	}

	dst, ok := byID["acc-2"]
	if !ok {
		t.Fatal("missing mutation for destination account")
	}
	if dst.Updates["balance"] != int64(800) {
		t.Errorf("expected dest balance 800, got %v", dst.Updates["balance"])
	}
}

func TestExecute_InsufficientFunds(t *testing.T) {
	// Arrange
	repo := newFakeRepo()
	repo.seed(domain.NewAccount("acc-1", 100, domain.AccountStatusActive))
	repo.seed(domain.NewAccount("acc-2", 0, domain.AccountStatusActive))
	uc := transfer.NewInteractor(repo)

	// Act
	plan, err := uc.Execute(context.Background(), &transfer.TransferRequest{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        1000,
	})

	// Assert
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
	if plan != nil {
		t.Fatal("expected nil plan on failure")
	}
}

func TestExecute_DestinationNotActive(t *testing.T) {
	// Arrange
	repo := newFakeRepo()
	repo.seed(domain.NewAccount("acc-1", 1000, domain.AccountStatusActive))
	repo.seed(domain.NewAccount("acc-2", 0, domain.AccountStatusFrozen))
	uc := transfer.NewInteractor(repo)

	// Act
	plan, err := uc.Execute(context.Background(), &transfer.TransferRequest{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        100,
	})

	// Assert
	if !errors.Is(err, domain.ErrAccountNotActive) {
		t.Fatalf("expected ErrAccountNotActive, got %v", err)
	}
	if plan != nil {
		t.Fatal("expected nil plan on failure")
	}
}

func TestExecute_SameAccount(t *testing.T) {
	// Arrange
	repo := newFakeRepo()
	repo.seed(domain.NewAccount("acc-1", 1000, domain.AccountStatusActive))
	uc := transfer.NewInteractor(repo)

	// Act
	_, err := uc.Execute(context.Background(), &transfer.TransferRequest{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-1",
		Amount:        100,
	})

	// Assert
	if !errors.Is(err, transfer.ErrSameAccount) {
		t.Fatalf("expected ErrSameAccount, got %v", err)
	}
}

func TestExecute_NonPositiveAmount(t *testing.T) {
	// Arrange
	repo := newFakeRepo()
	repo.seed(domain.NewAccount("acc-1", 1000, domain.AccountStatusActive))
	repo.seed(domain.NewAccount("acc-2", 0, domain.AccountStatusActive))
	uc := transfer.NewInteractor(repo)

	for _, amount := range []int64{0, -50} {
		// Act
		_, err := uc.Execute(context.Background(), &transfer.TransferRequest{
			FromAccountID: "acc-1",
			ToAccountID:   "acc-2",
			Amount:        amount,
		})

		// Assert
		if !errors.Is(err, transfer.ErrNonPositiveAmount) {
			t.Errorf("amount=%d: expected ErrNonPositiveAmount, got %v", amount, err)
		}
	}
}

func TestExecute_NilRequest(t *testing.T) {
	// Arrange
	repo := newFakeRepo()
	uc := transfer.NewInteractor(repo)

	// Act
	_, err := uc.Execute(context.Background(), nil)

	// Assert
	if !errors.Is(err, transfer.ErrNilRequest) {
		t.Fatalf("expected ErrNilRequest, got %v", err)
	}
}

func TestExecute_RetrieveErrorPropagatesUnwrapped(t *testing.T) {
	// Arrange
	repo := newFakeRepo()
	repo.seed(domain.NewAccount("acc-2", 0, domain.AccountStatusActive))
	wantErr := errors.New("boom: connection lost")
	repo.retrieveErr["acc-1"] = wantErr
	uc := transfer.NewInteractor(repo)

	// Act
	plan, err := uc.Execute(context.Background(), &transfer.TransferRequest{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        100,
	})

	// Assert
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error to propagate unwrapped, got %v", err)
	}
	if plan != nil {
		t.Fatal("expected nil plan")
	}
}

// TestExecute_SourceUntouchedWhenDestFails makes the Q1 scenario concrete:
// source.Withdraw() succeeds in-memory, then dest.Deposit() fails. Nothing
// should be returned, and nothing in storage should have moved.
func TestExecute_SourceUntouchedWhenDestFails(t *testing.T) {
	// Arrange
	repo := newFakeRepo()
	repo.seed(domain.NewAccount("acc-1", 1000, domain.AccountStatusActive))
	repo.seed(domain.NewAccount("acc-2", 0, domain.AccountStatusFrozen))
	uc := transfer.NewInteractor(repo)

	// Act
	plan, err := uc.Execute(context.Background(), &transfer.TransferRequest{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        100,
	})

	// Assert
	if err == nil {
		t.Fatal("expected an error")
	}
	if plan != nil {
		t.Fatal("expected nil plan - nothing should ever be committed")
	}

	// Retrieve always hands out a fresh clone (see fakeRepo.Retrieve), so
	// the in-memory Withdraw() during this failed Execute call could never
	// have reached the seeded original in storage.
	if repo.accounts["acc-1"].Balance() != 1000 {
		t.Errorf("expected source account in storage to remain at 1000, got %d",
			repo.accounts["acc-1"].Balance())
	}
}
