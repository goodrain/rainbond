package kubeidentity

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// RegistryServiceObservation covers every selected instance, including pending
// or terminating Pods. Mixed deployments must not be mistaken for full coverage.
type RegistryServiceObservation struct {
	ServiceUID, ServiceVersion, VolumeUID string
	PodUIDs                               []string
	RegistryPort, ProxyPort               int32
}

// InspectRegistryService rejects empty, changing, oversized or mixed instance
// sets. A caller still needs participant, runtime and reference verification.
func InspectRegistryService(ctx context.Context, client kubernetes.Interface, request RegistryMountRequest) (RegistryServiceObservation, error) {
	denied := RegistryServiceObservation{}
	if client == nil || request.Namespace == "" || request.Service == "" {
		return denied, ErrBinding
	}
	service, err := client.CoreV1().Services(request.Namespace).Get(ctx, request.Service, metav1.GetOptions{})
	if err != nil || service.UID == "" || service.ResourceVersion == "" || len(service.Spec.Selector) == 0 {
		return denied, ErrBinding
	}
	result := RegistryServiceObservation{ServiceUID: string(service.UID), ServiceVersion: service.ResourceVersion}
	cursor, version := "", ""
	cursors, identities := map[string]bool{}, map[string]bool{}
	for pages := 0; ; pages++ {
		if pages >= 32 {
			return denied, ErrBinding
		}
		page, err := client.CoreV1().Pods(request.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(service.Spec.Selector).String(), Limit: 32, Continue: cursor})
		if err != nil || page.ResourceVersion == "" {
			return denied, ErrBinding
		}
		if version != "" && version != page.ResourceVersion {
			return denied, ErrBinding
		}
		version = page.ResourceVersion
		for i := range page.Items {
			pod := &page.Items[i]
			if pod.ResourceVersion == "" || pod.UID == "" || identities[string(pod.UID)] || len(identities) >= 32 {
				return denied, ErrBinding
			}
			identities[string(pod.UID)] = true
			ingress, err := InspectRegistryIngress(service, pod, request.RegistryContainer, request.CoordinatorContainer)
			if err != nil {
				return denied, err
			}
			bound := request
			bound.Pod = pod.Name
			bound.PodUID = string(pod.UID)
			mount, err := InspectRegistryMount(ctx, client, bound)
			if err != nil || mount.PodVersion != pod.ResourceVersion {
				return denied, ErrBinding
			}
			if result.VolumeUID != "" && (result.VolumeUID != mount.VolumeUID || result.ProxyPort != ingress.ProxyPort || result.RegistryPort != ingress.RegistryPort) {
				return denied, ErrBinding
			}
			result.VolumeUID = mount.VolumeUID
			result.ProxyPort = ingress.ProxyPort
			result.RegistryPort = ingress.RegistryPort
			result.PodUIDs = append(result.PodUIDs, string(pod.UID))
		}
		if page.Continue == "" {
			break
		}
		if cursors[page.Continue] {
			return denied, ErrBinding
		}
		cursors[page.Continue] = true
		cursor = page.Continue
	}
	if len(result.PodUIDs) == 0 {
		return denied, ErrBinding
	}
	current, err := client.CoreV1().Services(request.Namespace).Get(ctx, request.Service, metav1.GetOptions{})
	if err != nil || current.UID != service.UID || current.ResourceVersion != service.ResourceVersion {
		return denied, ErrBinding
	}
	sort.Strings(result.PodUIDs)
	return result, nil
}
