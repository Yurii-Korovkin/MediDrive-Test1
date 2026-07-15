// Command demo wires the full architecture end to end:
//
//	Service (this main func) -> Usecase (transfer.Interactor) ->
//	Domain (Account.Withdraw/Deposit) -> Repository (mutations only) ->
//	Committer (applies atomically)
//
// Run it with:
//
//	go run ./cmd/demo
package main

import (
	"context"
	"fmt"

	"money-transfer/domain"
	"money-transfer/repo"
	"money-transfer/usecases/transfer"
)

func main() {
	repository, committer := repo.NewInMemoryBackend()

	repository.Seed(domain.NewAccount("acc-1", 10000, domain.AccountStatusActive)) // $100.00
	repository.Seed(domain.NewAccount("acc-2", 2500, domain.AccountStatusActive))  // $25.00
	repository.Seed(domain.NewAccount("acc-3", 0, domain.AccountStatusFrozen))     // frozen: can't receive funds

	uc := transfer.NewInteractor(repository)

	fmt.Println("===Successful transfer===")
	successfulTransfer(uc, committer, repository)

	fmt.Println("\n=== Insufficient funds ===")
	insufficientFundsDemo(uc)

	fmt.Println("\n=== Recipient is inactive(scenario on Question 1)===")
	failedDepositDemo(uc, repository)

	fmt.Println("\n===Validation request===")
	validationDemo(uc)
}

func successfulTransfer(uc *transfer.Interactor, committer *repo.InMemoryCommitter, repository *repo.AccountRepo) {
	ctx := context.Background()

	before1, _ := repository.Retrieve(ctx, "acc-1")
	before2, _ := repository.Retrieve(ctx, "acc-2")
	fmt.Printf("До переказу:    acc-1=%d, acc-2=%d\n", before1.Balance(), before2.Balance())

	plan, err := uc.Execute(ctx, &transfer.TransferRequest{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        1500, // $15.00
	})
	if err != nil {
		fmt.Println("Error usecase:", err)
		return
	}
	fmt.Printf("Usecase returned a plan with %d mutations (still NOT applied to storage)\n", len(plan.Mutations()))

	// This is the "Service" step: only now, after the usecase handed back
	// a plan, does anything actually get written.
	if err := committer.Apply(ctx, plan); err != nil {
		fmt.Println("Error applying plan:", err)
		return
	}

	after1, _ := repository.Retrieve(ctx, "acc-1")
	after2, _ := repository.Retrieve(ctx, "acc-2")
	fmt.Printf("Після переказу: acc-1=%d, acc-2=%d\n", after1.Balance(), after2.Balance())
}

func insufficientFundsDemo(uc *transfer.Interactor) {
	plan, err := uc.Execute(context.Background(), &transfer.TransferRequest{
		FromAccountID: "acc-2",
		ToAccountID:   "acc-1",
		Amount:        999999,
	})
	fmt.Println("Error:", err)
	fmt.Println("Plan:", plan, "(nil - nothing was built and not applied)")
}

func failedDepositDemo(uc *transfer.Interactor, repository *repo.AccountRepo) {
	ctx := context.Background()

	before, _ := repository.Retrieve(ctx, "acc-1")
	fmt.Printf("Balance of acc-1 before attempt: %d\n", before.Balance())

	plan, err := uc.Execute(ctx, &transfer.TransferRequest{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-3", // frozen
		Amount:        500,
	})
	fmt.Println("Error (acc-3 is frozen):", err)
	fmt.Println("Plan:", plan)

	after, _ := repository.Retrieve(ctx, "acc-1")
	fmt.Printf("Balance of acc-1 after failed attempt: %d (should remain unchanged - money didn't go anywhere)\n",
		after.Balance())
}

func validationDemo(uc *transfer.Interactor) {
	ctx := context.Background()

	_, err := uc.Execute(ctx, &transfer.TransferRequest{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-1",
		Amount:        100,
	})
	fmt.Println("Transfer to the same account:", err)

	_, err = uc.Execute(ctx, &transfer.TransferRequest{
		FromAccountID: "acc-1",
		ToAccountID:   "acc-2",
		Amount:        0,
	})
	fmt.Println("Zero amount:", err)

	_, err = uc.Execute(ctx, nil)
	fmt.Println("nil request:", err)
}
