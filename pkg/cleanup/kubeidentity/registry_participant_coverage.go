package kubeidentity

import (
	"context"
	"reflect"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// InspectRegistryParticipantCoverage matches every selected live proxy to its
// durable startup record. Historical records cannot replace a missing instance.
// records must come from the Region database, never from an HTTP request body.
func InspectRegistryParticipantCoverage(ctx context.Context, client kubernetes.Interface, namespace, serviceName string, binding guard.StorageRegistration, records []guard.ParticipantRegistration) ([]guard.ParticipantRegistration, error) {
	if client == nil || namespace == "" || serviceName == "" || len(records) == 0 || len(records) > 256 {
		return nil, ErrBinding
	}
	fingerprint, err := binding.Fingerprint()
	if err != nil {
		return nil, ErrBinding
	}
	service, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil || service.UID == "" || service.ResourceVersion == "" || service.DeletionTimestamp != nil || len(service.Spec.Selector) == 0 {
		return nil, ErrBinding
	}
	list := func() (*corev1.PodList, error) {
		pods, e := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(service.Spec.Selector).String(), Limit: 33})
		if e != nil || pods == nil || pods.ResourceVersion == "" || pods.Continue != "" || len(pods.Items) == 0 || len(pods.Items) > 32 {
			return nil, ErrBinding
		}
		return pods, nil
	}
	pods, err := list()
	if err != nil {
		return nil, err
	}
	versions := map[string]string{}
	members := []guard.ParticipantRegistration{}
	for i := range pods.Items {
		pod := &pods.Items[i]
		uid := string(pod.UID)
		if uid == "" || pod.ResourceVersion == "" || versions[uid] != "" {
			return nil, ErrBinding
		}
		versions[uid] = pod.ResourceVersion
		var matched *guard.ParticipantRegistration
		for _, record := range records {
			if record.PodUID != uid || record.StorageID != binding.StorageID || record.Generation != binding.Generation || record.BindingFingerprint != fingerprint || record.Role != "registry-ingress" {
				continue
			}
			// Skip superseded runtime records before the more expensive mount/ingress
			// verification, retaining exact image and container identities in the match.
			runtimeMatches := false
			for _, status := range pod.Status.ContainerStatuses {
				if status.ContainerID == record.ContainerID && status.ImageID == record.ImageID && status.State.Running != nil {
					runtimeMatches = true
				}
			}
			if !runtimeMatches {
				continue
			}
			observed, e := InspectRegistryParticipant(ctx, client, namespace, serviceName, pod.Name, uid, record.Owner, binding)
			if e != nil || !reflect.DeepEqual(observed, record) {
				return nil, ErrBinding
			}
			if matched != nil {
				return nil, ErrBinding
			}
			copy := observed
			matched = &copy
		}
		if matched == nil {
			return nil, ErrBinding
		}
		members = append(members, *matched)
	}
	current, err := list()
	if err != nil || len(current.Items) != len(versions) {
		return nil, ErrBinding
	}
	for _, pod := range current.Items {
		uid := string(pod.UID)
		if versions[uid] == "" || versions[uid] != pod.ResourceVersion {
			return nil, ErrBinding
		}
		delete(versions, uid)
	}
	if len(versions) != 0 {
		return nil, ErrBinding
	}
	final, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil || final.UID != service.UID || final.ResourceVersion != service.ResourceVersion {
		return nil, ErrBinding
	}
	return members, nil
}
