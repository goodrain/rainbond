package controller

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	appmtypes "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kv "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	runtimefake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type vmPreservationStore struct{ closedAppStore }

func (vmPreservationStore) OnDeletes(...interface{}) {}

// capability_id: rainbond.vm-power.legacy-stop-preserves-disks
func TestLegacyWorkerStopCannotDeleteUnmappedVM(t *testing.T) {
	for _, tc := range []struct {
		name              string
		databaseAvailable bool
		renderedVM        bool
		typedVM           bool
	}{
		{name: "rendered VM without database", renderedVM: true},
		{name: "VM type before rendering", typedVM: true},
		{name: "no disk bindings", databaseAvailable: true, renderedVM: true, typedVM: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			previous := db.GetManager()
			db.SetTestManager(nil)
			defer db.SetTestManager(previous)
			if tc.databaseAvailable {
				database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "empty-bindings.db"))
				if err != nil {
					t.Fatal(err)
				}
				defer database.Close()
				database.LogMode(false)
				if err := database.AutoMigrate(&dbmodel.Tenants{}, &dbmodel.TenantServices{}, &dbmodel.ComponentK8sAttributes{}).Error; err != nil {
					t.Fatal(err)
				}
				if err := database.Create(&dbmodel.Tenants{UUID: "tenant", Namespace: "team"}).Error; err != nil {
					t.Fatal(err)
				}
				if err := database.Create(&dbmodel.TenantServices{TenantID: "tenant", ServiceID: "windows", ExtendMethod: "vm"}).Error; err != nil {
					t.Fatal(err)
				}
				db.SetTestManager(restoreGuardDB{database: database})
			}
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "keep-config", Namespace: "team"}}
			client := kubefake.NewSimpleClientset(secret,
				&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: "team", UID: "root-uid"}},
				&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "team", UID: "data-uid"}},
			)
			virtClient := kubecli.NewMockKubevirtClient(ctrl)
			vmClient := kubecli.NewMockVirtualMachineInterface(ctrl)
			deleted := false
			virtClient.EXPECT().VirtualMachine("team").Return(vmClient).AnyTimes()
			vmClient.EXPECT().Delete(gomock.Any(), "windows-vm", gomock.Any()).DoAndReturn(func(context.Context, string, metav1.DeleteOptions) error {
				deleted = true
				return nil
			}).AnyTimes()
			logger := event.NewMockLogger(ctrl)
			logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			app := appmtypes.AppService{AppServiceBase: appmtypes.AppServiceBase{TenantID: "tenant", ServiceID: "windows"}, Logger: logger}
			if tc.typedVM {
				app.ServiceType = appmtypes.TypeVirtualMachine
			}
			app.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team"}})
			app.SetSecret(secret)
			vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "windows-vm", Namespace: "team", UID: "original-vm"}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{}}}
			if tc.renderedVM {
				app.SetVirtualMachine(vm)
			}
			scheme := runtime.NewScheme()
			if err := kv.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			runtimeClient := runtimefake.NewClientBuilder().WithScheme(scheme).WithObjects(vm).Build()
			worker := &stopController{ctx: context.Background(), manager: &Manager{client: client, runtimeClient: runtimeClient, kubevirtCli: virtClient, store: vmPreservationStore{}}}
			err := worker.stopOne(app)
			if deleted {
				t.Fatal("legacy stop deleted the VM and would garbage-collect owned disks")
			}
			if err == nil {
				t.Fatal("legacy VM stop must fail closed and require the VM power API")
			}
			if len(client.Actions()) != 0 {
				t.Fatalf("legacy VM stop modified resources before rejecting: %#v", client.Actions())
			}
		})
	}
}
