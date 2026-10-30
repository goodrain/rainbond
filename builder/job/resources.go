package job

import (
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func applyBuildPodResources(pod *corev1.Pod) error {
	resources := corev1.ResourceList{}
	for _, setting := range []struct {
		env  string
		name corev1.ResourceName
	}{
		{env: "BUILD_POD_CPU", name: corev1.ResourceCPU},
		{env: "BUILD_POD_MEMORY", name: corev1.ResourceMemory},
	} {
		value := strings.TrimSpace(os.Getenv(setting.env))
		if value == "" {
			continue
		}
		quantity, err := resource.ParseQuantity(value)
		if err != nil || quantity.Sign() <= 0 {
			return fmt.Errorf("%s must be a positive Kubernetes resource quantity", setting.env)
		}
		if setting.name == corev1.ResourceCPU && !quantity.RoundUp(resource.Milli) {
			return fmt.Errorf("%s must use whole millicores (for example, 500m or 2)", setting.env)
		}
		resources[setting.name] = quantity
	}
	if len(resources) == 0 {
		return nil
	}
	for i := range pod.Spec.Containers {
		requirements := &pod.Spec.Containers[i].Resources
		if requirements.Requests == nil {
			requirements.Requests = corev1.ResourceList{}
		}
		if requirements.Limits == nil {
			requirements.Limits = corev1.ResourceList{}
		}
		for name, quantity := range resources {
			requirements.Requests[name] = quantity.DeepCopy()
			requirements.Limits[name] = quantity.DeepCopy()
		}
	}
	return nil
}
