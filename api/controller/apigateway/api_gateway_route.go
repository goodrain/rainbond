package apigateway

import (
	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/util/bcode"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

// UpdateAPIRoute -
func (g Struct) UpdateAPIRoute(w http.ResponseWriter, r *http.Request) {
	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()
	get, err := c.ApisixRoutes(r.URL.Query().Get("namespace")).Get(r.Context(), chi.URLParam(r, "name"), v1.GetOptions{})
	if err != nil {
		logrus.Errorf("get route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
		return
	}
	var spec v2.ApisixRouteSpec
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &spec, nil) {
		return
	}

	get.Spec = spec

	update, err := c.ApisixRoutes(r.URL.Query().Get("namespace")).Update(r.Context(), get, v1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteUpdate)
		return
	}
	httputil.ReturnSuccess(r, w, update)
}

// GetAPIRoute -
func (g Struct) GetAPIRoute(w http.ResponseWriter, r *http.Request) {
	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()

	list, err := c.ApisixRoutes(r.URL.Query().Get("namespace")).List(r.Context(), v1.ListOptions{})
	if err != nil {
		logrus.Errorf("get route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
		return
	}

	resp := make([]responseBody, 0)
	for _, item := range list.Items {
		resp = append(resp, responseBody{
			Name: item.Name,
			Body: item.Spec,
		})
	}

	httputil.ReturnSuccess(r, w, resp)
}

// CreateAPIRoute -
func (g Struct) CreateAPIRoute(w http.ResponseWriter, r *http.Request) {
	var spec v2.ApisixRouteSpec
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &spec, nil) {
		return
	}
	spec.IngressClassName = "apisix"

	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()
	var name string //从路由设置中拿到名称

	if len(spec.HTTP) > 0 {
		name = spec.HTTP[0].Name
	} else if len(spec.Stream) > 0 {
		name = spec.Stream[0].Name
	}
	route, err := c.ApisixRoutes(r.URL.Query().Get("namespace")).Create(r.Context(), &v2.ApisixRoute{
		ObjectMeta: v1.ObjectMeta{
			Name:         name,
			GenerateName: "rbd",
		},
		Spec: spec,
	}, v1.CreateOptions{})
	if err != nil {
		logrus.Errorf("create route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteCreate)
		return
	}
	httputil.ReturnSuccess(r, w, route)
}

// DeleteAPIRoute -
func (g Struct) DeleteAPIRoute(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()

	err := c.ApisixRoutes(r.URL.Query().Get("namespace")).Delete(r.Context(), name, v1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			httputil.ReturnSuccess(r, w, nil)
		} else {
			logrus.Errorf("delete route error %s", err.Error())
			httputil.ReturnBcodeError(r, w, bcode.ErrRouteDelete)
		}
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
