package dao

import (
	"fmt"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"strings"
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
func (t *LongVersionDaoImpl) ListVersionByLanguage(language string, show string) ([]*model.EnterpriseLanguageVersion, error) {
	var versions []*model.EnterpriseLanguageVersion

	query := t.DB.Where("lang = ?", language).Where("(build_strategy = ? OR build_strategy = '' OR build_strategy IS NULL)", model.LongVersionBuildStrategySlug)
	if show != "" {
		query = query.Where("is_show = ?", true)
	}
	if err := query.Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

// ListVersionByLanguageAndStrategy list by language and strategy
func (t *LongVersionDaoImpl) ListVersionByLanguageAndStrategy(language string, show string, buildStrategy string) ([]*model.EnterpriseLanguageVersion, error) {
	var versions []*model.EnterpriseLanguageVersion
	query := t.DB.Where("lang = ? and build_strategy = ?", language, normalizeLongVersionBuildStrategy(buildStrategy))
	if show != "" {
		query = query.Where("is_show = ?", true)
	}
	if err := query.Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

// GetVersionByLanguageAndVersion -
func (t *LongVersionDaoImpl) GetVersionByLanguageAndVersion(language, version string) (*model.EnterpriseLanguageVersion, error) {
	ver := new(model.EnterpriseLanguageVersion)
	if err := t.DB.Where("lang = ? and version = ?", language, version).Where("(build_strategy = ? OR build_strategy = '' OR build_strategy IS NULL)", model.LongVersionBuildStrategySlug).First(ver).Error; err != nil {
		return nil, err
	}
	return ver, nil
}

// GetVersionByLanguageAndVersionAndStrategy -
func (t *LongVersionDaoImpl) GetVersionByLanguageAndVersionAndStrategy(language, version, buildStrategy string) (*model.EnterpriseLanguageVersion, error) {
	ver := new(model.EnterpriseLanguageVersion)
	if err := t.DB.Where("lang = ? and version = ? and build_strategy = ?", language, version, normalizeLongVersionBuildStrategy(buildStrategy)).First(ver).Error; err != nil {
		return nil, err
	}
	return ver, nil
}

// GetDefaultVersionByLanguageAndVersion -
func (t *LongVersionDaoImpl) GetDefaultVersionByLanguageAndVersion(language string) (*model.EnterpriseLanguageVersion, error) {
	ver := new(model.EnterpriseLanguageVersion)
	if err := t.DB.Where("lang = ? and first_choice = ?", language, true).Where("(build_strategy = ? OR build_strategy = '' OR build_strategy IS NULL)", model.LongVersionBuildStrategySlug).First(ver).Error; err != nil {
		return nil, err
	}
	return ver, nil
}

// GetDefaultVersionByLanguageAndStrategy -
func (t *LongVersionDaoImpl) GetDefaultVersionByLanguageAndStrategy(language, buildStrategy string) (*model.EnterpriseLanguageVersion, error) {
	ver := new(model.EnterpriseLanguageVersion)
	if err := t.DB.Where("lang = ? and first_choice = ? and build_strategy = ?", language, true, normalizeLongVersionBuildStrategy(buildStrategy)).First(ver).Error; err != nil {
		return nil, err
	}
	return ver, nil
}

// DefaultLangVersion -
func (t *LongVersionDaoImpl) DefaultLangVersion(lang string, version string, buildStrategy string, show bool, firstChoice bool, isAllowed *bool) (*model.EnterpriseLanguageVersion, error) {
	buildStrategy = normalizeLongVersionBuildStrategy(buildStrategy)
	if firstChoice {
		defaultVersion := new(model.EnterpriseLanguageVersion)
		if err := t.DB.Where("lang = ? AND build_strategy = ? AND first_choice = ?", lang, buildStrategy, true).First(defaultVersion).Error; err != nil && err != gorm.ErrRecordNotFound {
			return nil, err
		}
		if defaultVersion.ID > 0 && defaultVersion.Version != version {
			defaultVersion.FirstChoice = false
			if err := t.UpdateModel(defaultVersion); err != nil {
				return nil, err
			}
		}
	}
	ver, err := t.GetVersionByLanguageAndVersionAndStrategy(lang, version, buildStrategy)
	if err != nil {
		return nil, err
	}
	ver.FirstChoice = firstChoice
	ver.Show = show
	if isAllowed != nil {
		ver.IsAllowed = *isAllowed
	}
	if err := t.UpdateModel(ver); err != nil {
		return nil, err
	}
	return ver, nil
}

// CreateLangVersion -
func (t *LongVersionDaoImpl) CreateLangVersion(lang, version, eventID, fileName, buildStrategy string, show bool, isAllowed bool) (*model.EnterpriseLanguageVersion, error) {
	buildStrategy = normalizeLongVersionBuildStrategy(buildStrategy)
	ver, err := t.GetVersionByLanguageAndVersionAndStrategy(lang, version, buildStrategy)
	if err == nil {
		return ver, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	ver = &model.EnterpriseLanguageVersion{
		Lang:          lang,
		Show:          show,
		Version:       version,
		BuildStrategy: buildStrategy,
		FirstChoice:   false,
		System:        false,
		EventID:       eventID,
		FileName:      fileName,
		IsAllowed:     isAllowed,
	}
	if err := t.DB.Create(ver).Error; err != nil {
		return nil, err
	}
	return ver, nil
}

// DeleteLangVersion -
func (t *LongVersionDaoImpl) DeleteLangVersion(lang, version, buildStrategy string) (string, error) {
	ver := new(model.EnterpriseLanguageVersion)
	if err := t.DB.Where("lang = ? and version = ? and build_strategy = ?", lang, version, normalizeLongVersionBuildStrategy(buildStrategy)).First(ver).Error; err != nil {
		return "", err
	}
	eventID := ver.EventID
	if err := t.DB.Delete(ver).Error; err != nil {
		return "", err
	}
	return eventID, nil
}

func normalizeLongVersionBuildStrategy(buildStrategy string) string {
	buildStrategy = strings.TrimSpace(buildStrategy)
	if buildStrategy == "" {
		return model.LongVersionBuildStrategySlug
	}
	return buildStrategy
}
