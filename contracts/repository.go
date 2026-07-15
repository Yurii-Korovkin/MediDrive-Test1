// Package contracts defines the interfaces and plain data types that sit
// between usecases and storage: AccountRepository (retrieve + describe
// mutations), Mutation/Plan (the data description of a change), and
// Committer (applies a Plan atomically). Usecases depend only on these
// interfaces, never on a concrete repository implementation.
package contracts

import (
	"context"
	"money-transfer/domain"
)

// AccountRepository retrieves accounts and DESCRIBES mutations - it never
// applies them. Applying is the Committer's job, invoked by the service
// layer once a full Plan has been assembled by a usecase.
type AccountRepository interface {
	Retrieve(ctx context.Context, id domain.AccountID) (*domain.Account, error)
	UpdateMut(account *domain.Account) *Mutation
}

// Mutation describes a single row-level change: which table, which row,
// and which columns to set. It carries no behaviour of its own - it's a
// plain data description that any Committer implementation (SQL, in-memory,
// etc.) knows how to apply.
type Mutation struct {
	Table   string
	ID      string
	Updates map[string]interface{}
}

// Plan is an ordered batch of mutations produced by a usecase. Building a
// Plan has zero side effects on storage - it is completely inert until a
// Committer applies it.
type Plan struct {
	mutations []*Mutation
}

func NewPlan() *Plan { return &Plan{} }

// Add appends a mutation to the plan. Nil mutations (e.g. what UpdateMut
// returns when nothing actually changed) are silently ignored, so callers
// never need to nil-check before adding.
func (p *Plan) Add(m *Mutation) {
	if m != nil {
		p.mutations = append(p.mutations, m)
	}
}

// Mutations exposes the accumulated mutations, in the order they were added.
func (p *Plan) Mutations() []*Mutation { return p.mutations }

// IsEmpty reports whether the plan has no mutations at all.
func (p *Plan) IsEmpty() bool { return len(p.mutations) == 0 }

// Committer applies a Plan atomically: either every mutation in it takes
// effect, or none do. This is the final stage of the pattern:
//
//	Service -> Usecase -> Domain -> Repository (mutations) -> Committer (applies)
type Committer interface {
	Apply(ctx context.Context, plan *Plan) error
}
