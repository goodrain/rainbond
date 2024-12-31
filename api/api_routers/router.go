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
		controller.CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Next()
		})).ServeHTTP(c.Writer, c.Request)
	})

	// ... 其余路由注册代码
} 