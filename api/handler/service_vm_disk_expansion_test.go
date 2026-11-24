package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	v1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
)

// Keep legacy canonical scenarios, but materialize their actual UID in the real shared Store.
func vmExpansionFixture(t *testing.T, observed string, gates []string, listErr error) (*ServiceAction, *dbmodel.TenantServices, *dbmodel.TenantServiceVolume, *dynamicfake.FakeDynamicClient, *k8sfake.Clientset) {
	t.Helper()
	f := newBindingConsumerFixture(t)
	dv := buildVMHotplugDataVolumeObject(f.scope.Namespace, f.scope.TenantID, f.volume, "manual42")
	dv.SetUID("dv-uid")
	dv.Object["status"] = map[string]interface{}{"claimName": "manual42"}
	if _, err := f.dynamic.Resource(dataVolumeGVR).Namespace(f.scope.Namespace).Create(context.Background(), dv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		binding := c.State.Bindings["42"]
		binding.ActivePVC = vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "manual42", UID: "b-uid"}
		binding.ActiveDV = &vmrestore.ObjectRef{Kind: "DataVolume", Namespace: f.scope.Namespace, Name: "manual42", UID: "dv-uid"}
		c.State.Bindings["42"] = binding
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	first, second := uint(1), uint(2)
	vm := &v1.VirtualMachine{TypeMeta: metav1.TypeMeta{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachine"}, ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: f.scope.Namespace, UID: "vm-uid", ResourceVersion: "1", Labels: map[string]string{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}}, Spec: v1.VirtualMachineSpec{Template: &v1.VirtualMachineInstanceTemplateSpec{Spec: v1.VirtualMachineInstanceSpec{Volumes: []v1.Volume{{Name: "root", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "root-disk"}}}}, {Name: "manual42", VolumeSource: v1.VolumeSource{DataVolume: &v1.DataVolumeSource{Name: "manual42"}}}}, Domain: v1.DomainSpec{Devices: v1.Devices{Disks: []v1.Disk{{Name: "root", BootOrder: &first}, {Name: "manual42", BootOrder: &second}}}}}}}}
	persistExpansionTestVM(t, f.dynamic, vm)
	f.action.getVirtualMachineByServiceIDHook = func(string) (*v1.VirtualMachine, error) {
		obj, err := f.dynamic.Resource(vmDiskVMGVR).Namespace(f.scope.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		result := &v1.VirtualMachine{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, result)
		return result, err
	}
	ctrl := gomock.NewController(t)
	kvClient := kubecli.NewMockKubevirtClient(ctrl)
	config := kubecli.NewMockKubeVirtInterface(ctrl)
	kvClient.EXPECT().KubeVirt("").Return(config).AnyTimes()
	config.EXPECT().List(gomock.Any(), gomock.Any()).Return(&v1.KubeVirtList{Items: []v1.KubeVirt{{Spec: v1.KubeVirtSpec{Configuration: v1.KubeVirtConfiguration{DeveloperConfiguration: &v1.DeveloperConfiguration{FeatureGates: gates}}}, Status: v1.KubeVirtStatus{Phase: v1.KubeVirtPhaseDeployed, ObservedKubeVirtVersion: observed}}}}, listErr).AnyTimes()
	f.action.kubevirtClient = kvClient
	return f.action, f.service, f.volume, f.dynamic, f.core
}

func persistExpansionTestVM(t *testing.T, client *dynamicfake.FakeDynamicClient, vm *v1.VirtualMachine) {
	t.Helper()
	raw, err := json.Marshal(vm)
	if err != nil {
		t.Fatal(err)
	}
	obj := &unstructured.Unstructured{}
	if err = obj.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	obj.SetAPIVersion("kubevirt.io/v1")
	obj.SetKind("VirtualMachine")
	resource := client.Resource(vmDiskVMGVR).Namespace("team")
	if _, err = resource.Get(context.Background(), vm.Name, metav1.GetOptions{}); err == nil {
		_, err = resource.Update(context.Background(), obj, metav1.UpdateOptions{})
	} else {
		_, err = resource.Create(context.Background(), obj, metav1.CreateOptions{})
	}
	if err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-disk.conditional-expansion
func TestVMDataDiskExpansionRequiresActualKubeVirtCapability(t *testing.T) {
	cases := []struct {
		name, version        string
		gates                []string
		err                  error
		wantAllowed, wantErr bool
	}{
		{name: "deployed 1.8.1 still needs explicit expansion gate", version: "v1.8.1"},
		{name: "deployed 1.8.1 with expansion gate", version: "v1.8.1", gates: []string{"ExpandDisks"}, wantAllowed: true},
		{name: "old version with explicit gate", version: "v1.5.0", gates: []string{"ExpandDisks"}, wantAllowed: true},
		{name: "old version gate absent", version: "v1.5.0"},
		{name: "unknown version gate absent", version: "unknown"},
		{name: "query error cannot enable", version: "v1.8.1", err: errors.New("configuration unavailable"), wantErr: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			action, service, volume, _, _ := vmExpansionFixture(t, tt.version, tt.gates, tt.err)
			status, claims, err := action.inspectVolumeExpansion(context.Background(), service, volume)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%t", err, tt.wantErr)
			}
			if status.AllowExpansion != tt.wantAllowed {
				t.Fatalf("status=%+v, wantAllowed=%t", status, tt.wantAllowed)
			}
			if tt.wantAllowed && (len(claims) != 1 || claims[0].Name != "manual42" || !strings.Contains(status.Message, "来宾")) {
				t.Fatalf("missing exact claim or guest limitation: status=%+v claims=%v", status, claims)
			}
		})
	}
}

// capability_id: rainbond.vm-disk.conditional-expansion
func TestVMDataDiskExpansionFollowsCurrentDataVolumeClaim(t *testing.T) {
	action, service, volume, client, kube := vmExpansionFixture(t, "v1.8.1", []string{"ExpandDisks"}, nil)
	vm, err := action.getVirtualMachineByServiceIDHook(service.ServiceID)
	if err != nil {
		t.Fatal(err)
	}
	vm.Spec.Template.Spec.Volumes[1].DataVolume.Name = "restored-data"
	persistExpansionTestVM(t, client, vm)
	dv := buildVMHotplugDataVolumeObject("team", service.TenantID, volume, "restored-data")
	dv.SetUID("restored-dv-uid")
	dv.Object["status"] = map[string]interface{}{"claimName": "current-claim"}
	if _, err := client.Resource(dataVolumeGVR).Namespace("team").Create(context.Background(), dv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	current := expansionPVC("team", "current-claim", "service-vm", "data-1", "fast", "1Gi", "1Gi")
	current.UID = "current-pvc-uid"
	if _, err := kube.CoreV1().PersistentVolumeClaims("team").Create(context.Background(), current, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	store := vmrestore.NewStore(action.getDBManager().DB())
	scope := vmrestore.Scope{TenantID: service.TenantID, ServiceID: service.ServiceID, Namespace: "team"}
	state, err := store.Read(scope, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Mutate(scope, vmrestore.Expectation{ControlRevision: state.State.ControlRevision, BindingGeneration: state.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		binding := c.State.Bindings["42"]
		binding.ActivePVC = vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: "team", Name: "current-claim", UID: "current-pvc-uid"}
		binding.ActiveDV = &vmrestore.ObjectRef{Kind: "DataVolume", Namespace: "team", Name: "restored-data", UID: "restored-dv-uid"}
		c.State.Bindings["42"] = binding
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := action.expandVolumeClaims(context.Background(), service, volume, 2); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"manual42": "1Gi", "current-claim": "2Gi"} {
		pvc, err := kube.CoreV1().PersistentVolumeClaims("team").Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		q := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		if q.String() != want {
			t.Fatalf("PVC %s request=%s want=%s", name, q.String(), want)
		}
	}
}

// capability_id: rainbond.vm-disk.conditional-expansion
func TestVMDataDiskExpansionRejectsUnsafeOrUnsupportedWithoutMutation(t *testing.T) {
	for _, scenario := range []string{"cdrom", "root", "shared", "foreign-owner", "missing-uid", "nfs-subdir", "expansion-disabled", "other-pod"} {
		t.Run(scenario, func(t *testing.T) {
			action, service, volume, _, kube := vmExpansionFixture(t, "v1.8.1", []string{"ExpandDisks"}, nil)
			switch scenario {
			case "cdrom":
				volume.VolumePath = "/cdrom"
			case "root":
				volume.VolumeName = "disk"
			case "shared":
				if err := action.getDBManager().DB().Create(&dbmodel.TenantServiceMountRelation{TenantID: service.TenantID, ServiceID: "consumer", DependServiceID: service.ServiceID, VolumeName: volume.VolumeName}).Error; err != nil {
					t.Fatal(err)
				}
			case "foreign-owner", "missing-uid":
				pvc, _ := kube.CoreV1().PersistentVolumeClaims("team").Get(context.Background(), "manual42", metav1.GetOptions{})
				if scenario == "foreign-owner" {
					pvc.Labels["service_id"] = "other-component"
				} else {
					pvc.UID = ""
				}
				_, _ = kube.CoreV1().PersistentVolumeClaims("team").Update(context.Background(), pvc, metav1.UpdateOptions{})
			case "nfs-subdir", "expansion-disabled":
				sc, _ := kube.StorageV1().StorageClasses().Get(context.Background(), "fast", metav1.GetOptions{})
				if scenario == "nfs-subdir" {
					sc.Provisioner = "cluster.local/nfs-subdir-external-provisioner"
				} else {
					allowed := false
					sc.AllowVolumeExpansion = &allowed
				}
				_, _ = kube.StorageV1().StorageClasses().Update(context.Background(), sc, metav1.UpdateOptions{})
			case "other-pod":
				_, _ = kube.CoreV1().Pods("team").Create(context.Background(), &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "consumer"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "manual42"}}}}}}, metav1.CreateOptions{})
			}
			kube.ClearActions()
			if err := action.expandVolumeClaims(context.Background(), service, volume, 2); err == nil {
				t.Fatal("unsafe expansion accepted")
			}
			for _, a := range kube.Actions() {
				if a.GetResource().Resource == "persistentvolumeclaims" && a.GetVerb() == "update" {
					t.Fatal("rejected request changed PVC")
				}
			}
		})
	}
}

// capability_id: rainbond.vm-disk.conditional-expansion
func TestVMDataDiskExpansionReportsPVCProgressWithoutClaimingGuestCompletion(t *testing.T) {
	action, service, volume, _, kube := vmExpansionFixture(t, "v1.8.1", []string{"ExpandDisks"}, nil)
	if err := action.expandVolumeClaims(context.Background(), service, volume, 2); err != nil {
		t.Fatal(err)
	}
	status, _, err := action.inspectVolumeExpansion(context.Background(), service, volume)
	if err != nil || status.Status != volumeExpansionResizing || status.ActualCapacity != 1 || status.RequestedCapacity != 2 || !strings.Contains(status.Message, "来宾") {
		t.Fatalf("wrong accepted state: %+v err=%v", status, err)
	}
	// Retrying the accepted target is idempotent even if a previous SQL commit failed.
	if err := action.expandVolumeClaims(context.Background(), service, volume, 2); err != nil {
		t.Fatal(err)
	}
	pvc, _ := kube.CoreV1().PersistentVolumeClaims("team").Get(context.Background(), "manual42", metav1.GetOptions{})
	pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}
	_, _ = kube.CoreV1().PersistentVolumeClaims("team").UpdateStatus(context.Background(), pvc, metav1.UpdateOptions{})
	status, _, err = action.inspectVolumeExpansion(context.Background(), service, volume)
	if err != nil || status.Status != volumeExpansionReady || status.ActualCapacity != 2 || !strings.Contains(status.Message, "来宾") {
		t.Fatalf("PVC ready lost guest limitation: %+v err=%v", status, err)
	}
}

// capability_id: rainbond.vm-disk.conditional-expansion
func TestVMDataDiskExpansionDoesNotResizeReplacedPVC(t *testing.T) {
	action, service, volume, _, kube := vmExpansionFixture(t, "v1.8.1", []string{"ExpandDisks"}, nil)
	gets := 0
	kube.PrependReactor("get", "persistentvolumeclaims", func(a k8stesting.Action) (bool, runtime.Object, error) {
		gets++
		if gets == 2 {
			pvc := expansionPVC("team", "manual42", "service-vm", "data-1", "fast", "1Gi", "1Gi")
			pvc.UID = "replaced-uid"
			return true, pvc, nil
		}
		return false, nil, nil
	})
	kube.ClearActions()
	if err := action.expandVolumeClaims(context.Background(), service, volume, 2); err == nil {
		t.Fatal("replaced claim was accepted")
	}
	for _, a := range kube.Actions() {
		if a.GetVerb() == "update" && a.GetResource().Resource == "persistentvolumeclaims" {
			t.Fatal("replaced claim was resized")
		}
	}
}

// capability_id: rainbond.vm-disk.conditional-expansion
func TestVMDataDiskExpansionAllowsOwnedLauncherAndRejectsOtherVM(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(fmt.Sprintf("foreign=%t", foreign), func(t *testing.T) {
			action, service, volume, client, kube := vmExpansionFixture(t, "v1.8.1", []string{"ExpandDisks"}, nil)
			vm, _ := action.getVirtualMachineByServiceIDHook(service.ServiceID)
			instance := &v1.VirtualMachineInstance{TypeMeta: metav1.TypeMeta{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachineInstance"}, ObjectMeta: metav1.ObjectMeta{Name: vm.Name, Namespace: vm.Namespace, UID: "own-vmi-uid", Labels: map[string]string{"service_id": service.ServiceID}, OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachine", Name: vm.Name, UID: vm.UID}}}, Spec: vm.Spec.Template.Spec}
			if foreign {
				instance.Name = "other-vm"
				instance.OwnerReferences[0].UID = "foreign-vm-uid"
			}
			raw, _ := json.Marshal(instance)
			obj := &unstructured.Unstructured{}
			_ = obj.UnmarshalJSON(raw)
			if _, err := client.Resource(vmDiskVMIGVR).Namespace("team").Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			_, _ = kube.CoreV1().Pods("team").Create(context.Background(), &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "launcher", OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachineInstance", Name: instance.Name, UID: instance.UID}}}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "manual42"}}}}}}, metav1.CreateOptions{})
			err := action.expandVolumeClaims(context.Background(), service, volume, 2)
			if (err != nil) != foreign {
				t.Fatalf("foreign=%t got err=%v", foreign, err)
			}
		})
	}
}

// capability_id: rainbond.vm-disk.conditional-expansion
func TestVMDataDiskExpansionProtectsActualBootDeviceAndNamespace(t *testing.T) {
	for _, scenario := range []string{"root-alias", "cdrom", "namespace"} {
		t.Run(scenario, func(t *testing.T) {
			action, service, volume, client, kube := vmExpansionFixture(t, "v1.8.1", []string{"ExpandDisks"}, nil)
			vm, err := action.getVirtualMachineByServiceIDHook(service.ServiceID)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "root-alias":
				vm.Spec.Template.Spec.Volumes[1].Name = "aliased-root"
				vm.Spec.Template.Spec.Domain.Devices.Disks[1].Name = "aliased-root"
				boot := uint(1)
				vm.Spec.Template.Spec.Domain.Devices.Disks[1].BootOrder = &boot
				second := uint(2)
				vm.Spec.Template.Spec.Domain.Devices.Disks[0].BootOrder = &second
			case "cdrom":
				vm.Spec.Template.Spec.Domain.Devices.Disks[1].DiskDevice = v1.DiskDevice{CDRom: &v1.CDRomTarget{}}
			case "namespace":
				vm.Namespace = "other-team"
			}
			if scenario == "namespace" {
				raw, e := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
				if e != nil {
					t.Fatal(e)
				}
				client.PrependReactor("list", "virtualmachines", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, &unstructured.UnstructuredList{Items: []unstructured.Unstructured{{Object: raw}}}, nil
				})
			} else {
				persistExpansionTestVM(t, client, vm)
			}
			kube.ClearActions()
			if err := action.expandVolumeClaims(context.Background(), service, volume, 2); err == nil {
				t.Fatal("unsafe current VM mapping was accepted")
			}
			for _, a := range kube.Actions() {
				if a.GetVerb() == "update" && a.GetResource().Resource == "persistentvolumeclaims" {
					t.Fatal("protected device was resized")
				}
			}
		})
	}
}

// capability_id: rainbond.vm-disk.conditional-expansion
func TestVMDataDiskExpansionDoesNotRoundPartialPVCToReady(t *testing.T) {
	action, service, volume, _, kube := vmExpansionFixture(t, "v1.8.1", []string{"ExpandDisks"}, nil)
	pvc, _ := kube.CoreV1().PersistentVolumeClaims("team").Get(context.Background(), "manual42", metav1.GetOptions{})
	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	pvc.Status.Capacity[corev1.ResourceStorage] = resource.MustParse("1536Mi")
	if _, err := kube.CoreV1().PersistentVolumeClaims("team").Update(context.Background(), pvc, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	status, _, err := action.inspectVolumeExpansion(context.Background(), service, volume)
	if err != nil || status.Status != volumeExpansionResizing {
		t.Fatalf("partial PVC capacity was rounded to completion: %+v err=%v", status, err)
	}
}
