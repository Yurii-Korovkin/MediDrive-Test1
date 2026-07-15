## 1. What happens if `source.Withdraw()` is successful, but `dest.Deposit()` is not?

Let's consider a specific case (also reproduced in the test `TestExecute_SourceUntouchedWhenDestFails`): `source` has enough funds, `dest` is a frozen account (`AccountStatusFrozen`).

Step by step inside `Execute`:

- `source, _ := uc.repo.Retrieve(ctx, req.FromAccountID)` — we get a **fresh, local** `*domain.Account`, isolated from the repository (each call to `Retrieve` returns a new hydrated copy).
- `dest, _ := uc.repo.Retrieve(ctx, req.ToAccountID)` — the same, a local copy.
- `source.Withdraw(req.Amount)` — successful. This changes **only the fields of the local Go variable `source`**: `source.balance` is decremented, `source.Changes` marks `"balance"` as a dirty field. There has been no deposit yet — neither the repository nor the committer has been called.
- `dest.Deposit(req.Amount)` — returns `domain.ErrAccountNotActive`, since the account is frozen. The function immediately executes `return nil, err`.

**Exact state after `Execute` returns:**

| What | State |
|---|---|
| Returned `*Plan` | `nil` — no mutation was ever created (execution did not reach `uc.repo.UpdateMut(...)`) |
| Returned error | `domain.ErrAccountNotActive` (unwrapped) |
| Local variable `source` in memory of the call | It has a reduced `balance` and a dirty field `"balance"` marked — but this is an intermediate, ephemeral Go object that will be destroyed by the garbage collector immediately after the function exits. It was never passed anywhere. |
| The real state of the source account in the repository | **Absolutely unchanged** — where `Retrieve` takes the data from, the original balance still lies |
| The real state of the destination account in the repository | Not changed at all — `Deposit` didn't even have time to touch the balance, the error occurs at the line `a.balance += amount` |

This is the main advantage of the "repository returns mutations, not applies" pattern: since nothing is committed until the full `Plan` is assembled and explicitly passed to `Committer.Apply` at the service level, a failure **inside the domain logic** can never leave a partial state in the repository — there is no intermediate "dangerous" persisted state, unlike buggy code, where individual `Apply` calls actually moved money into the database one after another.

---

## 2. Why does buggy code apply mutations one by one — and why is this a problem?

```go
if err := uc.db.Apply(mutation1); err != nil { return err }
if err := uc.db.Apply(mutation2); err != nil { return err }
```

`Apply(mutation1)` and `Apply(mutation2)` are two completely independent calls, not connected by any transaction. There is no mechanism to "roll back" `mutation1` if `mutation2` fails.

**Specific failure scenario:**

- `mutation1` (debiting $50 from account A) is successfully applied — the money has actually disappeared from account A in the database.
- Before calling `Apply(mutation2)`, a failure occurs: temporary loss of connection to the database, a competing process managed to delete/lock account B, transaction timeout was exceeded, etc.
- `Apply(mutation2)` returns an error. The function immediately `return err` — **without any attempt to compensate** for the already applied `mutation1`.

**Result: $50 was debited from account A and never credited to account B — the money simply disappeared** from the system. This is worse than double-entry (which is at least noticeable by the excess funds) — here the system balance becomes less than it should have been, and without manual auditing of transactions, this discrepancy is generally difficult to detect.

**Why this is a problem architecturally:** Two mutations that logically constitute one business operation (a transfer) must be applied as a single atomic unit — either both or neither. That is why, in a correct implementation, `Committer.Apply` accepts the entire `*Plan` in a single call and guarantees atomicity itself (in this project, through a two-phase approach: first check the existence of all target rows under a single lock, then apply all mutations; no mutation is applied if at least one of the others cannot be applied — see `InMemoryCommitter.Apply` and the `TestCommitter_RejectsPlanWithUnknownRow` test).

---

## 3. Why is dirty-fields-only important for competitive updates?

If `UpdateMut` always includes only the actually changed fields in the mutation (and not the entire row), each operation affects **only those columns for which it is the source of truth**, leaving all other fields unmanaged by this operation at all.

**A specific scenario without this protection:** imagine that `UpdateMut` would always return both `balance` and `status`, regardless of what actually changed.

- Goroutine A calls `TransferMoney`: gets the account, calls `Withdraw` (only `balance` changes), generates a mutation.

- **At the same time**, in a separate thread, goroutine B performs the operation "freeze account due to suspected fraud" - changes only `status` from `active` to `frozen`, and its mutation is applied **between** when goroutine A read the account and when goroutine A's mutation was applied.

- If goroutine A's mutation always includes `status` (the outdated value of `active` that was in its local copy at the time of `Retrieve`), the application