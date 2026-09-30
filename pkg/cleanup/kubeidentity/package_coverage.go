package kubeidentity

import (
	"context"
	"reflect"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// InspectManagedPackageWriterCoverage proves a stable current platform rollout
// around an empty set of native build Pods. Work accepted after this snapshot
// must use the registered producer protocol before it can touch package storage.
func InspectManagedPackageWriterCoverage(ctx context.Context, client kubernetes.Interface, namespace string) ([]guard.ReferenceWriter, error) {
	before, err := InspectReferenceWriterCoverage(ctx, client, namespace)
	if err != nil {
		return nil, err
	}
	builds, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "job=codebuild", FieldSelector: "status.phase!=Succeeded,status.phase!=Failed", Limit: 2})
	if err != nil || builds.ResourceVersion == "" || builds.Continue != "" {
		return nil, ErrBinding
	}
	for _, pod := range builds.Items {
		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			return nil, guard.ErrCoordinationBusy
		}
	}
	after, err := InspectReferenceWriterCoverage(ctx, client, namespace)
	if err != nil || !reflect.DeepEqual(before, after) {
		return nil, ErrBinding
	}
	return before, nil
}
