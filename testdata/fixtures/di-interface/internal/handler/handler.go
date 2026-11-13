package handler

import (
	"net/http"

	"example.com/fixturedi/internal/service"
)

type TxHandler struct{ svc service.TxService }

func New(s service.TxService) *TxHandler { return &TxHandler{svc: s} }

func (h *TxHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Create(r.Context(), 10); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
