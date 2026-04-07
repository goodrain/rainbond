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

	query := orderLongVersionQuery(t.DB.Where("lang = ?", language).Where("(build_strategy = ? OR build_strategy = '' OR build_strategy IS NULL)", model.LongVersionBuildStrategySlug))
	if show != "" {
		query = query.Where("is_show = ?", true)
	}
	if err := query.Find(&versions).Error; err != nil {
		return nil, err
	}
	return deduplicateListedLanguageVersions(versions), nil
}

// ListVersionByLanguageAndStrategy list by language and strategy
func (t *LongVersionDaoImpl) ListVersionByLanguageAndStrategy(language string, show string, buildStrategy string) ([]*model.EnterpriseLanguageVersion, error) {
	var versions []*model.EnterpriseLanguageVersion
	query := orderLongVersionQuery(t.DB.Where("lang = ? and build_strategy = ?", language, normalizeLongVersionBuildStrategy(buildStrategy)))
	if show != "" {
		query = query.Where("is_show = ?", true)
	}
	if err := query.Find(&versions).Error; err != nil {
		return nil, err
	}
	return deduplicateListedLanguageVersions(versions), nil
}

// GetVersionByLanguageAndVersion -
func (t *LongVersionDaoImpl) GetVersionByLanguageAndVersion(language, version string) (*model.EnterpriseLanguageVersion, error) {
	ver := new(model.EnterpriseLanguageVersion)
	if err := orderLongVersionQuery(t.DB.Where("lang = ? and version = ?", language, version).Where("(build_strategy = ? OR build_strategy = '' OR build_strategy IS NULL)", model.LongVersionBuildStrategySlug)).First(ver).Error; err != nil {
		return nil, err
	}
	ver.BuildStrategy = normalizeLongVersionBuildStrategy(ver.BuildStrategy)
	return ver, nil
}

// GetVersionByLanguageAndVersionAndStrategy -
func (t *LongVersionDaoImpl) GetVersionByLanguageAndVersionAndStrategy(language, version, buildStrategy string) (*model.EnterpriseLanguageVersion, error) {
	ver := new(model.EnterpriseLanguageVersion)
	if err := orderLongVersionQuery(t.DB.Where("lang = ? and version = ? and build_strategy = ?", language, version, normalizeLongVersionBuildStrategy(buildStrategy))).First(ver).Error; err != nil {
		return nil, err
	}
	return ver, nil
}

// GetDefaultVersionByLanguageAndVersion -
func (t *LongVersionDaoImpl) GetDefaultVersionByLanguageAndVersion(language string) (*model.EnterpriseLanguageVersion, error) {
	ver := new(model.EnterpriseLanguageVersion)
	if err := orderLongVersionQuery(t.DB.Where("lang = ? and first_choice = ?", language, true).Where("(build_strategy = ? OR build_strategy = '' OR build_strategy IS NULL)", model.LongVersionBuildStrategySlug)).First(ver).Error; err != nil {
		return nil, err
	}
	ver.BuildStrategy = normalizeLongVersionBuildStrategy(ver.BuildStrategy)
	return ver, nil
}

// GetDefaultVersionByLanguageAndStrategy -
func (t *LongVersionDaoImpl) GetDefaultVersionByLanguageAndStrategy(language, buildStrategy string) (*model.EnterpriseLanguageVersion, error) {
	ver := new(model.EnterpriseLanguageVersion)
	if err := orderLongVersionQuery(t.DB.Where("lang = ? and first_choice = ? and build_strategy = ?", language, true, normalizeLongVersionBuildStrategy(buildStrategy))).First(ver).Error; err != nil {
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

func orderLongVersionQuery(query *gorm.DB) *gorm.DB {
	for _, clause := range longVersionOrderClauses(query) {
		query = query.Order(clause)
	}
	return query
}

func longVersionOrderClauses(db *gorm.DB) []string {
	scope := db.NewScope(&model.EnterpriseLanguageVersion{})
	return []string{
		scope.Quote("first_choice") + " DESC",
		scope.Quote("is_show") + " DESC",
		scope.Quote("is_allowed") + " DESC",
		scope.Quote("system") + " DESC",
		scope.Quote("ID") + " ASC",
	}
}

func deduplicateListedLanguageVersions(versions []*model.EnterpriseLanguageVersion) []*model.EnterpriseLanguageVersion {
	if len(versions) < 2 {
		if len(versions) == 1 && versions[0] != nil {
			versions[0].BuildStrategy = normalizeLongVersionBuildStrategy(versions[0].BuildStrategy)
		}
		return versions
	}

	result := make([]*model.EnterpriseLanguageVersion, 0, len(versions))
	seen := make(map[string]*model.EnterpriseLanguageVersion, len(versions))
	for _, version := range versions {
		if version == nil {
			continue
		}
		version.BuildStrategy = normalizeLongVersionBuildStrategy(version.BuildStrategy)
		key := buildLongVersionDedupKey(version.Lang, version.Version, version.BuildStrategy)
		if existing, ok := seen[key]; ok {
			mergeListedLanguageVersion(existing, version)
			continue
		}
		seen[key] = version
		result = append(result, version)
	}
	return result
}

func buildLongVersionDedupKey(lang, version, buildStrategy string) string {
	return lang + "\x00" + version + "\x00" + normalizeLongVersionBuildStrategy(buildStrategy)
}

func mergeListedLanguageVersion(target, source *model.EnterpriseLanguageVersion) {
	if source.FirstChoice {
		target.FirstChoice = true
	}
	if source.Show {
		target.Show = true
	}
	if source.System {
		target.System = true
	}
	if source.IsAllowed {
		target.IsAllowed = true
	}
	if target.EventID == "" && source.EventID != "" {
		target.EventID = source.EventID
	}
	if target.FileName == "" && source.FileName != "" {
		target.FileName = source.FileName
	}
}
