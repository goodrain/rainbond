package handler

import (
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/stretchr/testify/assert"
)

type applicationPortsTestManager struct {
	db.Manager
	tenantServiceDao      dbdao.TenantServiceDao
	tenantServicesPortDao dbdao.TenantServicesPortDao
}

func (m applicationPortsTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.tenantServiceDao
}

func (m applicationPortsTestManager) TenantServicesPortDao() dbdao.TenantServicesPortDao {
	return m.tenantServicesPortDao
}

type applicationPortsTenantServiceDao struct {
	dbdao.TenantServiceDao
	services       []*dbmodel.TenantServices
	err            error
	requestedAppID string
}

func (d *applicationPortsTenantServiceDao) ListByAppID(appID string) ([]*dbmodel.TenantServices, error) {
	d.requestedAppID = appID
	return d.services, d.err
}

type applicationPortsTenantServicesPortDao struct {
	dbdao.TenantServicesPortDao
	ports          []*dbmodel.TenantServicesPort
	err            error
	requestedNames []string
}

func (d *applicationPortsTenantServicesPortDao) ListByK8sServiceNames(names []string) ([]*dbmodel.TenantServicesPort, error) {
	d.requestedNames = append([]string(nil), names...)
	return d.ports, d.err
}

// capability_id: rainbond.application.check-port-k8s-service-name-duplicate
func TestApplicationActionCheckPortsRejectsDuplicateK8sServiceName(t *testing.T) {
	tenantServiceDao := &applicationPortsTenantServiceDao{
		services: []*dbmodel.TenantServices{
			{ServiceID: "service-1"},
			{ServiceID: "service-2"},
		},
	}
	tenantServicesPortDao := &applicationPortsTenantServicesPortDao{
		ports: []*dbmodel.TenantServicesPort{
			{
				ServiceID:      "service-2",
				ContainerPort:  5000,
				K8sServiceName: "shared-service",
			},
		},
	}
	db.SetTestManager(applicationPortsTestManager{
		tenantServiceDao:      tenantServiceDao,
		tenantServicesPortDao: tenantServicesPortDao,
	})
	defer db.SetTestManager(nil)

	action := &ApplicationAction{}
	err := action.checkPorts("app-1", []*apimodel.AppPort{
		{
			ServiceID:      "service-1",
			ContainerPort:  5000,
			PortAlias:      "WEB",
			K8sServiceName: "shared-service",
		},
	})

	assert.Equal(t, "app-1", tenantServiceDao.requestedAppID)
	assert.Equal(t, []string{"shared-service"}, tenantServicesPortDao.requestedNames)
	assert.Error(t, err)
	assert.True(t, bcode.ErrK8sServiceNameExists.Equal(err))
}
