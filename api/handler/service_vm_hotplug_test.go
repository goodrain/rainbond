package handler

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	v1 "kubevirt.io/api/core/v1"
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
