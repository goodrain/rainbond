package kubeidentity

import (
	"context"
	"time"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RegisterReferenceWriterStartup waits briefly for the caller's own container
// status, pins the first Pod UID, and records only observed runtime evidence.
// It never adopts a replacement Pod or schedules background cleanup work.
func RegisterReferenceWriterStartup(ctx context.Context, database *gorm.DB, client kubernetes.Interface, namespace, podName, role string) error {
	if database == nil || client == nil || namespace == "" || podName == "" {
		return ErrBinding
	}
	if role != "api" && role != "worker" && role != "builder" && role != "console" {
		return ErrBinding
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil || pod == nil || pod.UID == "" {
		return ErrBinding
	}
	original := pod.UID
	for {
		if pod.UID != original || pod.DeletionTimestamp != nil || pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
			return ErrBinding
		}
		observed, err := InspectReferenceWriter(ctx, client, namespace, podName, string(original), role)
		if err == nil {
			return guard.RegisterReferenceWriter(database, observed)
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		pod, err = client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil || pod == nil {
			return ErrBinding
		}
	}
}
