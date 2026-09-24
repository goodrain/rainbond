package handler

import (
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/stretchr/testify/require"
)

type portOuterTestManager struct {
	db.Manager
	portDao           dbdao.TenantServicesPortDao
	serviceDao        dbdao.TenantServiceDao
	pluginRelationDao dbdao.TenantServicePluginRelationDao
}

func (m portOuterTestManager) TenantServicesPortDao() dbdao.TenantServicesPortDao {
	return m.portDao
}

func (m portOuterTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.serviceDao
}

func (m portOuterTestManager) TenantServicePluginRelationDao() dbdao.TenantServicePluginRelationDao {
	return m.pluginRelationDao
}

type portOuterPortDao struct {
	dbdao.TenantServicesPortDao
	port *dbmodel.TenantServicesPort
}

func (d *portOuterPortDao) GetPort(string, int) (*dbmodel.TenantServicesPort, error) {
	return d.port, nil
}

type portOuterServiceDao struct {
	dbdao.TenantServiceDao
	service *dbmodel.TenantServices
}

func (d *portOuterServiceDao) GetServiceByID(string) (*dbmodel.TenantServices, error) {
	return d.service, nil
}

type portOuterPluginRelationDao struct {
	dbdao.TenantServicePluginRelationDao
}

func (d *portOuterPluginRelationDao) CheckSomeModelPluginByServiceID(string, string) (bool, error) {
	return false, nil
}

// capability_id: rainbond.service.port-outer-idempotent-close
func TestServiceActionPortOuterAlreadyClosedReturnsPort(t *testing.T) {
	isOuter := false
	port := &dbmodel.TenantServicesPort{
		ServiceID:      "service-id",
		ContainerPort:  8080,
		IsOuterService: &isOuter,
	}
	db.SetTestManager(portOuterTestManager{
		portDao:           &portOuterPortDao{port: port},
		serviceDao:        &portOuterServiceDao{service: &dbmodel.TenantServices{ServiceID: "service-id"}},
		pluginRelationDao: &portOuterPluginRelationDao{},
	})
	defer db.SetTestManager(nil)

	request := &apimodel.ServicePortInnerOrOuter{}
	request.Body.Operation = "close"

	got, _, err := (&ServiceAction{}).PortOuter("tenant", "service-id", 8080, request)

	require.NoError(t, err)
	require.NotNil(t, got)
}
