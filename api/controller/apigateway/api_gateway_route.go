package apigateway

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/api/util/bcode"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// OpenOrCloseDomains -
func (g Struct) OpenOrCloseDomains(w http.ResponseWriter, r *http.Request) {
	c := k8s.Default().ApiSixClient.ApisixV2()
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	list, _ := c.ApisixRoutes(tenant.Namespace).List(r.Context(), v1.ListOptions{
		LabelSelector: r.URL.Query().Get("service_alias") + "=service_alias" + ",port=" + r.URL.Query().Get("port"),
	})
	for _, itemL := range list.Items {
		item := itemL
		var plugins = item.Spec.HTTP[0].Plugins
		var newPlugins = make([]v2.ApisixRoutePlugin, 0)
		for _, plugin := range plugins {
			if plugin.Name != util.ResponseRewrite {
				newPlugins = append(newPlugins, plugin)
			}
		}

		if r.URL.Query().Get("act") == "close" {
			newPlugins = append(newPlugins, v2.ApisixRoutePlugin{
				Name:   util.ResponseRewrite,
				Enable: true,
				Config: map[string]interface{}{
					"status_code": 404,
					"body":        "请打开对外访问",
				},
			})
		}
		item.Spec.HTTP[0].Plugins = newPlugins
		item.Status = v2.ApisixStatus{}
		_, err := c.ApisixRoutes(tenant.Namespace).Update(r.Context(), &item, v1.UpdateOptions{})
		if err != nil {
			if errors.IsConflict(err) {
				logrus.Warnf("update route %v conflict", item.Name)
				continue
			}
			logrus.Errorf("update route %v failure: %v", item.Name, err)
			httputil.ReturnBcodeError(r, w, bcode.ErrRouteUpdate)
		}
	}
	httputil.ReturnSuccess(r, w, nil)
}

// GetHTTPBindDomains -
func (g Struct) GetHTTPBindDomains(w http.ResponseWriter, r *http.Request) {
	c := k8s.Default().ApiSixClient.ApisixV2()
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)

	list, err := c.ApisixRoutes(tenant.Namespace).List(r.Context(), v1.ListOptions{
		LabelSelector: r.URL.Query().Get("service_alias") + "=service_alias" + ",port=" + r.URL.Query().Get("port"),
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
			if plugin.Name == util.ResponseRewrite {
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

func (g Struct) GetTCPBindDomains(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)

	k := k8s.Default().Clientset.CoreV1()
	serviceAlias := r.URL.Query().Get("service_alias")
	labelSelector := fmt.Sprintf("tcp=true,service_alias=%v,outer=true" + serviceAlias)

	list, err := k.Services(tenant.Namespace).List(r.Context(), v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
		return
	}
	var resp []int32
	for _, v := range list.Items {
		resp = append(resp, v.Spec.Ports[0].NodePort)
	}
	httputil.ReturnSuccess(r, w, resp)
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
		labels := v.Labels
		service_alias := ""
		regionAppID := ""
		for labelK, labelV := range labels {
			if labelV == "service_alias" {
				service_alias = service_alias + "-" + labelK
			}
			if labelK == "app_id" {
				regionAppID = labelV
			}
		}
		httpRoute.Name = regionAppID + "|" + v.Name + "|" + service_alias
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
		if v.Name == util.ResponseRewrite {
			return apisixRouteHTTP
		}
	}
	apisixRouteHTTP.Plugins = append(apisixRouteHTTP.Plugins, v2.ApisixRoutePlugin{
		Name:   util.ResponseRewrite,
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
	sa := r.URL.Query().Get("service_alias")
	sLabel := strings.Split(sa, ",")
	// 如果没有绑定appId，那么不要加这个lable
	labels := make(map[string]string)
	labels["creator"] = "Rainbond"
	labels["port"] = r.URL.Query().Get("port")
	labels["component_sort"] = sa
	if r.URL.Query().Get("appID") != "" {
		labels["app_id"] = r.URL.Query().Get("appID")
	}
	defaultDomain := r.URL.Query().Get("default") == "true"

	for _, sl := range sLabel {
		labels[sl] = "service_alias"
	}

	c := k8s.Default().ApiSixClient.ApisixV2()

	routeName := apisixRouteHTTP.Match.Hosts[0] + apisixRouteHTTP.Match.Paths[0]

	routeName = strings.ReplaceAll(routeName, "/", "p-p")
	routeName = strings.ReplaceAll(routeName, "*", "s-s")
	name := r.URL.Query().Get("name")

	for _, host := range apisixRouteHTTP.Match.Hosts {
		labels[host] = "host"
		labelSelector := host + "=host"
		roueList, err := c.ApisixRoutes(tenant.Namespace).List(r.Context(), v1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			logrus.Errorf("list check route failure: %v", err)
			httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
			return
		}
		if roueList != nil && len(roueList.Items) > 0 && r.URL.Query().Get("intID")+roueList.Items[0].Name != name && !defaultDomain {
			logrus.Errorf("list check route failure: %v", err)
			httputil.ReturnBcodeError(r, w, bcode.ErrRouteExist)
			return
		}
	}

	apisixRouteHTTP.Name = uuid.NewV4().String()[0:8] //每次都让他变化，让 apisix controller去更新

	route, err := c.ApisixRoutes(tenant.Namespace).Create(r.Context(), &v2.ApisixRoute{
		TypeMeta: v1.TypeMeta{
			Kind:       util.ApisixRoute,
			APIVersion: util.APIVersion,
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
			name = removeLeadingDigits(name)
			err = c.ApisixRoutes(tenant.Namespace).Delete(r.Context(), name, v1.DeleteOptions{})
			if err != nil {
				logrus.Errorf("delete route %v failure: %v", name, err)
				httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
				return
			}
		}
		httputil.ReturnSuccess(r, w, marshalApisixRoute(route))
		return
	}
	logrus.Warnf("create route error %s, will update route", err.Error())
	// 创建失败去更新路由
	get, err := c.ApisixRoutes(tenant.Namespace).Get(r.Context(), routeName, v1.GetOptions{})
	if err != nil {
		logrus.Errorf("get route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
		return
	}
	if defaultDomain {
		httputil.ReturnSuccess(r, w, marshalApisixRoute(get))
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
	r.TypeMeta.Kind = util.ApisixRoute
	r.TypeMeta.APIVersion = util.APIVersion

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
	name = removeLeadingDigits(name)
	c := k8s.Default().ApiSixClient.ApisixV2()

	err := c.ApisixRoutes(tenant.Namespace).Delete(r.Context(), name, v1.DeleteOptions{})
	if err == nil {
		deleteName = append(deleteName, name)
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
	serviceID := r.URL.Query().Get("service_id")
	k := k8s.Default().Clientset.CoreV1()

	var apisixRouteStream v2.ApisixRouteStream
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &apisixRouteStream, nil) {
		return
	}

	serviceName := apisixRouteStream.Backend.ServiceName
	logrus.Infof("apisixRouteStream.Match.IngressPort is %v", apisixRouteStream.Match.IngressPort)
	if apisixRouteStream.Match.IngressPort == 0 {
		logrus.Infof("change ingressPort")
		h := handler.GetGatewayHandler()
		res, err := h.GetAvailablePort("0.0.0.0", true)
		if err != nil {
			logrus.Errorf("GetAvailablePort error %s", err.Error())
			httputil.ReturnBcodeError(r, w, bcode.ErrPortExists)
			return
		}
		apisixRouteStream.Match.IngressPort = int32(res)
	}
	name := fmt.Sprintf("%v-%v", serviceName, apisixRouteStream.Match.IngressPort)
	spec := corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Protocol:   corev1.Protocol(strings.ToUpper(apisixRouteStream.Protocol)),
				Name:       name,
				Port:       apisixRouteStream.Backend.ServicePort.IntVal,
				TargetPort: apisixRouteStream.Backend.ServicePort,
				NodePort:   apisixRouteStream.Match.IngressPort,
			},
		},
		Type: corev1.ServiceTypeNodePort,
	}
	// If not a third-party component, bind the service_alias
	if r.URL.Query().Get("service_type") != "third_party" {
		spec.Selector = map[string]string{
			"service_alias": serviceName,
		}
	} else {
		defer func() {
			// For third-party components, update the third component state
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
					_, err = k8s.Default().RainbondClient.RainbondV1alpha1().ThirdComponents(tenant.Namespace).Update(r.Context(), &v, v1.UpdateOptions{})
					if err != nil {
						logrus.Errorf("update third component failure: %v", err)
						httputil.ReturnBcodeError(r, w, bcode.ErrRouteUpdate)
						return
					}
				}
			}
		}()
	}

	// Try to get the existing service first
	service, err := k.Services(tenant.Namespace).Get(r.Context(), name, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			logrus.Errorf("get route error %s", err.Error())
			httputil.ReturnBcodeError(r, w, bcode.ErrPortExists)
			return
		}
		// Service doesn't exist, create a new one
		logrus.Infof("Service %s does not exist, creating a new service", name)
		labels := make(map[string]string)
		labels["creator"] = "Rainbond"
		labels["tcp"] = "true"
		labels["app_id"] = r.URL.Query().Get("appID")
		labels["service_id"] = r.URL.Query().Get("service_id")
		labels["service_alias"] = serviceName
		labels["outer"] = "true"
		labels["port"] = apisixRouteStream.Backend.ServicePort.String()
		service = &corev1.Service{
			ObjectMeta: v1.ObjectMeta{
				Labels: labels,
				Name:   name,
			},
			Spec: spec,
		}
		for {
			// 设置服务的 NodePort
			nodePort := service.Spec.Ports[0].NodePort
			// 创建服务
			_, err = k.Services(tenant.Namespace).Create(r.Context(), service, v1.CreateOptions{})
			if err != nil {
				if strings.Contains(err.Error(), "provided port is already allocated") {
					// 如果端口已被占用，增加端口号并重新尝试
					logrus.Infof("NodePort %d is already allocated, trying next port...", nodePort)
					nodePort++
					continue // 重新尝试创建服务
				} else {
					// 其他错误，返回失败
					logrus.Errorf("create tcp rule func, create svc failure: %s", err.Error())
					httputil.ReturnBcodeError(r, w, bcode.ErrPortExists)
					return
				}
			}
			apisixRouteStream.Match.IngressPort = nodePort
			// 如果创建成功，退出循环
			logrus.Infof("Service created successfully with NodePort %d", nodePort)
			break
		}
	} else {
		// Service exists, update it
		logrus.Infof("Service %s already exists, updating it", name)
		service.Spec = spec
		_, err = k.Services(tenant.Namespace).Update(r.Context(), service, v1.UpdateOptions{})
		if err != nil {
			logrus.Errorf("update route error %s", err.Error())
			httputil.ReturnBcodeError(r, w, bcode.ErrPortExists)
			return
		}
	}

	// Add or update the TCP rule in the database
	tcpRule := &dbmodel.TCPRule{
		UUID:          r.URL.Query().Get("service_id"),
		ServiceID:     r.URL.Query().Get("service_id"),
		ContainerPort: int(apisixRouteStream.Backend.ServicePort.IntVal),
		IP:            "0.0.0.0",
		Port:          int(apisixRouteStream.Match.IngressPort),
	}
	if err := db.GetManager().TCPRuleDao().AddModel(tcpRule); err != nil {
		logrus.Errorf("add tcp %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrPortExists)
		return
	}

	// Return success response with NodePort
	httputil.ReturnSuccess(r, w, service.Spec.Ports[0].NodePort)
	return
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
	err := k.Services(tenant.Namespace).Delete(r.Context(), name, v1.DeleteOptions{})
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

func removeLeadingDigits(name string) string {
	// 使用正则表达式匹配前面的数字
	re := regexp.MustCompile(`^\d+`)
	// 将匹配到的数字替换为空字符串
	return re.ReplaceAllString(name, "")
}
