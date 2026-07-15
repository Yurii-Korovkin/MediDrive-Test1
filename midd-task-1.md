# Backend Developer Assessment - Mid Level

## Instructions

1. Complete all tasks below
2. Push your solution to a **public GitHub repository**

---

## Task: Money Transfer Usecase

Implement a `TransferMoney` usecase that transfers funds between two accounts following our architecture pattern.

### Architecture Rule

```
Service → Usecase → Domain → Repository (returns mutations) → Committer (applies)
```

**Critical:** Repositories RETURN mutations, they NEVER apply them. The service layer applies all mutations atomically.

### Given Interfaces

```go
type AccountRepository interface {
    Retrieve(ctx context.Context, id AccountID) (*Account, error)
    UpdateMut(account *Account) *Mutation  // Returns mutation, doesn't apply
}

type Mutation struct {
    Table   string
    ID      string
    Updates map[string]interface{}
}

type Plan struct {
    mutations []*Mutation
}

func NewPlan() *Plan { return &Plan{} }
func (p *Plan) Add(m *Mutation) { if m != nil { p.mutations = append(p.mutations, m) } }
```

### Given Domain Entity

```go
type Account struct {
    id      AccountID
    balance int64  // cents
    status  AccountStatus
    Changes ChangeTracker
}

func (a *Account) Withdraw(amount int64) error  // You implement
func (a *Account) Deposit(amount int64) error   // You implement
```

### Your Task

Implement the usecase:

```go
type TransferRequest struct {
    FromAccountID AccountID
    ToAccountID   AccountID
    Amount        int64
}

func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) (*Plan, error) {
    // 1. Validate request
    // 2. Retrieve both accounts
    // 3. Call domain methods (Withdraw from source, Deposit to dest)
    // 4. Get mutations FROM repository
    // 5. Return plan (do NOT apply)
}
```

And the repository method:

```go
func (r *AccountRepo) UpdateMut(account *Account) *Mutation {
    // Only include dirty fields
    // Return nil if nothing changed
}
```

---

## Buggy Code - Find All Issues

This usecase has multiple bugs. List ALL of them in `REVIEW.md`:

```go
func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) error {
    source, _ := uc.repo.Retrieve(ctx, req.FromAccountID)
    dest, _ := uc.repo.Retrieve(ctx, req.ToAccountID)

    source.balance -= req.Amount
    dest.balance += req.Amount

    mutation1 := &Mutation{
        Table:   "accounts",
        ID:      string(source.id),
        Updates: map[string]interface{}{"balance": source.balance},
    }

    mutation2 := &Mutation{
        Table:   "accounts",
        ID:      string(dest.id),
        Updates: map[string]interface{}{"balance": dest.balance},
    }

    if err := uc.db.Apply(mutation1); err != nil {
        return err
    }
    if err := uc.db.Apply(mutation2); err != nil {
        return err
    }

    return nil
}
```

Find at least 6 issues.

---

## Questions - Answer in ANSWERS.md

**Q1:** In your implementation, what happens if `source.Withdraw()` succeeds but `dest.Deposit()` fails? Show the exact state of both accounts and the returned plan.

**Q2:** The buggy code applies mutations one at a time. Why is this a problem? Give a specific failure scenario.

**Q3:** Your `UpdateMut` should only include dirty fields. If an account has `balance` changed but `status` unchanged, the mutation should NOT include `status`. Why does this matter for concurrent updates?

**Q4:** Look at this alternative approach:

```go
func (r *AccountRepo) UpdateMut(account *Account) *Mutation {
    return &Mutation{
        Updates: map[string]interface{}{
            "balance": account.Balance(),
            "status":  account.Status(),  // Always include all fields
        },
    }
}
```

What problem does this cause that the dirty-field approach avoids?

---

## Repository Structure

```
your-repo/
├── domain/
│   └── account.go
├── contracts/
│   └── repository.go
├── usecases/
│   └── transfer/
│       ├── interactor.go
│       └── interactor_test.go
├── repo/
│   └── account_repo.go
├── REVIEW.md
└── ANSWERS.md
```

---

## Evaluation

Your submission will be evaluated against our engineering standards document. Key areas:
- Repositories return mutations, never apply them
- Usecases get mutations FROM repository, don't create them directly
- Domain methods called (not direct field access)
- Only dirty fields in mutations
- Validation before domain calls
- Domain errors not wrapped
