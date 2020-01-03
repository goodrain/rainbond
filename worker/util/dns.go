package util

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	defaultRBDSystem = "rbd-system"
	defaultRBDDNS    = "rbd-dns"
)

func dns2Config(endpoint *corev1.Endpoints, podNamespace string) (podDNSConfig *corev1.PodDNSConfig, err error) {
	if endpoint == nil {
		return nil, fmt.Errorf("rbd-dns endpoints is nil")
	}
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
		Options:     []corev1.PodDNSConfigOption{corev1.PodDNSConfigOption{Name: "ndots", Value: &ndotsValue}},
		Searches:    []string{searchRBDDNS, "svc.cluster.local", "cluster.local"},
	}, nil
}

// MakePodDNSConfig make pod dns config
func MakePodDNSConfig(clientset *kubernetes.Clientset, podNamespace string) (podDNSConfig *corev1.PodDNSConfig, err error) {
	namespace := os.Getenv("RBD_SYSTEM")
	rbdName := os.Getenv("RBD_DNS")
	if namespace == "" {
		namespace = defaultRBDSystem
	}
	if rbdName == "" {
		rbdName = defaultRBDDNS
	}
	endpoints, err := clientset.CoreV1().Endpoints(namespace).Get(rbdName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("found rbd-dns error: %s", err.Error())
	}
	podDNSConfig, err = dns2Config(endpoints, podNamespace)
	if err != nil {
		return nil, fmt.Errorf("parse rbd-dns to dnsconfig error: %s", err.Error())
	}
	return podDNSConfig, nil
}
