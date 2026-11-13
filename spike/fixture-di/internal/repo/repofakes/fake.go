package repofakes

import (
	"context"

	"example.com/fixturedi/internal/repo"
)

type FakeTxRepo struct {
	SaveStub func(context.Context, repo.Tx) error
}

func (f *FakeTxRepo) Save(ctx context.Context, tx repo.Tx) error { return f.SaveStub(ctx, tx) }
