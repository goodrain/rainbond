package dao

import (
	"fmt"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type AppDaoImpl struct {
	DB *gorm.DB
}

func (a *AppDaoImpl) AddModel(mo model.Interface) error {
	app, ok := mo.(*model.AppStatus)
	if !ok {
		return errors.New("Failed to convert interface to AppStatus")
	}

	var old model.AppStatus
	if ok := a.DB.Where("event_id = ?", app.EventID).Find(&old).RecordNotFound(); ok {
		if err := a.DB.Create(app).Error; err != nil {
			return err
		}
	}

	return nil
}

func (a *AppDaoImpl) UpdateModel(mo model.Interface) error {
	app, ok := mo.(*model.AppStatus)
	if !ok {
		return errors.New("Failed to convert interface to AppStatus")
	}

	return a.DB.Table(app.TableName()).
		Where("event_id = ?", app.EventID).
		Update(app).Error
}

func (a *AppDaoImpl) DeleteModelByEventId(eventID string) error {
	var app model.AppStatus
	if ok := a.DB.Where("event_id = ?", eventID).Find(&app).RecordNotFound(); ok {
		return nil
	}

	return a.DB.Where("event_id = ?", eventID).Delete(&app).Error
}

func (a *AppDaoImpl) GetByEventId(eventID string) (*model.AppStatus, error) {
	var app model.AppStatus
	err := a.DB.Where("event_id = ?", eventID).First(&app).Error

	return &app, err
}

//AppBackupDaoImpl group app backup info store mysql impl
type AppBackupDaoImpl struct {
	DB *gorm.DB
}

//AddModel AddModel
func (a *AppBackupDaoImpl) AddModel(mo model.Interface) error {
	app, ok := mo.(*model.AppBackup)
	if !ok {
		return errors.New("Failed to convert interface to AppStatus")
	}

	var old model.AppBackup
	if ok := a.DB.Where("backup_id = ?", app.BackupID).Find(&old).RecordNotFound(); ok {
		if err := a.DB.Create(app).Error; err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("backup info exist with id %s", app.BackupID)
}

//UpdateModel UpdateModel
func (a *AppBackupDaoImpl) UpdateModel(mo model.Interface) error {
	app, ok := mo.(*model.AppBackup)
	if !ok {
		return errors.New("Failed to convert interface to AppStatus")
	}
	if app.ID == 0 {
		return errors.New("Primary id can not be 0 when update")
	}
	return a.DB.Table(app.TableName()).Where("backup_id = ?", app.BackupID).Update(app).Error
}

//CheckHistory CheckHistory
func (a *AppBackupDaoImpl) CheckHistory(groupID, version string) bool {
	var app model.AppBackup
	exist := a.DB.Where("((group_id = ? and status in (?)) or version=?) and deleted=? ", groupID, []string{"starting", "restore"}, version, false).Find(&app).RecordNotFound()
	return !exist
}

//GetAppBackups GetAppBackups
func (a *AppBackupDaoImpl) GetAppBackups(groupID string) ([]*model.AppBackup, error) {
	var apps []*model.AppBackup
	if err := a.DB.Where("group_id = ? and deleted=?", groupID, false).Find(&apps).Error; err != nil {
		return nil, err
	}
	return apps, nil
}

//DeleteAppBackup DeleteAppBackup
func (a *AppBackupDaoImpl) DeleteAppBackup(backupID string) error {
	var app model.AppBackup
	if err := a.DB.Where("backup_id = ?", backupID).Delete(&app).Error; err != nil {
		return err
	}
	return nil
}

//GetAppBackup GetAppBackup
func (a *AppBackupDaoImpl) GetAppBackup(backupID string) (*model.AppBackup, error) {
	var app model.AppBackup
	if err := a.DB.Where("backup_id = ? and deleted=?", backupID, false).Find(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

//GetDeleteAppBackup GetDeleteAppBackup
func (a *AppBackupDaoImpl) GetDeleteAppBackup(backupID string) (*model.AppBackup, error) {
	var app model.AppBackup
	if err := a.DB.Where("backup_id = ? and deleted=?", backupID, true).Find(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

//GetDeleteAppBackups GetDeleteAppBackups
func (a *AppBackupDaoImpl) GetDeleteAppBackups() ([]*model.AppBackup, error) {
	var apps []*model.AppBackup
	if err := a.DB.Where("deleted=?", true).Find(&apps).Error; err != nil {
		return nil, err
	}
	return apps, nil
}
