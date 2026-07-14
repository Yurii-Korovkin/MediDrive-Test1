type AccountRepository interface {
	Retrieve(ctx context.Context, id AccountID) (*Account, error)
	UpdateMut(account *Account) *Mutation // Returns mutation, doesn't apply
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
func (p *Plan) Add(m *Mutation) {
	if m != nil {
		p.mutations = append(p.mutations, m)
	}
}