package apigateway

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	v13 "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	v12 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	kbutil "github.com/goodrain/rainbond/util/kubeblocks"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

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
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// OpenOrCloseDomains -
func (g Struct) OpenOrCloseDomains(w http.ResponseWriter, r *http.Request) {
	c := k8s.Default().ApiSixClient.ApisixV2()
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	serviceAlias := r.URL.Query().Get("service_alias")
	// Only keep the value before the comma
	if idx := strings.Index(serviceAlias, ","); idx != -1 {
		serviceAlias = serviceAlias[:idx]
	}
	list, _ := c.ApisixRoutes(tenant.Namespace).List(r.Context(), v1.ListOptions{
		LabelSelector: serviceAlias + "=service_alias" + ",port=" + r.URL.Query().Get("port"),
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

	serviceAlias := r.URL.Query().Get("service_alias")
	// Only keep the value before the comma
	if idx := strings.Index(serviceAlias, ","); idx != -1 {
		serviceAlias = serviceAlias[:idx]
	}
	list, err := c.ApisixRoutes(tenant.Namespace).List(r.Context(), v1.ListOptions{
		LabelSelector: serviceAlias + "=service_alias" + ",port=" + r.URL.Query().Get("port"),
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
	port := r.URL.Query().Get("port")
	// Only keep the value before the comma
	if idx := strings.Index(serviceAlias, ","); idx != -1 {
		serviceAlias = serviceAlias[:idx]
	}
	labelSelector := fmt.Sprintf("tcp=true,service_alias=%v,outer=true", serviceAlias)

	// If port is specified, filter by port label to get only the specific port's NodePort
	if port != "" {
		labelSelector = fmt.Sprintf("tcp=true,service_alias=%v,outer=true,port=%v", serviceAlias, port)
	}

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

	type routeResponse struct {
		*v2.ApisixRouteHTTP
		Enabled     bool   `json:"enabled"`
		RegionAppID string `json:"region_app_id"`
	}

	var resp = make([]*routeResponse, 0)

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
		enabled := false // Default to enabled if not specified
		for labelK, labelV := range labels {
			if labelV == "service_alias" {
				service_alias = service_alias + "-" + labelK
			}
			if labelK == "app_id" {
				regionAppID = labelV
			}
			if labelK == "cert-manager-enabled" {
				enabled = labelV == "true"
			}
		}
		httpRoute.Name = regionAppID + "|" + v.Name + "|" + service_alias
		resp = append(resp, &routeResponse{
			ApisixRouteHTTP: httpRoute,
			Enabled:         enabled,
			RegionAppID:     regionAppID,
		})
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
	// Only keep the value before the comma
	if idx := strings.Index(sa, ","); idx != -1 {
		sa = sa[:idx]
	}
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
		if sl != "" {
			labels[sl] = "service_alias"
		}
	}

	c := k8s.Default().ApiSixClient.ApisixV2()

	routeName := strings.ToLower(strings.ReplaceAll(apisixRouteHTTP.Match.Hosts[0], "*", "wildcard") + apisixRouteHTTP.Match.Paths[0])

	routeName = strings.ReplaceAll(routeName, "/", "p-p")
	routeName = strings.ReplaceAll(routeName, "*", "s-s")
	routeName = strings.ReplaceAll(routeName, "_", "")
	//name := r.URL.Query().Get("name")

	for _, host := range apisixRouteHTTP.Match.Hosts {
		safeHost := sanitizeLabelKey(host)
		labels[safeHost] = "host"
		//labelSelector := host + "=host"
		//roueList, err := c.ApisixRoutes(tenant.Namespace).List(r.Context(), v1.ListOptions{
		//	LabelSelector: labelSelector,
		//})
		//if err != nil {
		//	logrus.Errorf("list check route failure: %v", err)
		//	httputil.ReturnBcodeError(r, w, bcode.ErrRouteNotFound)
		//	return
		//}
		//parts := strings.Split(name, "-")
		//bName := strings.Join(parts[:len(parts)-1], "-")
		//if roueList != nil && len(roueList.Items) > 0 && r.URL.Query().Get("intID")+roueList.Items[0].Name != bName && !defaultDomain {
		//	logrus.Errorf("list check route failure: %v", err)
		//	httputil.ReturnBcodeError(r, w, bcode.ErrRouteExist)
		//	return
		//}
	}

	apisixRouteHTTP.Name = uuid.New().String()[0:8] //每次都让他变化，让 apisix controller去更新

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
	if get.ObjectMeta.Labels["cert-manager-enabled"] == "true" {
		labels["cert-manager-enabled"] = "true"
	}
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

	// kubeblocks_component should use specific selector
	rbdService, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		logrus.Errorf("get service by id %s error: %v", serviceID, err)
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteUpdate)
		return
	}
	if rbdService.ExtendMethod == "kubeblocks_component" {
		spec.Selector = kbutil.GenerateKubeBlocksSelector(rbdService.K8sComponentName)
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
					httputil.ReturnBcodeError(r, w, fmt.Errorf("create tcp rule func, create svc failure: %s", err.Error()))
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

	// Get the Service first to verify it exists and log its labels for debugging
	service, err := k.Services(tenant.Namespace).Get(r.Context(), name, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Infof("Service %s not found, treating as already deleted", name)
			httputil.ReturnSuccess(r, w, name)
			return
		}
		logrus.Errorf("failed to get service %s: %v", name, err)
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteDelete)
		return
	}

	// Log the Service details for debugging
	logrus.Infof("Deleting TCP route Service: %s, labels: %v, port: %v",
		name, service.Labels, service.Spec.Ports[0].Port)

	// Delete the Service
	err = k.Services(tenant.Namespace).Delete(r.Context(), name, v1.DeleteOptions{})
	if err != nil {
		logrus.Errorf("delete route error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ErrRouteDelete)
		return
	}

	logrus.Infof("Successfully deleted TCP route Service: %s", name)
	httputil.ReturnSuccess(r, w, name)
}

func removeLeadingDigits(name string) string {
	// 使用正则表达式移除开头的数字
	re := regexp.MustCompile(`^\d+`)
	name = re.ReplaceAllString(name, "")

	// 按照 "-" 切割
	parts := strings.Split(name, "-")

	// 如果切割后长度小于等于1，直接返回空字符串
	if len(parts) <= 1 {
		return ""
	}

	// 如果最后一个部分是 "s"，直接返回整个字符串
	if parts[len(parts)-1] == "s" {
		return name
	}

	// 移除最后一个部分并重新拼接
	return strings.Join(parts[:len(parts)-1], "-")
}

func (g Struct) CheckCertManager(w http.ResponseWriter, r *http.Request) {
	// 创建 Kubernetes 客户端
	kubeConfig := config.GetConfigOrDie()
	apiextensionsClient, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		logrus.Errorf("failed to create apiextensions client: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("failed to create apiextensions client: %v", err))
		return
	}

	// 检查 certificates.cert-manager.io CRD 是否存在
	crdName := "certificates.cert-manager.io"
	_, err = apiextensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(r.Context(), crdName, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			// CRD 不存在
			httputil.ReturnSuccess(r, w, map[string]interface{}{
				"exists":  false,
				"message": "Certificate CRD not found. cert-manager may not be installed.",
			})
			return
		}
		// 其他错误
		logrus.Errorf("error checking Certificate CRD: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("error checking Certificate CRD: %v", err))
		return
	}

	// CRD 存在
	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"exists":  true,
		"message": "Certificate CRD exists. cert-manager is installed.",
	})
}

// CreateCertManager creates cert-manager resources and updates apisix route labels
func (g Struct) CreateCertManager(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	// Parse request body
	var req struct {
		RouteName   string   `json:"route_name"`
		Domains     []string `json:"domains"`
		RegionAppID string   `json:"region_app_id"`
	}
	if err := httputil.ReadEntity(r, &req); err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	resourceLabel := make(map[string]string)
	resourceLabel["app_id"] = req.RegionAppID
	// Validate request
	if len(req.Domains) == 0 {
		httputil.ReturnError(r, w, 400, "domains cannot be empty")
		return
	}
	req.RouteName = removeLeadingDigits(req.RouteName)
	cert := &cmapi.Certificate{
		ObjectMeta: v1.ObjectMeta{
			Name:      req.RouteName,
			Namespace: tenant.Namespace,
			Labels:    resourceLabel,
		},
		Spec: cmapi.CertificateSpec{
			DNSNames:   req.Domains,
			SecretName: req.RouteName,
			IssuerRef: v12.ObjectReference{
				Kind: "ClusterIssuer",
				Name: "letsencrypt-http",
			},
		},
	}

	// Create Certificate using controller-runtime client
	scheme := runtime.NewScheme()
	_ = cmapi.AddToScheme(scheme)
	kubeConfig := config.GetConfigOrDie()
	k8sClient, err := client.New(kubeConfig, client.Options{Scheme: scheme})
	if err != nil {
		logrus.Errorf("failed to create k8s client: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("failed to create k8s client: %v", err))
		return
	}

	// Create Certificate
	err = k8sClient.Create(r.Context(), cert)
	if err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("create certificate error: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("create certificate error: %v", err))
		return
	}

	// Create ApisixTls resource
	apisixTls := &v2.ApisixTls{
		TypeMeta: v1.TypeMeta{
			Kind:       util.ApisixTLS,
			APIVersion: util.APIVersion,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      req.RouteName,
			Namespace: tenant.Namespace,
			Labels:    resourceLabel,
		},
		Spec: &v2.ApisixTlsSpec{
			IngressClassName: "apisix",
			Hosts: func() []v2.HostType {
				hosts := make([]v2.HostType, len(req.Domains))
				for i, domain := range req.Domains {
					hosts[i] = v2.HostType(domain)
				}
				return hosts
			}(),
			Secret: v2.ApisixSecret{
				Name:      req.RouteName,
				Namespace: tenant.Namespace,
			},
		},
	}

	// Create the ApisixTls resource
	c := k8s.Default().ApiSixClient.ApisixV2()
	_, err = c.ApisixTlses(tenant.Namespace).Create(r.Context(), apisixTls, v1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("create certificate error: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("create certificate error: %v", err))
		return
	}

	// Update ApisixRoute with cert-manager label
	route, err := c.ApisixRoutes(tenant.Namespace).Get(r.Context(), req.RouteName, v1.GetOptions{})
	if err != nil {
		logrus.Errorf("get apisix route error: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get apisix route error: %v", err))
		return
	}

	// Add or update cert-manager label
	if route.Labels == nil {
		route.Labels = make(map[string]string)
	}
	route.Labels["cert-manager-enabled"] = "true"

	// Update the route
	_, err = c.ApisixRoutes(tenant.Namespace).Update(r.Context(), route, v1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update apisix route error: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("update apisix route error: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, nil)
}

// GetCertManager 获取证书管理器信息
func (g Struct) GetCertManager(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)

	// 定义响应结构
	type CertificateInfo struct {
		Domains     []string  `json:"domains"`      // 域名列表
		Status      string    `json:"status"`       // 签发状态
		ExpiryDate  time.Time `json:"expiry_date"`  // 过期时间
		AutoRenew   bool      `json:"auto_renew"`   // 自动续签
		IssueDetail string    `json:"issue_detail"` // 签发详情
		Name        string    `json:"name"`         // 证书名称
	}

	// 设置 scheme
	scheme := runtime.NewScheme()
	if err := cmapi.AddToScheme(scheme); err != nil {
		logrus.Errorf("failed to add cert-manager scheme: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("failed to add cert-manager scheme: %v", err))
		return
	}
	// 添加 ACME scheme
	if err := v13.AddToScheme(scheme); err != nil {
		logrus.Errorf("failed to add acme scheme: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("failed to add acme scheme: %v", err))
		return
	}

	// 创建 k8s 客户端
	kubeConfig := config.GetConfigOrDie()
	k8sClient, err := client.New(kubeConfig, client.Options{Scheme: scheme})
	if err != nil {
		logrus.Errorf("failed to create k8s client: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("failed to create k8s client: %v", err))
		return
	}

	// 获取 app_id 参数
	appID := r.URL.Query().Get("region_app_id")
	if appID == "" {
		httputil.ReturnError(r, w, 400, "region_app_id is required")
		return
	}

	selector, _ := labels.Parse(labels.FormatLabels(map[string]string{
		"app_id": appID,
	}))

	// 获取证书列表
	certList := &cmapi.CertificateList{}
	err = k8sClient.List(r.Context(), certList, &client.ListOptions{
		Namespace:     tenant.Namespace,
		LabelSelector: selector,
	})

	if err != nil {
		logrus.Errorf("list certificates error: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("list certificates error: %v", err))
		return
	}

	// 获取所有的 Challenge 资源
	challengeList := &v13.ChallengeList{}
	err = k8sClient.List(r.Context(), challengeList, &client.ListOptions{
		Namespace: tenant.Namespace,
	})
	if err != nil {
		logrus.Errorf("list challenges error: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("list challenges error: %v", err))
		return
	}

	// 创建 Challenge 映射表
	challengeMap := make(map[string]*v13.Challenge)
	for i := range challengeList.Items {
		challenge := &challengeList.Items[i]
		// 提取基础名称（去除后缀）
		baseName := extractBaseName(challenge.Name)
		challengeMap[baseName] = challenge
	}

	// 构建响应信息列表
	var certInfoList []CertificateInfo
	for _, cert := range certList.Items {
		certInfo := CertificateInfo{
			Name:      cert.Name,
			Domains:   cert.Spec.DNSNames,
			AutoRenew: true, // cert-manager 默认会自动续签
		}

		// 获取证书状态
		if len(cert.Status.Conditions) > 0 {
			for _, condition := range cert.Status.Conditions {
				if condition.Type == cmapi.CertificateConditionReady {
					certInfo.Status = string(condition.Status)
					certInfo.IssueDetail = condition.Message
					break
				}
			}
		}

		// 获取过期时间
		if cert.Status.NotAfter != nil {
			certInfo.ExpiryDate = cert.Status.NotAfter.Time
		}

		// 查找对应的 Challenge 并补充详细信息
		if challenge, exists := challengeMap[cert.Name]; exists {
			// 如果证书未就绪，使用 Challenge 的状态信息
			if certInfo.Status != "True" {
				certInfo.IssueDetail = fmt.Sprintf("%s: %s",
					challenge.Status.State,
					challenge.Status.Reason)

				// 如果有详细错误信息，添加到详情中
				if challenge.Status.Processing {
					certInfo.IssueDetail = fmt.Sprintf("%v\nProcessing: %v", certInfo.IssueDetail, challenge.Status.Presented)
				}
			}
		}

		certInfoList = append(certInfoList, certInfo)
	}

	httputil.ReturnSuccess(r, w, certInfoList)
}

func (g Struct) DeleteCertManager(w http.ResponseWriter, r *http.Request) {
	// 从上下文中获取租户信息
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)

	// 解析请求参数
	RouteName := r.URL.Query().Get("route_name")

	// 验证路由名称
	if RouteName == "" {
		httputil.ReturnError(r, w, 400, "route_name is required")
		return
	}

	RouteName = removeLeadingDigits(RouteName)

	// 删除 Certificate 资源
	scheme := runtime.NewScheme()
	_ = cmapi.AddToScheme(scheme)
	kubeConfig := config.GetConfigOrDie()
	k8sClient, err := client.New(kubeConfig, client.Options{Scheme: scheme})
	if err != nil {
		logrus.Errorf("failed to create k8s client: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("failed to create k8s client: %v", err))
		return
	}

	// 删除 Certificate
	cert := &cmapi.Certificate{
		ObjectMeta: v1.ObjectMeta{
			Name:      RouteName,
			Namespace: tenant.Namespace,
		},
	}
	err = k8sClient.Delete(r.Context(), cert)
	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("delete certificate error: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("delete certificate error: %v", err))
		return
	}

	// 删除 ApisixTls 资源
	c := k8s.Default().ApiSixClient.ApisixV2()
	err = c.ApisixTlses(tenant.Namespace).Delete(r.Context(), RouteName, v1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("delete apisix tls error: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("delete apisix tls error: %v", err))
		return
	}

	// 更新 ApisixRoute，移除 cert-manager 标签
	route, err := c.ApisixRoutes(tenant.Namespace).Get(r.Context(), RouteName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// 如果路由不存在，返回成功
			httputil.ReturnSuccess(r, w, nil)
			return
		}
		logrus.Errorf("get apisix route error: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get apisix route error: %v", err))
		return
	}

	// 移除 cert-manager 标签
	if route.Labels != nil {
		delete(route.Labels, "cert-manager-enabled")

		// 更新路由
		_, err = c.ApisixRoutes(tenant.Namespace).Update(r.Context(), route, v1.UpdateOptions{})
		if err != nil {
			logrus.Errorf("update apisix route error: %v", err)
			httputil.ReturnError(r, w, 500, fmt.Sprintf("update apisix route error: %v", err))
			return
		}
	}

	httputil.ReturnSuccess(r, w, nil)
}

// extractBaseName 从 Challenge 名称中提取基础名称
func extractBaseName(challengeName string) string {
	// 按照 "-" 分割
	parts := strings.Split(challengeName, "-")

	// 如果部分数量小于4，返回原始名称
	if len(parts) < 4 {
		return challengeName
	}

	// 移除最后三个部分（数字后缀）
	return strings.Join(parts[:len(parts)-3], "-")
}

// 新增：label key 合法化函数
func sanitizeLabelKey(key string) string {
	// 这里将 * 替换为 wildcard
	key = strings.ReplaceAll(key, "*", "wildcard")
	return key
}
