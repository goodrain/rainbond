package dao

import (
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// KeyValueImpl -
type KeyValueImpl struct {
	DB *gorm.DB
}

// Put -
func (k KeyValueImpl) Put(key, value string) error {
	keyValue := dbmodel.KeyValue{K: key, V: value}

	if err := k.DB.Create(&keyValue).Error; err != nil {
		return err
	}

	return nil
}

// Get -
func (k KeyValueImpl) Get(key string) (*dbmodel.KeyValue, error) {
	var keyValue dbmodel.KeyValue
	if err := k.DB.Where("k = ?", key).First(&keyValue).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	return &keyValue, nil
}

// Delete -
func (k KeyValueImpl) Delete(key string) error {
	return k.DB.Where("k = ?", key).Delete(dbmodel.KeyValue{}).Error
}
