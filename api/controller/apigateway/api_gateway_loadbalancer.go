// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package apigateway

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// CreateLoadBalancer 创建LoadBalancer服务
func (g Struct) CreateLoadBalancer(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	serviceID := r.URL.Query().Get("service_id")
	appID := r.URL.Query().Get("appID")
	k := k8s.Default().Clientset.CoreV1()

	var createLBReq model.CreateLoadBalancerStruct
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &createLBReq, nil) {
		return
	}

	// 验证协议类型
	protocol := strings.ToUpper(createLBReq.Protocol)
	if protocol != "TCP" && protocol != "UDP" {
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("protocol must be TCP or UDP"))
		return
	}

	// 生成服务名称
	serviceName := fmt.Sprintf("%s-lb-%d", createLBReq.ServiceName, createLBReq.ServicePort)

	// 检查服务是否已存在
	_, err := k.Services(tenant.Namespace).Get(r.Context(), serviceName, v1.GetOptions{})
	if err == nil {
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("LoadBalancer service already exists"))
		return
	}
	if !errors.IsNotFound(err) {
		logrus.Errorf("check LoadBalancer service error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ServerErr)
		return
	}

	// 创建服务规格
	spec := corev1.ServiceSpec{
		Type: corev1.ServiceTypeLoadBalancer,
		Ports: []corev1.ServicePort{
			{
				Name:       serviceName,
				Protocol:   corev1.Protocol(protocol),
				Port:       int32(createLBReq.ServicePort),
				TargetPort: intstr.FromInt(createLBReq.ServicePort),
			},
		},
	}

	// 直接生成服务选择器
	spec.Selector = map[string]string{
		"service_alias": createLBReq.ServiceName,
	}

	// 创建标签
	labels := make(map[string]string)
	labels["creator"] = "Rainbond"
	labels["loadbalancer"] = "true"
	if appID != "" {
		labels["app_id"] = appID
	}
	if serviceID != "" {
		labels["service_id"] = serviceID
	}
	labels["service_alias"] = createLBReq.ServiceName
	labels["port"] = fmt.Sprintf("%d", createLBReq.ServicePort)

	// 创建注解
	annotations := make(map[string]string)
	if createLBReq.Annotations != nil {
		for k, v := range createLBReq.Annotations {
			annotations[k] = v
		}
	}

	// 创建LoadBalancer服务
	service := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:        serviceName,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: spec,
	}

	createdService, err := k.Services(tenant.Namespace).Create(r.Context(), service, v1.CreateOptions{})
	if err != nil {
		logrus.Errorf("create LoadBalancer service error %s", err.Error())
		httputil.ReturnBcodeError(r, w, fmt.Errorf("create LoadBalancer service error: %s", err.Error()))
		return
	}

	// 构造响应
	response := &model.LoadBalancerResponse{
		Name:           createdService.Name,
		Namespace:      createdService.Namespace,
		ServiceName:    createLBReq.ServiceName,
		ServicePort:    createLBReq.ServicePort,
		Protocol:       protocol,
		LoadBalancerIP: createdService.Spec.LoadBalancerIP,
		ExternalIPs:    createdService.Spec.ExternalIPs,
		Annotations:    createdService.Annotations,
		Status:         "Creating",
		CreatedAt:      createdService.CreationTimestamp.Format(time.RFC3339),
	}

	// 如果LoadBalancer已分配IP，更新状态
	if len(createdService.Status.LoadBalancer.Ingress) > 0 {
		response.Status = "Ready"
		var externalIPs []string
		for _, ingress := range createdService.Status.LoadBalancer.Ingress {
			if ingress.IP != "" {
				externalIPs = append(externalIPs, ingress.IP)
			}
			if ingress.Hostname != "" {
				externalIPs = append(externalIPs, ingress.Hostname)
			}
		}
		response.ExternalIPs = externalIPs
	}

	logrus.Infof("LoadBalancer service created successfully: %s", serviceName)
	httputil.ReturnSuccess(r, w, response)
}

// GetLoadBalancer 获取LoadBalancer服务
func (g Struct) GetLoadBalancer(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	k := k8s.Default().Clientset.CoreV1()

	// 获取查询参数
	appID := r.URL.Query().Get("appID")
	serviceName := r.URL.Query().Get("service_name")

	// 构建标签选择器
	labelSelector := "loadbalancer=true"
	if appID != "" {
		labelSelector += ",app_id=" + appID
	}
	if serviceName != "" {
		labelSelector += ",service_alias=" + serviceName
	}

	// 列出LoadBalancer服务
	list, err := k.Services(tenant.Namespace).List(r.Context(), v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		logrus.Errorf("get LoadBalancer services error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ServerErr)
		return
	}

	var responses []*model.LoadBalancerResponse
	for _, service := range list.Items {
		if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}

		response := &model.LoadBalancerResponse{
			Name:           service.Name,
			Namespace:      service.Namespace,
			ServiceName:    service.Labels["service_alias"],
			ServicePort:    int(service.Spec.Ports[0].Port),
			Protocol:       string(service.Spec.Ports[0].Protocol),
			LoadBalancerIP: service.Spec.LoadBalancerIP,
			ExternalIPs:    service.Spec.ExternalIPs,
			Annotations:    service.Annotations,
			Status:         "Creating",
			CreatedAt:      service.CreationTimestamp.Format(time.RFC3339),
		}

		// 检查LoadBalancer状态
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			response.Status = "Ready"
			var externalIPs []string
			for _, ingress := range service.Status.LoadBalancer.Ingress {
				if ingress.IP != "" {
					externalIPs = append(externalIPs, ingress.IP)
				}
				if ingress.Hostname != "" {
					externalIPs = append(externalIPs, ingress.Hostname)
				}
			}
			if len(externalIPs) > 0 {
				response.ExternalIPs = externalIPs
			}
		}

		responses = append(responses, response)
	}

	httputil.ReturnSuccess(r, w, responses)
}

// DeleteLoadBalancer 删除LoadBalancer服务
func (g Struct) DeleteLoadBalancer(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	serviceName := chi.URLParam(r, "name")
	if serviceName == "" {
		serviceName = r.URL.Query().Get("name")
	}

	if serviceName == "" {
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("service name is required"))
		return
	}

	k := k8s.Default().Clientset.CoreV1()

	// 检查服务是否存在且为LoadBalancer类型
	service, err := k.Services(tenant.Namespace).Get(r.Context(), serviceName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			httputil.ReturnBcodeError(r, w, bcode.NotFound)
			return
		}
		logrus.Errorf("get LoadBalancer service error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ServerErr)
		return
	}

	// 验证是否为LoadBalancer服务
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("service is not a LoadBalancer type"))
		return
	}

	// 验证是否为Rainbond创建的LoadBalancer
	if service.Labels["creator"] != "Rainbond" || service.Labels["loadbalancer"] != "true" {
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("service is not a Rainbond LoadBalancer"))
		return
	}

	// 删除服务
	err = k.Services(tenant.Namespace).Delete(r.Context(), serviceName, v1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("delete LoadBalancer service error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.ServerErr)
		return
	}

	logrus.Infof("LoadBalancer service deleted successfully: %s", serviceName)
	httputil.ReturnSuccess(r, w, map[string]string{
		"message": "LoadBalancer service deleted successfully",
		"name":    serviceName,
	})
}
