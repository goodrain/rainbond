package api_gateway

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

func (g APIGatewayStruct) GetRBDService(w http.ResponseWriter, r *http.Request) {
	//TODO implement me
	c := handler.GetAPIGatewayHandler().GetK8sClient()
	list, err := c.CoreV1().Services(r.URL.Query().Get("namespace")).List(r.Context(), v1.ListOptions{})
	if err != nil {
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

func (g APIGatewayStruct) GetAPIService(w http.ResponseWriter, r *http.Request) {
	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()

	list, err := c.ApisixUpstreams(r.URL.Query().Get("namespace")).List(r.Context(), v1.ListOptions{})
	if err != nil {
		logrus.Errorf("get route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFount)
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

func (g APIGatewayStruct) UpdateAPIService(w http.ResponseWriter, r *http.Request) {
	var spec v2.ApisixUpstreamSpec
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &spec, nil) {
		return
	}
	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()
	get, err := c.ApisixUpstreams(r.URL.Query().Get("namespace")).Get(r.Context(), chi.URLParam(r, "name"), v1.GetOptions{})
	if err != nil {
		logrus.Errorf("get service error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrServiceNotFount)
		return
	}
	get.Spec = &spec
	update, err := c.ApisixUpstreams(r.URL.Query().Get("namespace")).Update(r.Context(), get, v1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update service error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrServiceUpdate)
		return
	}
	httputil.ReturnSuccess(r, w, update)
}

func (g APIGatewayStruct) CreateAPIService(w http.ResponseWriter, r *http.Request) {
	var spec v2.ApisixUpstreamSpec
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &spec, nil) {
		return
	}
	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()
	create, err := c.ApisixUpstreams(r.URL.Query().Get("namespace")).Create(r.Context(), &v2.ApisixUpstream{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "rbd",
		},
		Spec: &spec,
	}, v1.CreateOptions{})
	if err != nil {
		logrus.Errorf("create service error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrServiceCreate)
		return
	}
	httputil.ReturnSuccess(r, w, create)
}

func (g APIGatewayStruct) DeleteAPIService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	c := handler.GetAPIGatewayHandler().GetClient().ApisixV2()
	err := c.ApisixUpstreams(r.URL.Query().Get("namespace")).Delete(r.Context(), name, v1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			httputil.ReturnSuccess(r, w, nil)
		} else {
			logrus.Errorf("delete service error %s", err.Error())
			httputil.ReturnBcodeError(r, w, bcode.ErrServiceDelete)
		}
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
