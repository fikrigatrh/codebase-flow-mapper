package routes

import (
	"github.com/gin-gonic/gin"

	"example.com/ginsplit/internal/mw"
	"example.com/ginsplit/internal/users"
)

func Register(r *gin.Engine) {
	api := r.Group("/api")
	v1 := api.Group("/v1")
	v2 := api.Group("/v2", mw.Auth)
	registerUsers(v1)
	registerUsers(v2)
	s := &Server{router: r.Group("/admin")}
	s.routes()
	nest(api.Group("/nest"))
}

// One body reached with two prefixes yields two endpoints per route.
func registerUsers(g *gin.RouterGroup) {
	g.GET("/users", users.List)
	registerDeep(g.Group("/x"))
}

func registerDeep(g *gin.RouterGroup) { g.GET("/deep", users.List) }

// Server holds its router in a field assigned exactly once.
type Server struct{ router *gin.RouterGroup }

func (s *Server) routes() { s.router.GET("/stats", users.List) }

// nest recurses forever: the depth cap stops it.
func nest(g *gin.RouterGroup) {
	g.GET("/n", users.List)
	nest(g.Group("/n"))
}

// RegisterOrphan is never called: its prefix is unknown.
func RegisterOrphan(g gin.IRouter) { g.GET("/orphan", users.List) }
