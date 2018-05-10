package dao

import (
	"github.com/jinzhu/gorm"
	"github.com/goodrain/rainbond/db/model"
	"github.com/pkg/errors"
)

type AppStatus struct {
	GroupKey  string `gorm:"column:group_key;size:64;primary_key"`
	GroupName string `gorm:"column:group_name;size:64"`
	Version   string `gorm:"column:version;size:32"`
	Format    string `gorm:"column:format;size:32"` // only rainbond-app/docker-compose
	EventID   string `gorm:"column:event_id;size:32"`
	SourceDir string `gorm:"column:source_dir;size:255"`
	Status    string `gorm:"column:status;size:32"` // only exporting/importing/failed/success
	TarFile   string `gorm:"column:tar_file;size:255"`
	TimeStamp int    `gorm:"column:timestamp"`
}

//TableName 表名
func (t *AppStatus) TableName() string {
	return "app_status"
}

type AppDaoImpl struct {
	DB *gorm.DB
}

func (a *AppDaoImpl) AddModel(mo model.Interface) error {
	app, ok := mo.(*AppStatus)
	if !ok {
		return errors.New("Failed to convert interface to AppStatus")
	}

	var old AppStatus
	if ok := a.DB.Where("group_key = ? and version = ?", app.GroupKey, app.Version).Find(&old).RecordNotFound(); ok {
		if err := a.DB.Create(app).Error; err != nil {
			return err
		}
	}

	return nil
}

func (a *AppDaoImpl) UpdateModel(mo model.Interface) error {
	app, ok := mo.(*AppStatus)
	if !ok {
		return errors.New("Failed to convert interface to AppStatus")
	}

	return a.DB.Table(app.TableName()).
		Where("group_key = ? and version = ?", app.GroupKey, app.Version).
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

	var app AppStatus
	return a.DB.Where("group_key = ? and version = ?", groupKey, version).Delete(&app).Error
}

func (a *AppDaoImpl) Get(groupKey, version string) (interface{}, error) {
	var app AppStatus
	err := a.DB.Where("group_key = ? and version = ?", groupKey, version).First(&app).Error

	return &app, err
}
