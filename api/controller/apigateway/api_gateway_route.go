package apigateway

import (
	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/util/bcode"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"sigs.k8s.io/yaml"
	"strings"
)

// OpenOrCloseDomains -
func (g Struct) OpenOrCloseDomains(w http.ResponseWriter, r *http.Request) {
	c := k8s.Default().ApiSixClient.ApisixV2()
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	list, _ := c.ApisixRoutes(tenant.Namespace).List(r.Context(), v1.ListOptions{
		LabelSelector: "service_alias=" + r.URL.Query().Get("service_alias"),
	})
	for _, item := range list.Items {
		var plugins = item.Spec.HTTP[0].Plugins
		var newPlugins = make([]v2.ApisixRoutePlugin, 0)
		for _, plugin := range plugins {
			if plugin.Name != ResponseRewrite {
				newPlugins = append(newPlugins, plugin)
			}
		}

		if r.URL.Query().Get("act") == "close" {
			newPlugins = append(newPlugins, v2.ApisixRoutePlugin{
				Name:   ResponseRewrite,
				Enable: true,
				Config: map[string]interface{}{
					"status_code": 404,
					"body":        "请打开对外访问",
				},
			})
		}
		item.Spec.HTTP[0].Plugins = newPlugins
		item.Status = v2.ApisixStatus{}
		c.ApisixRoutes(tenant.Namespace).Update(r.Context(), &item, v1.UpdateOptions{})
		c.ApisixRoutes(tenant.Namespace).UpdateStatus(r.Context(), &item, v1.UpdateOptions{})
	}
	httputil.ReturnSuccess(r, w, nil)
}

// GetBindDomains -
func (g Struct) GetBindDomains(w http.ResponseWriter, r *http.Request) {
	c := k8s.Default().ApiSixClient.ApisixV2()
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)

	list, err := c.ApisixRoutes(tenant.Namespace).List(r.Context(), v1.ListOptions{
		LabelSelector: "service_alias=" + r.URL.Query().Get("service_alias"),
	})
	if err != nil {
		logrus.Errorf("get route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
		return
	}

	var hosts = make([]string, 0)
	for _, item := range list.Items {
		var has bool
		for _, plugin := range item.Spec.HTTP[0].Plugins {
			if plugin.Name == ResponseRewrite {
				has = true
				break
			}
		}
		if !has {
			hosts = append(hosts, item.Spec.HTTP[0].Match.Hosts[0])
		}
	}
	httputil.ReturnSuccess(r, w, hosts)
}

// GetHTTPAPIRoute -
func (g Struct) GetHTTPAPIRoute(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	var resp = make([]*v2.ApisixRouteHTTP, 0)

	c := k8s.Default().ApiSixClient.ApisixV2()
	appID := r.URL.Query().Get("appID")
	labelSelector := ""
	if appID != "" {
		labelSelector = "app_id=" + appID
	}
	list, err := c.ApisixRoutes(tenant.Namespace).List(r.Context(), v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		logrus.Errorf("get route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
		return
	}
	for _, v := range list.Items {
		httpRoute := v.Spec.HTTP[0].DeepCopy()
		httpRoute.Name = v.Name + "|" + v.ObjectMeta.Labels["service_alias"]
		resp = append(resp, httpRoute)
	}
	httputil.ReturnSuccess(r, w, resp)
}

// UpdateHTTPAPIRoute -
func (g Struct) UpdateHTTPAPIRoute(w http.ResponseWriter, r *http.Request) {
	panic("implement me")
}

func addResponseRewritePlugin(apisixRouteHTTP v2.ApisixRouteHTTP) v2.ApisixRouteHTTP {
	for _, v := range apisixRouteHTTP.Plugins {
		if v.Name == ResponseRewrite {
			return apisixRouteHTTP
		}
	}
	apisixRouteHTTP.Plugins = append(apisixRouteHTTP.Plugins, v2.ApisixRoutePlugin{
		Name:   ResponseRewrite,
		Enable: false,
		Config: map[string]interface{}{
			"status_code": 404,
		},
	})
	return apisixRouteHTTP
}

// CreateHTTPAPIRoute -
func (g Struct) CreateHTTPAPIRoute(w http.ResponseWriter, r *http.Request) {

	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	var apisixRouteHTTP v2.ApisixRouteHTTP
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &apisixRouteHTTP, nil) {
		return
	}
	s := strings.ReplaceAll(r.URL.Query().Get("service_alias"), ",", "-")

	// 如果没有绑定appId，那么不要加这个lable
	labels := make(map[string]string)
	labels["creator"] = "Rainbond"
	if r.URL.Query().Get("appID") != "" {
		labels["app_id"] = r.URL.Query().Get("appID")
	}
	if s != "" {
		labels["service_alias"] = s
	}
	c := k8s.Default().ApiSixClient.ApisixV2()

	routeName := r.URL.Query().Get("intID") + apisixRouteHTTP.Match.Hosts[0] + apisixRouteHTTP.Match.Paths[0]

	routeName = strings.ReplaceAll(routeName, "/", "p-p")
	routeName = strings.ReplaceAll(routeName, "*", "s-s")
	if len(routeName) > 30 {
		apisixRouteHTTP.Name = routeName[:30]
	} else {
		apisixRouteHTTP.Name = routeName
	}

	route, err := c.ApisixRoutes(tenant.Namespace).Create(r.Context(), &v2.ApisixRoute{
		TypeMeta: v1.TypeMeta{
			Kind:       ApisixRoute,
			APIVersion: APIVersion,
		},
		ObjectMeta: v1.ObjectMeta{
			Labels:       labels,
			Name:         routeName,
			GenerateName: "rbd",
		},
		Spec: v2.ApisixRouteSpec{
			IngressClassName: "apisix",
			HTTP: []v2.ApisixRouteHTTP{
				apisixRouteHTTP,
			},
		},
	}, v1.CreateOptions{})
	if err == nil {
		name := r.URL.Query().Get("name")
		if name != "" {
			c.ApisixRoutes(tenant.Namespace).Delete(r.Context(), name, v1.DeleteOptions{})
		}
		httputil.ReturnSuccess(r, w, marshalApisixRoute(route))
		return
	}
	// 创建失败去更新路由
	get, err := c.ApisixRoutes(tenant.Namespace).Get(r.Context(), routeName, v1.GetOptions{})
	if err != nil {
		logrus.Errorf("get route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
		return
	}
	get.Spec.HTTP[0] = apisixRouteHTTP
	get.ObjectMeta.Labels = labels

	update, err := c.ApisixRoutes(tenant.Namespace).Update(r.Context(), get, v1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteUpdate)
		return
	}
	httputil.ReturnSuccess(r, w, marshalApisixRoute(update))
}

func marshalApisixRoute(r *v2.ApisixRoute) map[string]interface{} {
	r.TypeMeta.Kind = ApisixRoute
	r.TypeMeta.APIVersion = APIVersion

	r.ObjectMeta.ManagedFields = nil
	resp := make(map[string]interface{})
	contentBytes, _ := yaml.Marshal(r)
	resp["name"] = r.Name
	resp["kind"] = r.TypeMeta.Kind
	resp["content"] = string(contentBytes)
	return resp
}

// DeleteHTTPAPIRoute -
func (g Struct) DeleteHTTPAPIRoute(w http.ResponseWriter, r *http.Request) {

	var deleteName = make([]string, 0)
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	name := chi.URLParam(r, "name")

	c := k8s.Default().ApiSixClient.ApisixV2()

	err := c.ApisixRoutes(tenant.Namespace).Delete(r.Context(), name, v1.DeleteOptions{})
	if err == nil {
		deleteName = append(deleteName, name)
		httputil.ReturnSuccess(r, w, deleteName)
		return
	}
	if errors.IsNotFound(err) {
		list, _ := c.ApisixRoutes(tenant.Namespace).List(r.Context(), v1.ListOptions{
			LabelSelector: "host=" + name,
		})

		for _, item := range list.Items {
			c.ApisixRoutes(tenant.Namespace).Delete(r.Context(), item.Spec.HTTP[0].Name, v1.DeleteOptions{})
			deleteName = append(deleteName, item.Spec.HTTP[0].Name)

		}
		httputil.ReturnSuccess(r, w, deleteName)
		return
	}
	logrus.Errorf("delete route error %s", err.Error())
	httputil.ReturnBcodeError(r, w, bcode.ErrRouteDelete)
}

// GetTCPRoute -
func (g Struct) GetTCPRoute(w http.ResponseWriter, r *http.Request) {

	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)

	k := k8s.Default().Clientset.CoreV1()

	appID := r.URL.Query().Get("appID")
	labelSelector := "tcp=true"
	if appID != "" {
		labelSelector += ",app_id=" + appID
	}

	list, err := k.Services(tenant.Namespace).List(r.Context(), v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return
	}
	if err != nil {
		logrus.Errorf("get route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
		return
	}
	var resp []corev1.ServicePort
	for _, v := range list.Items {
		resp = append(resp, v.Spec.Ports[0])
	}
	httputil.ReturnSuccess(r, w, resp)
}

// CreateTCPRoute -
func (g Struct) CreateTCPRoute(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)

	k := k8s.Default().Clientset.CoreV1()

	var apisixRouteStream v2.ApisixRouteStream
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &apisixRouteStream, nil) {
		return
	}

	serviceName := apisixRouteStream.Backend.ServiceName
	spec := corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Protocol:   "TCP",
				Name:       serviceName,
				Port:       apisixRouteStream.Backend.ServicePort.IntVal,
				TargetPort: apisixRouteStream.Backend.ServicePort,
				NodePort:   apisixRouteStream.Match.IngressPort,
			},
		},
		Type: "NodePort",
	}
	serviceID := r.URL.Query().Get("service_id")

	// 如果不是第三方组件，需要绑定 service_alias，第三方组件会从ep中自动读取
	if r.URL.Query().Get("service_type") != "third_party" {
		spec.Selector = map[string]string{
			"service_alias": serviceName,
		}
	} else {
		// 创建一个空的endpoint
		k.Endpoints(tenant.Namespace).Create(r.Context(), &corev1.Endpoints{
			ObjectMeta: v1.ObjectMeta{
				Name: serviceName + "-tcp",
				Labels: map[string]string{
					"tcp":        "true",
					"app_id":     r.URL.Query().Get("appID"),
					"service_id": r.URL.Query().Get("service_id"),
				},
			},
		}, v1.CreateOptions{})

		// 找到这个第三方组件，去更新状态
		list, err := k8s.Default().RainbondClient.RainbondV1alpha1().ThirdComponents(tenant.Namespace).List(r.Context(), v1.ListOptions{
			LabelSelector: "service_id=" + serviceID,
		})
		if err != nil {
			logrus.Errorf("get route error %s", err.Error())
			httputil.ReturnBcodeError(r, w, bcode.ErrRouteUpdate)
			return
		}
		for _, v := range list.Items {
			for i := range v.Spec.Ports {
				v.Spec.Ports[i].OpenOuter = !v.Spec.Ports[i].OpenOuter
				k8s.Default().RainbondClient.RainbondV1alpha1().ThirdComponents(tenant.Namespace).Update(r.Context(), &v, v1.UpdateOptions{})
			}
		}
	}
	e, err := k.Services(tenant.Namespace).Create(r.Context(), &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				"tcp":        "true",
				"app_id":     r.URL.Query().Get("appID"),
				"service_id": r.URL.Query().Get("service_id"),
			},
			Name: serviceName + "-tcp",
		},
		Spec: spec,
	}, v1.CreateOptions{})
	if err == nil {
		httputil.ReturnSuccess(r, w, e.Spec.Ports[0].NodePort)
		return
	}
	get, err := k.Services(tenant.Namespace).Get(r.Context(), serviceName+"-tcp", v1.GetOptions{})
	if err != nil {
		logrus.Errorf("get route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteUpdate)
		return
	}
	get.Spec = spec
	update, err := k.Services(tenant.Namespace).Update(r.Context(), get, v1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteUpdate)
		return
	}
	httputil.ReturnSuccess(r, w, update.Spec.Ports[0].NodePort)
}

// UpdateTCPRoute -
func (g Struct) UpdateTCPRoute(w http.ResponseWriter, r *http.Request) {
	//TODO implement me
	panic("implement me")
}

// DeleteTCPRoute -
func (g Struct) DeleteTCPRoute(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	name := chi.URLParam(r, "name")

	k := k8s.Default().Clientset.CoreV1()
	err := k.Services(tenant.Namespace).Delete(r.Context(), name+"-tcp", v1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			httputil.ReturnSuccess(r, w, name)
		} else {
			logrus.Errorf("delete route error %s", err.Error())
			httputil.ReturnBcodeError(r, w, bcode.ErrRouteDelete)
		}
		return
	}
	httputil.ReturnSuccess(r, w, name)
}
