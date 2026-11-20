package handler

import (
	"context"
	"fmt"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kt "k8s.io/client-go/testing"
)

// capability_id: rainbond.vm-disk.active-binding-retry-repair
func TestIndependentRepairRepeatedPreflightRefusalKeepsUnknownRetry(t *testing.T) {
	f := newBindingConsumerFixture(t)
	fail := true
	updates := 0
	f.core.PrependReactor("update", "persistentvolumeclaims", func(kt.Action) (bool, runtime.Object, error) {
		updates++
		if fail {
			fail = false
			return true, nil, fmt.Errorf("independent unknown update")
		}
		return false, nil, nil
	})
	req := &apimodel.UpdVolumeReq{VolumeName: f.volume.VolumeName, VolumePath: f.volume.VolumePath, VolumeType: f.volume.VolumeType, VolumeCapacity: int64PtrForBinding(2)}
	if err := f.action.UpdVolume(f.scope.ServiceID, req); err == nil {
		t.Fatal("setup needs an issued unknown update")
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "temporary-ref", Namespace: f.scope.Namespace, UID: "temporary-ref-uid"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "active-a"}}}}}}
	if _, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		err := f.action.UpdVolume(f.scope.ServiceID, req)
		code, ok := err.(bcode.Coder)
		if !ok || code.GetStatus() != 409 {
			t.Fatalf("referenced retry not409: %v", err)
		}
		state, readErr := f.store.Read(f.scope, "")
		if readErr != nil {
			t.Fatal(readErr)
		}
		unknown := state.State.Admission != nil && state.State.Admission.OutcomeUnknown
		t.Logf("refused_attempt=%d admission_unknown=%t", attempt, unknown)
		if !unknown {
			t.Error("completed refused call looks permanently in-flight")
		}
	}
	if err := f.core.CoreV1().Pods(f.scope.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	err := f.action.UpdVolume(f.scope.ServiceID, req)
	state, _ := f.store.Read(f.scope, "")
	t.Logf("after_reference_release=%v admission_retained=%t update_calls=%d", err, state.State.Admission != nil, updates)
	if err != nil || state.State.Admission != nil {
		t.Error("same original request cannot resume after the reference was released")
	}
}

type independentRealAddManager struct{ bindingConsumerDB }

func (m independentRealAddManager) TenantServiceVolumeDaoTransactions(tx *gorm.DB) dbdao.TenantServiceVolumeDao {
	return &mysqldao.TenantServiceVolumeDaoImpl{DB: tx}
}

func (m independentRealAddManager) TenantServiceConfigFileDaoTransactions(tx *gorm.DB) dbdao.TenantServiceConfigFileDao {
	return &mysqldao.TenantServiceConfigFileDaoImpl{DB: tx}
}

// capability_id: rainbond.vm-disk.active-binding-retry-repair
func TestIndependentRepairConfigAddUnknownSyncCanResumeUsingRealDAO(t *testing.T) {
	f := newBindingConsumerFixture(t)
	f.action.dbmanager = independentRealAddManager{bindingConsumerDB{database: f.database}}
	c, _ := f.store.Read(f.scope, "")
	_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		c.State.Bindings = map[string]vmrestore.Binding{}
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	syncs := 0
	f.action.syncVirtualMachineSpecHook = func(string) error {
		syncs++
		if syncs == 1 {
			return fmt.Errorf("independent sync response unknown")
		}
		return nil
	}
	request := func() *dbmodel.TenantServiceVolume {
		return &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: "retry-config", VolumePath: "/etc/retry-config", VolumeType: dbmodel.ConfigFileVolumeType.String()}
	}
	if first := f.action.VolumnVar(request(), f.scope.TenantID, "fixture-only", "add"); first == nil {
		t.Fatal("setup expected post-commit sync failure")
	}
	if syncs != 1 {
		t.Fatal("setup did not reach the post-commit VM synchronization")
	}
	second := f.action.VolumnVar(request(), f.scope.TenantID, "fixture-only", "add")
	var rows, configs int
	f.database.Model(&dbmodel.TenantServiceVolume{}).Where("service_id = ? AND volume_name = ?", f.scope.ServiceID, "retry-config").Count(&rows)
	f.database.Model(&dbmodel.TenantServiceConfigFile{}).Where("service_id = ? AND volume_name = ?", f.scope.ServiceID, "retry-config").Count(&configs)
	state, _ := f.store.Read(f.scope, "")
	t.Logf("retry=%v volume_rows=%d config_rows=%d syncs=%d admission_retained=%t", second, rows, configs, syncs, state.State.Admission != nil)
	if second != nil || rows != 1 || configs != 1 || state.State.Admission != nil {
		t.Error("unknown-outcome config retry did not resume post-commit synchronization and finish")
	}
	unrelated := f.action.UpdVolume(f.scope.ServiceID, &apimodel.UpdVolumeReq{VolumeName: f.volume.VolumeName, VolumePath: f.volume.VolumePath, VolumeType: f.volume.VolumeType, VolumeCapacity: int64PtrForBinding(2)})
	if code, ok := unrelated.(bcode.Coder); ok {
		t.Logf("unrelated_resize_status=%d", code.GetStatus())
	}
}
