package dao

import (
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

func pluginVersionScopes(version *model.TenantPluginBuildVersion) []string {
	var scopes []string
	for _, image := range []string{version.BaseImage, version.BuildLocalImage} {
		if image != "" {
			scopes = append(scopes, guard.ReferenceScopesForImage(image)...)
		}
	}
	return scopes
}

// AddModel protects every image reference created by a plugin build record.
func (t *PluginBuildVersionDaoImpl) AddModel(mo model.Interface) error {
	version := mo.(*model.TenantPluginBuildVersion)
	return guard.WithReferenceMutation(t.DB, pluginVersionScopes(version), func(tx *gorm.DB) error {
		return (&PluginBuildVersionDaoImpl{DB: tx}).addBuildVersionRecord(version)
	})
}

// UpdateModel updates only an existing immutable plugin version identity. Late
// callbacks must not turn GORM Save's insert fallback into reference creation.
func (t *PluginBuildVersionDaoImpl) UpdateModel(mo model.Interface) error {
	version := mo.(*model.TenantPluginBuildVersion)
	if version.ID == 0 || version.PluginID == "" || version.VersionID == "" || version.DeployVersion == "" {
		return gorm.ErrRecordNotFound
	}
	return guard.WithReferenceMutation(t.DB, pluginVersionScopes(version), func(tx *gorm.DB) error {
		changes := map[string]interface{}{}
		for _, field := range tx.NewScope(version).Fields() {
			if !field.IsNormal || field.IsPrimaryKey {
				continue
			}
			switch field.DBName {
			case "plugin_id", "version_id", "deploy_version":
				continue
			}
			changes[field.DBName] = field.Field.Interface()
		}
		query := "ID = ? AND plugin_id = ? AND version_id = ? AND deploy_version = ?"
		args := []interface{}{version.ID, version.PluginID, version.VersionID, version.DeployVersion}
		result := tx.Model(&model.TenantPluginBuildVersion{}).Where(query, args...).Updates(changes)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var count int
			if err := tx.Model(&model.TenantPluginBuildVersion{}).Where(query, args...).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
}
