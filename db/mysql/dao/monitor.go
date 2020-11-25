package dao

import (
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

//TenantServiceMonitorDaoImpl -
type TenantServiceMonitorDaoImpl struct {
	DB *gorm.DB
}

//AddModel create service monitor
func (t *TenantServiceMonitorDaoImpl) AddModel(mo model.Interface) error {
	m := mo.(*model.TenantServiceMonitor)
	var oldTSM model.TenantServiceMonitor
	if ok := t.DB.Where("name = ? and tenant_id = ?", m.Name, m.TenantID).Find(&oldTSM).RecordNotFound(); ok {
		if err := t.DB.Create(m).Error; err != nil {
			return err
		}
	} else {
		return bcode.ErrServiceMonitorNameExist
	}
	return nil
}

//UpdateModel update service monitor
func (t *TenantServiceMonitorDaoImpl) UpdateModel(mo model.Interface) error {
	tsm := mo.(*model.TenantServiceMonitor)
	if err := t.DB.Save(tsm).Error; err != nil {
		return err
	}
	return nil
}

//DeleteServiceMonitor delete service monitor
func (t *TenantServiceMonitorDaoImpl) DeleteServiceMonitor(mo *model.TenantServiceMonitor) error {
	if err := t.DB.Delete(mo).Error; err != nil {
		return err
	}
	return nil
}

//DeleteServiceMonitorByServiceID delete service monitor by service id
func (t *TenantServiceMonitorDaoImpl) DeleteServiceMonitorByServiceID(serviceID string) error {
	if err := t.DB.Where("service_id=?", serviceID).Delete(&model.TenantServiceMonitor{}).Error; err != nil {
		return err
	}
	return nil
}

//GetByServiceID get tsm by service id
func (t *TenantServiceMonitorDaoImpl) GetByServiceID(serviceID string) ([]*model.TenantServiceMonitor, error) {
	var tsm []*model.TenantServiceMonitor
	if err := t.DB.Where("service_id=?", serviceID).Find(&tsm).Error; err != nil {
		return nil, err
	}
	return tsm, nil
}

//GetByName get by name
func (t *TenantServiceMonitorDaoImpl) GetByName(serviceID, name string) (*model.TenantServiceMonitor, error) {
	var tsm model.TenantServiceMonitor
	if err := t.DB.Where("service_id=? and name=?", serviceID, name).Find(&tsm).Error; err != nil {
		return nil, err
	}
	return &tsm, nil
}
