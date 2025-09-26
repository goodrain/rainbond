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
	"context"
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

// getNodeIPs 获取集群节点的IP地址列表
func getNodeIPs(k8sComponent *k8s.Component) ([]string, error) {
	nodeList, err := k8sComponent.Clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var nodeIPs []string
	for _, node := range nodeList.Items {
		// 优先使用外网IP，如果没有则使用内网IP
		var nodeIP string
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeExternalIP && address.Address != "" {
				nodeIP = address.Address
				break
			}
		}
		if nodeIP == "" {
			for _, address := range node.Status.Addresses {
				if address.Type == corev1.NodeInternalIP && address.Address != "" {
					nodeIP = address.Address
					break
				}
			}
		}
		if nodeIP != "" {
			nodeIPs = append(nodeIPs, nodeIP)
		}
	}
	return nodeIPs, nil
}

// generateAccessURLs 生成访问地址列表
func generateAccessURLs(ips []string, ports []model.LoadBalancerPort, useServicePort bool) []string {
	var accessURLs []string
	for _, ip := range ips {
		for _, port := range ports {
			var portNum int
			if useServicePort {
				// 使用 LoadBalancer 的服务端口
				portNum = port.Port
			} else {
				// 使用 NodePort
				if port.NodePort > 0 {
					portNum = int(port.NodePort)
				} else {
					continue
				}
			}
			url := fmt.Sprintf("%s:%d", ip, portNum)
			accessURLs = append(accessURLs, url)
		}
	}
	return accessURLs
}

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

	// 验证端口配置
	if len(createLBReq.Ports) == 0 {
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("ports cannot be empty"))
		return
	}

	// 验证每个端口的协议类型
	for _, port := range createLBReq.Ports {
		protocol := strings.ToUpper(port.Protocol)
		if protocol != "TCP" && protocol != "UDP" {
			httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("protocol must be TCP or UDP"))
			return
		}
	}

	// 生成服务名称（使用第一个端口）
	serviceName := fmt.Sprintf("%s-lb", createLBReq.ServiceName)

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

	// 创建服务端口配置
	var servicePorts []corev1.ServicePort
	for _, port := range createLBReq.Ports {
		portName := port.Name
		if portName == "" {
			portName = fmt.Sprintf("port-%d", port.Port)
		}
		servicePorts = append(servicePorts, corev1.ServicePort{
			Name:       portName,
			Protocol:   corev1.Protocol(strings.ToUpper(port.Protocol)),
			Port:       int32(port.Port),
			TargetPort: intstr.FromInt(port.TargetPort),
		})
	}

	// 创建服务规格
	spec := corev1.ServiceSpec{
		Type:  corev1.ServiceTypeLoadBalancer,
		Ports: servicePorts,
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
	// 记录端口信息（多个端口用下划线分隔，符合K8s标签规范）
	var portStrings []string
	for _, port := range createLBReq.Ports {
		portStrings = append(portStrings, fmt.Sprintf("%d", port.Port))
	}
	labels["ports"] = strings.Join(portStrings, "_")

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

	// 转换端口信息，包含NodePort
	var responsePorts []model.LoadBalancerPort
	for _, port := range createdService.Spec.Ports {
		responsePorts = append(responsePorts, model.LoadBalancerPort{
			Port:       int(port.Port),
			TargetPort: port.TargetPort.IntValue(),
			Protocol:   string(port.Protocol),
			Name:       port.Name,
			NodePort:   port.NodePort,
		})
	}

	// 生成访问地址 - 优先使用 LoadBalancer Ingress IP
	var accessURLs []string
	var ingressIPs []string

	// 检查是否已有 LoadBalancer Ingress IP
	if len(createdService.Status.LoadBalancer.Ingress) > 0 {
		for _, ingress := range createdService.Status.LoadBalancer.Ingress {
			if ingress.IP != "" {
				ingressIPs = append(ingressIPs, ingress.IP)
			}
			if ingress.Hostname != "" {
				ingressIPs = append(ingressIPs, ingress.Hostname)
			}
		}
	}

	if len(ingressIPs) > 0 {
		// 使用 LoadBalancer Ingress IP 和服务端口
		accessURLs = generateAccessURLs(ingressIPs, responsePorts, true)
	} else {
		// 回退到使用节点IP和NodePort
		nodeIPs, err := getNodeIPs(k8s.Default())
		if err != nil {
			logrus.Warnf("get node IPs error %s", err.Error())
		} else if len(nodeIPs) > 0 {
			accessURLs = generateAccessURLs(nodeIPs, responsePorts, false)
		}
	}

	// 构造响应
	response := &model.LoadBalancerResponse{
		Name:        createdService.Name,
		Namespace:   createdService.Namespace,
		ServiceName: createLBReq.ServiceName,
		Ports:       responsePorts,
		ExternalIPs: createdService.Spec.ExternalIPs,
		AccessURLs:  accessURLs,
		Annotations: createdService.Annotations,
		Status:      "Creating",
		CreatedAt:   createdService.CreationTimestamp.Format(time.RFC3339),
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
	appID := r.URL.Query().Get("intID")
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

	// 获取节点IP列表（只获取一次，避免重复调用）
	nodeIPs, err := getNodeIPs(k8s.Default())
	if err != nil {
		logrus.Warnf("get node IPs error %s", err.Error())
	}

	var responses []*model.LoadBalancerResponse
	for _, service := range list.Items {
		if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}

		// 转换端口信息，包含NodePort
		var servicePorts []model.LoadBalancerPort
		for _, port := range service.Spec.Ports {
			servicePorts = append(servicePorts, model.LoadBalancerPort{
				Port:       int(port.Port),
				TargetPort: port.TargetPort.IntValue(),
				Protocol:   string(port.Protocol),
				Name:       port.Name,
				NodePort:   port.NodePort,
			})
		}

		// 生成访问地址 - 优先使用 LoadBalancer Ingress IP
		var accessURLs []string
		var ingressIPs []string

		// 检查是否已有 LoadBalancer Ingress IP
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			for _, ingress := range service.Status.LoadBalancer.Ingress {
				if ingress.IP != "" {
					ingressIPs = append(ingressIPs, ingress.IP)
				}
				if ingress.Hostname != "" {
					ingressIPs = append(ingressIPs, ingress.Hostname)
				}
			}
		}

		if len(ingressIPs) > 0 {
			// 使用 LoadBalancer Ingress IP 和服务端口
			accessURLs = generateAccessURLs(ingressIPs, servicePorts, true)
		} else if len(nodeIPs) > 0 {
			// 回退到使用节点IP和NodePort
			accessURLs = generateAccessURLs(nodeIPs, servicePorts, false)
		}

		response := &model.LoadBalancerResponse{
			Name:        service.Name,
			Namespace:   service.Namespace,
			ServiceName: service.Labels["service_alias"],
			Ports:       servicePorts,
			ExternalIPs: service.Spec.ExternalIPs,
			AccessURLs:  accessURLs,
			Annotations: service.Annotations,
			Status:      "Creating",
			CreatedAt:   service.CreationTimestamp.Format(time.RFC3339),
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

// UpdateLoadBalancer 更新LoadBalancer服务
func (g Struct) UpdateLoadBalancer(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*dbmodel.Tenants)
	serviceName := chi.URLParam(r, "name")
	if serviceName == "" {
		serviceName = r.URL.Query().Get("name")
	}

	if serviceName == "" {
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("service name is required"))
		return
	}

	var updateLBReq model.UpdateLoadBalancerStruct
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &updateLBReq, nil) {
		return
	}

	k := k8s.Default().Clientset.CoreV1()

	// 获取现有服务
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

	// 更新端口配置
	if len(updateLBReq.Ports) > 0 {
		// 验证每个端口的协议类型
		for _, port := range updateLBReq.Ports {
			protocol := strings.ToUpper(port.Protocol)
			if protocol != "TCP" && protocol != "UDP" {
				httputil.ReturnBcodeError(r, w, bcode.NewBadRequest("protocol must be TCP or UDP"))
				return
			}
		}

		// 更新服务端口配置
		var servicePorts []corev1.ServicePort
		for _, port := range updateLBReq.Ports {
			portName := port.Name
			if portName == "" {
				portName = fmt.Sprintf("port-%d", port.Port)
			}
			servicePorts = append(servicePorts, corev1.ServicePort{
				Name:       portName,
				Protocol:   corev1.Protocol(strings.ToUpper(port.Protocol)),
				Port:       int32(port.Port),
				TargetPort: intstr.FromInt(port.TargetPort),
			})
		}
		service.Spec.Ports = servicePorts

		// 更新标签中的端口信息（多个端口用下划线分隔，符合K8s标签规范）
		var updatePortStrings []string
		for _, port := range updateLBReq.Ports {
			updatePortStrings = append(updatePortStrings, fmt.Sprintf("%d", port.Port))
		}
		service.Labels["ports"] = strings.Join(updatePortStrings, "_")
	}

	// 更新注解
	if updateLBReq.Annotations != nil {
		if service.Annotations == nil {
			service.Annotations = make(map[string]string)
		}
		for k, v := range updateLBReq.Annotations {
			service.Annotations[k] = v
		}
	}

	// 更新服务
	updatedService, err := k.Services(tenant.Namespace).Update(r.Context(), service, v1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("update LoadBalancer service error %s", err.Error())
		httputil.ReturnBcodeError(r, w, fmt.Errorf("update LoadBalancer service error: %s", err.Error()))
		return
	}

	// 转换端口信息，包含NodePort
	var updatedPorts []model.LoadBalancerPort
	for _, port := range updatedService.Spec.Ports {
		updatedPorts = append(updatedPorts, model.LoadBalancerPort{
			Port:       int(port.Port),
			TargetPort: port.TargetPort.IntValue(),
			Protocol:   string(port.Protocol),
			Name:       port.Name,
			NodePort:   port.NodePort,
		})
	}

	// 生成访问地址 - 优先使用 LoadBalancer Ingress IP
	var accessURLs []string
	var ingressIPs []string

	// 检查是否已有 LoadBalancer Ingress IP
	if len(updatedService.Status.LoadBalancer.Ingress) > 0 {
		for _, ingress := range updatedService.Status.LoadBalancer.Ingress {
			if ingress.IP != "" {
				ingressIPs = append(ingressIPs, ingress.IP)
			}
			if ingress.Hostname != "" {
				ingressIPs = append(ingressIPs, ingress.Hostname)
			}
		}
	}

	if len(ingressIPs) > 0 {
		// 使用 LoadBalancer Ingress IP 和服务端口
		accessURLs = generateAccessURLs(ingressIPs, updatedPorts, true)
	} else {
		// 回退到使用节点IP和NodePort
		nodeIPs, err := getNodeIPs(k8s.Default())
		if err != nil {
			logrus.Warnf("get node IPs error %s", err.Error())
		} else if len(nodeIPs) > 0 {
			accessURLs = generateAccessURLs(nodeIPs, updatedPorts, false)
		}
	}

	// 构造响应
	response := &model.LoadBalancerResponse{
		Name:        updatedService.Name,
		Namespace:   updatedService.Namespace,
		ServiceName: updatedService.Labels["service_alias"],
		Ports:       updatedPorts,
		ExternalIPs: updatedService.Spec.ExternalIPs,
		AccessURLs:  accessURLs,
		Annotations: updatedService.Annotations,
		Status:      "Creating",
		CreatedAt:   updatedService.CreationTimestamp.Format(time.RFC3339),
	}

	// 检查LoadBalancer状态
	if len(updatedService.Status.LoadBalancer.Ingress) > 0 {
		response.Status = "Ready"
		var externalIPs []string
		for _, ingress := range updatedService.Status.LoadBalancer.Ingress {
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

	logrus.Infof("LoadBalancer service updated successfully: %s", serviceName)
	httputil.ReturnSuccess(r, w, response)
}
