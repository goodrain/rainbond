package volume

import (
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	appmtypes "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func newVMAppServiceForVolumeTest() *appmtypes.AppService {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
	}
	as := &appmtypes.AppService{
		AppServiceBase: appmtypes.AppServiceBase{
			ServiceID:    "service-1",
			ServiceAlias: "demo-vm",
			TenantID:     "tenant-1",
			AppID:        "app-1",
		},
	}
	as.SetTenant(namespace)
	as.SetVirtualMachine(&kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo-vm",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
		},
	})
	return as
}

// capability_id: rainbond.vm-volume-selected-storage-class
func TestNewVolumeManagerUsesSelectedStorageClassForVMDisks(t *testing.T) {
	as := newVMAppServiceForVolumeTest()
	serviceVolume := &dbmodel.TenantServiceVolume{
		Model:          dbmodel.Model{ID: 1},
		ServiceID:      "service-1",
		VolumeName:     "disk",
		VolumePath:     "/disk",
		VolumeType:     "nfs-storage",
		AccessMode:     "RWX",
		VolumeCapacity: 20,
	}

	manager := NewVolumeManager(as, serviceVolume, nil, nil, nil, nil, nil, false)
	shareVolume, ok := manager.(*ShareFileVolume)
	if !ok {
		t.Fatalf("expected vm storage volume to use ShareFileVolume, got %T", manager)
	}

	define := &Define{as: as}
	if err := shareVolume.CreateVolume(define); err != nil {
		t.Fatalf("create vm volume: %v", err)
	}

	claims := as.GetClaims()
	if len(claims) != 1 {
		t.Fatalf("expected exactly one claim, got %d", len(claims))
	}
	if claims[0].Spec.StorageClassName == nil {
		t.Fatal("expected vm claim to keep selected storage class")
	}
	if *claims[0].Spec.StorageClassName != "nfs-storage" {
		t.Fatalf("expected storage class nfs-storage, got %q", *claims[0].Spec.StorageClassName)
	}
	if len(claims[0].Spec.AccessModes) != 1 || claims[0].Spec.AccessModes[0] != corev1.ReadWriteMany {
		t.Fatalf("expected RWX access mode, got %#v", claims[0].Spec.AccessModes)
	}
	if len(define.GetVMDataVolumeTemplates()) != 1 {
		t.Fatalf("expected vm blank root disk to create one data volume template, got %d", len(define.GetVMDataVolumeTemplates()))
	}
}

// capability_id: rainbond.vm-volume-vm-file-backward-compatible
func TestShareFileVolumeVMStorageClassFallsBackToLocalPathForLegacyVMFile(t *testing.T) {
	as := newVMAppServiceForVolumeTest()
	serviceVolume := &dbmodel.TenantServiceVolume{
		Model:          dbmodel.Model{ID: 2},
		ServiceID:      "service-1",
		VolumeName:     "disk",
		VolumePath:     "/disk",
		VolumeType:     dbmodel.VMVolumeType.String(),
		AccessMode:     "RWO",
		VolumeCapacity: 10,
	}

	manager := NewVolumeManager(as, serviceVolume, nil, nil, nil, nil, nil, false)
	shareVolume, ok := manager.(*ShareFileVolume)
	if !ok {
		t.Fatalf("expected legacy vm-file volume to use ShareFileVolume, got %T", manager)
	}

	define := &Define{as: as}
	if err := shareVolume.CreateVolume(define); err != nil {
		t.Fatalf("create legacy vm volume: %v", err)
	}

	claims := as.GetClaims()
	if len(claims) != 1 {
		t.Fatalf("expected exactly one claim, got %d", len(claims))
	}
	if claims[0].Spec.StorageClassName == nil || *claims[0].Spec.StorageClassName != "local-path" {
		t.Fatalf("expected legacy vm-file to keep local-path fallback, got %#v", claims[0].Spec.StorageClassName)
	}
}
