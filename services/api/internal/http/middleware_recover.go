package http

import "github.com/gin-gonic/gin"

func RecoverMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		recoverPanic(c, recovered)
		c.Abort()
	})
}
