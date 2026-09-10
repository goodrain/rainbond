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
	r.Route("/routes/http", func(r chi.Router) {
		r.Get("/domains", controller.GetManager().GetHTTPBindDomains)
		r.Get("/port", controller.GetManager().OpenOrCloseDomains)
		r.Get("/", controller.GetManager().GetHTTPAPIRoute)
		r.Post("/", controller.GetManager().CreateHTTPAPIRoute)
		r.Delete("/{name}", controller.GetManager().DeleteHTTPAPIRoute)

		//自动化签发证书
		r.Get("/cert-manager/check", controller.GetManager().CheckCertManager)
		r.Post("/cert-manager", controller.GetManager().CreateCertManager)
		r.Get("/cert-manager", controller.GetManager().GetCertManager)
		r.Delete("/cert-manager", controller.GetManager().DeleteCertManager)

	})

	// 创建 LoadBalancer 接口
	r.Route("/loadbalancer", func(r chi.Router) {
		r.Post("/", controller.GetManager().CreateLoadBalancer)
		r.Get("/", controller.GetManager().GetLoadBalancer)
		r.Post("/{name}", controller.GetManager().UpdateLoadBalancer)
		r.Delete("/{name}", controller.GetManager().DeleteLoadBalancer)
	})

	// 关于路由的接口
	r.Route("/routes/tcp", func(r chi.Router) {
		r.Get("/domains", controller.GetManager().GetTCPBindDomains)
		r.Get("/", controller.GetManager().GetTCPRoute)
		r.Post("/", controller.GetManager().CreateTCPRoute)
		r.Delete("/{name}", controller.GetManager().DeleteTCPRoute)
	})

	// 关于目标服务的接口
	r.Route("/service", func(r chi.Router) {
		r.Get("/", controller.GetManager().GetAPIService)
		r.Post("/{name}", controller.GetManager().CreateAPIService)
		r.Delete("/{name}", controller.GetManager().DeleteAPIService)
	})

	r.Route("/cert", func(r chi.Router) {
		r.Get("/", controller.GetManager().GetCert)
		r.Post("/{name}", controller.GetManager().CreateCert)
		r.Delete("/{name}", controller.GetManager().DeleteCert)
	})

	return r
}
