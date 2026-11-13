package service

import (
	"context"
	"errors"

	"example.com/fixturedi/internal/repo"
)

type TxService interface {
	Create(ctx context.Context, amount int) error
}

type txService struct{ repo repo.TxRepo }

func New(r repo.TxRepo) TxService { return &txService{repo: r} }

func (s *txService) Create(ctx context.Context, amount int) error {
	if amount <= 0 {
		return errors.New("invalid amount")
	}
	return s.repo.Save(ctx, repo.Tx{Amount: amount})
}
