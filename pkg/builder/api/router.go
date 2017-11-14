package api

import (
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/pkg/builder/api/controller"
)

func APIServer() *chi.Mux {
	r := chi.NewRouter()

	r.Route("/api", func(r chi.Router) {
		//r.Get("/ping", controller.Ping)
		r.Post("/codecheck", controller.AddCodeCheck)
		r.Put("/codecheck/{serviceID}", controller.Update)
		r.Get("/codecheck/{serviceID}", controller.GetCodeCheck)

	})
	return r
}

