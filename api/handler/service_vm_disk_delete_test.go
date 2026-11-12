package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	v1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVolumnVarRetainsVolumeWhenDetachFails(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	gdb, err := gorm.Open("mysql", sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	tx := gdb.Begin()
	mock.ExpectRollback()
	volumeDao := &hotunplugTenantServiceVolumeDao{volume: &dbmodel.TenantServiceVolume{
		Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1",
	}}
	db.SetTestManager(hotunplugVolumeDeleteTestManager{tx: tx, volumeDao: volumeDao,
		mountRelationDao: &hotunplugTenantServiceMountRelationDao{}, configFileDao: &hotunplugTenantServiceConfigFileDao{}})
	defer db.SetTestManager(nil)
	mq := &recordingMQClient{}
	action := &ServiceAction{MQClient: mq, hotunplugVMDataDiskHook: func(*dbmodel.TenantServiceVolume) error {
		return apierrors.NewConflict(dataVolumeGVR.GroupResource(), "manual42", fmt.Errorf("stop the VM before deleting this disk"))
	}}
	apiErr := action.VolumnVar(&dbmodel.TenantServiceVolume{ServiceID: "service-vm", VolumeName: "data-1"}, "tenant-1", "", "delete")
	if apiErr == nil || apiErr.Code != 409 {
		t.Fatalf("expected retryable 409, got %v", apiErr)
	}
	if volumeDao.deletedVolume != "" || len(mq.tasks) != 0 {
		t.Fatal("failed detach deleted the retry record or queued GC")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func diskDeleteFixture(t *testing.T, extras ...runtime.Object) (*ServiceAction, *dynamicfake.FakeDynamicClient, *k8sfake.Clientset) {
	t.Helper()
	volume := &dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"}
	dv := buildVMHotplugDataVolumeObject("demo-ns", "tenant-1", volume, "manual42")
	dv.SetUID(types.UID("dv-uid"))
	objs := []runtime.Object{dv}
	objs = append(objs, extras...)
	listKinds := map[schema.GroupVersionResource]string{
		dataVolumeGVR: "DataVolumeList",
		{Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "datasources"}:                         "DataSourceList",
		{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}:                              "VirtualMachineList",
		{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}:                      "VirtualMachineInstanceList",
		{Group: "snapshot.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinesnapshots"}:        "VirtualMachineSnapshotList",
		{Group: "snapshot.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinesnapshotcontents"}: "VirtualMachineSnapshotContentList",
		{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}:                  "VolumeSnapshotList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objs...)
	oldClient := vmHotplugDynamicClient
	vmHotplugDynamicClient = func() dynamic.Interface { return client }
	t.Cleanup(func() { vmHotplugDynamicClient = oldClient; db.SetTestManager(nil) })
	db.SetTestManager(hotplugVolumeTestManager{serviceDao: &hotplugTenantServiceDao{service: &dbmodel.TenantServices{
		ServiceID: "service-vm", ExtendMethod: "vm", Namespace: "demo-ns",
	}}})
	kube := k8sfake.NewSimpleClientset(&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: "manual42", Namespace: "demo-ns", UID: types.UID("pvc-uid"), Labels: map[string]string{"service_id": "service-vm"},
	}})
	action := &ServiceAction{kubeClient: kube, getVirtualMachineByServiceIDHook: func(string) (*v1.VirtualMachine, error) { return nil, nil }}
	return action, client, kube
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteWithoutVMReclaimsOwnedResourcesWithUID(t *testing.T) {
	action, dynamicClient, kube := diskDeleteFixture(t)
	recorder := &diskDeleteRecorder{Interface: dynamicClient}
	vmHotplugDynamicClient = func() dynamic.Interface { return recorder }
	err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dynamicClient.Resource(dataVolumeGVR).Namespace("demo-ns").Get(context.Background(), "manual42", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("DV still present: %v", err)
	}
	if _, err := kube.CoreV1().PersistentVolumeClaims("demo-ns").Get(context.Background(), "manual42", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("PVC still present: %v", err)
	}
	if len(recorder.options) != 1 || recorder.options[0].Preconditions == nil || recorder.options[0].Preconditions.UID == nil || *recorder.options[0].Preconditions.UID != "dv-uid" {
		t.Fatal("DV delete lacks original UID precondition")
	}
	for _, actions := range [][]k8stesting.Action{kube.Actions()} {
		for _, a := range actions {
			if a.GetVerb() == "delete" {
				opts := a.(k8stesting.DeleteAction).GetDeleteOptions()
				if opts.Preconditions == nil || opts.Preconditions.UID == nil || *opts.Preconditions.UID == "" {
					t.Fatal("delete lacks UID precondition")
				}
			}
		}
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteProtectsSnapshotReferences(t *testing.T) {
	snapshot := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshot",
		"metadata": map[string]interface{}{"name": "saved-data", "namespace": "demo-ns"},
		"spec":     map[string]interface{}{"source": map[string]interface{}{"persistentVolumeClaimName": "manual42"}},
	}}
	action, client, kube := diskDeleteFixture(t, snapshot)
	err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
	if !apierrors.IsConflict(err) {
		t.Fatalf("expected referenced disk conflict, got %v", err)
	}
	for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
		for _, a := range actions {
			if a.GetVerb() == "delete" {
				t.Fatal("referenced disk was deleted")
			}
		}
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteProtectsPodAndForeignOwner(t *testing.T) {
	for _, scenario := range []string{"pod", "foreign-owner", "missing-uid", "root"} {
		t.Run(scenario, func(t *testing.T) {
			action, client, kube := diskDeleteFixture(t)
			volume := &dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"}
			switch scenario {
			case "pod":
				_, err := kube.CoreV1().Pods("demo-ns").Create(context.Background(), &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "consumer"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "shared", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "manual42"}}}}}}, metav1.CreateOptions{})
				if err != nil {
					t.Fatal(err)
				}
			case "foreign-owner":
				pvc, _ := kube.CoreV1().PersistentVolumeClaims("demo-ns").Get(context.Background(), "manual42", metav1.GetOptions{})
				pvc.Labels["service_id"] = "another-component"
				_, _ = kube.CoreV1().PersistentVolumeClaims("demo-ns").Update(context.Background(), pvc, metav1.UpdateOptions{})
			case "missing-uid":
				dv, _ := client.Resource(dataVolumeGVR).Namespace("demo-ns").Get(context.Background(), "manual42", metav1.GetOptions{})
				dv.SetUID("")
				_, _ = client.Resource(dataVolumeGVR).Namespace("demo-ns").Update(context.Background(), dv, metav1.UpdateOptions{})
			case "root":
				volume.VolumeName = "disk"
			}
			err := action.hotunplugVMDataDisk(volume)
			if err == nil {
				t.Fatal("expected unsafe deletion to be rejected")
			}
			for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
				for _, a := range actions {
					if a.GetVerb() == "delete" {
						t.Fatal("unsafe disk was deleted")
					}
				}
			}
		})
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteWaitsForPhysicalPVCRemoval(t *testing.T) {
	action, _, kube := diskDeleteFixture(t)
	kube.PrependReactor("delete", "persistentvolumeclaims", func(k8stesting.Action) (bool, runtime.Object, error) { return true, nil, nil })
	err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
	if !apierrors.IsConflict(err) || !strings.Contains(err.Error(), "回收") {
		t.Fatalf("expected pending reclaim conflict, got %v", err)
	}
}

// The pinned dynamic fake drops DeleteOptions; this wrapper records the real call.
type diskDeleteRecorder struct {
	dynamic.Interface
	options []metav1.DeleteOptions
}

func (c *diskDeleteRecorder) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &diskDeleteNamespace{NamespaceableResourceInterface: c.Interface.Resource(gvr), recorder: c}
}

type diskDeleteNamespace struct {
	dynamic.NamespaceableResourceInterface
	recorder *diskDeleteRecorder
}

func (c *diskDeleteNamespace) Namespace(ns string) dynamic.ResourceInterface {
	return &diskDeleteResource{ResourceInterface: c.NamespaceableResourceInterface.Namespace(ns), recorder: c.recorder}
}

type diskDeleteResource struct {
	dynamic.ResourceInterface
	recorder *diskDeleteRecorder
}

func (c *diskDeleteResource) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	c.recorder.options = append(c.recorder.options, opts)
	return c.ResourceInterface.Delete(ctx, name, opts, subresources...)
}

func diskDeleteVMFixture(t *testing.T, running, hotpluggable bool) (*ServiceAction, *dynamicfake.FakeDynamicClient, *k8sfake.Clientset) {
	t.Helper()
	boot := uint(1)
	vm := &v1.VirtualMachine{TypeMeta: metav1.TypeMeta{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachine"}, ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "demo-ns", UID: "vm-uid", ResourceVersion: "1", Labels: map[string]string{"service_id": "service-vm"}}, Spec: v1.VirtualMachineSpec{Template: &v1.VirtualMachineInstanceTemplateSpec{Spec: v1.VirtualMachineInstanceSpec{Volumes: []v1.Volume{
		{Name: "root", VolumeSource: v1.VolumeSource{DataVolume: &v1.DataVolumeSource{Name: "root"}}},
		{Name: "manual42", VolumeSource: v1.VolumeSource{DataVolume: &v1.DataVolumeSource{Name: "manual42", Hotpluggable: hotpluggable}}},
	}, Domain: v1.DomainSpec{Devices: v1.Devices{Disks: []v1.Disk{{Name: "root", BootOrder: &boot}, {Name: "manual42"}}}}}}}}
	raw, err := json.Marshal(vm)
	obj := &unstructured.Unstructured{}
	if err == nil {
		err = obj.UnmarshalJSON(raw)
	}
	if err != nil {
		t.Fatal(err)
	}
	extras := []runtime.Object{obj}
	if running {
		vmi := &v1.VirtualMachineInstance{TypeMeta: metav1.TypeMeta{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachineInstance"}, ObjectMeta: metav1.ObjectMeta{Name: vm.Name, Namespace: vm.Namespace, UID: "vmi-uid", Labels: map[string]string{"service_id": "service-vm"}, OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachine", Name: vm.Name, UID: vm.UID}}}, Spec: vm.Spec.Template.Spec}
		raw, err := json.Marshal(vmi)
		instance := &unstructured.Unstructured{}
		if err == nil {
			err = instance.UnmarshalJSON(raw)
		}
		if err != nil {
			t.Fatal(err)
		}
		extras = append(extras, instance)
	}
	action, client, kube := diskDeleteFixture(t, extras...)
	action.getVirtualMachineByServiceIDHook = func(string) (*v1.VirtualMachine, error) { return vm.DeepCopy(), nil }
	return action, client, kube
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteStoppedVMRemovesTemplateAndReclaims(t *testing.T) {
	action, client, kube := diskDeleteVMFixture(t, false, false)
	if err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"}); err != nil {
		t.Fatal(err)
	}
	vm, err := client.Resource(vmDiskVMGVR).Namespace("demo-ns").Get(context.Background(), "demo-vm", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if vmDiskObjectReferences(vm.Object["spec"], "manual42") || !vmDiskObjectReferences(vm.Object["spec"], "root") {
		t.Fatal("VM data disk removal corrupted template")
	}
	if _, err := kube.CoreV1().PersistentVolumeClaims("demo-ns").Get(context.Background(), "manual42", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatal("PVC remains")
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteRejectsRestartedStaticDiskBeforeMutation(t *testing.T) {
	action, client, kube := diskDeleteVMFixture(t, true, false)
	err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
	if !apierrors.IsConflict(err) || !strings.Contains(err.Error(), "关闭虚拟机") {
		t.Fatalf("expected actionable shutdown conflict, got %v", err)
	}
	for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
		for _, a := range actions {
			if a.GetVerb() == "delete" || a.GetVerb() == "update" {
				t.Fatal("static live disk was mutated")
			}
		}
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteHotplugWaitsForDetach(t *testing.T) {
	action, client, kube := diskDeleteVMFixture(t, true, true)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVM := kubecli.NewMockVirtualMachineInterface(ctrl)
	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVM)
	mockVM.EXPECT().RemoveVolume(gomock.Any(), "demo-vm", gomock.Any()).Return(nil)
	action.kubevirtClient = mockClient
	err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
	if !apierrors.IsConflict(err) || !strings.Contains(err.Error(), "卸载") {
		t.Fatalf("expected pending detach conflict, got %v", err)
	}
	for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
		for _, a := range actions {
			if a.GetVerb() == "delete" {
				t.Fatal("disk deleted before detach")
			}
		}
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteProtectsAliasedRootAndForgedVMI(t *testing.T) {
	for _, scenario := range []string{"root-alias", "forged-vmi-owner"} {
		t.Run(scenario, func(t *testing.T) {
			action, client, kube := diskDeleteVMFixture(t, true, false)
			if scenario == "root-alias" {
				oldHook := action.getVirtualMachineByServiceIDHook
				action.getVirtualMachineByServiceIDHook = func(sid string) (*v1.VirtualMachine, error) {
					vm, err := oldHook(sid)
					vm.Spec.Template.Spec.Volumes[1].Name = "aliased-root"
					vm.Spec.Template.Spec.Domain.Devices.Disks[1].Name = "aliased-root"
					boot := uint(1)
					vm.Spec.Template.Spec.Domain.Devices.Disks[1].BootOrder = &boot
					second := uint(2)
					vm.Spec.Template.Spec.Domain.Devices.Disks[0].BootOrder = &second
					return vm, err
				}
			} else {
				instance, _ := client.Resource(vmDiskVMIGVR).Namespace("demo-ns").Get(context.Background(), "demo-vm", metav1.GetOptions{})
				instance.SetOwnerReferences([]metav1.OwnerReference{{Kind: "VirtualMachine", Name: "demo-vm", UID: "another-vm-uid"}})
				_, err := client.Resource(vmDiskVMIGVR).Namespace("demo-ns").Update(context.Background(), instance, metav1.UpdateOptions{})
				if err != nil {
					t.Fatal(err)
				}
			}
			client.ClearActions()
			kube.ClearActions()
			err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
			if !apierrors.IsConflict(err) {
				t.Fatalf("expected safety conflict, got %v", err)
			}
			for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
				for _, a := range actions {
					if a.GetVerb() == "delete" || a.GetVerb() == "update" {
						t.Fatal("unsafe disk was mutated")
					}
				}
			}
		})
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteProtectsCrossNamespaceClone(t *testing.T) {
	clone := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "clone-data", "namespace": "other-team"}, "spec": map[string]interface{}{"source": map[string]interface{}{"pvc": map[string]interface{}{"namespace": "demo-ns", "name": "manual42"}}}}}
	action, client, kube := diskDeleteFixture(t, clone)
	err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
	if !apierrors.IsConflict(err) {
		t.Fatalf("expected clone reference conflict, got %v", err)
	}
	for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
		for _, a := range actions {
			if a.GetVerb() == "delete" {
				t.Fatal("cloned source was deleted")
			}
		}
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteFailsClosedWhenReferencesUnreadable(t *testing.T) {
	action, client, kube := diskDeleteFixture(t)
	client.PrependReactor("list", "volumesnapshots", func(a k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(a.GetResource().GroupResource(), "", fmt.Errorf("forbidden"))
	})
	err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
	if err == nil {
		t.Fatal("unreadable references were ignored")
	}
	for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
		for _, a := range actions {
			if a.GetVerb() == "delete" {
				t.Fatal("disk deleted without reference visibility")
			}
		}
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteRequiresNamedRecord(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	gdb, err := gorm.Open("mysql", sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	tx := gdb.Begin()
	mock.ExpectRollback()
	db.SetTestManager(hotplugVolumeTestManager{tx: tx, serviceDao: &hotplugTenantServiceDao{service: &dbmodel.TenantServices{ServiceID: "service-vm", ExtendMethod: "vm"}}})
	defer db.SetTestManager(nil)
	apiErr := (&ServiceAction{}).VolumnVar(&dbmodel.TenantServiceVolume{ServiceID: "service-vm", VolumePath: "/disk"}, "tenant-1", "", "delete")
	if apiErr == nil || apiErr.Code != 400 {
		t.Fatalf("expected named deletion requirement, got %v", apiErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteProtectsDataSourceAndPendingAdd(t *testing.T) {
	for _, scenario := range []string{"datasource", "pending-add"} {
		t.Run(scenario, func(t *testing.T) {
			var extras []runtime.Object
			if scenario == "datasource" {
				extras = []runtime.Object{&unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataSource", "metadata": map[string]interface{}{"name": "clone-source", "namespace": "other-team"}, "spec": map[string]interface{}{"source": map[string]interface{}{"pvc": map[string]interface{}{"name": "manual42", "namespace": "demo-ns"}}}}}, &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "clone", "namespace": "other-team"}, "spec": map[string]interface{}{"sourceRef": map[string]interface{}{"kind": "DataSource", "name": "clone-source"}}}}}
			} else {
				extras = []runtime.Object{&unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachine", "metadata": map[string]interface{}{"name": "consumer", "namespace": "demo-ns"}, "status": map[string]interface{}{"volumeRequests": []interface{}{map[string]interface{}{"addVolumeOptions": map[string]interface{}{"name": "pending-disk", "volumeSource": map[string]interface{}{"dataVolume": map[string]interface{}{"name": "manual42"}}}}}}}}}
			}
			action, client, kube := diskDeleteFixture(t, extras...)
			err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
			if !apierrors.IsConflict(err) {
				t.Fatalf("expected reference conflict, got %v", err)
			}
			for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
				for _, a := range actions {
					if a.GetVerb() == "delete" {
						t.Fatal("referenced disk deleted")
					}
				}
			}
		})
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteProtectsOwnPendingAddBeforeMutation(t *testing.T) {
	action, client, kube := diskDeleteVMFixture(t, false, false)
	obj, _ := client.Resource(vmDiskVMGVR).Namespace("demo-ns").Get(context.Background(), "demo-vm", metav1.GetOptions{})
	raw, _ := json.Marshal(buildVMHotplugAddVolumeOptions("manual42", "/disk-1"))
	options := map[string]interface{}{}
	_ = json.Unmarshal(raw, &options)
	obj.Object["status"] = map[string]interface{}{"volumeRequests": []interface{}{map[string]interface{}{"addVolumeOptions": options}}}
	if _, err := client.Resource(vmDiskVMGVR).Namespace("demo-ns").Update(context.Background(), obj, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	client.ClearActions()
	kube.ClearActions()
	err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
	if !apierrors.IsConflict(err) {
		t.Fatalf("expected pending own request conflict, got %v", err)
	}
	for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
		for _, a := range actions {
			if a.GetVerb() == "update" || a.GetVerb() == "delete" {
				t.Fatal("pending disk operation was mutated")
			}
		}
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteRetainsInitialResourceUIDsAcrossPreflight(t *testing.T) {
	for _, scenario := range []string{"replaced", "initially-missing"} {
		t.Run(scenario, func(t *testing.T) {
			action, client, kube := diskDeleteFixture(t)
			gets := 0
			client.PrependReactor("get", "datavolumes", func(a k8stesting.Action) (bool, runtime.Object, error) {
				gets++
				if scenario == "initially-missing" && gets == 1 {
					return true, nil, apierrors.NewNotFound(dataVolumeGVR.GroupResource(), "manual42")
				}
				if scenario == "replaced" && gets == 2 {
					obj, err := client.Tracker().Get(dataVolumeGVR, "demo-ns", "manual42")
					if err != nil {
						t.Fatal(err)
					}
					dv := obj.(*unstructured.Unstructured).DeepCopy()
					dv.SetUID("replacement-dv-uid")
					if err := client.Tracker().Update(dataVolumeGVR, dv, "demo-ns"); err != nil {
						t.Fatal(err)
					}
					pvcObj, err := kube.Tracker().Get(corev1.SchemeGroupVersion.WithResource("persistentvolumeclaims"), "demo-ns", "manual42")
					if err != nil {
						t.Fatal(err)
					}
					pvc := pvcObj.(*corev1.PersistentVolumeClaim).DeepCopy()
					pvc.UID = "replacement-pvc-uid"
					if err := kube.Tracker().Update(corev1.SchemeGroupVersion.WithResource("persistentvolumeclaims"), pvc, "demo-ns"); err != nil {
						t.Fatal(err)
					}
				}
				return false, nil, nil
			})
			err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
			if !apierrors.IsConflict(err) {
				t.Fatalf("expected identity change conflict, got %v", err)
			}
			for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
				for _, a := range actions {
					if a.GetVerb() == "delete" {
						t.Fatal("replacement storage was deleted")
					}
				}
			}
		})
	}
}

func mappedDiskDeleteFixture(t *testing.T, scenario string) (*ServiceAction, *dynamicfake.FakeDynamicClient, *k8sfake.Clientset) {
	t.Helper()
	action, client, kube := diskDeleteVMFixture(t, false, false)
	if scenario == "vm-source-remapped" {
		oldHook := action.getVirtualMachineByServiceIDHook
		action.getVirtualMachineByServiceIDHook = func(sid string) (*v1.VirtualMachine, error) {
			vm, err := oldHook(sid)
			vm.Spec.Template.Spec.Volumes[1].DataVolume.Name = "restored-dv"
			return vm, err
		}
	} else {
		dv, err := client.Resource(dataVolumeGVR).Namespace("demo-ns").Get(context.Background(), "manual42", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		dv.Object["status"] = map[string]interface{}{"claimName": "current-claim"}
		if _, err := client.Resource(dataVolumeGVR).Namespace("demo-ns").Update(context.Background(), dv, metav1.UpdateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	current := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "current-claim", Namespace: "demo-ns", UID: "current-pvc-uid", Labels: map[string]string{"service_id": "service-vm"}}}
	if _, err := kube.CoreV1().PersistentVolumeClaims("demo-ns").Create(context.Background(), current, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	client.ClearActions()
	kube.ClearActions()
	return action, client, kube
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteRejectsRemappedBackingBeforeAnyMutation(t *testing.T) {
	for _, scenario := range []string{"vm-source-remapped", "dv-claim-remapped"} {
		t.Run(scenario, func(t *testing.T) {
			action, client, kube := mappedDiskDeleteFixture(t, scenario)
			err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
			if !apierrors.IsConflict(err) {
				t.Fatalf("expected remapped disk conflict before reclaim, got %v", err)
			}
			for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
				for _, a := range actions {
					if a.GetVerb() == "delete" || a.GetVerb() == "update" {
						t.Fatal("remapped disk operation mutated VM or storage")
					}
				}
			}
			for _, name := range []string{"manual42", "current-claim"} {
				if _, err := kube.CoreV1().PersistentVolumeClaims("demo-ns").Get(context.Background(), name, metav1.GetOptions{}); err != nil {
					t.Fatalf("preserved PVC %s: %v", name, err)
				}
			}
		})
	}
}

type mappedDiskDeleteManager struct {
	hotunplugVolumeDeleteTestManager
	serviceDao dbdao.TenantServiceDao
}

func (m mappedDiskDeleteManager) TenantServiceDao() dbdao.TenantServiceDao { return m.serviceDao }

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVolumnVarRetainsRecordForRemappedDataDisk(t *testing.T) {
	for _, scenario := range []string{"vm-source-remapped", "dv-claim-remapped"} {
		t.Run(scenario, func(t *testing.T) {
			action, _, _ := mappedDiskDeleteFixture(t, scenario)
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			gdb, err := gorm.Open("mysql", sqlDB)
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			tx := gdb.Begin()
			mock.ExpectRollback()
			volumeDao := &hotunplugTenantServiceVolumeDao{volume: &dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"}}
			db.SetTestManager(mappedDiskDeleteManager{hotunplugVolumeDeleteTestManager: hotunplugVolumeDeleteTestManager{tx: tx, volumeDao: volumeDao, mountRelationDao: &hotunplugTenantServiceMountRelationDao{}, configFileDao: &hotunplugTenantServiceConfigFileDao{}}, serviceDao: &hotplugTenantServiceDao{service: &dbmodel.TenantServices{ServiceID: "service-vm", ExtendMethod: "vm", Namespace: "demo-ns"}}})
			mq := &recordingMQClient{}
			action.MQClient = mq
			apiErr := action.VolumnVar(&dbmodel.TenantServiceVolume{ServiceID: "service-vm", VolumeName: "data-1"}, "tenant-1", "", "delete")
			if apiErr == nil || apiErr.Code != 409 {
				t.Fatalf("expected 409 retaining record, got %v", apiErr)
			}
			if volumeDao.deletedVolume != "" || len(mq.tasks) > 0 {
				t.Fatal("remapped volume record or GC state was changed")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// capability_id: rainbond.vm-disk.retry-safe-delete
func TestVMDataDiskDeleteRejectsNamespaceMismatchBeforeMutation(t *testing.T) {
	action, client, kube := diskDeleteVMFixture(t, false, false)
	vm, _ := action.getVirtualMachineByServiceIDHook("service-vm")
	vm.Namespace = "other-team"
	vm.UID = "other-vm-uid"
	action.getVirtualMachineByServiceIDHook = func(string) (*v1.VirtualMachine, error) { return vm.DeepCopy(), nil }
	raw, _ := json.Marshal(vm)
	obj := &unstructured.Unstructured{}
	_ = obj.UnmarshalJSON(raw)
	if _, err := client.Resource(vmDiskVMGVR).Namespace("other-team").Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	dv, _ := client.Resource(dataVolumeGVR).Namespace("demo-ns").Get(context.Background(), "manual42", metav1.GetOptions{})
	dv.SetNamespace("other-team")
	dv.SetUID("other-dv-uid")
	if _, err := client.Resource(dataVolumeGVR).Namespace("other-team").Create(context.Background(), dv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	pvc, _ := kube.CoreV1().PersistentVolumeClaims("demo-ns").Get(context.Background(), "manual42", metav1.GetOptions{})
	pvc.Namespace = "other-team"
	pvc.UID = "other-pvc-uid"
	if _, err := kube.CoreV1().PersistentVolumeClaims("other-team").Create(context.Background(), pvc, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	client.ClearActions()
	kube.ClearActions()
	err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: "service-vm", VolumeName: "data-1", VolumePath: "/disk-1"})
	if !apierrors.IsConflict(err) || !strings.Contains(err.Error(), "命名空间") {
		t.Fatalf("expected namespace safety conflict, got %v", err)
	}
	for _, actions := range [][]k8stesting.Action{client.Actions(), kube.Actions()} {
		for _, a := range actions {
			if a.GetVerb() == "delete" || a.GetVerb() == "update" {
				t.Fatal("namespace mismatch changed a VM or disk")
			}
		}
	}
	for _, namespace := range []string{"demo-ns", "other-team"} {
		if _, err := kube.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), "manual42", metav1.GetOptions{}); err != nil {
			t.Fatal(err)
		}
	}
}
