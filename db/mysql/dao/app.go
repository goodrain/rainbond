package dao

import (
	"github.com/jinzhu/gorm"
	"github.com/goodrain/rainbond/db/model"
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

func (a *AppDaoImpl) DeleteModel(groupKey string, arg ...interface{}) error {
	if len(arg) < 1 {
		return errors.New("Must define version for delete AppStatus in mysql.")
	}

	version, ok := arg[0].(string)
	if !ok {
		return errors.New("Failed to convert interface to string")
	}

	var app model.AppStatus
	if ok := a.DB.Where("group_key = ? and version = ?", app.GroupKey, app.Version).Find(&app).RecordNotFound(); ok {
		return nil
	}

	return a.DB.Where("group_key = ? and version = ?", groupKey, version).Delete(&app).Error
}

func (a *AppDaoImpl) DeleteModelByEventId(eventId string) error {
	var app model.AppStatus
	if ok := a.DB.Where("event_id = ?", eventId).Find(&app).RecordNotFound(); ok {
		return nil
	}

	return a.DB.Where("event_id = ?", eventId).Delete(&app).Error
}

func (a *AppDaoImpl) Get(groupKey, version string) (interface{}, error) {
	var app model.AppStatus
	err := a.DB.Where("group_key = ? and version = ?", groupKey, version).First(&app).Error

	return &app, err
}

func (a *AppDaoImpl) GetByEventId(eventId string) (interface{}, error) {
	var app model.AppStatus
	err := a.DB.Where("event_id = ?", eventId).First(&app).Error

	return &app, err
}
