package handler

import (
	"context"
	"testing"

	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kv "kubevirt.io/api/core/v1"
)

type recycleRecoveryDB struct{ bindingConsumerDB }

func (m recycleRecoveryDB) TenantDao() dbdao.TenantDao {
	return &mysqldao.TenantDaoImpl{DB: m.database}
}
func (m recycleRecoveryDB) TenantServiceVolumeDao() dbdao.TenantServiceVolumeDao {
	return &mysqldao.TenantServiceVolumeDaoImpl{DB: m.database}
}

// capability_id: rainbond.vm-recycle.original-disk-preflight
func TestVMRecycleRecoveryRequiresStoppedOriginalVMAndLiveDisks(t *testing.T) {
	for _, mode := range []string{"ready", "missing-disk", "wrong-tenant", "running", "wrong-runtime-disk", "unconfirmed-power"} {
		t.Run(mode, func(t *testing.T) {
			f := newBindingConsumerFixture(t)
			f.action.dbmanager = recycleRecoveryDB{bindingConsumerDB{database: f.database}}
			if err := f.database.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 42).UpdateColumn("volume_name", "disk").Error; err != nil {
				t.Fatal(err)
			}
			current, _ := f.store.Read(f.scope, "")
			if _, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
				binding := c.State.Bindings["42"]
				binding.DiskKey, binding.Role = "disk", "root"
				c.State.Bindings["42"] = binding
				c.State.BindingGeneration++
				return vmrestore.Mutation{State: c.State}, nil
			}); err != nil {
				t.Fatal(err)
			}
			strategy := kv.RunStrategyHalted
			claim := "active-a"
			if mode == "wrong-runtime-disk" {
				claim = "manual42"
			}
			vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "original-vm", Namespace: f.scope.Namespace, UID: "original-vm-uid", Labels: map[string]string{"service_id": f.scope.ServiceID}}, Spec: kv.VirtualMachineSpec{RunStrategy: &strategy, Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Volumes: []kv.Volume{{Name: "manual42", VolumeSource: kv.VolumeSource{PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim}}}}}}}}}
			if mode == "unconfirmed-power" {
				vm.Spec.RunStrategy = nil
			}
			f.action.getVirtualMachineByServiceIDHook = func(string) (*kv.VirtualMachine, error) { return vm, nil }
			if mode == "missing-disk" {
				if err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Delete(context.Background(), "active-a", metav1.DeleteOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "running" {
				vmi := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachineInstance", "metadata": map[string]interface{}{"name": vm.Name, "namespace": vm.Namespace, "uid": "running-vmi"}}}
				if _, err := f.dynamic.Resource(vmDiskVMIGVR).Namespace(vm.Namespace).Create(context.Background(), vmi, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			tenant := f.scope.TenantID
			if mode == "wrong-tenant" {
				tenant = "foreign-tenant"
			}
			ctx := context.WithValue(context.Background(), ctxutil.ContextKey("tenant_id"), tenant)
			result, err := f.action.GetVMRecycleRecovery(ctx, f.scope.ServiceID)
			if mode == "ready" {
				if err != nil || result.VMUID != "original-vm-uid" || !result.StorageReady || len(result.Disks) != 1 || result.Disks[0].PVCUID != "a-uid" {
					t.Fatal("original disk identity was not preserved", result, err)
				}
			} else if err == nil {
				t.Fatal("unverified VM could be recovered", mode, result)
			}
		})
	}
}
