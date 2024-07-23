package dao

import "github.com/goodrain/rainbond/db/model"

func (t *TenantServicesDaoImpl) ListByAppID(appID string) ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Where("app_id=?", appID).Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

func (t *TenantServicesDaoImpl) ListByAppIDs(appID []string) ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Where("app_id in (?)", appID).Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

func (t *TenantServicesDaoImpl) ListComponentIDsByAppID(appID string) ([]string, error) {
	var componentIDs []string
	if err := t.DB.Model(&model.TenantServices{}).Where("app_id=?", appID).Pluck("service_id", &componentIDs).Error; err != nil {
		return nil, err
	}
	return componentIDs, nil
}
