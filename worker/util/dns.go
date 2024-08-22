// 本文件实现了与 Kubernetes 集群中的 DNS 配置相关的实用函数，主要用于为 Pod 生成自定义的 DNS 配置。

// 1. `dns2Config` 函数：
//    - 该函数接收一个 Kubernetes 的 `Endpoints` 对象和 Pod 所在的命名空间，并返回一个 `PodDNSConfig` 对象。
//    - `PodDNSConfig` 对象用于定义 Pod 的 DNS 配置，包括 DNS 服务器、搜索域和 DNS 选项。
//    - 函数通过遍历 `Endpoints` 对象的 `Subsets`，收集所有的 IP 地址并将其作为 DNS 服务器列表。
//    - 它还生成了搜索域列表，默认包括 Pod 所在的命名空间以及集群的基础域名。
//    - 最后，函数返回配置好的 `PodDNSConfig` 对象。

// 2. `MakePodDNSConfig` 函数：
//    - 该函数用于从 Kubernetes API 中获取指定命名空间和名称的 `Endpoints` 对象，然后基于该 `Endpoints` 对象生成 Pod 的 DNS 配置。
//    - 函数首先调用 Kubernetes 客户端的 `Endpoints` API 获取指定的 `Endpoints` 对象。
//    - 如果获取过程中出现错误，函数会记录警告日志并返回 `nil`。
//    - 成功获取 `Endpoints` 对象后，函数调用 `dns2Config` 函数生成 `PodDNSConfig`，并将其返回。

// 总体来说，本文件提供了实用函数，帮助在 Kubernetes 环境中自动生成 Pod 的 DNS 配置。这些配置通常用于指定自定义的 DNS 服务器、搜索域和 DNS 选项，以满足应用程序在集群内的域名解析需求。

package util

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func dns2Config(endpoint *corev1.Endpoints, podNamespace string) (podDNSConfig *corev1.PodDNSConfig) {
	servers := make([]string, 0)
	for _, sub := range endpoint.Subsets {
		for _, addr := range sub.Addresses {
			servers = append(servers, addr.IP)
		}
	}
	searchRBDDNS := fmt.Sprintf("%s.svc.cluster.local", podNamespace)
	ndotsValue := "5"
	return &corev1.PodDNSConfig{
		Nameservers: servers,
		Options:     []corev1.PodDNSConfigOption{{Name: "ndots", Value: &ndotsValue}},
		Searches:    []string{searchRBDDNS, "svc.cluster.local", "cluster.local"},
	}
}

// MakePodDNSConfig make pod dns config
func MakePodDNSConfig(clientset kubernetes.Interface, podNamespace, rbdNamespace, rbdEndpointDNSName string) (podDNSConfig *corev1.PodDNSConfig) {
	endpoints, err := clientset.CoreV1().Endpoints(rbdNamespace).Get(context.Background(), rbdEndpointDNSName, metav1.GetOptions{})
	if err != nil {
		logrus.Warningf("get rbd-dns[namespace: %s, name: %s] endpoints error: %s", rbdNamespace, rbdEndpointDNSName, err.Error())
		return nil
	}
	return dns2Config(endpoints, podNamespace)
}
