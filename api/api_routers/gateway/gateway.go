package gateway

import (
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/controller"
)

// GatewayRoutes -
func GatewayRoutes() chi.Router {
	r := chi.NewRouter()
	// 关于路由的接口
	r.Route("/routes", func(r chi.Router) {
		r.Get("/", controller.GetManager().GetAPIRoute)
		r.Post("/", controller.GetManager().CreateAPIRoute)
		r.Put("/{name}", controller.GetManager().UpdateAPIRoute)
		r.Delete("/{name}", controller.GetManager().DeleteAPIRoute)
	})

	// 关于目标服务的接口
	r.Route("/service", func(r chi.Router) {
		r.Get("/rbd", controller.GetManager().GetRBDService)
		r.Get("/", controller.GetManager().GetAPIService)
		r.Post("/", controller.GetManager().CreateAPIService)
		r.Put("/{name}", controller.GetManager().UpdateAPIService)
		r.Delete("/{name}", controller.GetManager().DeleteAPIService)
	})

	r.Route("/cert", func(r chi.Router) {
		r.Get("/", controller.GetManager().GetCert)
		r.Post("/", controller.GetManager().CreateCert)
		r.Put("/{name}", controller.GetManager().UpdateCert)
		r.Delete("/{name}", controller.GetManager().DeleteCert)
	})

	return r
}
