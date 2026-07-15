package repo

import "sync"

type row map[string]interface{}

type store struct {
	mu     sync.RWMutex
	tables map[string]map[string]row // table -> row ID -> row
}

func newStore() *store {
	return &store{
		tables: map[string]map[string]row{
			"accounts": {},
		},
	}
}
