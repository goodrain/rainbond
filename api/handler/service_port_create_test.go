package handler

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dberr "github.com/goodrain/rainbond/db/errors"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/require"
)

type portCreateManager struct {
	db.Manager
	database *gorm.DB
	ports    *portCreateDAO
}

func (m portCreateManager) Begin() *gorm.DB                                    { return m.database.Begin() }
func (m portCreateManager) TenantServicesPortDao() dbdao.TenantServicesPortDao { return m.ports }
func (m portCreateManager) TenantServicesPortDaoTransactions(*gorm.DB) dbdao.TenantServicesPortDao {
	return m.ports
}

type portCreateDAO struct {
	dbdao.TenantServicesPortDao
	existing *dbmodel.TenantServicesPort
}

func (d *portCreateDAO) AddModel(m dbmodel.Interface) error {
	if d.existing != nil {
		return dberr.ErrRecordAlreadyExist
	}
	d.existing = m.(*dbmodel.TenantServicesPort)
	return nil
}
func (d *portCreateDAO) GetPort(string, int) (*dbmodel.TenantServicesPort, error) {
	return d.existing, nil
}

type portCreateGateway struct {
	GatewayHandler
	tasks []map[string]interface{}
	fail  bool
}

func (g *portCreateGateway) SendTaskDeprecated(task map[string]interface{}) error {
	g.tasks = append(g.tasks, task)
	if g.fail {
		return errors.New("queue unavailable")
	}
	return nil
}

// capability_id: rainbond.service.new-port-reconciles-access
func TestCreatePortsReconcilesEnabledAccessAndRetriesDispatch(t *testing.T) {
	for _, access := range []string{"inner", "outer", "closed"} {
		t.Run(access, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer sqlDB.Close()
			database, err := gorm.Open("mysql", sqlDB)
			require.NoError(t, err)
			ports := &portCreateDAO{}
			db.SetTestManager(portCreateManager{database: database, ports: ports})
			defer db.SetTestManager(nil)
			gateway := &portCreateGateway{}
			oldGateway := defaultGatewayHandler
			defaultGatewayHandler = gateway
			defer func() { defaultGatewayHandler = oldGateway }()
			request := &apimodel.ServicePorts{Port: []*apimodel.TenantServicesPort{}}
			request.Port = append(request.Port, &apimodel.TenantServicesPort{ContainerPort: 22, Protocol: "tcp", IsInnerService: access == "inner", IsOuterService: access == "outer"})
			mock.ExpectBegin()
			mock.ExpectCommit()
			if access != "closed" {
				gateway.fail = true
			}
			err = (&ServiceAction{}).CreatePorts("tenant", "service", request)
			if access == "closed" {
				require.NoError(t, err)
				require.Empty(t, gateway.tasks)
			} else {
				require.ErrorContains(t, err, "queue unavailable")
				require.Len(t, gateway.tasks, 1)
				require.Equal(t, "port-open", gateway.tasks[0]["action"])
				require.Equal(t, access == "inner", gateway.tasks[0]["is_inner"])
				// The DB row was saved before dispatch failed. The same request can retry.
				mock.ExpectBegin()
				mock.ExpectCommit()
				gateway.fail = false
				require.NoError(t, (&ServiceAction{}).CreatePorts("tenant", "service", request))
				require.Len(t, gateway.tasks, 2)
			}
			// Retrying must never overwrite a different existing port configuration.
			before := len(gateway.tasks)
			request.Port[0].Protocol = "http"
			mock.ExpectBegin()
			mock.ExpectRollback()
			require.Error(t, (&ServiceAction{}).CreatePorts("tenant", "service", request))
			require.Equal(t, before, len(gateway.tasks))
			require.Equal(t, "tcp", ports.existing.Protocol)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
