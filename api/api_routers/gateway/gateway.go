package gateway

import (
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/controller"
	"github.com/goodrain/rainbond/api/middleware"
)

// Routes -
func Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.InitTenant)
	// 关于路由的接口
	r.Route("/routes", func(r chi.Router) {
		r.Get("/domains", controller.GetManager().GetBindDomains)
		r.Get("/port", controller.GetManager().OpenOrCloseDomains)
		r.Get("/", controller.GetManager().GetHTTPAPIRoute)
		r.Post("/", controller.GetManager().CreateHTTPAPIRoute)
		r.Delete("/{name}", controller.GetManager().DeleteHTTPAPIRoute)
	})

	// 关于路由的接口
	r.Route("/routes/tcp", func(r chi.Router) {
		r.Get("/", controller.GetManager().GetTCPRoute)
		r.Post("/", controller.GetManager().CreateTCPRoute)
		r.Delete("/{name}", controller.GetManager().DeleteTCPRoute)
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
