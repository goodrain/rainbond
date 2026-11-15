package handler

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
)

func TestBuildVMHotplugAddVolumeOptionsUsesSCSIBusForIndexedDiskPath(t *testing.T) {
	opts := buildVMHotplugAddVolumeOptions("manual99", "/disk-1")

	if opts == nil || opts.Disk == nil || opts.Disk.DiskDevice.Disk == nil {
		t.Fatalf("expected hotplug add volume options to create a disk target, got %#v", opts)
	}
	if opts.Disk.DiskDevice.Disk.Bus != v1.DiskBusSCSI {
		t.Fatalf("expected indexed vm hotplug disk to use scsi bus, got %q", opts.Disk.DiskDevice.Disk.Bus)
	}
}

func TestResolveVMHotplugAccessModesConvertsShorthand(t *testing.T) {
	modes := resolveVMHotplugAccessModes(&dbmodel.TenantServiceVolume{AccessMode: "RWX"})

	if !reflect.DeepEqual(modes, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}) {
		t.Fatalf("expected RWX to convert to ReadWriteMany, got %#v", modes)
	}
}

func TestResolveVMHotplugAccessModesKeepsExpandedValues(t *testing.T) {
	modes := resolveVMHotplugAccessModes(&dbmodel.TenantServiceVolume{AccessMode: "ReadWriteOnce,ReadOnlyMany"})

	expected := []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce, corev1.ReadOnlyMany}
	if !reflect.DeepEqual(modes, expected) {
		t.Fatalf("expected expanded access modes to be preserved, got %#v", modes)
	}
}

func TestResolveVMHotplugAccessModesDefaultsToReadWriteMany(t *testing.T) {
	modes := resolveVMHotplugAccessModes(&dbmodel.TenantServiceVolume{})

	if !reflect.DeepEqual(modes, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}) {
		t.Fatalf("expected empty access mode to default to ReadWriteMany, got %#v", modes)
	}
}

// capability_id: rainbond.vm-hotplug.data-volume-capacity-gi
func TestBuildVMHotplugDataVolumeObjectUsesGiCapacity(t *testing.T) {
	obj := buildVMHotplugDataVolumeObject("demo-ns", "tenant-1", &dbmodel.TenantServiceVolume{
		ServiceID:      "service-vm",
		VolumeName:     "data-1",
		VolumePath:     "/disk-1",
		VolumeType:     "nfs-storage",
		VolumeCapacity: 10,
		AccessMode:     "RWX",
	}, "manual99")

	storage, found, err := unstructured.NestedString(obj.Object, "spec", "storage", "resources", "requests", "storage")
	if err != nil || !found {
		t.Fatalf("expected DataVolume storage request, found=%t err=%v obj=%#v", found, err, obj.Object)
	}
	if storage != "10Gi" {
		t.Fatalf("expected 10GB vm hotplug disk to request 10Gi, got %q", storage)
	}
}

type hotplugVolumeTestManager struct {
	db.Manager
	tx         *gorm.DB
	volumeDao  dbdao.TenantServiceVolumeDao
	serviceDao dbdao.TenantServiceDao
}

func (m hotplugVolumeTestManager) Begin() *gorm.DB {
	return m.tx
}

func (m hotplugVolumeTestManager) TenantServiceVolumeDaoTransactions(*gorm.DB) dbdao.TenantServiceVolumeDao {
	return m.volumeDao
}

func (m hotplugVolumeTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.serviceDao
}

type hotplugTenantServiceVolumeDao struct {
	dbdao.TenantServiceVolumeDao
	added *dbmodel.TenantServiceVolume
}

func (d *hotplugTenantServiceVolumeDao) AddModel(arg dbmodel.Interface) error {
	volume, ok := arg.(*dbmodel.TenantServiceVolume)
	if !ok {
		return nil
	}
	copied := *volume
	copied.ID = 99
	volume.ID = 99
	d.added = &copied
	return nil
}

type hotplugTenantServiceDao struct {
	dbdao.TenantServiceDao
	service *dbmodel.TenantServices
}

func (d *hotplugTenantServiceDao) GetServiceByID(serviceID string) (*dbmodel.TenantServices, error) {
	return d.service, nil
}

type hotunplugVolumeDeleteTestManager struct {
	db.Manager
	tx               *gorm.DB
	volumeDao        dbdao.TenantServiceVolumeDao
	mountRelationDao dbdao.TenantServiceMountRelationDao
	configFileDao    dbdao.TenantServiceConfigFileDao
}

func (m hotunplugVolumeDeleteTestManager) Begin() *gorm.DB {
	return m.tx
}

func (m hotunplugVolumeDeleteTestManager) TenantServiceVolumeDaoTransactions(*gorm.DB) dbdao.TenantServiceVolumeDao {
	return m.volumeDao
}

func (m hotunplugVolumeDeleteTestManager) TenantServiceMountRelationDaoTransactions(*gorm.DB) dbdao.TenantServiceMountRelationDao {
	return m.mountRelationDao
}

func (m hotunplugVolumeDeleteTestManager) TenantServiceConfigFileDaoTransactions(*gorm.DB) dbdao.TenantServiceConfigFileDao {
	return m.configFileDao
}

type hotunplugTenantServiceVolumeDao struct {
	dbdao.TenantServiceVolumeDao
	volume        *dbmodel.TenantServiceVolume
	deletedVolume string
}

func (d *hotunplugTenantServiceVolumeDao) GetVolumeByServiceIDAndName(serviceID, name string) (*dbmodel.TenantServiceVolume, error) {
	return d.volume, nil
}

func (d *hotunplugTenantServiceVolumeDao) DeleteModel(serviceID string, args ...interface{}) error {
	if len(args) > 0 {
		if volumeName, ok := args[0].(string); ok {
			d.deletedVolume = volumeName
		}
	}
	return nil
}

type hotunplugTenantServiceMountRelationDao struct {
	dbdao.TenantServiceMountRelationDao
}

func (d *hotunplugTenantServiceMountRelationDao) GetTenantServiceMountRelationsByDepServiceAndVolumeName(serviceID, volumeName string) ([]*dbmodel.TenantServiceMountRelation, error) {
	return nil, nil
}

type hotunplugTenantServiceConfigFileDao struct {
	dbdao.TenantServiceConfigFileDao
	serviceID  string
	volumeName string
	created    *dbmodel.TenantServiceConfigFile
}

func (d *hotunplugTenantServiceConfigFileDao) AddModel(arg dbmodel.Interface) error {
	cfg, ok := arg.(*dbmodel.TenantServiceConfigFile)
	if !ok {
		return nil
	}
	copied := *cfg
	d.created = &copied
	return nil
}

func (d *hotunplugTenantServiceConfigFileDao) DelByVolumeID(serviceID, volumeName string) error {
	d.serviceID = serviceID
	d.volumeName = volumeName
	return nil
}

type hotplugConfigFileTestManager struct {
	db.Manager
	tx            *gorm.DB
	volumeDao     dbdao.TenantServiceVolumeDao
	configFileDao dbdao.TenantServiceConfigFileDao
	serviceDao    dbdao.TenantServiceDao
}

func (m hotplugConfigFileTestManager) Begin() *gorm.DB {
	return m.tx
}

func (m hotplugConfigFileTestManager) TenantServiceVolumeDaoTransactions(*gorm.DB) dbdao.TenantServiceVolumeDao {
	return m.volumeDao
}

func (m hotplugConfigFileTestManager) TenantServiceConfigFileDaoTransactions(*gorm.DB) dbdao.TenantServiceConfigFileDao {
	return m.configFileDao
}

func (m hotplugConfigFileTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.serviceDao
}

type envAttrTestManager struct {
	db.Manager
	envDao     dbdao.TenantServiceEnvVarDao
	serviceDao dbdao.TenantServiceDao
}

func (m envAttrTestManager) TenantServiceEnvVarDao() dbdao.TenantServiceEnvVarDao {
	return m.envDao
}

func (m envAttrTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.serviceDao
}

type envAttrDaoStub struct {
	dbdao.TenantServiceEnvVarDao
	updated bool
}

func (d *envAttrDaoStub) AddModel(arg dbmodel.Interface) error {
	return nil
}

func (d *envAttrDaoStub) DeleteModel(serviceID string, args ...interface{}) error {
	return nil
}

func (d *envAttrDaoStub) UpdateModelByAttrName(env *dbmodel.TenantServiceEnvVar, oldAttrName string) error {
	d.updated = true
	return nil
}

func TestVolumnVarHotplugsRunningVMDataDisk(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer sqlDB.Close()

	gdb, err := gorm.Open("mysql", sqlDB)
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}
	defer gdb.Close()

	mock.ExpectBegin()
	tx := gdb.Begin()
	if err := tx.Error; err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	mock.ExpectCommit()

	volumeDao := &hotplugTenantServiceVolumeDao{}
	serviceDao := &hotplugTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:    "service-vm",
			ExtendMethod: "vm",
		},
	}
	db.SetTestManager(hotplugVolumeTestManager{tx: tx, volumeDao: volumeDao, serviceDao: serviceDao})
	defer db.SetTestManager(nil)

	hotplugCalled := false
	syncCalled := false
	action := &ServiceAction{
		hotplugVMDataDiskHook: func(tenantID string, volume *dbmodel.TenantServiceVolume) error {
			hotplugCalled = true
			if volume.ID != 99 {
				t.Fatalf("expected persisted volume id 99, got %d", volume.ID)
			}
			return nil
		},
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncCalled = true
			return nil
		},
	}
	apiErr := action.VolumnVar(&dbmodel.TenantServiceVolume{
		ServiceID:      "service-vm",
		VolumeName:     "data-1",
		VolumePath:     "/disk",
		VolumeType:     dbmodel.VMVolumeType.String(),
		VolumeCapacity: 10240,
	}, "tenant-1", "", "add")
	if apiErr != nil {
		t.Fatalf("expected no error, got %v", apiErr)
	}
	if !hotplugCalled {
		t.Fatal("expected running vm volume add to hotplug data disk")
	}
	if syncCalled {
		t.Fatal("did not expect running vm volume add to sync vm spec directly")
	}
	if volumeDao.added == nil || volumeDao.added.VolumeName != "data-1" {
		t.Fatalf("expected volume to be persisted, got %#v", volumeDao.added)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestVolumnVarAddSyncsVMConfigFileImmediately(t *testing.T) {
	fixture := newBindingConsumerFixture(t)
	current, _ := fixture.store.Read(fixture.scope, "")
	_, err := fixture.store.Mutate(fixture.scope, vmrestore.Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		c.State.Bindings = map[string]vmrestore.Binding{}
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.action.dbmanager = independentAddManager{bindingConsumerDB{database: fixture.database}}
	hotplugCalled, syncCalled := false, false
	fixture.action.hotplugVMDataDiskHook = func(string, *dbmodel.TenantServiceVolume) error {
		hotplugCalled = true
		return nil
	}
	fixture.action.syncVirtualMachineSpecHook = func(serviceID string) error {
		syncCalled = true
		if serviceID != fixture.scope.ServiceID {
			t.Fatalf("expected vm spec sync for service-vm, got %s", serviceID)
		}
		state, err := fixture.store.Read(fixture.scope, "")
		if err != nil || state.State.Admission == nil {
			t.Fatal("config creation synchronized without its admission")
		}
		return nil
	}
	apiErr := fixture.action.VolumnVar(&dbmodel.TenantServiceVolume{
		ServiceID: fixture.scope.ServiceID, VolumeName: "test", VolumePath: "test.txt", VolumeType: dbmodel.ConfigFileVolumeType.String(),
	}, fixture.scope.TenantID, "demo=1", "add")
	if apiErr != nil {
		t.Fatalf("expected no error, got %v", apiErr)
	}
	if hotplugCalled {
		t.Fatal("did not expect config-file add to go through vm hotplug path")
	}
	if !syncCalled {
		t.Fatal("expected config-file add to sync vm spec immediately")
	}
	var volume dbmodel.TenantServiceVolume
	if err := fixture.database.Where("service_id = ? AND volume_name = ?", fixture.scope.ServiceID, "test").First(&volume).Error; err != nil || volume.VolumeType != dbmodel.ConfigFileVolumeType.String() {
		t.Fatalf("expected config-file volume to be persisted: %v", err)
	}
	var config dbmodel.TenantServiceConfigFile
	if err := fixture.database.Where("service_id = ? AND volume_name = ?", fixture.scope.ServiceID, "test").First(&config).Error; err != nil || config.VolumeName != "test" || config.FileContent != "demo=1" {
		t.Fatalf("expected config-file content to be persisted: %v", err)
	}
}

func TestEnvAttrSyncsVMSpecAfterUpdate(t *testing.T) {
	serviceDao := &hotplugTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:    "service-vm",
			ExtendMethod: "vm",
		},
	}
	envDao := &envAttrDaoStub{}
	db.SetTestManager(envAttrTestManager{
		envDao:     envDao,
		serviceDao: serviceDao,
	})
	defer db.SetTestManager(nil)

	syncCalled := false
	action := &ServiceAction{
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncCalled = true
			if serviceID != "service-vm" {
				t.Fatalf("expected service-vm sync target, got %s", serviceID)
			}
			return nil
		},
	}

	err := action.EnvAttr("update", &dbmodel.TenantServiceEnvVar{
		ServiceID: "service-vm",
		AttrName:  "DEMO_HOST",
		AttrValue: "demo-service",
	}, "DEMO_HOST")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !syncCalled {
		t.Fatal("expected vm env update to sync vm spec")
	}
	if !envDao.updated {
		t.Fatal("expected env dao update to be called")
	}
}

func TestHotplugVMDataDiskSyncsStoppedVMForSelectedStorageClassVolume(t *testing.T) {
	serviceDao := &hotplugTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:    "service-vm",
			ExtendMethod: "vm",
		},
	}
	db.SetTestManager(hotplugVolumeTestManager{serviceDao: serviceDao})
	defer db.SetTestManager(nil)

	syncCalled := false
	action := &ServiceAction{
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncCalled = true
			if serviceID != "service-vm" {
				t.Fatalf("expected service-vm sync target, got %s", serviceID)
			}
			return nil
		},
	}
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			Status: v1.VirtualMachineStatus{
				PrintableStatus: v1.VirtualMachineStatusStopped,
			},
		}, nil
	}

	err := action.hotplugVMDataDisk("tenant-1", &dbmodel.TenantServiceVolume{
		ServiceID:      "service-vm",
		VolumeName:     "data-1",
		VolumePath:     "/disk-1",
		VolumeType:     "nfs-storage",
		VolumeCapacity: 20,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !syncCalled {
		t.Fatal("expected selected storage class vm disk to sync stopped vm spec")
	}
}

// capability_id: rainbond.vm-hotplug.add-volume-conflict-retry
func TestPerformVMHotplugAddVolumeRetriesConflicts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVM := kubecli.NewMockVirtualMachineInterface(ctrl)
	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVM).Times(2)

	attempts := 0
	mockVM.EXPECT().AddVolume(gomock.Any(), "demo-vm", gomock.Any()).Times(2).DoAndReturn(
		func(_ context.Context, _ string, opts *v1.AddVolumeOptions) error {
			if opts == nil || opts.Name != "manual99" {
				t.Fatalf("expected add volume options for manual99, got %#v", opts)
			}
			attempts++
			if attempts == 1 {
				return k8serrors.NewConflict(schema.GroupResource{Group: "kubevirt.io", Resource: "virtualmachines"}, "demo-vm", errors.New("the object has been modified"))
			}
			return nil
		},
	)

	action := &ServiceAction{
		kubevirtClient: mockClient,
	}
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-vm",
				Namespace: "demo-ns",
			},
			Status: v1.VirtualMachineStatus{
				PrintableStatus: v1.VirtualMachineStatusRunning,
			},
		}, nil
	}

	if err := action.performVMHotplugAddVolume("service-vm", buildVMHotplugAddVolumeOptions("manual99", "/disk-1")); err != nil {
		t.Fatalf("expected conflict retry to succeed, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected add volume to retry once after conflict, got %d attempts", attempts)
	}
}

// capability_id: rainbond.vm-hotplug.remove-volume-running-vm
func TestHotunplugVMDataDiskRemovesVolumeFromRunningVM(t *testing.T) {
	TestVolumnVarDeleteHotunplugsRunningVMDataDisk(t)
}

// capability_id: rainbond.vm-hotplug.remove-volume-running-vm
func TestHotunplugVMDataDiskDeletesBackingDataVolumeAndPVC(t *testing.T) {
	f := newBindingConsumerFixture(t)
	dv := buildVMHotplugDataVolumeObject(f.scope.Namespace, f.scope.TenantID, f.volume, "active-a-dv")
	dv.SetUID("active-a-dv-uid")
	dv.Object["status"] = map[string]interface{}{"claimName": "active-a"}
	if _, err := f.dynamic.Resource(dataVolumeGVR).Namespace(f.scope.Namespace).Create(context.Background(), dv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	state, err := f.store.Read(f.scope, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Mutate(f.scope, vmrestore.Expectation{ControlRevision: state.State.ControlRevision, BindingGeneration: state.State.BindingGeneration}, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		binding := c.State.Bindings["42"]
		binding.ActiveDV = &vmrestore.ObjectRef{Kind: "DataVolume", Namespace: f.scope.Namespace, Name: dv.GetName(), UID: string(dv.GetUID())}
		c.State.Bindings["42"] = binding
		c.State.BindingGeneration++
		return vmrestore.Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.action.hotunplugVMDataDisk(f.volume); err != nil {
		t.Fatal(err)
	}
	if _, err = f.dynamic.Resource(dataVolumeGVR).Namespace(f.scope.Namespace).Get(context.Background(), dv.GetName(), metav1.GetOptions{}); !k8serrors.IsNotFound(err) {
		t.Fatal("active DV remains")
	}
	if _, err = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "active-a", metav1.GetOptions{}); !k8serrors.IsNotFound(err) {
		t.Fatal("active PVC remains")
	}
	if _, err = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "manual42", metav1.GetOptions{}); err != nil {
		t.Fatal("backing cleanup reclaimed B")
	}
}

// capability_id: rainbond.vm-hotplug.remove-volume-running-vm
func TestVolumnVarDeleteHotunplugsRunningVMDataDisk(t *testing.T) {
	f := newBindingConsumerFixture(t)
	first, second := uint(1), uint(2)
	vm := &v1.VirtualMachine{TypeMeta: metav1.TypeMeta{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachine"}, ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: f.scope.Namespace, UID: "running-vm-uid", ResourceVersion: "1", Labels: map[string]string{"service_id": f.scope.ServiceID}}, Spec: v1.VirtualMachineSpec{Template: &v1.VirtualMachineInstanceTemplateSpec{Spec: v1.VirtualMachineInstanceSpec{Volumes: []v1.Volume{{Name: "root", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "root"}}}}, {Name: "manual42", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "active-a"}, Hotpluggable: true}}}}, Domain: v1.DomainSpec{Devices: v1.Devices{Disks: []v1.Disk{{Name: "root", BootOrder: &first}, {Name: "manual42", BootOrder: &second}}}}}}}}
	persistExpansionTestVM(t, f.dynamic, vm)
	f.action.getVirtualMachineByServiceIDHook = func(string) (*v1.VirtualMachine, error) {
		object, e := f.dynamic.Resource(vmDiskVMGVR).Namespace(f.scope.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
		if e != nil {
			return nil, e
		}
		actual := &v1.VirtualMachine{}
		e = runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, actual)
		return actual, e
	}
	instance := &v1.VirtualMachineInstance{TypeMeta: metav1.TypeMeta{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachineInstance"}, ObjectMeta: metav1.ObjectMeta{Name: vm.Name, Namespace: vm.Namespace, UID: "running-vmi-uid", Labels: map[string]string{"service_id": f.scope.ServiceID}, OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachine", Name: vm.Name, UID: vm.UID}}}, Spec: vm.Spec.Template.Spec}
	raw, err := json.Marshal(instance)
	if err != nil {
		t.Fatal(err)
	}
	object := &unstructured.Unstructured{}
	if err = object.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	if _, err = f.dynamic.Resource(vmDiskVMIGVR).Namespace(f.scope.Namespace).Create(context.Background(), object, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err = f.database.Create(&dbmodel.TenantServiceConfigFile{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName, FileContent: "fixture"}).Error; err != nil {
		t.Fatal(err)
	}
	ctrl := gomock.NewController(t)
	client := kubecli.NewMockKubevirtClient(ctrl)
	virtualMachine := kubecli.NewMockVirtualMachineInterface(ctrl)
	client.EXPECT().VirtualMachine(f.scope.Namespace).Return(virtualMachine).Times(1)
	virtualMachine.EXPECT().RemoveVolume(gomock.Any(), vm.Name, gomock.Any()).DoAndReturn(func(_ context.Context, _ string, options *v1.RemoveVolumeOptions) error {
		if options.Name != "manual42" {
			t.Fatal("hot-unplug did not use canonical device name")
		}
		return nil
	}).Times(1)
	f.action.kubevirtClient = client
	syncCalled := false
	f.action.syncVirtualMachineSpecHook = func(string) error { syncCalled = true; return nil }
	request := &dbmodel.TenantServiceVolume{ServiceID: f.scope.ServiceID, VolumeName: f.volume.VolumeName}
	pending := f.action.VolumnVar(request, f.scope.TenantID, "", "delete")
	if pending == nil || pending.Code != 409 || !strings.Contains(pending.Error(), "卸载") {
		t.Fatalf("expected pending hot-unplug with durable record, got %v", pending)
	}
	var volumes, configs int
	f.database.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 42).Count(&volumes)
	f.database.Model(&dbmodel.TenantServiceConfigFile{}).Where("service_id = ? AND volume_name = ?", f.scope.ServiceID, f.volume.VolumeName).Count(&configs)
	if volumes != 1 || configs != 1 {
		t.Fatal("pending detach removed logical volume or config")
	}
	if err = f.dynamic.Resource(vmDiskVMIGVR).Namespace(f.scope.Namespace).Delete(context.Background(), instance.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := f.action.VolumnVar(request, f.scope.TenantID, "", "delete"); err != nil {
		t.Fatal(err)
	}
	f.database.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 42).Count(&volumes)
	f.database.Model(&dbmodel.TenantServiceConfigFile{}).Where("service_id = ? AND volume_name = ?", f.scope.ServiceID, f.volume.VolumeName).Count(&configs)
	if volumes != 0 || configs != 0 {
		t.Fatal("confirmed detach did not remove only its records")
	}
	if syncCalled || len(f.mq.tasks) != 0 {
		t.Fatal("mapped disk deletion fell back to generic sync or broad GC")
	}
	if _, err = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "manual42", metav1.GetOptions{}); err != nil {
		t.Fatal("mapped deletion reclaimed B")
	}
}
