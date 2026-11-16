package handler

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	typedcore "k8s.io/client-go/kubernetes/typed/core/v1"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	k8stesting "k8s.io/client-go/testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	corefake "k8s.io/client-go/kubernetes/fake"
	kv "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
)

type bindingConsumerDB struct {
	db.Manager
	database *gorm.DB
}

func (m bindingConsumerDB) DB() *gorm.DB    { return m.database }
func (m bindingConsumerDB) Begin() *gorm.DB { return m.database.Begin() }
func (m bindingConsumerDB) TenantServiceDao() dbdao.TenantServiceDao {
	return bindingConsumerServices{database: m.database}
}
func (m bindingConsumerDB) TenantServiceVolumeDao() dbdao.TenantServiceVolumeDao {
	return bindingConsumerVolumes{database: m.database}
}
func (m bindingConsumerDB) TenantServiceVolumeDaoTransactions(tx *gorm.DB) dbdao.TenantServiceVolumeDao {
	return bindingConsumerVolumes{database: tx}
}
func (m bindingConsumerDB) TenantServiceMountRelationDao() dbdao.TenantServiceMountRelationDao {
	return bindingConsumerMounts{database: m.database}
}
func (m bindingConsumerDB) TenantServiceMountRelationDaoTransactions(tx *gorm.DB) dbdao.TenantServiceMountRelationDao {
	return bindingConsumerMounts{database: tx}
}
func (m bindingConsumerDB) TenantServiceConfigFileDaoTransactions(tx *gorm.DB) dbdao.TenantServiceConfigFileDao {
	return bindingConsumerConfig{database: tx}
}

type bindingConsumerServices struct {
	dbdao.TenantServiceDao
	database *gorm.DB
}

func (d bindingConsumerServices) GetServiceByID(id string) (*dbmodel.TenantServices, error) {
	var row dbmodel.TenantServices
	err := d.database.Where("service_id = ?", id).First(&row).Error
	return &row, err
}

type bindingConsumerVolumes struct {
	dbdao.TenantServiceVolumeDao
	database *gorm.DB
}

func (d bindingConsumerVolumes) GetVolumeByServiceIDAndName(id, name string) (*dbmodel.TenantServiceVolume, error) {
	var row dbmodel.TenantServiceVolume
	err := d.database.Where("service_id = ? AND volume_name = ?", id, name).First(&row).Error
	return &row, err
}
func (d bindingConsumerVolumes) DeleteModel(id string, args ...interface{}) error {
	return d.database.Where("service_id = ? AND volume_name = ?", id, args[0]).Delete(&dbmodel.TenantServiceVolume{}).Error
}
func (d bindingConsumerVolumes) UpdateModel(v dbmodel.Interface) error {
	return d.database.Save(v).Error
}

type bindingConsumerMounts struct {
	dbdao.TenantServiceMountRelationDao
	database *gorm.DB
}

func (d bindingConsumerMounts) GetTenantServiceMountRelationsByDepServiceAndVolumeName(id, name string) ([]*dbmodel.TenantServiceMountRelation, error) {
	var rows []*dbmodel.TenantServiceMountRelation
	err := d.database.Where("dep_service_id = ? AND volume_name = ?", id, name).Find(&rows).Error
	return rows, err
}

type bindingConsumerConfig struct {
	dbdao.TenantServiceConfigFileDao
	database *gorm.DB
}

func (d bindingConsumerConfig) DelByVolumeID(id, name string) error {
	return d.database.Where("service_id = ? AND volume_name = ?", id, name).Delete(&dbmodel.TenantServiceConfigFile{}).Error
}

type bindingConsumerFixture struct {
	action   *ServiceAction
	database *gorm.DB
	store    *vmrestore.Store
	resolver *vmrestore.Resolver
	scope    vmrestore.Scope
	service  *dbmodel.TenantServices
	volume   *dbmodel.TenantServiceVolume
	core     *corefake.Clientset
	dynamic  *dynamicfake.FakeDynamicClient
	mq       *recordingMQClient
}

func newBindingConsumerFixture(t *testing.T) bindingConsumerFixture {
	t.Helper()
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "consumer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.LogMode(false)
	if err := database.AutoMigrate(&dbmodel.Tenants{}, &dbmodel.TenantServices{}, &dbmodel.TenantServiceVolume{}, &dbmodel.TenantServiceConfigFile{}, &dbmodel.TenantServiceMountRelation{}, &dbmodel.ComponentK8sAttributes{}).Error; err != nil {
		t.Fatal(err)
	}
	scope := vmrestore.Scope{TenantID: "tenant", ServiceID: "service-vm", Namespace: "team"}
	tenant := &dbmodel.Tenants{UUID: scope.TenantID, Name: scope.Namespace, Namespace: scope.Namespace}
	service := &dbmodel.TenantServices{TenantID: scope.TenantID, ServiceID: scope.ServiceID, Namespace: scope.Namespace, ExtendMethod: "vm"}
	volume := &dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 42}, ServiceID: scope.ServiceID, VolumeName: "data-1", VolumePath: "/disk-1", VolumeType: "fast", VolumeCapacity: 1}
	for _, row := range []interface{}{tenant, service, volume} {
		if err := database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	pvc := func(name, uid string) *corev1.PersistentVolumeClaim {
		return &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: scope.Namespace, UID: types.UID(uid), ResourceVersion: "1", Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: stringPtrForBinding("fast"), Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound, Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}
	}
	core := corefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: scope.Namespace, UID: "ns-uid"}}, pvc("manual42", "b-uid"), pvc("active-a", "a-uid"), expansionStorageClassWithProvisioner("fast", true, "cephfs.csi.ceph.com"))
	kinds := map[schema.GroupVersionResource]string{dataVolumeGVR: "DataVolumeList", vmDiskVMGVR: "VirtualMachineList", vmDiskVMIGVR: "VirtualMachineInstanceList", {Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "datasources"}: "DataSourceList", {Group: "snapshot.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinesnapshots"}: "VirtualMachineSnapshotList", {Group: "snapshot.kubevirt.io", Version: "v1beta1", Resource: "virtualmachinesnapshotcontents"}: "VirtualMachineSnapshotContentList", {Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}: "VolumeSnapshotList", {Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotcontents"}: "VolumeSnapshotContentList"}
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), kinds)
	old := vmHotplugDynamicClient
	vmHotplugDynamicClient = func() dynamic.Interface { return dc }
	t.Cleanup(func() { vmHotplugDynamicClient = old })
	ctrl := gomock.NewController(t)
	client := kubecli.NewMockKubevirtClient(ctrl)
	config := kubecli.NewMockKubeVirtInterface(ctrl)
	client.EXPECT().KubeVirt("").Return(config).AnyTimes()
	config.EXPECT().List(gomock.Any(), gomock.Any()).Return(&kv.KubeVirtList{Items: []kv.KubeVirt{{Spec: kv.KubeVirtSpec{Configuration: kv.KubeVirtConfiguration{DeveloperConfiguration: &kv.DeveloperConfiguration{FeatureGates: []string{"ExpandDisks"}}}}, Status: kv.KubeVirtStatus{Phase: kv.KubeVirtPhaseDeployed, ObservedKubeVirtVersion: "v1.8.1"}}}}, nil).AnyTimes()
	mq := &recordingMQClient{}
	action := &ServiceAction{dbmanager: bindingConsumerDB{database: database}, kubeClient: core, kubevirtClient: client, MQClient: mq, resolveTenantNamespaceHook: func(string) (string, error) { return scope.Namespace, nil }, getVirtualMachineByServiceIDHook: func(string) (*kv.VirtualMachine, error) { return nil, nil }}
	store := vmrestore.NewStore(database)
	_, err = store.Mutate(scope, vmrestore.Expectation{}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		c.State.Bindings["42"] = vmrestore.Binding{VolumeID: 42, DiskKey: "data-1", CanonicalName: "manual42", Role: "data", Attached: true, ActivePVC: vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "active-a", UID: "a-uid"}}
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return bindingConsumerFixture{action, database, store, vmrestore.NewResolver(store, core, dc), scope, service, volume, core, dc, mq}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingDeleteWithoutVMNeverReclaimsB(t *testing.T) {
	f := newBindingConsumerFixture(t)
	if err := f.action.VolumnVar(&dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}, f.scope.TenantID, "", "delete"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "manual42", metav1.GetOptions{}); err != nil {
		t.Fatal("B was reclaimed")
	}
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{}); err == nil {
		t.Fatal("current A remains after successful delete")
	}
	var count int
	f.database.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 42).Count(&count)
	if count != 0 {
		t.Fatal("logical row remains")
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := current.State.Bindings["42"]; ok || current.State.Admission != nil {
		t.Fatal("binding/admission not retired")
	}
	if len(f.mq.tasks) != 0 {
		t.Fatal("VM deletion queued broad GC")
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingExpansionWithoutVMOnlyUpdatesA(t *testing.T) {
	f := newBindingConsumerFixture(t)
	if err := f.action.expandVolumeClaims(context.Background(), f.service, f.volume, 2); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"manual42": "1Gi", "active-a": "2Gi"} {
		pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		q := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		if q.String() != want {
			t.Fatalf("%s request=%s want=%s", name, q.String(), want)
		}
	}
}

func stringPtrForBinding(v string) *string { return &v }

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingDeleteResumesAfterSQLFailureAndLostACK(t *testing.T) {
	f := newBindingConsumerFixture(t)
	fail := true
	f.database.Callback().Delete().Before("gorm:delete").Register("consumer-fail-volume-delete", func(scope *gorm.Scope) {
		if scope.TableName() == "tenant_services_volume" && fail {
			fail = false
			scope.Err(fmt.Errorf("fixture SQL commit path failure"))
		}
	})
	request := &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}
	if err := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); err == nil {
		t.Fatal("expected SQL failure")
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil || current.State.Admission == nil {
		t.Fatal("retry admission lost")
	}
	if _, err := f.resolver.ResolveVolume(context.Background(), f.scope, 42); !errors.Is(err, vmrestore.ErrConflict) {
		t.Fatal("missing A must not resolve B")
	}
	if err := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); err != nil {
		t.Fatal(err)
	}
	if err := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); err != nil {
		t.Fatal("completed receipt did not replay lost ACK")
	}
	if len(f.mq.tasks) != 0 {
		t.Fatal("VM queued broad GC")
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingDeletePendingFinalizerKeepsIdentityAndPermit(t *testing.T) {
	f := newBindingConsumerFixture(t)
	hold := true
	f.core.PrependReactor("delete", "persistentvolumeclaims", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if hold {
			return true, nil, nil
		}
		return false, nil, nil
	})
	request := &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}
	first := f.action.VolumnVar(request, f.scope.TenantID, "", "delete")
	if first == nil || first.Code != 409 {
		t.Fatalf("expected pending409 got %v", first)
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil || current.State.Admission == nil || !current.State.Admission.OutcomeUnknown {
		t.Fatal("pending outcome was released")
	}
	status, _, err := f.action.inspectVolumeExpansion(context.Background(), f.service, f.volume)
	if err != nil || status.Status != volumeExpansionResizing || status.ActualCapacity != 1 {
		t.Fatalf("busy read hid A capacity: %+v %v", status, err)
	}
	hold = false
	if err := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingRollbackBlocksDeleteButNotIndependentExpansion(t *testing.T) {
	f := newBindingConsumerFixture(t)
	c, _ := f.store.Read(f.scope, "")
	_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		b := c.State.Bindings["42"]
		b.PreviousPVC = &vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "manual42", UID: "b-uid"}
		c.State.Bindings["42"] = b
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 42)
	if err != nil || refs.CanDelete || refs.CanDetach || !refs.CanExpand {
		t.Fatalf("unexpected shared policy: %+v %v", refs, err)
	}
	failure := f.action.VolumnVar(&dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}, f.scope.TenantID, "", "delete")
	if failure == nil || failure.Code != 409 {
		t.Fatalf("expected refs409 got %v", failure)
	}
	if err := f.action.expandVolumeClaims(context.Background(), f.service, f.volume, 2); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingAdmissionFailureReturns409BeforeKubernetesWrites(t *testing.T) {
	f := newBindingConsumerFixture(t)
	c, _ := f.store.Read(f.scope, "")
	_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		if err := c.State.AdmitMutation("other-operation", "other"); err != nil {
			return vmrestore.Mutation{}, err
		}
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f.core.ClearActions()
	f.dynamic.ClearActions()
	err = f.action.UpdVolume(f.scope.ServiceID, &apimodel.UpdVolumeReq{VolumeName: f.volume.VolumeName, VolumePath: f.volume.VolumePath, VolumeCapacity: int64PtrForBinding(2), VolumeType: f.volume.VolumeType})
	code, ok := err.(bcode.Coder)
	if !ok || code.GetStatus() != 409 {
		t.Fatalf("expectedHTTP409 coder got %v", err)
	}
	for _, actions := range [][]k8stesting.Action{f.core.Actions(), f.dynamic.Actions()} {
		for _, a := range actions {
			if a.GetVerb() == "update" || a.GetVerb() == "delete" {
				t.Fatal("busy operation mutated Kubernetes")
			}
		}
	}
}
func int64PtrForBinding(v int64) *int64 { return &v }

type bindingBlockingCore struct {
	kubernetes.Interface
	entered chan struct{}
	release chan struct{}
}

func (c bindingBlockingCore) CoreV1() typedcore.CoreV1Interface {
	return bindingBlockingCoreV1{CoreV1Interface: c.Interface.CoreV1(), entered: c.entered, release: c.release}
}

type bindingBlockingCoreV1 struct {
	typedcore.CoreV1Interface
	entered chan struct{}
	release chan struct{}
}

func (c bindingBlockingCoreV1) PersistentVolumeClaims(ns string) typedcore.PersistentVolumeClaimInterface {
	return bindingBlockingPVC{PersistentVolumeClaimInterface: c.CoreV1Interface.PersistentVolumeClaims(ns), entered: c.entered, release: c.release}
}

type bindingBlockingPVC struct {
	typedcore.PersistentVolumeClaimInterface
	entered chan struct{}
	release chan struct{}
}

func (c bindingBlockingPVC) Update(ctx context.Context, pvc *corev1.PersistentVolumeClaim, opts metav1.UpdateOptions) (*corev1.PersistentVolumeClaim, error) {
	close(c.entered)
	<-c.release
	return c.PersistentVolumeClaimInterface.Update(ctx, pvc, opts)
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingConcurrentRetryCannotReleaseIssuedMutation(t *testing.T) {
	f := newBindingConsumerFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	f.action.kubeClient = bindingBlockingCore{Interface: f.core, entered: entered, release: release}
	result := make(chan error, 1)
	go func() { result <- f.action.expandVolumeClaims(context.Background(), f.service, f.volume, 2) }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("first operation did not reach external write")
	}
	_, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(context.Background(), &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foreign-consumer", Namespace: f.scope.Namespace, UID: "foreign-pod"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "foreign-data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "active-a"}}}}}}, metav1.CreateOptions{})
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	second := f.action.UpdVolume(f.scope.ServiceID, &apimodel.UpdVolumeReq{VolumeName: f.volume.VolumeName, VolumePath: f.volume.VolumePath, VolumeType: f.volume.VolumeType, VolumeCapacity: int64PtrForBinding(2)})
	if code, ok := second.(bcode.Coder); !ok || code.GetStatus() != 409 {
		t.Errorf("second operation must reject with409: %v", second)
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil || current.State.Admission == nil {
		t.Error("second preflight released the first in-flight admission")
	}
	close(release)
	if err := <-result; err != nil {
		t.Errorf("first accepted operation could not complete: %v", err)
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingDeleteACKCannotMaskNewLogicalBinding(t *testing.T) {
	f := newBindingConsumerFixture(t)
	request := &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}
	if err := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); err != nil {
		t.Fatal(err)
	}
	current, _ := f.store.Read(f.scope, "")
	_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		c.State.Bindings["99"] = vmrestore.Binding{VolumeID: 99, DiskKey: f.volume.VolumeName, CanonicalName: "manual99", Role: "data", Attached: true, ActivePVC: vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "missing-new-a", UID: "new-a-uid"}}
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	rejection := f.action.VolumnVar(request, f.scope.TenantID, "", "delete")
	if rejection == nil || rejection.Code != 409 {
		t.Fatalf("historical receipt acknowledged a new missing binding: %v", rejection)
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingConfigDeletionGuardAndLegacyNonPVCBehavior(t *testing.T) {
	for _, mapped := range []bool{true, false} {
		t.Run(fmt.Sprintf("mapped=%t", mapped), func(t *testing.T) {
			f := newBindingConsumerFixture(t)
			config := &dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 99}, ServiceID: f.scope.ServiceID, VolumeName: "config", VolumeType: dbmodel.ConfigFileVolumeType.String(), VolumePath: "/etc/config"}
			if err := f.database.Create(config).Error; err != nil {
				t.Fatal(err)
			}
			if !mapped {
				current, _ := f.store.Read(f.scope, "")
				_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
					c.State.Bindings = map[string]vmrestore.Binding{}
					c.State.BindingGeneration++
					return vmrestore.Mutation{State: c.State}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			synced := false
			f.action.syncVirtualMachineSpecHook = func(string) error {
				synced = true
				current, err := f.store.Read(f.scope, "")
				if err != nil || current.State.Admission == nil {
					t.Fatal("legacy config sync ran without shared admission")
				}
				return nil
			}
			rejection := f.action.VolumnVar(&dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: "config"}, f.scope.TenantID, "", "delete")
			if mapped {
				if rejection == nil || rejection.Code != 409 || synced {
					t.Fatalf("mapped config must preserve409: %v", rejection)
				}
			} else {
				if rejection != nil || !synced {
					t.Fatalf("unmapped config legacy path failed: %v", rejection)
				}
				if len(f.mq.tasks) != 0 {
					t.Fatal("VM config delete queued broad GC")
				}
			}
		})
	}
}

func (d bindingConsumerVolumes) GetTenantServiceVolumesByServiceID(id string) ([]*dbmodel.TenantServiceVolume, error) {
	var rows []*dbmodel.TenantServiceVolume
	err := d.database.Where("service_id = ?", id).Find(&rows).Error
	return rows, err
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingReadUsesAWhileBusyAndRejectsMissingTarget(t *testing.T) {
	f := newBindingConsumerFixture(t)
	pvc, _ := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{})
	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("3Gi")
	pvc.Status.Capacity[corev1.ResourceStorage] = resource.MustParse("3Gi")
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(context.Background(), pvc, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	current, _ := f.store.Read(f.scope, "")
	_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		if err := c.State.AdmitMutation("fixture-resize", "disk-resize-fixture"); err != nil {
			return vmrestore.Mutation{}, err
		}
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	list, apiErr := f.action.GetVolumes(f.scope.ServiceID)
	if apiErr != nil || len(list) != 1 {
		t.Fatalf("read failed: %v", apiErr)
	}
	if list[0].ActualCapacity != 3 || list[0].RequestedCapacity != 3 || list[0].ExpansionStatus != volumeExpansionResizing {
		t.Fatalf("busy read hid A: %+v", list[0])
	}
	if err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Delete(context.Background(), "active-a", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	list, apiErr = f.action.GetVolumes(f.scope.ServiceID)
	if apiErr == nil || apiErr.Code != 409 || len(list) != 0 {
		t.Fatalf("missing A read returned stale data: %v %+v", apiErr, list)
	}
}

func (d bindingConsumerMounts) AddModel(row dbmodel.Interface) error {
	return d.database.Create(row).Error
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingSharedSourceAdmissionAndReference(t *testing.T) {
	for _, held := range []bool{true, false} {
		t.Run(fmt.Sprintf("held=%t", held), func(t *testing.T) {
			f := newBindingConsumerFixture(t)
			previous := db.GetManager()
			db.SetTestManager(f.action.dbmanager)
			t.Cleanup(func() { db.SetTestManager(previous) })
			if held {
				c, _ := f.store.Read(f.scope, "")
				_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
					if err := c.State.AdmitMutation("fixture-delete", "disk-delete-fixture"); err != nil {
						return vmrestore.Mutation{}, err
					}
					return vmrestore.Mutation{State: c.State}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			f.core.ClearActions()
			f.dynamic.ClearActions()
			synced := false
			f.action.syncVirtualMachineSpecHook = func(string) error { synced = true; return nil }
			relation := &dbmodel.TenantServiceMountRelation{ServiceID: "consumer", DependServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}
			failure := f.action.VolumeDependency(relation, "add")
			var count int
			f.database.Model(&dbmodel.TenantServiceMountRelation{}).Count(&count)
			if held {
				if failure == nil || failure.Code != 409 || count != 0 || synced {
					t.Fatalf("source admission bypassed: %v rows=%d", failure, count)
				}
			} else {
				if failure != nil || count != 1 {
					t.Fatalf("shared add failed: %v", failure)
				}
				refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 42)
				if err != nil || refs.CanDelete || refs.CanExpand {
					t.Fatalf("actual shared ref lost: %+v %v", refs, err)
				}
			}
		})
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingSameCapacityMySQLZeroChangedRows(t *testing.T) {
	f := newBindingConsumerFixture(t)
	request := &apimodel.UpdVolumeReq{VolumeName: f.volume.VolumeName, VolumePath: f.volume.VolumePath, VolumeType: f.volume.VolumeType, VolumeCapacity: int64PtrForBinding(2)}
	if err := f.action.UpdVolume(f.scope.ServiceID, request); err != nil {
		t.Fatal(err)
	}
	// Real SQLite rows are retained; only the unchanged UPDATE result is adapted
	// to MySQL's default changed-rows semantics, rather than mocking the Store.
	f.database.Callback().Update().After("gorm:update").Register("mysql-zero-changed-rows", func(scope *gorm.Scope) {
		if scope.TableName() == "tenant_services_volume" {
			scope.DB().RowsAffected = 0
		}
	})
	if err := f.action.UpdVolume(f.scope.ServiceID, request); err != nil {
		t.Fatalf("same target was not idempotent: %v", err)
	}
	current, err := f.store.Read(f.scope, "")
	if err != nil || current.State.Admission != nil {
		t.Fatal("known unchanged target left admission pending")
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingLegacyConfigDeleteRetriesUnknownSync(t *testing.T) {
	f := newBindingConsumerFixture(t)
	c, _ := f.store.Read(f.scope, "")
	_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		c.State.Bindings = map[string]vmrestore.Binding{}
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.database.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 99}, ServiceID: f.scope.ServiceID, VolumeName: "config", VolumeType: dbmodel.ConfigFileVolumeType.String(), VolumePath: "/etc/config"}).Error; err != nil {
		t.Fatal(err)
	}
	fail := true
	f.action.syncVirtualMachineSpecHook = func(string) error {
		if fail {
			fail = false
			return fmt.Errorf("fixture uncertain config sync")
		}
		return nil
	}
	request := &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: "config"}
	if failure := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); failure == nil {
		t.Fatal("expected unknown sync")
	}
	if failure := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); failure != nil {
		t.Fatalf("config retry remained stuck after its row was removed: %v", failure)
	}
	c, err = f.store.Read(f.scope, "")
	if err != nil || c.State.Admission != nil {
		t.Fatal("config completion did not release its permit")
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingLegacyConfigConcurrentCallCannotReleasePermit(t *testing.T) {
	for _, scenario := range []string{"new", "retry-unknown"} {
		t.Run(scenario, func(t *testing.T) {
			f := newBindingConsumerFixture(t)
			current, _ := f.store.Read(f.scope, "")
			_, err := f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
				c.State.Bindings = map[string]vmrestore.Binding{}
				c.State.BindingGeneration++
				return vmrestore.Mutation{State: c.State}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "retry-unknown" {
				if failure := f.action.withLegacyVMConfig(f.service, "update-config:config", func() error { return fmt.Errorf("fixture unknown outcome") }); failure == nil {
					t.Fatal("expected an unknown initial config outcome")
				}
			}
			started, release := make(chan struct{}), make(chan struct{})
			finished := make(chan error, 1)
			go func() {
				finished <- f.action.withLegacyVMConfig(f.service, "update-config:config", func() error {
					close(started)
					<-release
					return nil
				})
			}()
			select {
			case <-started:
			case first := <-finished:
				t.Fatalf("first config operation failed before entering its callback: %v", first)
			case <-time.After(10 * time.Second):
				t.Fatal("first config operation did not enter its callback")
			}
			secondRan := false
			second := f.action.withLegacyVMConfig(f.service, "update-config:config", func() error {
				secondRan = true
				return nil
			})
			observed, readErr := f.store.Read(f.scope, "")
			close(release)
			first := <-finished
			if !errors.Is(second, vmrestore.ErrBusy) || secondRan {
				t.Errorf("concurrent config call entered an already running operation: err=%v ran=%t", second, secondRan)
			}
			if readErr != nil || observed.State.Admission == nil {
				t.Errorf("concurrent config call released the first operation permit: %v", readErr)
			}
			if first != nil {
				t.Errorf("first operation failed to finish: %v", first)
			}
		})
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingRetryRejectsGenerationChangedSincePrepare(t *testing.T) {
	f := newBindingConsumerFixture(t)
	consumer, err := f.action.vmVolumes(f.service)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := consumer.begin(context.Background(), f.volume, "delete", ""); err != nil {
		t.Fatal(err)
	}
	original, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	next := original.DeepCopy()
	next.Name, next.UID = "next-a", "next-a-uid"
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), next, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	current, _ := f.store.Read(f.scope, "")
	_, err = f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, AdmissionOwner: current.State.Admission.Token}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		binding := c.State.Bindings["42"]
		binding.ActivePVC.Name, binding.ActivePVC.UID = next.Name, string(next.UID)
		c.State.Bindings["42"] = binding
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f.core.ClearActions()
	failure := f.action.VolumnVar(&dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}, f.scope.TenantID, "", "delete")
	if failure == nil || failure.Code != 409 {
		t.Errorf("stale retry accepted a newer binding generation: %v", failure)
	}
	for _, name := range []string{"active-a", "next-a", "manual42"} {
		if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
			t.Errorf("generation conflict did not preserve %s: %v", name, err)
		}
	}
	for _, action := range f.core.Actions() {
		if action.GetVerb() == "delete" {
			t.Error("stale retry issued resource deletion")
		}
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingProductionVMDiscoveryAndDetach(t *testing.T) {
	for _, scenario := range []string{"absent", "stopped", "duplicate", "missing-uid", "missing-template", "list-failure"} {
		t.Run(scenario, func(t *testing.T) {
			f := newBindingConsumerFixture(t)
			f.action.getVirtualMachineByServiceIDHook = nil
			vm := &kv.VirtualMachine{TypeMeta: metav1.TypeMeta{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachine"},
				ObjectMeta: metav1.ObjectMeta{Name: "actual-vm", Namespace: f.scope.Namespace, UID: "vm-uid", ResourceVersion: "1", Labels: map[string]string{"service_id": f.scope.ServiceID}},
				Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{
					Volumes: []kv.Volume{{Name: "manual42", VolumeSource: kv.VolumeSource{PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "active-a"}}}}},
					Domain:  kv.DomainSpec{Devices: kv.Devices{Disks: []kv.Disk{{Name: "manual42", DiskDevice: kv.DiskDevice{Disk: &kv.DiskTarget{Bus: kv.DiskBusSCSI}}}}}},
				}}}}
			if scenario == "missing-uid" {
				vm.UID = ""
			}
			if scenario == "missing-template" {
				vm.Spec.Template = nil
			}
			if scenario != "absent" && scenario != "list-failure" {
				raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.dynamic.Resource(vmDiskVMGVR).Namespace(f.scope.Namespace).Create(context.Background(), &unstructured.Unstructured{Object: raw}, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
				if scenario == "duplicate" {
					other := &unstructured.Unstructured{Object: raw}
					other.SetName("duplicate-vm")
					other.SetUID("duplicate-vm-uid")
					if _, err := f.dynamic.Resource(vmDiskVMGVR).Namespace(f.scope.Namespace).Create(context.Background(), other, metav1.CreateOptions{}); err != nil {
						t.Fatal(err)
					}
				}
			}
			if scenario == "list-failure" {
				f.dynamic.PrependReactor("list", "virtualmachines", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("fixture VM read unavailable")
				})
			}
			f.core.ClearActions()
			failure := f.action.VolumnVar(&dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}, f.scope.TenantID, "", "delete")
			if scenario == "absent" || scenario == "stopped" {
				if failure != nil {
					t.Fatalf("actual VM discovery path rejected safe deletion: %v", failure)
				}
				if scenario == "stopped" {
					updated, err := f.dynamic.Resource(vmDiskVMGVR).Namespace(f.scope.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
					if err != nil {
						t.Fatal(err)
					}
					volumes, _, _ := unstructured.NestedSlice(updated.Object, "spec", "template", "spec", "volumes")
					if len(volumes) != 0 {
						t.Fatal("stopped VM still references the deleted data disk")
					}
				}
			} else {
				if failure == nil {
					t.Fatal("unsafe or unreadable VM identity accepted deletion")
				}
				for _, action := range f.core.Actions() {
					if action.GetVerb() == "delete" {
						t.Fatal("rejected VM identity deleted storage")
					}
				}
			}
			if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "manual42", metav1.GetOptions{}); err != nil {
				t.Fatal("production discovery changed retained B")
			}
		})
	}
}

// capability_id: rainbond.vm-disk.active-binding-consumers
func TestActiveBindingRejectedCapacityPreservesIntentAndReleasesPermit(t *testing.T) {
	for _, scenario := range []string{"unsupported-class", "unavailable-capability", "requested-shrink"} {
		t.Run(scenario, func(t *testing.T) {
			f := newBindingConsumerFixture(t)
			target := int64(2)
			switch scenario {
			case "unsupported-class":
				storage, err := f.core.StorageV1().StorageClasses().Get(context.Background(), "fast", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				allowed := false
				storage.AllowVolumeExpansion = &allowed
				if _, err := f.core.StorageV1().StorageClasses().Update(context.Background(), storage, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			case "unavailable-capability":
				f.action.kubevirtClient = nil
			case "requested-shrink":
				pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
				if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(context.Background(), pvc, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
				target = 1
			}
			f.core.ClearActions()
			err := f.action.UpdVolume(f.scope.ServiceID, &apimodel.UpdVolumeReq{VolumeName: f.volume.VolumeName, VolumePath: f.volume.VolumePath, VolumeType: f.volume.VolumeType, VolumeCapacity: &target})
			if err == nil {
				t.Fatal("unsupported capacity request was accepted")
			}
			for _, action := range f.core.Actions() {
				if action.GetResource().Resource == "persistentvolumeclaims" && action.GetVerb() == "update" {
					t.Fatal("rejected request changed PVC capacity")
				}
			}
			var stored dbmodel.TenantServiceVolume
			if err := f.database.Where("ID = ?", f.volume.ID).First(&stored).Error; err != nil || stored.VolumeCapacity != 1 {
				t.Fatalf("rejected request changed logical capacity: %v", err)
			}
			current, err := f.store.Read(f.scope, "")
			if err != nil || current.State.Admission != nil {
				t.Fatalf("known rejected request retained a live permit: %v", err)
			}
		})
	}
}
