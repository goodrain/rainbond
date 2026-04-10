package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func TestDiscoverVMExportDisks(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Volumes: []kubevirtv1.Volume{
						{
							Name: "rootdisk",
							VolumeSource: kubevirtv1.VolumeSource{
								PersistentVolumeClaim: &kubevirtv1.PersistentVolumeClaimVolumeSource{
									PersistentVolumeClaimVolumeSource: corePersistentVolumeClaimSource("rootdisk-pvc"),
								},
							},
						},
						{
							Name: "datadisk",
							VolumeSource: kubevirtv1.VolumeSource{
								PersistentVolumeClaim: &kubevirtv1.PersistentVolumeClaimVolumeSource{
									PersistentVolumeClaimVolumeSource: corePersistentVolumeClaimSource("datadisk-pvc"),
								},
							},
						},
					},
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							Disks: []kubevirtv1.Disk{
								{
									Name:      "rootdisk",
									BootOrder: uintPtr(1),
								},
								{
									Name:      "datadisk",
									BootOrder: uintPtr(2),
								},
							},
						},
					},
				},
			},
		},
	}

	disks := discoverVMExportDisks(vm)
	if assert.Len(t, disks, 2) {
		assert.Equal(t, "root", disks[0].DiskRole)
		assert.Equal(t, uint(1), disks[0].BootOrder)
		assert.Equal(t, "rootdisk-pvc", disks[0].PVCName)
		assert.Equal(t, "data", disks[1].DiskRole)
		assert.Equal(t, uint(2), disks[1].BootOrder)
		assert.Equal(t, "datadisk-pvc", disks[1].PVCName)
	}
}

func TestDiscoverVMExportDisksWithoutPersistentRootDisk(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Volumes: []kubevirtv1.Volume{
						{
							Name: "vmimage",
							VolumeSource: kubevirtv1.VolumeSource{
								ContainerDisk: &kubevirtv1.ContainerDiskSource{
									Image: "demo/rootdisk:latest",
								},
							},
						},
						{
							Name: "datadisk",
							VolumeSource: kubevirtv1.VolumeSource{
								PersistentVolumeClaim: &kubevirtv1.PersistentVolumeClaimVolumeSource{
									PersistentVolumeClaimVolumeSource: corePersistentVolumeClaimSource("datadisk-pvc"),
								},
							},
						},
					},
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							Disks: []kubevirtv1.Disk{
								{
									Name:      "vmimage",
									BootOrder: uintPtr(1),
								},
								{
									Name:      "datadisk",
									BootOrder: uintPtr(2),
								},
							},
						},
					},
				},
			},
		},
	}

	disks := discoverVMExportDisks(vm)
	if assert.Len(t, disks, 1) {
		assert.Equal(t, "data", disks[0].DiskRole)
	}
	assert.False(t, hasPersistentRootDisk(disks))
}

func TestCreateVMDataExports(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		vmDataExportGVR: "DataExportList",
	})
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
	}
	disks := []VMExportDisk{
		{DiskKey: "rootdisk", DiskName: "rootdisk", DiskRole: "root", BootOrder: 1, PVCName: "rootdisk-pvc", PVCNamespace: "demo-ns"},
		{DiskKey: "datadisk", DiskName: "datadisk", DiskRole: "data", BootOrder: 2, PVCName: "datadisk-pvc", PVCNamespace: "demo-ns"},
	}

	err := createVMDataExports(client, "evt-1", "service-1", vm, disks)
	assert.NoError(t, err)

	list, err := client.Resource(vmDataExportGVR).Namespace("demo-ns").List(t.Context(), metav1.ListOptions{})
	assert.NoError(t, err)
	if assert.Len(t, list.Items, 2) {
		itemsByName := make(map[string]unstructured.Unstructured, len(list.Items))
		for _, item := range list.Items {
			itemsByName[item.GetName()] = item
		}
		rootDisk := itemsByName["evt-1-rootdisk"]
		assert.Equal(t, "service-1", rootDisk.GetLabels()["service_id"])
		assert.Equal(t, "evt-1", rootDisk.GetLabels()["vm_export_id"])
		pvcName, _, _ := unstructured.NestedString(rootDisk.Object, "spec", "source", "pvc", "name")
		assert.Equal(t, "rootdisk-pvc", pvcName)
		assert.Equal(t, "1", rootDisk.GetAnnotations()["vm_export_boot_order"])
		assert.Equal(t, "rootdisk", rootDisk.GetAnnotations()["vm_export_disk_name"])
	}
}

func TestBuildVMExportStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		vmDataExportGVR: "DataExportList",
	},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cdi.kubevirt.io/v1beta1",
				"kind":       "DataExport",
				"metadata": map[string]interface{}{
					"name":      "evt-1-rootdisk",
					"namespace": "demo-ns",
					"labels": map[string]interface{}{
						"service_id":          "service-1",
						"vm_export_id":        "evt-1",
						"vm_export_disk_key":  "rootdisk",
						"vm_export_disk_role": "root",
					},
					"annotations": map[string]interface{}{
						"vm_export_boot_order": "1",
						"vm_export_disk_name":  "rootdisk",
					},
				},
				"spec": map[string]interface{}{
					"source": map[string]interface{}{
						"pvc": map[string]interface{}{
							"name":      "rootdisk-pvc",
							"namespace": "demo-ns",
						},
					},
				},
				"status": map[string]interface{}{
					"phase": "Ready",
					"links": map[string]interface{}{
						"external": map[string]interface{}{
							"urls": []interface{}{
								map[string]interface{}{"url": "https://download/rootdisk"},
							},
						},
					},
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cdi.kubevirt.io/v1beta1",
				"kind":       "DataExport",
				"metadata": map[string]interface{}{
					"name":      "evt-1-datadisk",
					"namespace": "demo-ns",
					"labels": map[string]interface{}{
						"service_id":          "service-1",
						"vm_export_id":        "evt-1",
						"vm_export_disk_key":  "datadisk",
						"vm_export_disk_role": "data",
					},
					"annotations": map[string]interface{}{
						"vm_export_boot_order": "2",
						"vm_export_disk_name":  "datadisk",
					},
				},
				"spec": map[string]interface{}{
					"source": map[string]interface{}{
						"pvc": map[string]interface{}{
							"name":      "datadisk-pvc",
							"namespace": "demo-ns",
						},
					},
				},
				"status": map[string]interface{}{
					"phase": "Pending",
					"conditions": []interface{}{
						map[string]interface{}{
							"message": "waiting for export pod",
						},
					},
				},
			},
		},
	)

	status, err := BuildVMExportStatus(client, "service-1", "evt-1")
	assert.NoError(t, err)
	assert.Equal(t, "exporting", status.Status)
	if assert.Len(t, status.Disks, 2) {
		assert.Equal(t, uint(1), status.Disks[0].BootOrder)
		assert.Equal(t, "rootdisk", status.Disks[0].DiskName)
		assert.Equal(t, "https://download/rootdisk", status.Disks[0].DownloadURL)
		assert.Equal(t, "ready", status.Disks[0].Status)
		assert.Equal(t, uint(2), status.Disks[1].BootOrder)
		assert.Equal(t, "waiting for export pod", status.Disks[1].Message)
		assert.Equal(t, "exporting", status.Disks[1].Status)
	}
}

func TestVMExportRequiresClosedVM(t *testing.T) {
	assert.True(t, vmExportRequiresClosedVM(nil))
	assert.True(t, vmExportRequiresClosedVM(&VMExportRequest{}))
	assert.True(t, vmExportRequiresClosedVM(&VMExportRequest{SourceKind: "vm"}))
	assert.False(t, vmExportRequiresClosedVM(&VMExportRequest{SourceKind: "snapshot", SnapshotName: "snap-1"}))
}

func corePersistentVolumeClaimSource(name string) corev1.PersistentVolumeClaimVolumeSource {
	return corev1.PersistentVolumeClaimVolumeSource{ClaimName: name}
}

func uintPtr(v uint) *uint {
	return &v
}
