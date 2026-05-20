package handler

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	tx            *gorm.DB
	volumeDao     dbdao.TenantServiceVolumeDao
	configFileDao dbdao.TenantServiceConfigFileDao
}

func (m hotunplugVolumeDeleteTestManager) Begin() *gorm.DB {
	return m.tx
}

func (m hotunplugVolumeDeleteTestManager) TenantServiceVolumeDaoTransactions(*gorm.DB) dbdao.TenantServiceVolumeDao {
	return m.volumeDao
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

type hotunplugTenantServiceConfigFileDao struct {
	dbdao.TenantServiceConfigFileDao
	serviceID  string
	volumeName string
}

func (d *hotunplugTenantServiceConfigFileDao) DelByVolumeID(serviceID, volumeName string) error {
	d.serviceID = serviceID
	d.volumeName = volumeName
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &hotplugTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:    "service-vm",
			ExtendMethod: "vm",
		},
	}
	db.SetTestManager(hotplugVolumeTestManager{serviceDao: serviceDao})
	defer db.SetTestManager(nil)

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVM := kubecli.NewMockVirtualMachineInterface(ctrl)
	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVM)
	mockVM.EXPECT().RemoveVolume(gomock.Any(), "demo-vm", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, opts *v1.RemoveVolumeOptions) error {
			if opts == nil || opts.Name != "manual42" {
				t.Fatalf("expected remove volume options for manual42, got %#v", opts)
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

	err := action.hotunplugVMDataDisk(&dbmodel.TenantServiceVolume{
		Model:          dbmodel.Model{ID: 42},
		ServiceID:      "service-vm",
		VolumeName:     "data-1",
		VolumePath:     "/disk-1",
		VolumeType:     "nfs-storage",
		VolumeCapacity: 20,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// capability_id: rainbond.vm-hotplug.remove-volume-running-vm
func TestVolumnVarDeleteHotunplugsRunningVMDataDisk(t *testing.T) {
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

	volumeDao := &hotunplugTenantServiceVolumeDao{
		volume: &dbmodel.TenantServiceVolume{
			Model:      dbmodel.Model{ID: 42},
			ServiceID:  "service-vm",
			VolumeName: "data-1",
			VolumePath: "/disk-1",
			VolumeType: "nfs-storage",
		},
	}
	configFileDao := &hotunplugTenantServiceConfigFileDao{}
	db.SetTestManager(hotunplugVolumeDeleteTestManager{
		tx:            tx,
		volumeDao:     volumeDao,
		configFileDao: configFileDao,
	})
	defer db.SetTestManager(nil)

	hotunplugCalled := false
	syncCalled := false
	action := &ServiceAction{
		MQClient: &noopMQClient{},
		hotunplugVMDataDiskHook: func(volume *dbmodel.TenantServiceVolume) error {
			hotunplugCalled = true
			if volume == nil || volume.ID != 42 {
				t.Fatalf("expected deleted vm volume with id 42, got %#v", volume)
			}
			return nil
		},
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncCalled = true
			return nil
		},
	}

	apiErr := action.VolumnVar(&dbmodel.TenantServiceVolume{
		ServiceID:  "service-vm",
		VolumeName: "data-1",
	}, "tenant-1", "", "delete")
	if apiErr != nil {
		t.Fatalf("expected no error, got %v", apiErr)
	}
	if !hotunplugCalled {
		t.Fatal("expected delete to hotunplug running vm data disk")
	}
	if syncCalled {
		t.Fatal("did not expect delete to fall back to vm spec sync when hotunplug hook handled it")
	}
	if volumeDao.deletedVolume != "data-1" {
		t.Fatalf("expected volume data-1 to be deleted, got %q", volumeDao.deletedVolume)
	}
	if configFileDao.serviceID != "service-vm" || configFileDao.volumeName != "data-1" {
		t.Fatalf("expected config file cleanup for deleted volume, got service=%q volume=%q", configFileDao.serviceID, configFileDao.volumeName)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
