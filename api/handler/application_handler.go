package handler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// ApplicationAction -
type ApplicationAction struct {
	statusCli  *client.AppRuntimeSyncClient
	promClient prometheus.Interface
}

// ApplicationHandler defines handler methods to TenantApplication.
type ApplicationHandler interface {
	CreateApp(req *model.Application) (*model.Application, error)
	BatchCreateApp(req *model.CreateAppRequest, tenantID string) ([]model.CreateAppResponse, error)
	UpdateApp(srcApp *dbmodel.Application, req model.UpdateAppRequest) (*dbmodel.Application, error)
	ListApps(tenantID, appName string, page, pageSize int) (*model.ListAppResponse, error)
	GetAppByID(appID string) (*dbmodel.Application, error)
	BatchBindService(appID string, req model.BindServiceRequest) error
	DeleteApp(appID string) error

	AddConfigGroup(appID string, req *model.ApplicationConfigGroup) (*model.ApplicationConfigGroupResp, error)
	UpdateConfigGroup(appID, configGroupName string, req *model.UpdateAppConfigGroupReq) (*model.ApplicationConfigGroupResp, error)

	BatchUpdateComponentPorts(appID string, ports []*model.AppPort) error
	GetStatus(appID string) (*model.AppStatus, error)

	DeleteConfigGroup(appID, configGroupName string) error
	ListConfigGroups(appID string, page, pageSize int) (*model.ListApplicationConfigGroupResp, error)
	SyncComponents(app *dbmodel.Application, components []*model.Component) error
	SyncComponentConfigGroupRels(tx *gorm.DB, app *dbmodel.Application, components []*model.Component) error
	SyncAppConfigGroups(app *dbmodel.Application, appConfigGroups []model.AppConfigGroup) error
}

// NewApplicationHandler creates a new Tenant Application Handler.
func NewApplicationHandler(statusCli *client.AppRuntimeSyncClient, promClient prometheus.Interface) ApplicationHandler {
	return &ApplicationAction{
		statusCli:  statusCli,
		promClient: promClient,
	}
}

// CreateApp -
func (a *ApplicationAction) CreateApp(req *model.Application) (*model.Application, error) {
	appReq := &dbmodel.Application{
		AppName:  req.AppName,
		AppID:    util.NewUUID(),
		TenantID: req.TenantID,
	}
	req.AppID = appReq.AppID

	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()

	if err := db.GetManager().ApplicationDao().AddModel(appReq); err != nil {
		tx.Rollback()
		return nil, err
	}
	if len(req.ServiceIDs) != 0 {
		if err := db.GetManager().TenantServiceDao().BindAppByServiceIDs(appReq.AppID, req.ServiceIDs); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	return req, nil
}

// BatchCreateApp -
func (a *ApplicationAction) BatchCreateApp(apps *model.CreateAppRequest, tenantID string) ([]model.CreateAppResponse, error) {
	var (
		resp     model.CreateAppResponse
		respList []model.CreateAppResponse
	)
	for _, app := range apps.AppsInfo {
		app.TenantID = tenantID
		regionApp, err := GetApplicationHandler().CreateApp(&app)
		if err != nil {
			logrus.Errorf("Batch Create App [%v] error is [%v] ", app.AppName, err)
			continue
		}
		resp.AppID = app.ConsoleAppID
		resp.RegionAppID = regionApp.AppID
		respList = append(respList, resp)
	}
	return respList, nil
}

// UpdateApp -
func (a *ApplicationAction) UpdateApp(srcApp *dbmodel.Application, req model.UpdateAppRequest) (*dbmodel.Application, error) {
	if req.AppName != "" {
		srcApp.AppName = req.AppName
	}
	if req.GovernanceMode != "" {
		if !dbmodel.IsGovernanceModeValid(req.GovernanceMode) {
			return nil, bcode.NewBadRequest(fmt.Sprintf("governance mode '%s' is valid", req.GovernanceMode))
		}
		srcApp.GovernanceMode = req.GovernanceMode
	}
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

// BatchUpdateComponentPorts -
func (a *ApplicationAction) BatchUpdateComponentPorts(appID string, ports []*model.AppPort) error {
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

// GetStatus -
func (a *ApplicationAction) GetStatus(appID string) (*model.AppStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := a.statusCli.GetAppStatus(ctx, &pb.AppStatusReq{
		AppId: appID,
	})
	if err != nil {
		return nil, err
	}

	diskUsage := a.getDiskUsage(appID)

	res := &model.AppStatus{
		Status: status.Status.String(),
		Cpu:    status.Cpu,
		Memory: status.Memory,
		Disk:   int64(diskUsage),
	}
	return res, nil
}

func (a *ApplicationAction) getDiskUsage(appID string) float64 {
	var result float64
	query := fmt.Sprintf(`sum(max(app_resource_appfs{app_id=~"%s"}) by(app_id))`, appID)
	metric := a.promClient.GetMetric(query, time.Now())
	for _, m := range metric.MetricData.MetricValues {
		result += m.Sample.Value()
	}
	return result
}

// BatchBindService -
func (a *ApplicationAction) BatchBindService(appID string, req model.BindServiceRequest) error {
	var serviceIDs []string
	for _, sid := range req.ServiceIDs {
		if _, err := db.GetManager().TenantServiceDao().GetServiceByID(sid); err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return err
		}
		serviceIDs = append(serviceIDs, sid)
	}
	return db.GetManager().TenantServiceDao().BindAppByServiceIDs(appID, serviceIDs)
}

// SyncComponents -
func (a *ApplicationAction) SyncComponents(app *dbmodel.Application, components []*model.Component) error {
	return db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		if err := GetServiceManager().SyncComponentBase(tx, app, components); err != nil {
			return err
		}
		if err := GetGatewayHandler().SyncHTTPRules(tx, components); err != nil {
			return err
		}
		if err := GetGatewayHandler().SyncTCPRules(tx, components); err != nil {
			return err
		}
		if err := GetServiceManager().SyncComponentMonitors(tx, app, components); err != nil {
			return err
		}
		if err := GetServiceManager().SyncComponentPlugins(tx, app, components); err != nil {
			return err
		}
		if err := GetServiceManager().SyncComponentPorts(tx, app, components); err != nil {
			return err
		}
		if err := GetServiceManager().SyncComponentRelations(tx, app, components); err != nil {
			return err
		}
		if err := GetServiceManager().SyncComponentEnvs(tx, app, components); err != nil {
			return err
		}
		if err := GetServiceManager().SyncComponentVolumeRels(tx, app, components); err != nil {
			return err
		}
		if err := GetServiceManager().SyncComponentVolumes(tx, components); err != nil {
			return err
		}
		if err := GetServiceManager().SyncComponentConfigFiles(tx, components); err != nil {
			return err
		}
		if err := GetServiceManager().SyncComponentProbes(tx, components); err != nil {
			return err
		}
		if err := GetApplicationHandler().SyncComponentConfigGroupRels(tx, app, components); err != nil {
			return err
		}
		if err := GetServiceManager().SyncComponentLabels(tx, components); err != nil {
			return err
		}
		return GetServiceManager().SyncComponentScaleRules(tx, components)
	})
}
