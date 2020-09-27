package handler

import (
	"fmt"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	"strconv"
)

// ApplicationAction -
type ApplicationAction struct{}

// ApplicationHandler defines handler methods to TenantApplication.
type ApplicationHandler interface {
	CreateApp(req *model.Application) (*model.Application, error)
	UpdateApp(srcApp *dbmodel.Application, req model.UpdateAppRequest) (*dbmodel.Application, error)
	ListApps(tenantID, appName string, page, pageSize int) (*model.ListAppResponse, error)
	GetAppByID(appID string) (*dbmodel.Application, error)
	DeleteApp(appID string) error
	AddConfigGroup(appID string, req *model.ApplicationConfigGroup) (*model.ApplicationConfigGroupResp, error)
}

// NewApplicationHandler creates a new Tenant Application Handler.
func NewApplicationHandler() ApplicationHandler {
	return &ApplicationAction{}
}

// CreateApp -
func (a *ApplicationAction) CreateApp(req *model.Application) (*model.Application, error) {
	appReq := &dbmodel.Application{
		AppName:  req.AppName,
		AppID:    util.NewUUID(),
		TenantID: req.TenantID,
	}
	req.AppID = appReq.AppID
	if err := db.GetManager().ApplicationDao().AddModel(appReq); err != nil {
		return nil, err
	}
	return req, nil
}

// UpdateApp -
func (a *ApplicationAction) UpdateApp(srcApp *dbmodel.Application, req model.UpdateAppRequest) (*dbmodel.Application, error) {
	srcApp.AppName = req.AppName
	if err := db.GetManager().ApplicationDao().UpdateModel(srcApp); err != nil {
		return nil, err
	}
	return srcApp, nil
}

// ListApps -
func (a *ApplicationAction) ListApps(tenantID, appName string, page, pageSize int) (*model.ListAppResponse, error) {
	var resp model.ListAppResponse
	apps, total, err := db.GetManager().ApplicationDao().ListApps(tenantID, appName, page, pageSize)
	if err != nil {
		return nil, err
	}
	if apps != nil {
		resp.Apps = apps
	} else {
		resp.Apps = make([]*dbmodel.Application, 0)
	}

	resp.Page = page
	resp.Total = total
	resp.PageSize = pageSize
	return &resp, nil
}

// GetAppByID -
func (a *ApplicationAction) GetAppByID(appID string) (*dbmodel.Application, error) {
	app, err := db.GetManager().ApplicationDao().GetAppByID(appID)
	if err != nil {
		return nil, err
	}
	return app, nil
}

// DeleteApp -
func (a *ApplicationAction) DeleteApp(appID string) error {
	// Get the number of services under the application
	total, err := db.GetManager().TenantServiceDao().CountServiceByAppID(appID)
	if err != nil {
		return err
	}
	if total != 0 {
		return bcode.ErrDeleteDueToBindService
	}
	return db.GetManager().ApplicationDao().DeleteApp(appID)
}

func (a *ApplicationAction) UpdatePorts(appID string, ports []*model.AppPort) error {
	if err := a.checkPorts(appID, ports); err != nil {
		return err
	}

	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()

	// update port
	for _, p := range ports {
		port, err := db.GetManager().TenantServicesPortDaoTransactions(tx).GetPort(p.ServiceID, p.ContainerPort)
		if err != nil {
			tx.Rollback()
			return err
		}
		port.PortAlias = p.PortAlias
		port.K8sServiceName = p.K8sServiceName
		err = db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(port)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func (a *ApplicationAction) checkPorts(appID string, ports []*model.AppPort) error {
	// check if the ports are belong to the given appID
	services, err := db.GetManager().TenantServiceDao().ListByAppID(appID)
	if err != nil {
		return err
	}
	set := make(map[string]struct{})
	for _, svc := range services {
		set[svc.ServiceID] = struct{}{}
	}
	var k8sServiceNames []string
	key2ports := make(map[string]*model.AppPort)
	for i := range ports {
		port := ports[i]
		if _, ok := set[port.ServiceID]; !ok {
			return bcode.NewBadRequest(fmt.Sprintf("port(%s) is not belong to app(%s)", port.ServiceID, appID))
		}
		k8sServiceNames = append(k8sServiceNames, port.ServiceID)
		key2ports[port.ServiceID+strconv.Itoa(port.ContainerPort)] = port
	}

	// check if k8s_service_name is unique
	servicesPorts, err := db.GetManager().TenantServicesPortDao().ListByK8sServiceNames(k8sServiceNames)
	if err != nil {
		return err
	}
	for _, port := range servicesPorts {
		// check if the port is as same as the one in request
		if _, ok := key2ports[port.ServiceID+strconv.Itoa(port.ContainerPort)]; !ok {
			logrus.Errorf("kubernetes service name(%s) already exists", port.K8sServiceName)
			return bcode.ErrK8sServiceNameExists
		}
	}

	return nil
}
