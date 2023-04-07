package dao

import "github.com/goodrain/rainbond/db/model"

//ListByAppID -
func (t *TenantServicesDaoImpl) ListByAppID(appID string) ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Where("app_id=?", appID).Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

//GetByAppIDComponentName -
func (t *TenantServicesDaoImpl) GetByAppIDComponentName(appID, componentName string) ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Where("app_id = ? and k8s_component_name = ?", appID, componentName).Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}
