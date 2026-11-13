package handler

import "github.com/gin-gonic/gin"

type Handler struct{}

func New() *Handler { return &Handler{} }

func (h *Handler) Ping(c *gin.Context)     { c.String(200, "pong") }
func (h *Handler) CreateTx(c *gin.Context) { c.Status(201) }
func (h *Handler) DeleteTx(c *gin.Context) { c.Status(204) }
