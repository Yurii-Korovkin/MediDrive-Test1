package repo_test

import (
	"context"
	"testing"

	"money-transfer/contracts"
	"money-transfer/domain"
	"money-transfer/repo"
)

func TestRetrieve_NotFound(t *testing.T) {
	r, _ := repo.NewInMemoryBackend()
	_, err := r.Retrieve(context.Background(), "missing")
	if err != repo.ErrAccountNotFound {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestUpdateMut_NoChanges_ReturnsNil(t *testing.T) {
	r, _ := repo.NewInMemoryBackend()
	acc := domain.NewAccount("acc-1", 100, domain.AccountStatusActive)

	if m := r.UpdateMut(acc); m != nil {
		t.Fatalf("expected nil mutation for untouched account, got %+v", m)
	}
}

func TestUpdateMut_OnlyDirtyFields(t *testing.T) {
	r, _ := repo.NewInMemoryBackend()
	acc := domain.NewAccount("acc-1", 100, domain.AccountStatusActive)

	if err := acc.Withdraw(30); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := r.UpdateMut(acc)
	if m == nil {
		t.Fatal("expected a mutation")
	}
	if len(m.Updates) != 1 {
		t.Fatalf("expected exactly 1 dirty field, got %d: %v", len(m.Updates), m.Updates)
	}
	if m.Updates["balance"] != int64(70) {
		t.Errorf("expected balance 70, got %v", m.Updates["balance"])
	}
	if _, hasStatus := m.Updates["status"]; hasStatus {
		t.Error("status should not be present - it was never changed")
	}
}

func TestCommitter_AppliesPlanAtomically(t *testing.T) {
	r, committer := repo.NewInMemoryBackend()
	r.Seed(domain.NewAccount("acc-1", 1000, domain.AccountStatusActive))
	r.Seed(domain.NewAccount("acc-2", 500, domain.AccountStatusActive))

	ctx := context.Background()

	source, _ := r.Retrieve(ctx, "acc-1")
	dest, _ := r.Retrieve(ctx, "acc-2")

	if err := source.Withdraw(300); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := dest.Deposit(300); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := contracts.NewPlan()
	plan.Add(r.UpdateMut(source))
	plan.Add(r.UpdateMut(dest))

	if err := committer.Apply(ctx, plan); err != nil {
		t.Fatalf("unexpected commit error: %v", err)
	}

	after1, _ := r.Retrieve(ctx, "acc-1")
	after2, _ := r.Retrieve(ctx, "acc-2")

	if after1.Balance() != 700 {
		t.Errorf("expected acc-1 balance 700, got %d", after1.Balance())
	}
	if after2.Balance() != 800 {
		t.Errorf("expected acc-2 balance 800, got %d", after2.Balance())
	}
}

// TestCommitter_RejectsPlanWithUnknownRow proves the all-or-nothing
// guarantee: if a plan references a row that doesn't exist (e.g. built
// against stale state), NOTHING in the plan is applied - not even the
// mutations that would otherwise have succeeded.
func TestCommitter_RejectsPlanWithUnknownRow(t *testing.T) {
	r, committer := repo.NewInMemoryBackend()
	r.Seed(domain.NewAccount("acc-1", 1000, domain.AccountStatusActive))

	plan := contracts.NewPlan()
	plan.Add(&contracts.Mutation{
		Table:   "accounts",
		ID:      "acc-1",
		Updates: map[string]interface{}{"balance": int64(700)},
	})
	plan.Add(&contracts.Mutation{
		Table:   "accounts",
		ID:      "does-not-exist",
		Updates: map[string]interface{}{"balance": int64(1)},
	})

	err := committer.Apply(context.Background(), plan)
	if err == nil {
		t.Fatal("expected an error for an unknown row")
	}

	// acc-1's mutation must NOT have been applied, even though it was
	// individually valid - the whole plan failed together.
	after, _ := r.Retrieve(context.Background(), "acc-1")
	if after.Balance() != 1000 {
		t.Errorf("expected acc-1 balance to remain 1000 (plan rejected atomically), got %d",
			after.Balance())
	}
}

func TestCommitter_EmptyPlanIsNoop(t *testing.T) {
	_, committer := repo.NewInMemoryBackend()
	if err := committer.Apply(context.Background(), contracts.NewPlan()); err != nil {
		t.Fatalf("expected no error for empty plan, got %v", err)
	}
	if err := committer.Apply(context.Background(), nil); err != nil {
		t.Fatalf("expected no error for nil plan, got %v", err)
	}
}
