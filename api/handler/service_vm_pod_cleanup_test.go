package handler

import (
	"context"
	"testing"
	"time"

	promcli "github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/server/pb"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.vm-pods.cleanup-completed-virt-launcher

type noopPrometheus struct{}

func (noopPrometheus) GetMetric(string, time.Time) promcli.Metric {
	return promcli.Metric{}
}

func (noopPrometheus) GetMetricOverTime(string, time.Time, time.Time, time.Duration) promcli.Metric {
	return promcli.Metric{}
}

func (noopPrometheus) GetMetadata(string) []promcli.Metadata {
	return nil
}

func (noopPrometheus) GetAppMetadata(string, string) []promcli.Metadata {
	return nil
}

func (noopPrometheus) GetComponentMetadata(string, string) []promcli.Metadata {
	return nil
}

func (noopPrometheus) GetMetricLabelSet(string, time.Time, time.Time) []map[string]string {
	return nil
}

func TestGetPodsCleansUpCompletedVMLauncherPodsAfterHotUpdate(t *testing.T) {
	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:    "service-vm",
			ServiceAlias: "service-vm",
			ExtendMethod: "vm",
		},
	}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   &resourceSyncEventDao{},
	})
	defer db.SetTestManager(nil)

	kubeClient := kubefake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "virt-launcher-nginx-mywin-6bh9b",
				Namespace: "demo-ns",
				Labels: map[string]string{
					"service_id":  "service-vm",
					"kubevirt.io": "virt-launcher",
				},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "virt-launcher-nginx-mywin-gpg7f",
				Namespace: "demo-ns",
				Labels: map[string]string{
					"service_id":  "service-vm",
					"kubevirt.io": "virt-launcher",
				},
			},
			Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
		},
	)

	action := &ServiceAction{
		kubeClient:    kubeClient,
		prometheusCli: noopPrometheus{},
		getServicePodsHook: func(serviceID string) (*pb.ServiceAppPodList, error) {
			return &pb.ServiceAppPodList{
				NewPods: []*pb.ServiceAppPod{
					{
						PodName:   "virt-launcher-nginx-mywin-6bh9b",
						PodIp:     "10.42.0.16",
						PodStatus: "RUNNING",
					},
				},
				OldPods: []*pb.ServiceAppPod{
					{
						PodName:   "virt-launcher-nginx-mywin-gpg7f",
						PodIp:     "10.42.0.15",
						PodStatus: "SUCCEEDED",
					},
				},
			}, nil
		},
	}

	pods, err := action.GetPods("service-vm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(pods.NewPods) != 1 || pods.NewPods[0].PodName != "virt-launcher-nginx-mywin-6bh9b" {
		t.Fatalf("expected running launcher pod to remain, got %#v", pods.NewPods)
	}
	if len(pods.OldPods) != 0 {
		t.Fatalf("expected completed launcher pod to be filtered, got %#v", pods.OldPods)
	}

	if _, err := kubeClient.CoreV1().Pods("demo-ns").Get(context.Background(), "virt-launcher-nginx-mywin-gpg7f", metav1.GetOptions{}); !k8sErrors.IsNotFound(err) {
		t.Fatalf("expected completed launcher pod to be deleted, got err=%v", err)
	}
	if _, err := kubeClient.CoreV1().Pods("demo-ns").Get(context.Background(), "virt-launcher-nginx-mywin-6bh9b", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected running launcher pod to remain, got err=%v", err)
	}
}

func TestGetPodsKeepsCompletedVMLauncherWhenReplacementIsNotRunning(t *testing.T) {
	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:    "service-vm",
			ServiceAlias: "service-vm",
			ExtendMethod: "vm",
		},
	}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   &resourceSyncEventDao{},
	})
	defer db.SetTestManager(nil)

	kubeClient := kubefake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "virt-launcher-nginx-mywin-gpg7f",
				Namespace: "demo-ns",
				Labels: map[string]string{
					"service_id":  "service-vm",
					"kubevirt.io": "virt-launcher",
				},
			},
			Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
		},
	)

	action := &ServiceAction{
		kubeClient:    kubeClient,
		prometheusCli: noopPrometheus{},
		getServicePodsHook: func(serviceID string) (*pb.ServiceAppPodList, error) {
			return &pb.ServiceAppPodList{
				OldPods: []*pb.ServiceAppPod{
					{
						PodName:   "virt-launcher-nginx-mywin-gpg7f",
						PodIp:     "10.42.0.15",
						PodStatus: "SUCCEEDED",
					},
				},
			}, nil
		},
	}

	pods, err := action.GetPods("service-vm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(pods.OldPods) != 1 || pods.OldPods[0].PodName != "virt-launcher-nginx-mywin-gpg7f" {
		t.Fatalf("expected completed launcher pod to remain for diagnostics, got %#v", pods.OldPods)
	}
	if _, err := kubeClient.CoreV1().Pods("demo-ns").Get(context.Background(), "virt-launcher-nginx-mywin-gpg7f", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected completed launcher pod to remain when no replacement is running, got err=%v", err)
	}
}
