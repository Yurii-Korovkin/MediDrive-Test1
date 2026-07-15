package repo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountRepoCreateAndGet(t *testing.T) {
	ctx := context.Background()
	repo := NewAccountRepo()

	account := Account{
		ID:      "acc1",
		Owner:   "Alice",
		Balance: 100,
	}

	err := repo.Create(ctx, account)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, "acc1")
	require.NoError(t, err)
	assert.Equal(t, account, got)
}

func TestAccountRepoUpdateBalance(t *testing.T) {
	ctx := context.Background()
	repo := NewAccountRepo()

	account := Account{
		ID:      "acc2",
		Owner:   "Bob",
		Balance: 200,
	}

	err := repo.Create(ctx, account)
	require.NoError(t, err)

	err = repo.UpdateBalance(ctx, "acc2", 300)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, "acc2")
	require.NoError(t, err)
	assert.Equal(t, 300, got.Balance)
}

func TestAccountRepoDelete(t *testing.T) {
	ctx := context.Background()
	repo := NewAccountRepo()

	account := Account{ID: "acc3", Owner: "Carol", Balance: 50}
	err := repo.Create(ctx, account)
	require.NoError(t, err)

	err = repo.Delete(ctx, "acc3")
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, "acc3")
	assert.Error(t, err)
}
