package contracts

import (
	"context"
	"money-transfer/domain"
)

type Mutation struct {
	Table   string
	ID      string
	Updates map[string]interface{}
}

type Plan struct {
	mutations []*Mutation
}

type AccountRepository interface {
	Retrieve(ctx context.Context, id domain.AccountID) (*domain.Account, error)
	UpdateMut(account *domain.Account) *Mutation
}

type Committer interface {
	Apply(ctx context.Context, plan *Plan) error
}
