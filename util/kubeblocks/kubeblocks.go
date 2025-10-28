// Package kubeblocks contains utilities for Kubeblocks interaction in Rainbond
package kubeblocks

import "strings"

// GenerateKubeBlocksSelector generate selector for kubeblocks component
func GenerateKubeBlocksSelector(k8sComponentName string) map[string]string {
	var (
		peer = map[string]bool{
			"rabbitmq": true,
		}
		clusterName   string
		componentName string
	)

	if k8sComponentName != "" {
		lastDashIndex := strings.LastIndex(k8sComponentName, "-")
		if lastDashIndex != -1 && lastDashIndex < len(k8sComponentName)-1 {
			clusterName = k8sComponentName[:lastDashIndex]
			componentName = k8sComponentName[lastDashIndex+1:]
		}
	}

	selector := map[string]string{
		"app.kubernetes.io/instance":        clusterName,
		"app.kubernetes.io/managed-by":      "kubeblocks",
		"apps.kubeblocks.io/component-name": componentName,
	}

	// add role selector for non-peer components
	if _, ok := peer[componentName]; !ok {
		selector["kubeblocks.io/role"] = "primary"
	}

	return selector
}
