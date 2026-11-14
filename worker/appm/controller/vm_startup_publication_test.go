package controller

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kt "k8s.io/client-go/testing"
	kv "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

// capability_id: rainbond.vm-restore.startup-publication-worker
func TestWorkerPublishesExactVMAndGeneratedMediaIntoPrivateStartupRecord(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "startup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&dbmodel.Tenants{}, &dbmodel.TenantServices{}, &dbmodel.ComponentK8sAttributes{}).Error; err != nil {
		t.Fatal(err)
	}
	scope := vmrestore.Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	if err := database.Create(&dbmodel.Tenants{UUID: scope.TenantID, Namespace: scope.Namespace}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&dbmodel.TenantServices{TenantID: scope.TenantID, ServiceID: scope.ServiceID, ExtendMethod: string(dbmodel.ServiceTypeVM)}).Error; err != nil {
		t.Fatal(err)
	}
	previous := db.GetManager()
	db.SetTestManager(restoreGuardDB{database: database})
	defer db.SetTestManager(previous)
	core := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: scope.Namespace, UID: "namespace-uid"}})
	core.PrependReactor("create", "configmaps", func(action kt.Action) (bool, runtime.Object, error) {
		action.(kt.CreateAction).GetObject().(*corev1.ConfigMap).UID = "environment-uid"
		return false, nil, nil
	})
	app := v1.AppService{AppServiceBase: v1.AppServiceBase{TenantID: scope.TenantID, ServiceID: scope.ServiceID, VMStartupHints: &vmrestore.VMStartupHints{NoCloudVolumeName: "cloud", NoCloudDiskName: "cloud", ConfigMaps: []vmrestore.VMStartupConfigMap{{VolumeName: "env", DiskName: "env", Name: "environment"}}}}}
	app.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: scope.Namespace}})
	app.SetVirtualMachine(&kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: scope.Namespace}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Volumes: []kv.Volume{{Name: "cloud", VolumeSource: kv.VolumeSource{CloudInitNoCloud: &kv.CloudInitNoCloudSource{UserData: "INITIALIZATION_SENTINEL"}}}, {Name: "env", VolumeSource: kv.VolumeSource{ConfigMap: &kv.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "environment"}}}}}}}}})
	app.SetConfigMap(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "environment", Namespace: scope.Namespace}, Data: map[string]string{"env": "ENV_SENTINEL"}})
	app.GetVirtualMachine().Spec.Template.Spec.Domain.Devices.Disks = []kv.Disk{{Name: "cloud"}, {Name: "env"}}
	ctrl := gomock.NewController(t)
	client := kubecli.NewMockKubevirtClient(ctrl)
	vms := kubecli.NewMockVirtualMachineInterface(ctrl)
	client.EXPECT().VirtualMachine(scope.Namespace).Return(vms)
	var created *kv.VirtualMachine
	vms.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, vm *kv.VirtualMachine, _ metav1.CreateOptions) (*kv.VirtualMachine, error) {
		created = vm.DeepCopy()
		created.UID = "created-vm-uid"
		return created, nil
	})
	controller := &startController{manager: &Manager{client: core, kubevirtCli: client, store: &vmFailureTestStore{}}, ctx: context.Background()}
	if err := controller.startOne(app); err != nil {
		t.Fatal(err)
	}
	store := vmrestore.NewStore(database)
	policy, err := store.ReadVMStartup(context.Background(), core, scope, created)
	if err != nil || policy.FirmwareUUID == "" || len(policy.GeneratedMedia) != 1 || policy.GeneratedMedia[0].Source.UID != "environment-uid" {
		t.Fatal("successful worker publication lacks exact private source proof", err)
	}
	if err := controller.startOne(app); !errors.Is(err, vmrestore.ErrConflict) {
		t.Fatal("old queued start bypassed startup ownership", err)
	}
	if err := controller.errorCallback(app); !errors.Is(err, vmrestore.ErrRestoreUnknown) {
		t.Fatal("unknown new VM publication attempted destructive cleanup", err)
	}
}
