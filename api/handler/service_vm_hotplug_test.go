package handler

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

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
