package handler

import (
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

type volumeUpdateTestManager struct {
	db.Manager
	tx        *gorm.DB
	volumeDao dbdao.TenantServiceVolumeDao
}

func (m volumeUpdateTestManager) Begin() *gorm.DB {
	return m.tx
}

func (m volumeUpdateTestManager) TenantServiceVolumeDaoTransactions(*gorm.DB) dbdao.TenantServiceVolumeDao {
	return m.volumeDao
}

type volumeUpdateTenantServiceVolumeDao struct {
	dbdao.TenantServiceVolumeDao
	volume        *dbmodel.TenantServiceVolume
	requestSID    string
	requestName   string
	updatedVolume *dbmodel.TenantServiceVolume
}

func (d *volumeUpdateTenantServiceVolumeDao) GetVolumeByServiceIDAndName(serviceID, name string) (*dbmodel.TenantServiceVolume, error) {
	d.requestSID = serviceID
	d.requestName = name
	return d.volume, nil
}

func (d *volumeUpdateTenantServiceVolumeDao) UpdateModel(arg dbmodel.Interface) error {
	volume, ok := arg.(*dbmodel.TenantServiceVolume)
	if !ok {
		return nil
	}
	copied := *volume
	d.updatedVolume = &copied
	return nil
}

type volumeDeleteSharedMountTestManager struct {
	db.Manager
	tx               *gorm.DB
	volumeDao        dbdao.TenantServiceVolumeDao
	mountRelationDao dbdao.TenantServiceMountRelationDao
	configFileDao    dbdao.TenantServiceConfigFileDao
}

func (m volumeDeleteSharedMountTestManager) Begin() *gorm.DB {
	return m.tx
}

func (m volumeDeleteSharedMountTestManager) TenantServiceVolumeDaoTransactions(*gorm.DB) dbdao.TenantServiceVolumeDao {
	return m.volumeDao
}

func (m volumeDeleteSharedMountTestManager) TenantServiceMountRelationDaoTransactions(*gorm.DB) dbdao.TenantServiceMountRelationDao {
	return m.mountRelationDao
}

func (m volumeDeleteSharedMountTestManager) TenantServiceConfigFileDaoTransactions(*gorm.DB) dbdao.TenantServiceConfigFileDao {
	return m.configFileDao
}

type volumeDeleteTenantServiceVolumeDao struct {
	dbdao.TenantServiceVolumeDao
	volume  *dbmodel.TenantServiceVolume
	deleted bool
}

func (d *volumeDeleteTenantServiceVolumeDao) GetVolumeByServiceIDAndName(serviceID, name string) (*dbmodel.TenantServiceVolume, error) {
	return d.volume, nil
}

func (d *volumeDeleteTenantServiceVolumeDao) DeleteModel(serviceID string, args ...interface{}) error {
	d.deleted = true
	return nil
}

type volumeDeleteMountRelationDao struct {
	dbdao.TenantServiceMountRelationDao
	relations []*dbmodel.TenantServiceMountRelation
}

func (d *volumeDeleteMountRelationDao) GetTenantServiceMountRelationsByDepServiceAndVolumeName(serviceID, volumeName string) ([]*dbmodel.TenantServiceMountRelation, error) {
	return d.relations, nil
}

type volumeDeleteConfigFileDao struct {
	dbdao.TenantServiceConfigFileDao
	deleted bool
}

func (d *volumeDeleteConfigFileDao) DelByVolumeID(serviceID, volumeName string) error {
	d.deleted = true
	return nil
}

// capability_id: rainbond.component.volume-update-persists-capacity
func TestServiceActionUpdVolumeUpdatesVolumeCapacity(t *testing.T) {
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

	volumeDao := &volumeUpdateTenantServiceVolumeDao{
		volume: &dbmodel.TenantServiceVolume{
			Model:          dbmodel.Model{ID: 1},
			ServiceID:      "service-1",
			VolumeName:     "data",
			VolumePath:     "/data",
			VolumeCapacity: 10,
		},
	}
	db.SetTestManager(volumeUpdateTestManager{tx: tx, volumeDao: volumeDao})
	defer db.SetTestManager(nil)

	volumeCapacity := int64(20)
	err = (&ServiceAction{}).UpdVolume("service-1", &apimodel.UpdVolumeReq{
		VolumeName:     "data",
		VolumeType:     "share-file",
		VolumePath:     "/data",
		VolumeCapacity: &volumeCapacity,
	})
	if err != nil {
		t.Fatalf("update volume: %v", err)
	}

	if volumeDao.requestSID != "service-1" || volumeDao.requestName != "data" {
		t.Fatalf("expected volume lookup by service-1/data, got %s/%s", volumeDao.requestSID, volumeDao.requestName)
	}
	if volumeDao.updatedVolume == nil {
		t.Fatal("expected updated volume to be saved")
	}
	if volumeDao.updatedVolume.VolumeCapacity != 20 {
		t.Fatalf("expected volume capacity 20, got %d", volumeDao.updatedVolume.VolumeCapacity)
	}
	if volumeDao.updatedVolume.VolumePath != "/data" {
		t.Fatalf("expected volume path /data, got %s", volumeDao.updatedVolume.VolumePath)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

// capability_id: rainbond.component.volume-delete-blocks-shared-mount
func TestServiceActionVolumnVarDeleteRejectsSharedMountedVolume(t *testing.T) {
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
	mock.ExpectRollback()

	volumeDao := &volumeDeleteTenantServiceVolumeDao{
		volume: &dbmodel.TenantServiceVolume{
			Model:      dbmodel.Model{ID: 7},
			ServiceID:  "component-a",
			VolumeName: "shared-data",
			VolumePath: "/data",
			VolumeType: dbmodel.ShareFileVolumeType.String(),
		},
	}
	mountRelationDao := &volumeDeleteMountRelationDao{
		relations: []*dbmodel.TenantServiceMountRelation{
			{
				ServiceID:       "component-b",
				DependServiceID: "component-a",
				VolumeName:      "shared-data",
			},
		},
	}
	configFileDao := &volumeDeleteConfigFileDao{}
	db.SetTestManager(volumeDeleteSharedMountTestManager{
		tx:               tx,
		volumeDao:        volumeDao,
		mountRelationDao: mountRelationDao,
		configFileDao:    configFileDao,
	})
	defer db.SetTestManager(nil)

	mq := &recordingMQClient{}
	apiErr := (&ServiceAction{
		MQClient: mq,
		hotunplugVMDataDiskHook: func(volume *dbmodel.TenantServiceVolume) error {
			t.Fatal("did not expect hotunplug when volume is shared mounted")
			return nil
		},
		syncVirtualMachineSpecHook: func(serviceID string) error {
			t.Fatal("did not expect vm spec sync when volume is shared mounted")
			return nil
		},
	}).VolumnVar(&dbmodel.TenantServiceVolume{
		ServiceID:  "component-a",
		VolumeName: "shared-data",
	}, "tenant-1", "", "delete")

	if apiErr == nil {
		t.Fatal("expected shared mounted volume deletion to be rejected")
	}
	if apiErr.Code != 400 {
		t.Fatalf("expected 400 error, got %d: %v", apiErr.Code, apiErr)
	}
	if !strings.Contains(apiErr.Error(), "共享挂载") {
		t.Fatalf("expected shared mount error, got %q", apiErr.Error())
	}
	if volumeDao.deleted {
		t.Fatal("did not expect volume delete to be called")
	}
	if configFileDao.deleted {
		t.Fatal("did not expect config file cleanup to be called")
	}
	if len(mq.tasks) != 0 {
		t.Fatalf("did not expect volume_gc task, got %d", len(mq.tasks))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
