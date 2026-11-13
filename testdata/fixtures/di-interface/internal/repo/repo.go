package repo

import (
	"context"
	"errors"
)

type Tx struct{ Amount int }

type TxRepo interface {
	Save(ctx context.Context, tx Tx) error
}

type pgRepo struct{}

func NewPg() TxRepo { return &pgRepo{} }

func (r *pgRepo) Save(ctx context.Context, tx Tx) error { return r.exec(ctx) }
func (r *pgRepo) exec(ctx context.Context) error        { return ctx.Err() }

// memRepo implements TxRepo but is never wired: VTA must not show it, CHA would.
type memRepo struct{}

func (m *memRepo) Save(ctx context.Context, tx Tx) error { return errors.New("unused") }
