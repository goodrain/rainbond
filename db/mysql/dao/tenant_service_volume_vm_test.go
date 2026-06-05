package dao

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.vm-volume.allow-shared-device-paths
func TestTenantServiceVolumeDaoAddModelAllowsDuplicateVMDevicePath(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()

	gdb, err := gorm.Open("mysql", db)
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}
	defer gdb.Close()

	dao := &TenantServiceVolumeDaoImpl{DB: gdb}
	volume := &model.TenantServiceVolume{
		ServiceID:      "service-vm",
		VolumeName:     "data-1",
		VolumePath:     "/disk",
		VolumeType:     "nfs-storage",
		VolumeCapacity: 10240,
	}

	mock.ExpectQuery("SELECT").WillReturnRows(
		sqlmock.NewRows([]string{"service_id", "extend_method"}).
			AddRow("service-vm", model.ServiceTypeVM.String()),
	)
	mock.ExpectQuery("SELECT").WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := dao.AddModel(volume); err != nil {
		t.Fatalf("expected vm duplicate device path insert to succeed, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

// capability_id: rainbond.volume.keep-non-vm-path-uniqueness
func TestTenantServiceVolumeDaoAddModelRejectsDuplicatePathForNonVMService(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()

	gdb, err := gorm.Open("mysql", db)
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}
	defer gdb.Close()

	dao := &TenantServiceVolumeDaoImpl{DB: gdb}
	volume := &model.TenantServiceVolume{
		ServiceID:      "service-app",
		VolumeName:     "data-1",
		VolumePath:     "/disk",
		VolumeType:     "nfs-storage",
		VolumeCapacity: 10240,
	}

	mock.ExpectQuery("SELECT").WillReturnRows(
		sqlmock.NewRows([]string{"service_id", "extend_method"}).
			AddRow("service-app", model.ServiceTypeStatelessMultiple.String()),
	)
	mock.ExpectQuery("SELECT").WillReturnRows(
		sqlmock.NewRows([]string{"service_id", "volume_name", "volume_path"}).
			AddRow("service-app", "disk", "/disk"),
	)

	if err := dao.AddModel(volume); err == nil {
		t.Fatal("expected non-vm duplicate path insert to fail")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
