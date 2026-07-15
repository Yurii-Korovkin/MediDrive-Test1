Analysis of all problems in the given buggy implementation of `Execute`. Found 9, required at least 6.

```go
func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) error {
    source, _ := uc.repo.Retrieve(ctx, req.FromAccountID)
    dest, _ := uc.repo.Retrieve(ctx, req.ToAccountID)

    source.balance -= req.Amount
    dest.balance += req.Amount
    ...
}
```

---

## Bug 1: Ignored `Retrieve` errors

- **Code:** `source, _ := uc.repo.Retrieve(ctx, req.FromAccountID)`
- **What happens:** `Retrieve` error is discarded because of `_`. If the account does not exist, `Retrieve` will return `nil, err`, and the next line `source.balance -= req.Amount` will fail with nil pointer dereference — panic instead of a clear error.
- **Production scenario:** client passed incorrect/deleted `AccountID` (e.g. typo in ID or account already closed and deleted from DB) → service crashes with panic to production instead of returning 404/400 to client.
- **Fix approach:** check and return error immediately after each `Retrieve` call.

## Bug 2: Direct access to private fields instead of domain methods

- **Code:** `source.balance -= req.Amount`, `dest.balance += req.Amount`
- **What happens:** business logic (checking for sufficient funds, account status, positiveness of the amount) is completely bypassed. The task requirement — "Domain methods called, not direct field access" — is directly violated.
- **Production scenario:** the account is frozen (`status == frozen`), but the balance changes anyway, because no one checks the status. Or: `source.balance` goes negative, because no one checked `balance >= amount` — the account gets a negative balance.
- **Fix approach:** call `source.Withdraw(req.Amount)` and `dest.Deposit(req.Amount)` — they encapsulate all invariants.

## Bug 3: Missing request validation

- **Code:** there is no check of `req` before use.
- **What happens:** it is not checked that `req.Amount > 0`, that `req` is not `nil`, and that `FromAccountID != ToAccountID`.
- **Production scenario:** the client accidentally (or maliciously) sends `Amount: -500` — the code will subtract a negative number from the source, i.e. effectively **replenish** the source with an account out of nowhere, and at the same time **reduce** the destination. Or: `FromAccountID == ToAccountID` — the account simultaneously debits and credits itself with the same amount with two separate mutations, which in case of non-atomic application (Bug 6) can lead to double debit/credit depending on the order of `Apply`.
- **Fix approach:** a separate validation function before any work with the repository — checking for `nil`, positivity of the sum, and differences in accounts.
## Bug 4: Usecase make mutation self, but dont get them from the repository

- **Code:**
  ```go
  mutation1 := &Mutation{Table: "accounts", ID: string(source.id), Updates: ...}
  ```
- **What happens:** the architectural rule explicitly says "Usecases get mutations FROM repository, don't create them directly". Here the usecase directly creates a `Mutation`, knowing the table name and column structure - this is the responsibility of the repository (`UpdateMut`), not the usecase layer. Violation of separation of responsibilities: the usecase is now tied to the details of the database schema.
- **Production scenario:** If tomorrow the name of a column or table in the database changes, you will have to edit the usecase code instead of changing just the repository — a logic leak through the layers of the architecture.
- **Fix approach:** usecase викликає `uc.repo.UpdateMut(source)` і `uc.repo.UpdateMut(dest)`, отримуючи вже готові `*Mutation` від репозиторію.

## Bug 5: Usecase self apply mutation

- **Code:** `uc.db.Apply(mutation1)`, `uc.db.Apply(mutation2)`
- **What happens:** architectural rule — "Repositories RETURN mutations, they NEVER apply them. The service layer applies all mutations atomically." Here, the usecase not only creates mutations itself (Bug 4), but also immediately applies them via `uc.db.Apply`, bypassing the service layer and Committer completely. The usecase no longer returns `Plan` — it returns nothing at all except `error`.
- **Production scenario:** everything that was supposed to be a single atomic transaction at the service layer is now scattered across the usecase call — it is impossible to combine this transfer with another business operation into one transaction (for example, writing off the transfer fee together with the transfer itself), because the application occurs immediately and without centralized control.
- **Fix approach:** `Execute` returns `(*Plan, error)` and does not apply anything; application is exclusively at the `Committer` level, called by the service layer.

## Bug 6: Mutations are applied one at a time, without atomicity

- **Code:**
```go
if err := uc.db.Apply(mutation1); err != nil { return err }
if err := uc.db.Apply(mutation2); err != nil { return err }
```
- **What happens:** Two separate, independent write operations, with no transaction or lock to unite them. There is no rollback if the second one fails.
- **Production scenario:** `mutation1` (debit from source) was successfully applied. At the time of calling `uc.db.Apply(mutation2)`, a network failure occurs, or the destination account is deleted concurrently — `Apply(mutation2)` returns an error. The function returns `err`, **the money has already been debited from the source, but never credited to the destination** — effectively gone. Manual reconciliation and recovery is required.
- **Fix approach:** both mutations are collected into a single `Plan` and applied with a single `Committer.Apply(plan)` call, which guarantees all-or-nothing (for details, see `Qestion 2` in ANSWERS.md).

##Bug 7: No tracking of dirty fields

- **Code:** `Updates: map[string]interface{}{"balance": source.balance}` — is manually generated every time.
- **What happens:** the mutation always explicitly lists only `balance`, because the code is manually written for this specific case — but there is no general mechanism that would guarantee that **only** the fields that were actually changed are included in the mutation. If this same template is copied into another operation that also touches `status`, there is nothing to prevent accidentally including an outdated value of a field that was not actually changed.
- **Production scenario:** see `Qestion 3`and `Question 4` in ANSWERS.md — a concurrent operation that changes only `status` risks being overwritten by an operation without tracking dirty fields.
- **Fix approach:** `ChangeTracker` on the domain entity, which `Withdraw`/`Deposit` mark (`Mark("balance")`), and `UpdateMut` reads (`Changes.Fields()`) to include only the actually changed columns in the mutation.

## Bug 8: `Execute` signature does not meet the architecture requirement

- **Code:** `func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) error`
- **What happens:** according to the task condition `Execute` must return `(*Plan, error)` — it is `Plan` that is the "contract" between the usecase layer and the service layer, which will then apply it. By returning only `error`, the usecase is forced to apply the changes itself (hence Bug 5) — the signature and the architectural violation go hand in hand.
- **Fix approach:** change signature to `(*Plan, error)` as specified in task requirements.

## Bug 9: No check that destination account is active

- **Code:** is missing at all — `dest.balance += req.Amount` is executed unconditionally.
- **What happens:** a closed (`closed`) or frozen (`frozen`) account still receives funds, because the account status is not checked anywhere.
- **Production scenario:** transfer to an account that was just closed by support due to suspected fraud — funds are still credited, although the account should have been completely blocked for any operations.
- **Fix approach:** status check is built into `Account.Deposit`/`Account.Withdraw` — both return `ErrAccountNotActive` if the account is not `active`.