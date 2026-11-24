package handler

// capability_id: rainbond.vm-snapshot.http-bounded-contract

import (
	"context"
	"testing"

	"errors"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/fake"
	kubecli "kubevirt.io/client-go/kubecli"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	kubevirtv1 "kubevirt.io/api/core/v1"
	snapshotv1 "kubevirt.io/api/snapshot/v1beta1"
)

type snapshotClientStub struct {
	created *snapshotv1.VirtualMachineSnapshot
}

func (s *snapshotClientStub) Create(_ context.Context, snapshot *snapshotv1.VirtualMachineSnapshot, _ metav1.CreateOptions) (*snapshotv1.VirtualMachineSnapshot, error) {
	s.created = snapshot
	return snapshot, nil
}

func (s *snapshotClientStub) Update(_ context.Context, snapshot *snapshotv1.VirtualMachineSnapshot, _ metav1.UpdateOptions) (*snapshotv1.VirtualMachineSnapshot, error) {
	return snapshot, nil
}

func (s *snapshotClientStub) UpdateStatus(_ context.Context, snapshot *snapshotv1.VirtualMachineSnapshot, _ metav1.UpdateOptions) (*snapshotv1.VirtualMachineSnapshot, error) {
	return snapshot, nil
}

func (s *snapshotClientStub) Delete(_ context.Context, _ string, _ metav1.DeleteOptions) error {
	return nil
}
func (s *snapshotClientStub) DeleteCollection(_ context.Context, _ metav1.DeleteOptions, _ metav1.ListOptions) error {
	return nil
}
func (s *snapshotClientStub) Get(_ context.Context, _ string, _ metav1.GetOptions) (*snapshotv1.VirtualMachineSnapshot, error) {
	return nil, nil
}
func (s *snapshotClientStub) List(_ context.Context, _ metav1.ListOptions) (*snapshotv1.VirtualMachineSnapshotList, error) {
	return &snapshotv1.VirtualMachineSnapshotList{}, nil
}
func (s *snapshotClientStub) Watch(_ context.Context, _ metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}
func (s *snapshotClientStub) Patch(_ context.Context, _ string, _ types.PatchType, _ []byte, _ metav1.PatchOptions, _ ...string) (*snapshotv1.VirtualMachineSnapshot, error) {
	return nil, nil
}

func TestBuildVMSnapshot(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
	}

	snapshot := buildVMSnapshot(vm, "snap-1", "demo snapshot", "service-1")

	if snapshot.Name != "snap-1" {
		t.Fatalf("expected snapshot name snap-1, got %s", snapshot.Name)
	}
	if snapshot.Namespace != "demo-ns" {
		t.Fatalf("expected namespace demo-ns, got %s", snapshot.Namespace)
	}
	if snapshot.Spec.Source.Kind != "VirtualMachine" || snapshot.Spec.Source.Name != "demo-vm" {
		t.Fatalf("unexpected snapshot source %#v", snapshot.Spec.Source)
	}
	if snapshot.Labels["service_id"] != "service-1" {
		t.Fatalf("expected service label, got %#v", snapshot.Labels)
	}
}

type snapshotFacadeManager struct {
	db.Manager
	database *gorm.DB
}

func (m snapshotFacadeManager) DB() *gorm.DB { return m.database }
func (m snapshotFacadeManager) TenantDao() dbdao.TenantDao {
	return &mysqldao.TenantDaoImpl{DB: m.database}
}
func (m snapshotFacadeManager) TenantServiceDao() dbdao.TenantServiceDao {
	return &mysqldao.TenantServicesDaoImpl{DB: m.database}
}

type snapshotFacadeBackend struct {
	calls   []string
	scope   vmrestore.Scope
	fail    error
	stopped bool
}

func (b *snapshotFacadeBackend) Context(ctx context.Context, scope vmrestore.Scope) (vmrestore.SnapshotContext, error) {
	b.scope = scope
	return vmrestore.SnapshotContext{SourceVM: vmrestore.ObjectRef{Kind: "VirtualMachine", Namespace: scope.Namespace, Name: "vm", UID: "vm-uid"}, WasRunning: !b.stopped, Stopped: b.stopped, Config: vmrestore.SnapshotConfig{CPUMilli: 1000, MemoryMiB: 1024, BootMode: "bios", DiskLayout: []vmrestore.SnapshotLayout{{VolumeID: 1, DiskKey: "root", CanonicalName: "manual1", Role: "root", Device: "disk", Bus: "virtio"}}}, Disks: []vmrestore.SnapshotDisk{{VolumeID: 1, DiskKey: "root", CanonicalName: "manual1", Role: "root", SourcePVC: vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "pvc", UID: "pvc-uid"}, StorageClass: "cephfs-vm-snapshot", RequestedBytes: 1 << 30}}}, b.fail
}
func (b *snapshotFacadeBackend) Exists(context.Context, vmrestore.Scope, string) (bool, error) {
	return false, b.fail
}
func (b *snapshotFacadeBackend) Observe(context.Context, vmrestore.Scope, vmrestore.SnapshotIntent, []vmrestore.ResourceRef) (vmrestore.SnapshotObservation, error) {
	return vmrestore.SnapshotObservation{SourceStopped: b.stopped, SourceRunning: !b.stopped, SourceLockClear: true, CleanupComplete: true}, b.fail
}
func (b *snapshotFacadeBackend) Halt(context.Context, vmrestore.Scope, vmrestore.SnapshotIntent) error {
	b.calls = append(b.calls, "halt")
	return b.fail
}
func (b *snapshotFacadeBackend) Create(context.Context, vmrestore.Scope, vmrestore.SnapshotIntent, string, string) (vmrestore.ObjectRef, error) {
	b.calls = append(b.calls, "create")
	return vmrestore.ObjectRef{Kind: "VirtualMachineSnapshot", Namespace: b.scope.Namespace, Name: "snap-1", UID: "snapshot-uid"}, b.fail
}
func (b *snapshotFacadeBackend) Resume(context.Context, vmrestore.Scope, vmrestore.SnapshotIntent) error {
	b.calls = append(b.calls, "resume")
	return b.fail
}
func (b *snapshotFacadeBackend) Delete(context.Context, vmrestore.Scope, []vmrestore.ResourceRef) error {
	b.calls = append(b.calls, "delete")
	return b.fail
}
func (b *snapshotFacadeBackend) DeleteReferences(context.Context, vmrestore.Scope, []vmrestore.ResourceRef) error {
	return b.fail
}
func snapshotFacadeFixture(t *testing.T) (*ServiceAction, *snapshotFacadeBackend, *gorm.DB, vmrestore.Scope) {
	t.Helper()
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "snapshot.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.LogMode(false)
	if err := database.AutoMigrate(&dbmodel.Tenants{}, &dbmodel.TenantServices{}, &dbmodel.ComponentK8sAttributes{}, &dbmodel.ServiceEvent{}).Error; err != nil {
		t.Fatal(err)
	}
	scope := vmrestore.Scope{TenantID: "tenant", ServiceID: "service-1", Namespace: "team"}
	if err := database.Create(&dbmodel.Tenants{UUID: scope.TenantID, Namespace: scope.Namespace}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&dbmodel.TenantServices{TenantID: scope.TenantID, ServiceID: scope.ServiceID, ExtendMethod: string(dbmodel.ServiceTypeVM)}).Error; err != nil {
		t.Fatal(err)
	}
	backend := &snapshotFacadeBackend{}
	engine := vmrestore.NewSnapshotEngine(vmrestore.NewStore(database), backend)
	action := &ServiceAction{dbmanager: snapshotFacadeManager{database: database}, snapshotEngineFactory: func(id string) (*vmrestore.SnapshotEngine, vmrestore.Scope, error) {
		if id != scope.ServiceID {
			return nil, vmrestore.Scope{}, vmrestore.ErrNotFound
		}
		return engine, scope, nil
	}}
	return action, backend, database, scope
}
func TestCreateVMSnapshot(t *testing.T) {
	action, backend, database, scope := snapshotFacadeFixture(t)
	status, err := action.CreateVMSnapshot(scope.ServiceID, &VMSnapshotRequest{Name: "snap-1", Description: "demo snapshot", RequestID: "op", ManifestID: "manifest"})
	if err != nil || status == nil || status.SnapshotName != "snap-1" || status.Finalized || status.ReadyToUse || len(backend.calls) != 0 {
		t.Fatalf("legacy name must reserve bounded operation: %v", err)
	}
	var event dbmodel.ServiceEvent
	if err := database.First(&event).Error; err != nil || event.Status != "checking" || event.FinalStatus != "" {
		t.Fatal("acceptance prematurely completed audit")
	}
	before := event
	got, err := action.GetVMSnapshot(context.Background(), scope.ServiceID, "snap-1")
	if err != nil || got.OperationID != status.OperationID || got.ManifestACKed {
		t.Fatal("read contract changed")
	}
	list, err := action.ListVMSnapshots(context.Background(), scope.ServiceID, 1, 20)
	if err != nil || list.Total != 1 || len(list.Items) != 1 {
		t.Fatal("managed catalog lost prepared snapshot")
	}
	if len(backend.calls) != 0 {
		t.Fatal("GET mutated backend")
	}
	database.First(&event)
	if event != before {
		t.Fatal("GET mutated audit")
	}
}
func TestVMSnapshotFacadeDispatchAndIdentity(t *testing.T) {
	action, backend, _, scope := snapshotFacadeFixture(t)
	status, err := action.PrepareVMSnapshot(context.Background(), scope.ServiceID, &VMSnapshotRequest{Name: "snap-1", RequestID: "op", ManifestID: "manifest"})
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range []vmrestore.SnapshotActionRequest{{OperationID: "foreign"}, {CoreHash: strings.Repeat("b", 64)}} {
		if _, err := action.VMSnapshotAction(context.Background(), scope.ServiceID, "snap-1", "halt", req); !errors.Is(err, vmrestore.ErrConflict) {
			t.Fatal("foreign identity accepted")
		}
	}
	if _, err := action.VMSnapshotAction(context.Background(), scope.ServiceID, "snap-1", "patch", vmrestore.SnapshotActionRequest{}); !errors.Is(err, vmrestore.ErrSnapshotInvalid) {
		t.Fatal("unknown action accepted")
	}
	request := vmrestore.SnapshotActionRequest{RequestID: "halt", ManifestID: "manifest"}
	halted, err := action.VMSnapshotAction(context.Background(), scope.ServiceID, "snap-1", "halt", request)
	if err != nil || halted.OperationID != status.OperationID || len(backend.calls) != 1 || backend.calls[0] != "halt" {
		t.Fatalf("halt dispatch failed: %v", err)
	}
	backend.stopped = true
	request.RequestID = "create"
	_, err = action.VMSnapshotAction(context.Background(), scope.ServiceID, "snap-1", "create", request)
	if err != nil || len(backend.calls) != 2 || backend.calls[1] != "create" {
		t.Fatalf("create dispatch failed %v", err)
	}
	for _, name := range []string{"resume", "finalize"} {
		request.RequestID = name
		if _, err := action.VMSnapshotAction(context.Background(), scope.ServiceID, "snap-1", name, request); !errors.Is(err, vmrestore.ErrConflict) {
			t.Fatalf("%s accepted missing durable manifest ACK: %v", name, err)
		}
	}
	request.RequestID = "cancel"
	cancelled, err := action.VMSnapshotAction(context.Background(), scope.ServiceID, "snap-1", "cancel", request)
	if err != nil || !cancelled.Cancelled || cancelled.Finalized || cancelled.ManifestACKed {
		t.Fatalf("cancel must retain unACKed completion state: %v", err)
	}
	request.RequestID = "delete"
	if _, err := action.VMSnapshotAction(context.Background(), scope.ServiceID, "snap-1", "delete", request); !errors.Is(err, vmrestore.ErrConflict) {
		t.Fatalf("delete accepted missing snapshot UID: %v", err)
	}
	if len(backend.calls) != 2 {
		t.Fatal("invalid ACK triggered an extra native mutation")
	}

	backend.fail = vmrestore.ErrReferenceUnavailable
	if _, err := action.GetVMSnapshot(context.Background(), scope.ServiceID, "snap-1"); !errors.Is(err, vmrestore.ErrReferenceUnavailable) {
		t.Fatal("dependency error lost")
	}
	if _, err := action.PrepareVMSnapshot(context.Background(), scope.ServiceID, nil); !errors.Is(err, vmrestore.ErrSnapshotInvalid) {
		t.Fatal("nil request accepted")
	}
}
func TestVMSnapshotFacadeDerivesScopeAndRejectsForeignTenant(t *testing.T) {
	action, _, database, scope := snapshotFacadeFixture(t)
	action.snapshotEngineFactory = nil
	if _, _, err := action.snapshotService(context.Background(), scope.ServiceID); !errors.Is(err, vmrestore.ErrReferenceUnavailable) {
		t.Fatal("missing clients accepted")
	}
	ctx := context.WithValue(context.Background(), ctxutil.ContextKey("tenant_id"), "foreign")
	if _, _, err := action.snapshotService(ctx, scope.ServiceID); !errors.Is(err, vmrestore.ErrInternalAttribute) {
		t.Fatal("foreign trusted tenant accepted")
	}
	if _, _, err := action.snapshotService(context.Background(), "missing"); !errors.Is(err, vmrestore.ErrNotFound) {
		t.Fatal("missing service not mapped")
	}
	ctrl := gomock.NewController(t)
	client := kubecli.NewMockKubevirtClient(ctrl)
	client.EXPECT().DynamicClient().Return(dynamic.Interface(nil))
	action.kubevirtClient = client
	action.kubeClient = fake.NewSimpleClientset()
	engine, got, err := action.snapshotService(context.Background(), scope.ServiceID)
	if err != nil || engine == nil || got != scope {
		t.Fatal("server scope not derived from database")
	}
	database.Model(&dbmodel.Tenants{}).Where("uuid = ?", scope.TenantID).Update("namespace", "")
	if _, _, err := action.snapshotService(context.Background(), scope.ServiceID); !errors.Is(err, vmrestore.ErrCorrupt) {
		t.Fatal("missing namespace accepted")
	}
}
