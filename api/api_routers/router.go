package api_routers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type RouteStruct struct {
	// Add any necessary fields here
}

func (r *RouteStruct) SetRoutes(engine *gin.Engine) {
	// 应用 CORS 中间件到所有路由
	engine.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// ... 其余路由注册代码
} 
