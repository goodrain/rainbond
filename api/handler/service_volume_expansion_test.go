package handler

import (
	"context"
	"fmt"
	"testing"

	"github.com/goodrain/rainbond/api/util/bcode"
	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func expansionStorageClass(name string, allowed bool) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		ObjectMeta:           metav1.ObjectMeta{Name: name},
		AllowVolumeExpansion: &allowed,
	}
}

func expansionPVC(namespace, name, serviceID, volumeName, storageClass, requested, actual string,
	conditions ...corev1.PersistentVolumeClaimConditionType,
) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      map[string]string{"service_id": serviceID},
			Annotations: map[string]string{"volume_name": volumeName},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(requested),
			}},
		},
		Status: corev1.PersistentVolumeClaimStatus{Capacity: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse(actual),
		}},
	}
	for _, conditionType := range conditions {
		pvc.Status.Conditions = append(pvc.Status.Conditions, corev1.PersistentVolumeClaimCondition{
			Type:    conditionType,
			Status:  corev1.ConditionTrue,
			Message: fmt.Sprintf("condition %s", conditionType),
		})
	}
	return pvc
}

// capability_id: rainbond.component.volume-expansion-status
func TestInspectVolumeExpansion(t *testing.T) {
	service := &dbmodel.TenantServices{ServiceID: "service-1", Namespace: "tenant-ns"}
	volume := &dbmodel.TenantServiceVolume{
		Model:          dbmodel.Model{ID: 7},
		ServiceID:      service.ServiceID,
		VolumeName:     "data",
		VolumeType:     "fast",
		VolumeCapacity: 20,
	}

	tests := []struct {
		name          string
		service       *dbmodel.TenantServices
		volume        *dbmodel.TenantServiceVolume
		objects       []runtime.Object
		wantStatus    string
		wantAllowed   bool
		wantActual    int64
		wantRequested int64
		wantPVCCount  int
	}{
		{
			name:    "stateful claims are aggregated",
			service: service,
			volume:  volume,
			objects: []runtime.Object{
				expansionStorageClass("fast", true),
				expansionPVC("tenant-ns", "manual7-app-0", "service-1", "data", "fast", "20Gi", "20Gi"),
				expansionPVC("tenant-ns", "manual7-app-1", "service-1", "data", "fast", "20Gi", "20Gi"),
			},
			wantStatus:    volumeExpansionReady,
			wantAllowed:   true,
			wantActual:    20,
			wantRequested: 20,
			wantPVCCount:  2,
		},
		{
			name:    "requested capacity ahead of actual is resizing",
			service: service,
			volume:  volume,
			objects: []runtime.Object{
				expansionStorageClass("fast", true),
				expansionPVC("tenant-ns", "manual7", "service-1", "data", "fast", "30Gi", "20Gi"),
			},
			wantStatus:    volumeExpansionResizing,
			wantAllowed:   true,
			wantActual:    20,
			wantRequested: 30,
			wantPVCCount:  1,
		},
		{
			name:    "filesystem condition takes precedence",
			service: service,
			volume:  volume,
			objects: []runtime.Object{
				expansionStorageClass("fast", true),
				expansionPVC(
					"tenant-ns", "manual7", "service-1", "data", "fast", "30Gi", "20Gi",
					corev1.PersistentVolumeClaimFileSystemResizePending,
				),
			},
			wantStatus:    volumeExpansionFileSystemResizePending,
			wantAllowed:   true,
			wantActual:    20,
			wantRequested: 30,
			wantPVCCount:  1,
		},
		{
			name:    "storage class can forbid expansion",
			service: service,
			volume:  volume,
			objects: []runtime.Object{
				expansionStorageClass("fast", false),
				expansionPVC("tenant-ns", "manual7", "service-1", "data", "fast", "20Gi", "20Gi"),
			},
			wantStatus:    volumeExpansionUnsupported,
			wantAllowed:   false,
			wantActual:    20,
			wantRequested: 20,
			wantPVCCount:  1,
		},
		{
			name:          "not yet deployed PVC can change desired capacity",
			service:       service,
			volume:        volume,
			wantStatus:    volumeExpansionUnbound,
			wantAllowed:   true,
			wantActual:    0,
			wantRequested: 0,
			wantPVCCount:  0,
		},
		{
			name: "virtual machine guest disks are not supported",
			service: &dbmodel.TenantServices{
				ServiceID:    "service-1",
				Namespace:    "tenant-ns",
				ExtendMethod: "vm",
			},
			volume:       volume,
			wantStatus:   volumeExpansionUnsupported,
			wantAllowed:  false,
			wantPVCCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := &ServiceAction{kubeClient: fake.NewSimpleClientset(tt.objects...)}
			got, _, err := action.inspectVolumeExpansion(context.Background(), tt.service, tt.volume)
			if err != nil {
				t.Fatalf("inspect volume expansion: %v", err)
			}
			if got.Status != tt.wantStatus || got.AllowExpansion != tt.wantAllowed ||
				got.ActualCapacity != tt.wantActual || got.RequestedCapacity != tt.wantRequested ||
				got.PVCCount != tt.wantPVCCount {
				t.Fatalf("unexpected runtime: %#v", got)
			}
		})
	}
}

func TestInspectVolumeExpansionResolvesTenantNamespace(t *testing.T) {
	service := &dbmodel.TenantServices{
		TenantID:  "tenant-uuid",
		ServiceID: "service-1",
		Namespace: "tenant-uuid",
	}
	volume := &dbmodel.TenantServiceVolume{
		Model:          dbmodel.Model{ID: 7},
		ServiceID:      service.ServiceID,
		VolumeName:     "data",
		VolumeType:     "fast",
		VolumeCapacity: 20,
	}
	action := &ServiceAction{
		kubeClient: fake.NewSimpleClientset(
			expansionStorageClass("fast", true),
			expansionPVC("tenant-ns", "manual7", "service-1", "data", "fast", "20Gi", "20Gi"),
		),
		resolveTenantNamespaceHook: func(tenantID string) (string, error) {
			if tenantID != "tenant-uuid" {
				t.Fatalf("tenant ID = %q, want tenant-uuid", tenantID)
			}
			return "tenant-ns", nil
		},
	}

	got, claims, err := action.inspectVolumeExpansion(context.Background(), service, volume)
	if err != nil {
		t.Fatalf("inspect volume expansion: %v", err)
	}
	if got.Status != volumeExpansionReady || len(claims) != 1 || claims[0].Namespace != "tenant-ns" {
		t.Fatalf("unexpected resolved runtime or claims: %#v, %#v", got, claims)
	}
}

// capability_id: rainbond.component.volume-expansion-updates-claims
func TestExpandVolumeClaims(t *testing.T) {
	service := &dbmodel.TenantServices{ServiceID: "service-1", Namespace: "tenant-ns"}
	volume := &dbmodel.TenantServiceVolume{
		Model:          dbmodel.Model{ID: 7},
		ServiceID:      service.ServiceID,
		VolumeName:     "data",
		VolumeType:     "fast",
		VolumeCapacity: 10,
	}
	client := fake.NewSimpleClientset(
		expansionStorageClass("fast", true),
		expansionPVC("tenant-ns", "manual7-app-0", "service-1", "data", "fast", "10Gi", "10Gi"),
		expansionPVC("tenant-ns", "manual7-app-1", "service-1", "data", "fast", "10Gi", "10Gi"),
	)
	action := &ServiceAction{kubeClient: client}

	if err := action.expandVolumeClaims(context.Background(), service, volume, 20); err != nil {
		t.Fatalf("expand claims: %v", err)
	}
	for _, name := range []string{"manual7-app-0", "manual7-app-1"} {
		pvc, err := client.CoreV1().PersistentVolumeClaims("tenant-ns").Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get PVC %s: %v", name, err)
		}
		if got := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse("20Gi")) != 0 {
			t.Fatalf("PVC %s request = %s, want 20Gi", name, got.String())
		}
	}
}

func TestExpandVolumeClaimsRejectsShrink(t *testing.T) {
	service := &dbmodel.TenantServices{ServiceID: "service-1", Namespace: "tenant-ns"}
	volume := &dbmodel.TenantServiceVolume{
		Model:          dbmodel.Model{ID: 7},
		ServiceID:      service.ServiceID,
		VolumeName:     "data",
		VolumeType:     "fast",
		VolumeCapacity: 20,
	}
	client := fake.NewSimpleClientset(
		expansionStorageClass("fast", true),
		expansionPVC("tenant-ns", "manual7", "service-1", "data", "fast", "20Gi", "20Gi"),
	)
	action := &ServiceAction{kubeClient: client}

	err := action.expandVolumeClaims(context.Background(), service, volume, 10)
	if err == nil {
		t.Fatal("expected shrink request to fail")
	}
	if got := bcode.Err2Coder(err).GetStatus(); got != 400 {
		t.Fatalf("error status = %d, want 400: %v", got, err)
	}
	pvc, err := client.CoreV1().PersistentVolumeClaims("tenant-ns").Get(context.Background(), "manual7", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse("20Gi")) != 0 {
		t.Fatalf("PVC request changed after rejected shrink: %s", got.String())
	}
}

func TestExpandVolumeClaimsRejectsDisabledStorageClass(t *testing.T) {
	service := &dbmodel.TenantServices{ServiceID: "service-1", Namespace: "tenant-ns"}
	volume := &dbmodel.TenantServiceVolume{
		Model:          dbmodel.Model{ID: 7},
		ServiceID:      service.ServiceID,
		VolumeName:     "data",
		VolumeType:     "fast",
		VolumeCapacity: 10,
	}
	action := &ServiceAction{kubeClient: fake.NewSimpleClientset(
		expansionStorageClass("fast", false),
		expansionPVC("tenant-ns", "manual7", "service-1", "data", "fast", "10Gi", "10Gi"),
	)}

	err := action.expandVolumeClaims(context.Background(), service, volume, 20)
	if err == nil {
		t.Fatal("expected disabled StorageClass to reject expansion")
	}
	if got := bcode.Err2Coder(err).GetStatus(); got != 400 {
		t.Fatalf("error status = %d, want 400: %v", got, err)
	}
}
