package service

import (
	"context"
	"net/http"
	"strings"

	"example.com/body/internal/dto"
)

type Service struct{ strict bool }

func New() *Service { return &Service{} }

func (s *Service) Review(id string) {}

func (s *Service) Topup(ctx context.Context, req dto.TopupReq) {
	switch strings.ToUpper(strings.TrimSpace(req.Source.Currency)) {
	case "HKD":
		s.local()
	case dto.CurrencySGD, "MYR":
		_, _ = http.Get("https://partner.example/send")
	case "IDR":
	default:
		s.reject()
	}
	s.pay(req.Source)
	if s.strict {
		s.reject()
	}
}

func (s *Service) pay(a dto.Amount) {
	if a.Currency == "SGD" || s.strict {
		s.local()
	} else if strings.EqualFold(a.Currency, "HKD") {
		s.local()
	}
}

func (s *Service) local()  {}
func (s *Service) reject() {}
