package mw

import "github.com/gin-gonic/gin"

// Logger is a middleware factory: the handler is the returned literal.
func Logger() gin.HandlerFunc { return func(c *gin.Context) { c.Next() } }

func Auth(c *gin.Context)  { c.Next() }
func Trace(c *gin.Context) { c.Next() }

func RateLimit(n int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if n < 0 {
			c.Abort()
		}
		c.Next()
	}
}
