package controller

import (
	"errors"
	"testing"

	appmtypes "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func manualClaimForTest(name string, size string) *corev1.PersistentVolumeClaim {
	storageClass := "nfs-storage"
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}
}

// capability_id: rainbond.manual-pvc-upgrade-updates-existing-claim
func TestUpgradeControllerUpgradeManualClaimsUpdatesExistingClaim(t *testing.T) {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	existingClaim := manualClaimForTest("manual1", "10Gi")

	oldApp := &appmtypes.AppService{AppServiceBase: appmtypes.AppServiceBase{ServiceID: "service-1", ServiceAlias: "demo"}}
	oldApp.SetTenant(namespace)
	oldApp.SetClaimManually(existingClaim.DeepCopy())

	newApp := appmtypes.AppService{AppServiceBase: appmtypes.AppServiceBase{ServiceID: "service-1", ServiceAlias: "demo"}}
	newApp.SetTenant(namespace)
	newApp.SetClaimManually(manualClaimForTest("manual1", "20Gi"))

	client := k8sfake.NewSimpleClientset(namespace, existingClaim.DeepCopy())
	controller := &upgradeController{
		manager: &Manager{
			client: client,
		},
	}

	if err := controller.upgradeManualClaims(oldApp, &newApp); err != nil {
		t.Fatalf("upgrade manual claims: %v", err)
	}

	pvc, err := client.CoreV1().PersistentVolumeClaims("default").Get(t.Context(), "manual1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pvc after upgrade: %v", err)
	}

	got := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	want := resource.MustParse("20Gi")
	if got.Cmp(want) != 0 {
		t.Fatalf("expected pvc storage request to be updated to %s, got %s", want.String(), got.String())
	}
}

// capability_id: rainbond.manual-pvc-upgrade-surfaces-update-errors
func TestUpgradeControllerUpgradeManualClaimsReturnsUpdateError(t *testing.T) {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	existingClaim := manualClaimForTest("manual1", "10Gi")

	oldApp := &appmtypes.AppService{AppServiceBase: appmtypes.AppServiceBase{ServiceID: "service-1", ServiceAlias: "demo"}}
	oldApp.SetTenant(namespace)
	oldApp.SetClaimManually(existingClaim.DeepCopy())

	newApp := appmtypes.AppService{AppServiceBase: appmtypes.AppServiceBase{ServiceID: "service-1", ServiceAlias: "demo"}}
	newApp.SetTenant(namespace)
	newApp.SetClaimManually(manualClaimForTest("manual1", "20Gi"))

	client := k8sfake.NewSimpleClientset(namespace, existingClaim.DeepCopy())
	client.PrependReactor("update", "persistentvolumeclaims", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("expansion not supported")
	})
	controller := &upgradeController{
		manager: &Manager{
			client: client,
		},
	}

	err := controller.upgradeManualClaims(oldApp, &newApp)
	if err == nil {
		t.Fatal("expected update error, got nil")
	}
	if err.Error() != "expansion not supported" {
		t.Fatalf("expected update error to be returned, got %v", err)
	}
}
