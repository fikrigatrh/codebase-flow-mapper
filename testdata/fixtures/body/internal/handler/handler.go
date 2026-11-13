package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"example.com/body/internal/dto"
	"example.com/body/internal/service"
)

type Handler struct{ svc *service.Service }

func New(s *service.Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Ping(c *gin.Context) { c.String(200, "pong") }

func (h *Handler) Topup(c *gin.Context) {
	var req dto.TopupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Status(400)
		return
	}
	if req.Express && req.Source.Value > 1000 {
		h.svc.Review(req.RequestID)
	}
	h.svc.Topup(c, req)
}

func Legacy(w http.ResponseWriter, r *http.Request) {
	var in dto.LegacyReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad body", 400)
		return
	}
	if 3 <= in.Count {
		w.WriteHeader(202)
	}
	if !in.DryRun {
		w.WriteHeader(200)
	}
}
