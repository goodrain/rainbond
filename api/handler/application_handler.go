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
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	util "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/commonutil"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// ApplicationAction -
type ApplicationAction struct {
	statusCli      *client.AppRuntimeSyncClient
	promClient     prometheus.Interface
	rainbondClient versioned.Interface
	kubeClient     clientset.Interface
}

// ApplicationHandler defines handler methods to TenantApplication.
type ApplicationHandler interface {
	CreateApp(ctx context.Context, req *model.Application) (*model.Application, error)
	BatchCreateApp(ctx context.Context, req *model.CreateAppRequest, tenantID string) ([]model.CreateAppResponse, error)
	UpdateApp(srcApp *dbmodel.Application, req model.UpdateAppRequest) (*dbmodel.Application, error)
	ListApps(tenantID, appName string, page, pageSize int) (*model.ListAppResponse, error)
	GetAppByID(appID string) (*dbmodel.Application, error)
	BatchBindService(appID string, req model.BindServiceRequest) error
	DeleteApp(appID string) error

	AddConfigGroup(appID string, req *model.ApplicationConfigGroup) (*model.ApplicationConfigGroupResp, error)
	UpdateConfigGroup(appID, configGroupName string, req *model.UpdateAppConfigGroupReq) (*model.ApplicationConfigGroupResp, error)

	BatchUpdateComponentPorts(appID string, ports []*model.AppPort) error
	GetStatus(ctx context.Context, app *dbmodel.Application) (*model.AppStatus, error)
	GetDetectProcess(ctx context.Context, app *dbmodel.Application) ([]*model.AppDetectProcess, error)
	Install(ctx context.Context, app *dbmodel.Application, values string) ([]*pb.AppService, error)
	ListServices(ctx context.Context, app *dbmodel.Application) ([]*model.AppService, error)

	DeleteConfigGroup(appID, configGroupName string) error
	ListConfigGroups(appID string, page, pageSize int) (*model.ListApplicationConfigGroupResp, error)
}

// NewApplicationHandler creates a new Tenant Application Handler.
func NewApplicationHandler(statusCli *client.AppRuntimeSyncClient, promClient prometheus.Interface, rainbondClient versioned.Interface, kubeClient clientset.Interface) ApplicationHandler {
	return &ApplicationAction{
		statusCli:      statusCli,
		promClient:     promClient,
		rainbondClient: rainbondClient,
		kubeClient:     kubeClient,
	}
}

// CreateApp -
func (a *ApplicationAction) CreateApp(ctx context.Context, req *model.Application) (*model.Application, error) {
	appReq := &dbmodel.Application{
		EID:             req.EID,
		TenantID:        req.TenantID,
		AppID:           util.NewUUID(),
		AppName:         req.AppName,
		AppType:         req.AppType,
		AppStoreName:    req.AppStoreName,
		AppStoreURL:     req.AppStoreURL,
		AppTemplateName: req.AppTemplateName,
		Version:         req.Version,
	}
	req.AppID = appReq.AppID

	err := db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		if err := db.GetManager().ApplicationDaoTransactions(tx).AddModel(appReq); err != nil {
			return err
		}
		if len(req.ServiceIDs) != 0 {
			if err := db.GetManager().TenantServiceDaoTransactions(tx).BindAppByServiceIDs(appReq.AppID, req.ServiceIDs); err != nil {
				return err
			}
		}

		// create helmapp.rainbond.goodrain.io
		return a.createHelmApp(ctx, appReq)
	})

	return req, err
}

func (a *ApplicationAction) createHelmApp(ctx context.Context, app *dbmodel.Application) error {
	helmApp := &v1alpha1.HelmApp{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.AppName,
			Namespace: app.TenantID,
			// TODO: rainbond labels.
		},
		Spec: v1alpha1.HelmAppSpec{
			EID:          app.EID,
			TemplateName: app.AppTemplateName,
			Version:      app.Version,
			Revision:     commonutil.Int32(0),
			AppStore: &v1alpha1.HelmAppStore{
				Version: "", // TODO: setup version.
				Name:    app.AppStoreName,
				URL:     app.AppStoreURL,
			},
		}}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := a.rainbondClient.RainbondV1alpha1().HelmApps(helmApp.Namespace).Create(ctx, helmApp, metav1.CreateOptions{})
	if k8sErrors.IsAlreadyExists(err) {
		return errors.Wrap(bcode.ErrApplicationExist, "create helm app")
	}
	return err
}

// BatchCreateApp -
func (a *ApplicationAction) BatchCreateApp(ctx context.Context, apps *model.CreateAppRequest, tenantID string) ([]model.CreateAppResponse, error) {
	var (
		resp     model.CreateAppResponse
		respList []model.CreateAppResponse
	)
	for _, app := range apps.AppsInfo {
		app.TenantID = tenantID
		regionApp, err := GetApplicationHandler().CreateApp(ctx, &app)
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
func (a *ApplicationAction) GetStatus(ctx context.Context, app *dbmodel.Application) (*model.AppStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	status, err := a.statusCli.GetAppStatus(ctx, &pb.AppStatusReq{
		AppId: app.AppID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "get app status")
	}

	diskUsage := a.getDiskUsage(app.AppID)

	res := &model.AppStatus{
		Status:         status.Status,
		Cpu:            status.Cpu,
		Memory:         status.Memory,
		Disk:           int64(diskUsage),
		Phase:          status.Phase,
		ValuesTemplate: status.ValuesTemplate,
		Readme:         status.Readme,
	}
	return res, nil
}

func (a *ApplicationAction) GetDetectProcess(ctx context.Context, app *dbmodel.Application) ([]*model.AppDetectProcess, error) {
	nctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	res, err := a.statusCli.ListHelmAppDetectConditions(nctx, &pb.AppReq{
		AppId: app.AppID,
	})
	if err != nil {
		return nil, err
	}

	var conditions []*model.AppDetectProcess
	for _, condition := range res.Conditions {
		conditions = append(conditions, &model.AppDetectProcess{
			Type:  condition.Type,
			Ready: condition.Ready,
			Error: condition.Error,
		})
	}

	return conditions, nil
}

func (a *ApplicationAction) Install(ctx context.Context, app *dbmodel.Application, values string) ([]*pb.AppService, error) {
	ctx1, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	helmApp, err := a.rainbondClient.RainbondV1alpha1().HelmApps(app.TenantID).Get(ctx1, app.AppName, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, errors.Wrap(bcode.ErrApplicationNotFound, "install app")
		}
		return nil, errors.Wrap(err, "install app")
	}

	//ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	//defer cancel()
	//var services []*pb.AppService
	//appServices, err := a.statusCli.ParseAppServices(ctx2, &pb.ParseAppServicesReq{
	//	AppID:  app.AppID,
	//	Values: values,
	//})
	//if err != nil {
	//	logrus.Warningf("[ApplicationAction] [Install] parse services: %v", err)
	//} else {
	//	services = appServices.Services
	//}

	ctx3, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	helmApp.Spec.Values = values
	_, err = a.rainbondClient.RainbondV1alpha1().HelmApps(app.TenantID).Update(ctx3, helmApp, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return nil, errors.Wrap(err, "install app")
}

func (a *ApplicationAction) ListServices(ctx context.Context, app *dbmodel.Application) ([]*model.AppService, error) {
	nctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	appServices, err := a.statusCli.ListAppServices(nctx, &pb.AppReq{AppId: app.AppID})
	if err != nil {
		return nil, err
	}

	var services []*model.AppService
	for _, service := range appServices.Services {
		svc := &model.AppService{
			ServiceName: service.Name,
		}

		var pods []*model.AppPod
		for _, pod := range service.Pods {
			pods = append(pods, &model.AppPod{
				PodName:   pod.Name,
				PodStatus: pod.Status,
			})
		}
		svc.Pods = pods

		for _, port := range service.TcpPorts {
			svc.TCPPorts = append(svc.TCPPorts, port)
		}
		for _, port := range service.UdpPorts {
			svc.UDPPorts = append(svc.UDPPorts, port)
		}

		services = append(services, svc)
	}

	return services, nil
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
	for _, sid := range req.ServiceIDs {
		if _, err := db.GetManager().TenantServiceDao().GetServiceByID(sid); err != nil {
			return err
		}
	}
	if err := db.GetManager().TenantServiceDao().BindAppByServiceIDs(appID, req.ServiceIDs); err != nil {
		return err
	}
	return nil
}
