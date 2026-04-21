// capability_id: rainbond.vm-export.discover-datavolume-disks
// capability_id: rainbond.vm-export.machine-manifest-build
// capability_id: rainbond.vm-export.restore-plan-all-disks
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

func TestDiscoverVMExportDisksSupportsDataVolumeRootDisk(t *testing.T) {
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
								DataVolume: &kubevirtv1.DataVolumeSource{
									Name: "manual-root",
								},
							},
						},
						{
							Name: "datadisk",
							VolumeSource: kubevirtv1.VolumeSource{
								DataVolume: &kubevirtv1.DataVolumeSource{
									Name: "manual-data",
								},
							},
						},
						{
							Name: "vmimage",
							VolumeSource: kubevirtv1.VolumeSource{
								ContainerDisk: &kubevirtv1.ContainerDiskSource{
									Image: "demo/rootdisk:latest",
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
									Name:      "vmimage",
									BootOrder: uintPtr(2),
								},
								{
									Name:      "datadisk",
									BootOrder: uintPtr(3),
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
		assert.Equal(t, "manual-root", disks[0].PVCName)
		assert.Equal(t, "data", disks[1].DiskRole)
		assert.Equal(t, uint(3), disks[1].BootOrder)
		assert.Equal(t, "manual-data", disks[1].PVCName)
	}
	assert.True(t, hasPersistentRootDisk(disks))
}

func TestDiscoverVMExportDisksSupportsISOInstallerRootDataVolume(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			DataVolumeTemplates: []kubevirtv1.DataVolumeTemplateSpec{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "manual-root",
						Annotations: map[string]string{"volume_name": "disk"},
					},
				},
			},
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Volumes: []kubevirtv1.Volume{
						{
							Name: "vmimage",
							VolumeSource: kubevirtv1.VolumeSource{
								ContainerDisk: &kubevirtv1.ContainerDiskSource{
									Image: "demo/installer:latest",
								},
							},
						},
						{
							Name: "rootdisk",
							VolumeSource: kubevirtv1.VolumeSource{
								DataVolume: &kubevirtv1.DataVolumeSource{
									Name: "manual-root",
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
									Name:      "rootdisk",
									BootOrder: uintPtr(2),
								},
								{
									Name:      "datadisk",
									BootOrder: uintPtr(3),
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
		assert.Equal(t, uint(2), disks[0].BootOrder)
		assert.Equal(t, "manual-root", disks[0].PVCName)
		assert.Equal(t, "data", disks[1].DiskRole)
		assert.Equal(t, uint(3), disks[1].BootOrder)
		assert.Equal(t, "datadisk-pvc", disks[1].PVCName)
	}
	assert.True(t, hasPersistentRootDisk(disks))
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
		vmDataExportGVR: "VirtualMachineExportList",
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
		kind, _, _ := unstructured.NestedString(rootDisk.Object, "spec", "source", "kind")
		pvcName, _, _ := unstructured.NestedString(rootDisk.Object, "spec", "source", "name")
		assert.Equal(t, "PersistentVolumeClaim", kind)
		assert.Equal(t, "rootdisk-pvc", pvcName)
		assert.Equal(t, "1", rootDisk.GetAnnotations()["vm_export_boot_order"])
		assert.Equal(t, "rootdisk", rootDisk.GetAnnotations()["vm_export_disk_name"])
	}
}

func TestBuildVMExportStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		vmDataExportGVR: "VirtualMachineExportList",
	},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "export.kubevirt.io/v1beta1",
				"kind":       "VirtualMachineExport",
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
						"kind": "PersistentVolumeClaim",
						"name": "rootdisk-pvc",
					},
				},
				"status": map[string]interface{}{
					"phase": "Ready",
					"links": map[string]interface{}{
						"external": map[string]interface{}{
							"volumes": []interface{}{
								map[string]interface{}{
									"name": "rootdisk",
									"formats": []interface{}{
										map[string]interface{}{"format": "gzip", "url": "https://download/rootdisk"},
									},
								},
							},
						},
					},
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "export.kubevirt.io/v1beta1",
				"kind":       "VirtualMachineExport",
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
						"kind": "PersistentVolumeClaim",
						"name": "datadisk-pvc",
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

func TestDeleteVMExportResources(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		vmDataExportGVR: "VirtualMachineExportList",
	},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "export.kubevirt.io/v1beta1",
				"kind":       "VirtualMachineExport",
				"metadata": map[string]interface{}{
					"name":      "service-1-rootdisk",
					"namespace": "default",
					"labels": map[string]interface{}{
						"service_id":          "service-1",
						"vm_export_id":        "service-1",
						"vm_export_disk_key":  "rootdisk",
						"vm_export_disk_role": "root",
					},
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "export.kubevirt.io/v1beta1",
				"kind":       "VirtualMachineExport",
				"metadata": map[string]interface{}{
					"name":      "service-2-rootdisk",
					"namespace": "default",
					"labels": map[string]interface{}{
						"service_id":          "service-2",
						"vm_export_id":        "service-2",
						"vm_export_disk_key":  "rootdisk",
						"vm_export_disk_role": "root",
					},
				},
			},
		},
	)

	err := deleteVMExportResources(client, "service-1", "service-1")
	assert.NoError(t, err)

	list, err := client.Resource(vmDataExportGVR).Namespace("default").List(t.Context(), metav1.ListOptions{})
	assert.NoError(t, err)
	if assert.Len(t, list.Items, 1) {
		assert.Equal(t, "service-2-rootdisk", list.Items[0].GetName())
	}
}

func TestDeleteVMExportResourcesIgnoresMissingExport(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		vmDataExportGVR: "VirtualMachineExportList",
	})

	err := deleteVMExportResources(client, "service-1", "service-1")
	assert.NoError(t, err)
}

func TestVMExportRequiresClosedVM(t *testing.T) {
	assert.True(t, vmExportRequiresClosedVM(nil))
	assert.True(t, vmExportRequiresClosedVM(&VMExportRequest{}))
	assert.True(t, vmExportRequiresClosedVM(&VMExportRequest{SourceKind: "vm"}))
	assert.False(t, vmExportRequiresClosedVM(&VMExportRequest{SourceKind: "snapshot", SnapshotName: "snap-1"}))
}

func TestBuildVMMachineManifestIncludesAllPersistedDisks(t *testing.T) {
	status := &VMExportStatus{
		ExportID: "evt-1",
		Status:   "ready",
		Disks: []VMExportDisk{
			{
				DiskKey:     "rootdisk",
				DiskName:    "rootdisk",
				DiskRole:    "root",
				BootOrder:   1,
				PVCName:     "manual-root",
				DownloadURL: "https://download/rootdisk.qcow2",
			},
			{
				DiskKey:     "data-1",
				DiskName:    "data-1",
				DiskRole:    "data",
				BootOrder:   2,
				PVCName:     "manual-data",
				DownloadURL: "https://download/data-1.qcow2",
			},
		},
	}
	uploaded := map[string]VMExportUploadedDisk{
		"rootdisk": {
			DiskKey:    "rootdisk",
			ObjectKey:  "vm-export/tenant-a/asset-101/rootdisk.qcow2",
			ObjectURI:  "s3://vm-assets/vm-export/tenant-a/asset-101/rootdisk.qcow2",
			SizeBytes:  40 << 30,
			Format:     "qcow2",
			StorageURL: "https://minio.example/vm-assets/rootdisk.qcow2",
		},
		"data-1": {
			DiskKey:    "data-1",
			ObjectKey:  "vm-export/tenant-a/asset-101/data-1.qcow2",
			ObjectURI:  "s3://vm-assets/vm-export/tenant-a/asset-101/data-1.qcow2",
			SizeBytes:  200 << 30,
			Format:     "qcow2",
			StorageURL: "https://minio.example/vm-assets/data-1.qcow2",
		},
	}

	manifest, rootObjectURI, err := buildVMMachineManifest("amd64", "uefi", status, uploaded)
	assert.NoError(t, err)
	assert.Equal(t, "s3://vm-assets/vm-export/tenant-a/asset-101/rootdisk.qcow2", rootObjectURI)
	if assert.NotNil(t, manifest) {
		assert.Equal(t, "v1", manifest.Version)
		assert.Equal(t, "amd64", manifest.Arch)
		assert.Equal(t, "uefi", manifest.BootMode)
		assert.Equal(t, "rootdisk", manifest.RootDiskKey)
		if assert.Len(t, manifest.Disks, 2) {
			assert.Equal(t, "root", manifest.Disks[0].DiskRole)
			assert.Equal(t, "vm-export/tenant-a/asset-101/rootdisk.qcow2", manifest.Disks[0].ObjectKey)
			assert.Equal(t, int64(40<<30), manifest.Disks[0].SizeBytes)
			assert.Equal(t, "data", manifest.Disks[1].DiskRole)
			assert.Equal(t, "vm-export/tenant-a/asset-101/data-1.qcow2", manifest.Disks[1].ObjectKey)
			assert.Equal(t, int64(200<<30), manifest.Disks[1].SizeBytes)
		}
	}
}

func TestBuildVMAssetRestorePlanIncludesRootAndDataDiskImports(t *testing.T) {
	manifest := &VMMachineManifest{
		Version:     "v1",
		Arch:        "amd64",
		BootMode:    "uefi",
		RootDiskKey: "rootdisk",
		Disks: []VMMachineManifestDisk{
			{
				DiskKey:   "rootdisk",
				DiskName:  "rootdisk",
				DiskRole:  "root",
				BootOrder: 1,
				ObjectKey: "vm-export/tenant-a/asset-101/rootdisk.qcow2",
				ObjectURI: "s3://vm-assets/vm-export/tenant-a/asset-101/rootdisk.qcow2",
				Format:    "qcow2",
				SizeBytes: 40 << 30,
			},
			{
				DiskKey:   "data-1",
				DiskName:  "data-1",
				DiskRole:  "data",
				BootOrder: 2,
				ObjectKey: "vm-export/tenant-a/asset-101/data-1.qcow2",
				ObjectURI: "s3://vm-assets/vm-export/tenant-a/asset-101/data-1.qcow2",
				Format:    "qcow2",
				SizeBytes: 200 << 30,
			},
		},
	}

	plan, err := buildVMAssetRestorePlan(manifest, func(objectKey string) (string, error) {
		return "https://signed.example/" + objectKey, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "disk", plan.BootSourceFormat)
	if assert.Len(t, plan.DiskImports, 2) {
		assert.Equal(t, "disk", plan.DiskImports[0].VolumeName)
		assert.Equal(t, "rootdisk", plan.DiskImports[0].DiskKey)
		assert.Equal(t, "https://signed.example/vm-export/tenant-a/asset-101/rootdisk.qcow2", plan.DiskImports[0].ImageURL)
		assert.Equal(t, "data-1", plan.DiskImports[1].VolumeName)
		assert.Equal(t, "data-1", plan.DiskImports[1].DiskKey)
		assert.Equal(t, "https://signed.example/vm-export/tenant-a/asset-101/data-1.qcow2", plan.DiskImports[1].ImageURL)
	}
	if assert.Len(t, plan.DiskLayout, 2) {
		assert.Equal(t, "root", plan.DiskLayout[0].DiskRole)
		assert.True(t, plan.DiskLayout[0].Boot)
		assert.Equal(t, "data", plan.DiskLayout[1].DiskRole)
		assert.False(t, plan.DiskLayout[1].Boot)
	}
}

func corePersistentVolumeClaimSource(name string) corev1.PersistentVolumeClaimVolumeSource {
	return corev1.PersistentVolumeClaimVolumeSource{ClaimName: name}
}

func uintPtr(v uint) *uint {
	return &v
}
