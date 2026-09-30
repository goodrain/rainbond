package kubeidentity

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.cleanup.package-writer-coverage
func TestManagedPackageCoverageRejectsBuildsAndChangingRollout(t *testing.T) {
	ctx := context.Background()
	client := writerCoverageFixture()
	client.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.(ktesting.ListAction).GetListRestrictions().Labels.String() != "job=codebuild" {
			return false, nil, nil
		}
		obj, err := client.Tracker().List(corev1.SchemeGroupVersion.WithResource("pods"), corev1.SchemeGroupVersion.WithKind("Pod"), action.GetNamespace())
		if err != nil {
			return true, nil, err
		}
		all := obj.(*corev1.PodList)
		list := &corev1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "stable"}}
		for _, pod := range all.Items {
			if pod.Labels["job"] == "codebuild" {
				list.Items = append(list.Items, pod)
			}
		}
		return true, list, nil
	})
	writers, err := InspectManagedPackageWriterCoverage(ctx, client, "system")
	if err != nil || len(writers) != 4 {
		t.Fatal("stable package writers rejected", writers, err)
	}
	if _, err := client.CoreV1().Pods("system").Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "active-build", Labels: map[string]string{"job": "codebuild"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectManagedPackageWriterCoverage(ctx, client, "system"); err == nil {
		t.Fatal("active build was ignored")
	}
}
