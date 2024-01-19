package dao

import (
	"fmt"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// LongVersionDaoImpl lg pack
type LongVersionDaoImpl struct {
	DB *gorm.DB
}

// AddModel add model
func (t *LongVersionDaoImpl) AddModel(mo model.Interface) error {
	version, ok := mo.(*model.EnterpriseLanguageVersion)
	if !ok {
		return fmt.Errorf("mo.(*model.K8sResource) err")
	}
	return t.DB.Create(version).Error
}

// UpdateModel update model
func (t *LongVersionDaoImpl) UpdateModel(mo model.Interface) error {
	version, ok := mo.(*model.EnterpriseLanguageVersion)
	if !ok {
		return fmt.Errorf("mo.(*model.K8sResource) err")
	}
	return t.DB.Save(version).Error
}

// ListVersionByLanguage list by language
func (t *LongVersionDaoImpl) ListVersionByLanguage(language string) ([]*model.EnterpriseLanguageVersion, error) {
	var versions []*model.EnterpriseLanguageVersion
	if err := t.DB.Where("lang = ?", language).Order("version desc").Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

// GetVersionByLanguageAndVersion -
func (t *LongVersionDaoImpl) GetVersionByLanguageAndVersion(language, version string) (*model.EnterpriseLanguageVersion, error) {
	ver := new(model.EnterpriseLanguageVersion)
	if err := t.DB.Where("lang = ? and version = ?", language, version).Find(ver).Error; err != nil {
		return nil, err
	}
	return ver, nil
}

// DefaultLangVersion -
func (t *LongVersionDaoImpl) DefaultLangVersion(lang string, version string) error {
	defaultVersion := new(model.EnterpriseLanguageVersion)
	if err := t.DB.Debug().Where("lang = ? AND first_choice = ?", lang, true).Find(defaultVersion).Error; err != nil {
		return err
	}
	if defaultVersion != nil {
		defaultVersion.FirstChoice = false
		err := t.UpdateModel(defaultVersion)
		if err != nil {
			return err
		}
	}

	ver := new(model.EnterpriseLanguageVersion)
	if err := t.DB.Where("lang = ? and version = ?", lang, version).Find(ver).Error; err != nil {
		return err
	}
	if ver != nil {
		ver.FirstChoice = true
		err := t.UpdateModel(ver)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateLangVersion -
func (t *LongVersionDaoImpl) CreateLangVersion(lang, version, eventID, fileName string) error {
	ver := new(model.EnterpriseLanguageVersion)
	err := t.DB.Where("lang = ? and version = ?", lang, version).Find(ver).Error
	if err == gorm.ErrRecordNotFound {
		if err := t.DB.Create(&model.EnterpriseLanguageVersion{
			Lang:        lang,
			Version:     version,
			FirstChoice: false,
			System:      false,
			EventID:     eventID,
			FileName:    fileName,
		}).Error; err != nil {
			return err
		}
		return nil
	}
	return err
}

// DeleteLangVersion -
func (t *LongVersionDaoImpl) DeleteLangVersion(lang, version string) (string, error) {
	ver := new(model.EnterpriseLanguageVersion)
	if err := t.DB.Where("lang = ? and version = ?", lang, version).Find(ver).Error; err != nil {
		return "", err
	}
	eventID := ver.EventID
	if err := t.DB.Delete(ver).Error; err != nil {
		return "", err
	}
	return eventID, nil
}
