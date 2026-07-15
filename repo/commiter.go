package repo

import (
	"context"
	"fmt"

	"money-transfer/contracts"
)

// InMemoryCommitter applies a Plan atomically against the shared in-memory
// store: either every mutation in the plan takes effect, or (if any target
// row is missing) none do.
//
// It deliberately has no knowledge of the Account domain type - it only
// merges the generic column updates in each Mutation into the matching
// row. That's the whole point of the Mutation abstraction: the committer
// stays swappable (in-memory today, a real SQL transaction tomorrow)
// without the usecase or domain layers ever changing.
type InMemoryCommitter struct {
	store *store
}

// Apply commits every mutation in the plan as a single atomic operation.
func (c *InMemoryCommitter) Apply(ctx context.Context, plan *contracts.Plan) error {
	if plan == nil || plan.IsEmpty() {
		return nil
	}

	c.store.mu.Lock()
	defer c.store.mu.Unlock()

	muts := plan.Mutations()

	// Phase 1: validate every target row exists before changing anything,
	// so a plan either applies fully or leaves storage completely
	// untouched - never half-applied.
	for _, m := range muts {
		table, ok := c.store.tables[m.Table]
		if !ok {
			return fmt.Errorf("committer: unknown table %q", m.Table)
		}
		if _, ok := table[m.ID]; !ok {
			return fmt.Errorf("committer: row %q not found in table %q", m.ID, m.Table)
		}
	}

	// Phase 2: apply. Safe without re-checking - we've held the lock
	// continuously since phase 1 started, so nothing else could have
	// removed a row in between.
	for _, m := range muts {
		target := c.store.tables[m.Table][m.ID]
		for col, val := range m.Updates {
			target[col] = val
		}
		c.store.tables[m.Table][m.ID] = target
	}

	return nil
}
