package repo

import (
	"context"
	"errors"
	"money-transfer/contracts"
	"money-transfer/domain"
)

// ErrAccountNotFound is returned by Retrieve when no account exists with
// the given ID.
var ErrAccountNotFound = errors.New("account not found")

// AccountRepo is an in-memory implementation of contracts.AccountRepository.
// It only ever READS from storage and DESCRIBES changes as Mutations - it
// never writes to storage itself. Writing is InMemoryCommitter's job.
type AccountRepo struct {
	store *store
}

// NewInMemoryBackend builds an AccountRepo and an InMemoryCommitter that
// share the same underlying store - so a Plan built via the repo and
// applied via the committer actually lands on the data the repo reads
// from, mirroring how, in a real system, both would point at the same
// database connection.
func NewInMemoryBackend() (*AccountRepo, *InMemoryCommitter) {
	s := newStore()
	return &AccountRepo{store: s}, &InMemoryCommitter{store: s}
}

// Seed inserts or overwrites an account directly into storage. It's a
// bootstrap/test helper, not part of contracts.AccountRepository - real
// accounts would be created through a separate onboarding flow.
func (r *AccountRepo) Seed(a *domain.Account) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.tables["accounts"][string(a.ID())] = row{
		"balance": a.Balance(),
		"status":  a.Status(),
	}
}

// Retrieve hydrates a *domain.Account from storage.
func (r *AccountRepo) Retrieve(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	rw, ok := r.store.tables["accounts"][string(id)]
	if !ok {
		return nil, ErrAccountNotFound
	}

	balance, _ := rw["balance"].(int64)
	status, _ := rw["status"].(domain.AccountStatus)

	return domain.NewAccount(id, balance, status), nil
}

// UpdateMut inspects account.Changes and returns a Mutation containing
// only the fields that were actually marked dirty (see domain.ChangeTracker).
// Returns nil if nothing changed - Plan.Add treats nil as "skip it", so
// callers never need to nil-check before adding.
func (r *AccountRepo) UpdateMut(account *domain.Account) *contracts.Mutation {
	if !account.Changes.Any() {
		return nil
	}

	updates := make(map[string]interface{})
	for _, field := range account.Changes.Fields() {
		switch field {
		case "balance":
			updates["balance"] = account.Balance()
		case "status":
			updates["status"] = account.Status()
		}
	}

	if len(updates) == 0 {
		return nil
	}

	return &contracts.Mutation{
		Table:   "accounts",
		ID:      string(account.ID()),
		Updates: updates,
	}
}
