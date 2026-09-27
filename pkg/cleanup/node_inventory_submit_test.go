package cleanup

import (
	"context"
	"errors"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.cleanup.node-inventory-submission
func TestInventoryRequestReusesJobAfterLostCreate(t *testing.T) {
	no, yes := false, true
	zero, one := int32(0), int32(1)
	template := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: "system"}, Spec: batchv1.JobSpec{Suspend: &yes, BackoffLimit: &zero, Parallelism: &one, Completions: &one, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{NodeName: "node", RestartPolicy: corev1.RestartPolicyNever, AutomountServiceAccountToken: &no, Containers: []corev1.Container{{Name: "node-inventory", Command: []string{"/app/node-inventory"}, Image: "example.test/plugin@sha256:" + strings.Repeat("b", 64)}}}}}}
	kube := fake.NewSimpleClientset()
	creates := 0
	client := gcTestJobClient{JobInterface: kube.BatchV1().Jobs("system")}
	client.create = func(ctx context.Context, j *batchv1.Job, o metav1.CreateOptions) (*batchv1.Job, error) {
		if len(o.DryRun) > 0 {
			if _, err := client.JobInterface.Get(ctx, j.Name, metav1.GetOptions{}); err == nil {
				return nil, apierrors.NewAlreadyExists(schema.GroupResource{Group: "batch", Resource: "jobs"}, j.Name)
			} else if !apierrors.IsNotFound(err) {
				return nil, err
			}
			return j.DeepCopy(), nil
		}
		creates++
		j = j.DeepCopy()
		j.UID = types.UID("owned-inventory")
		j.ResourceVersion = "1"
		if _, err := client.JobInterface.Create(ctx, j, o); err != nil {
			t.Fatal(err)
		}
		if creates == 1 {
			return nil, errors.New("lost response")
		}
		return j, nil
	}
	fresh := func() (*batchv1.Job, error) { return template.DeepCopy(), nil }
	if _, err := RunNodeInventoryJob(context.Background(), client, "scan", template, fresh); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal(err)
	}
	job, err := RunNodeInventoryJob(context.Background(), client, "scan", template, fresh)
	if err != nil || creates != 1 || *job.Spec.Suspend {
		t.Fatal("original scan not resumed", creates, err)
	}
	again, err := RunNodeInventoryJob(context.Background(), client, "scan", template, fresh)
	if err != nil || again.UID != job.UID || creates != 1 {
		t.Fatal("scan recreated", err)
	}
	job.Status.Succeeded = 1
	if _, err := client.JobInterface.UpdateStatus(context.Background(), job, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	completed, err := RunNodeInventoryJob(context.Background(), client, "scan", template, fresh)
	if err != nil || completed.UID != job.UID || completed.Status.Succeeded != 1 || creates != 1 {
		t.Fatal("completed report cannot be polled", err)
	}
	changed := template.DeepCopy()
	changed.Spec.Template.Spec.NodeName = "replacement"
	// A changed source just before unsuspension must not run.
	if _, err := RunNodeInventoryJob(context.Background(), client, "other", template, func() (*batchv1.Job, error) { return changed, nil }); err == nil {
		t.Fatal("changed source started")
	}
}
