package handler

import (
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
