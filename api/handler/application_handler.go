package handler

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond/api/handler/app_governance_mode/adaptor"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"os"
	"sigs.k8s.io/yaml"
	"sort"
	"strconv"
	"strings"
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
	"github.com/goodrain/rainbond/util/constants"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
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
	dynamicClient  dynamic.Interface
}

// ApplicationHandler defines handler methods to TenantApplication.
type ApplicationHandler interface {
	CreateApp(ctx context.Context, req *model.Application) (*model.Application, error)
	BatchCreateApp(ctx context.Context, req *model.CreateAppRequest, tenantID string) ([]model.CreateAppResponse, error)
	UpdateApp(ctx context.Context, app *dbmodel.Application, req model.UpdateAppRequest) (*dbmodel.Application, error)
	ListApps(tenantID, appName string, page, pageSize int) (*model.ListAppResponse, error)
	GetAppByID(appID string) (*dbmodel.Application, error)
	BatchBindService(appID string, req model.BindServiceRequest) error
	DeleteApp(ctx context.Context, app *dbmodel.Application) error

	AddConfigGroup(appID string, req *model.ApplicationConfigGroup) (*model.ApplicationConfigGroupResp, error)
	UpdateConfigGroup(appID, configGroupName string, req *model.UpdateAppConfigGroupReq) (*model.ApplicationConfigGroupResp, error)

	BatchUpdateComponentPorts(appID string, ports []*model.AppPort) error
	GetStatus(ctx context.Context, app *dbmodel.Application) (*model.AppStatus, error)
	Install(ctx context.Context, app *dbmodel.Application, overrides []string) error
	ListServices(ctx context.Context, app *dbmodel.Application) ([]*model.AppService, error)
	ListHelmAppReleases(ctx context.Context, app *dbmodel.Application) ([]*model.HelmAppRelease, error)

	DeleteConfigGroup(appID, configGroupName string) error
	ListConfigGroups(appID string, page, pageSize int) (*model.ListApplicationConfigGroupResp, error)
	SyncComponents(app *dbmodel.Application, components []*model.Component, deleteComponentIDs []string) error
	SyncComponentConfigGroupRels(tx *gorm.DB, app *dbmodel.Application, components []*model.Component) error
	SyncAppConfigGroups(app *dbmodel.Application, appConfigGroups []model.AppConfigGroup) error
	ListAppStatuses(ctx context.Context, appIDs []string) ([]*model.AppStatus, error)
	ListGovernanceMode() ([]model.GovernanceMode, error)
	CheckGovernanceMode(ctx context.Context, governanceMode string) error
	GetAndHandleOperatorManaged(appID string) (*pb.OperatorManaged, error)
	CreateServiceMeshCR(app *dbmodel.Application, governance string) (content string, err error)
	UpdateServiceMeshCR(app *dbmodel.Application, governance string) (content string, err error)
	DeleteServiceMeshCR(app *dbmodel.Application) error
	ChangeVolumes(app *dbmodel.Application) error
}

// NewApplicationHandler creates a new Tenant Application Handler.
func NewApplicationHandler(statusCli *client.AppRuntimeSyncClient, promClient prometheus.Interface, rainbondClient versioned.Interface, kubeClient clientset.Interface, dynamicClient dynamic.Interface) ApplicationHandler {
	return &ApplicationAction{
		statusCli:      statusCli,
		promClient:     promClient,
		rainbondClient: rainbondClient,
		kubeClient:     kubeClient,
		dynamicClient:  dynamicClient,
	}
}

// CreateApp -
func (a *ApplicationAction) CreateApp(ctx context.Context, req *model.Application) (*model.Application, error) {
	appID := util.NewUUID()
	if req.K8sApp == "" {
		req.K8sApp = fmt.Sprintf("app-%s", appID[:8])
	}
	appReq := &dbmodel.Application{
		EID:             req.EID,
		TenantID:        req.TenantID,
		AppID:           appID,
		AppName:         req.AppName,
		AppType:         req.AppType,
		AppStoreName:    req.AppStoreName,
		AppStoreURL:     req.AppStoreURL,
		AppTemplateName: req.AppTemplateName,
		Version:         req.Version,
		K8sApp:          req.K8sApp,
	}
	req.AppID = appReq.AppID

	err := db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		if db.GetManager().ApplicationDaoTransactions(tx).IsK8sAppDuplicate(appReq.TenantID, appID, appReq.K8sApp) {
			return bcode.ErrK8sAppExists
		}
		if err := db.GetManager().ApplicationDaoTransactions(tx).AddModel(appReq); err != nil {
			return err
		}
		if len(req.ServiceIDs) != 0 {
			if err := db.GetManager().TenantServiceDaoTransactions(tx).BindAppByServiceIDs(appReq.AppID, req.ServiceIDs); err != nil {
				return err
			}
		}

		if appReq.AppType == model.AppTypeHelm {
			// create helmapp.rainbond.io
			return a.createHelmApp(ctx, appReq)
		}
		return nil
	})

	return req, err
}

func (a *ApplicationAction) createHelmApp(ctx context.Context, app *dbmodel.Application) error {
	labels := map[string]string{
		constants.ResourceManagedByLabel: constants.Rainbond,
	}
	tenant, err := GetTenantManager().GetTenantsByUUID(app.TenantID)
	if err != nil {
		return errors.Wrap(err, "get tenant for helm app failed")
	}
	helmApp := &v1alpha1.HelmApp{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.AppName,
			Namespace: tenant.Namespace,
			Labels:    labels,
		},
		Spec: v1alpha1.HelmAppSpec{
			EID:          app.EID,
			TemplateName: app.AppTemplateName,
			Version:      app.Version,
			AppStore: &v1alpha1.HelmAppStore{
				Name: app.AppStoreName,
				URL:  app.AppStoreURL,
			},
		}}
	ctx1, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = a.kubeClient.CoreV1().Namespaces().Create(ctx1, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   tenant.Namespace,
			Labels: labels,
		},
	}, metav1.CreateOptions{})
	if err != nil && !k8sErrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "create namespace for helm app")
	}

	ctx2, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = a.rainbondClient.RainbondV1alpha1().HelmApps(helmApp.Namespace).Create(ctx2, helmApp, metav1.CreateOptions{})
	if err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			return errors.Wrap(bcode.ErrApplicationExist, "create helm app")
		}
		return errors.Wrap(err, "create helm app")
	}
	return nil
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
func (a *ApplicationAction) UpdateApp(ctx context.Context, app *dbmodel.Application, req model.UpdateAppRequest) (*dbmodel.Application, error) {
	if req.AppName != "" {
		app.AppName = req.AppName
	}
	if req.GovernanceMode != "" {
		if !adaptor.IsGovernanceModeValid(req.GovernanceMode, a.dynamicClient) {
			return nil, bcode.ErrInvalidGovernanceMode
		}
		app.GovernanceMode = req.GovernanceMode
	}
	app.K8sApp = req.K8sApp

	err := db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		if db.GetManager().ApplicationDaoTransactions(tx).IsK8sAppDuplicate(app.TenantID, app.AppID, req.K8sApp) {
			return bcode.ErrK8sAppExists
		}
		if err := db.GetManager().ApplicationDaoTransactions(tx).UpdateModel(app); err != nil {
			return err
		}
		if req.NeedUpdateHelmApp() {
			if err := a.updateHelmApp(ctx, app, req); err != nil {
				return err
			}
		}

		return nil
	})

	return app, err
}

func (a *ApplicationAction) generateServiceMeshObj(app *dbmodel.Application, governance string, team *dbmodel.Tenants) *unstructured.Unstructured {
	desired := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rainbond.io/v1alpha1",
			"kind":       "ServiceMesh",
			"metadata": map[string]interface{}{
				"namespace": team.Namespace,
				"name":      app.K8sApp,
				"labels": map[string]interface{}{
					"rainbond.io/app": app.K8sApp,
					"rainbond_app":    app.K8sApp,
					"app_id":          app.AppID,
				},
			},
			"provisioner": governance,
			"selector": map[string]interface{}{
				"app_id": app.AppID,
			},
		},
	}
	return desired
}

// CreateServiceMeshCR create service mesh custom resources
func (a *ApplicationAction) CreateServiceMeshCR(app *dbmodel.Application, governance string) (content string, err error) {
	team, err := GetTenantManager().GetTenantsByUUID(app.TenantID)
	if err != nil {
		return "", err
	}
	var resource = schema.GroupVersionResource{Group: "rainbond.io", Version: "v1alpha1", Resource: "servicemeshes"}
	desired := a.generateServiceMeshObj(app, governance, team)
	// get service mesh cr
	var smcr *unstructured.Unstructured
	smcr, err = a.dynamicClient.Resource(resource).Namespace(team.Namespace).Get(context.Background(), app.K8sApp, metav1.GetOptions{})
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return "", err
		}
		smcr, err = a.dynamicClient.Resource(resource).Namespace(team.Namespace).Create(context.Background(), desired, metav1.CreateOptions{})
		if err != nil {
			return "", err
		}
	}
	return a.handleDBK8sResource(app, smcr, "create")
}

func (a *ApplicationAction) handleDBK8sResource(app *dbmodel.Application, smcr *unstructured.Unstructured, action string) (string, error) {
	contentBytes, err := yaml.Marshal(smcr)
	if err != nil {
		logrus.Warningf("marshal service mesh cr error: %v", err)
	}
	resource := &dbmodel.K8sResource{
		AppID:   app.AppID,
		Name:    app.K8sApp,
		State:   model.CreateSuccess,
		Content: string(contentBytes),
	}
	switch action {
	case "create":
		resource.Kind = smcr.GetKind()
		if err := db.GetManager().K8sResourceDao().AddModel(resource); err != nil {
			return "", err
		}
	case "update":
		old, err := db.GetManager().K8sResourceDao().GetK8sResourceByName(app.AppID, app.K8sApp, smcr.GetKind())
		if err != nil {
			return "", err
		}
		old.Content = string(contentBytes)
		if err := db.GetManager().K8sResourceDao().UpdateModel(&old); err != nil {
			return "", err
		}
	case "delete":
		if err := db.GetManager().K8sResourceDao().DeleteK8sResource(app.AppID, app.K8sApp, "ServiceMesh"); err != nil {
			return "", err
		}
	}
	return string(contentBytes), nil
}

// UpdateServiceMeshCR update service mesh custom resources
func (a *ApplicationAction) UpdateServiceMeshCR(app *dbmodel.Application, governance string) (content string, err error) {
	team, err := GetTenantManager().GetTenantsByUUID(app.TenantID)
	if err != nil {
		return "", err
	}
	var resource = schema.GroupVersionResource{Group: "rainbond.io", Version: "v1alpha1", Resource: "servicemeshes"}
	desired := a.generateServiceMeshObj(app, governance, team)
	// get service mesh cr
	var smcr *unstructured.Unstructured
	smcr, err = a.dynamicClient.Resource(resource).Namespace(team.Namespace).Get(context.Background(), app.K8sApp, metav1.GetOptions{})
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return "", err
		}
		smcr, err = a.dynamicClient.Resource(resource).Namespace(team.Namespace).Create(context.Background(), desired, metav1.CreateOptions{})
		if err != nil {
			return "", err
		}
	}
	smcr.Object["provisioner"] = governance
	smcr, err = a.dynamicClient.Resource(resource).Namespace(team.Namespace).Update(context.Background(), smcr, metav1.UpdateOptions{})
	if err != nil {
		return "", err
	}
	return a.handleDBK8sResource(app, smcr, "update")
}

// DeleteServiceMeshCR delete service mesh custom resources
func (a *ApplicationAction) DeleteServiceMeshCR(app *dbmodel.Application) error {
	team, err := GetTenantManager().GetTenantsByUUID(app.TenantID)
	if err != nil {
		return err
	}
	var resource = schema.GroupVersionResource{Group: "rainbond.io", Version: "v1alpha1", Resource: "servicemeshes"}
	// get service mesh cr
	_, err = a.dynamicClient.Resource(resource).Namespace(team.Namespace).Get(context.Background(), app.K8sApp, metav1.GetOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	err = a.dynamicClient.Resource(resource).Namespace(team.Namespace).Delete(context.Background(), app.K8sApp, metav1.DeleteOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	_, err = a.handleDBK8sResource(app, nil, "delete")
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	return nil
}

func (a *ApplicationAction) updateHelmApp(ctx context.Context, app *dbmodel.Application, req model.UpdateAppRequest) error {
	tenant, err := GetTenantManager().GetTenantsByUUID(app.TenantID)
	if err != nil {
		return errors.Wrap(err, "get tenant for helm app failed")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	helmApp, err := a.rainbondClient.RainbondV1alpha1().HelmApps(tenant.Namespace).Get(ctx, app.AppName, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return errors.Wrap(bcode.ErrApplicationNotFound, "update app")
		}
		return errors.Wrap(err, "update app")
	}
	helmApp.Spec.Overrides = req.Overrides
	if req.Version != "" {
		helmApp.Spec.Version = req.Version
	}
	if req.Revision != 0 {
		helmApp.Spec.Revision = req.Revision
	}
	_, err = a.rainbondClient.RainbondV1alpha1().HelmApps(tenant.Namespace).Update(ctx, helmApp, metav1.UpdateOptions{})
	return err
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
func (a *ApplicationAction) DeleteApp(ctx context.Context, app *dbmodel.Application) error {
	if app.AppType == dbmodel.AppTypeHelm {
		return a.deleteHelmApp(ctx, app)
	}

	return a.deleteRainbondApp(app)
}

func (a *ApplicationAction) deleteRainbondApp(app *dbmodel.Application) error {
	// can't delete rainbond app with components
	if err := a.isContainComponents(app.AppID); err != nil {
		return err
	}

	return db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		return errors.WithMessage(a.deleteApp(tx, app), "delete app from db")
	})
}

// isContainComponents checks if the app contains components.
func (a *ApplicationAction) isContainComponents(appID string) error {
	total, err := db.GetManager().TenantServiceDao().CountServiceByAppID(appID)
	if err != nil {
		return err
	}
	if total != 0 {
		return bcode.ErrDeleteDueToBindService
	}
	return nil
}

func (a *ApplicationAction) deleteHelmApp(ctx context.Context, app *dbmodel.Application) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tenant, err := GetTenantManager().GetTenantsByUUID(app.TenantID)
	if err != nil {
		return errors.Wrap(err, "get tenant for helm app failed")
	}
	return db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		if err := a.deleteApp(tx, app); err != nil {
			return err
		}

		if err := a.rainbondClient.RainbondV1alpha1().HelmApps(tenant.Namespace).Delete(ctx, app.AppName, metav1.DeleteOptions{}); err != nil {
			if !k8sErrors.IsNotFound(err) {
				return err
			}
		}
		return nil
	})
}

func (a *ApplicationAction) deleteApp(tx *gorm.DB, app *dbmodel.Application) error {
	// delete app config group service
	if err := db.GetManager().AppConfigGroupServiceDaoTransactions(tx).DeleteByAppID(app.AppID); err != nil {
		return err
	}

	// delete config group items
	if err := db.GetManager().AppConfigGroupItemDaoTransactions(tx).DeleteByAppID(app.AppID); err != nil {
		return err
	}

	// delete config group
	if err := db.GetManager().AppConfigGroupDaoTransactions(tx).DeleteByAppID(app.AppID); err != nil {
		return err
	}

	// delete application
	return db.GetManager().ApplicationDaoTransactions(tx).DeleteApp(app.AppID)
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
		if svc, ok := key2ports[port.ServiceID+strconv.Itoa(port.ContainerPort)]; !ok && port.ServiceID != svc.ServiceID {
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

	var conditions []*model.AppStatusCondition
	for _, cdt := range status.Conditions {
		conditions = append(conditions, &model.AppStatusCondition{
			Type:    cdt.Type,
			Status:  cdt.Status,
			Reason:  cdt.Reason,
			Message: cdt.Message,
		})
	}

	diskUsage := a.getDiskUsage(app.AppID)

	var cpu *int64
	if status.SetCPU {
		cpu = commonutil.Int64(status.Cpu)
	}
	var memory *int64
	if status.SetMemory {
		memory = commonutil.Int64(status.Memory)
	}

	res := &model.AppStatus{
		Status:     status.Status,
		CPU:        cpu,
		Memory:     memory,
		Disk:       int64(diskUsage),
		Phase:      status.Phase,
		Overrides:  status.Overrides,
		Version:    status.Version,
		Conditions: conditions,
		AppID:      app.AppID,
		AppName:    app.AppName,
		K8sApp:     app.K8sApp,
	}
	return res, nil
}

// Install installs the application.
func (a *ApplicationAction) Install(ctx context.Context, app *dbmodel.Application, overrides []string) error {
	ctx1, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(app.TenantID)
	if err != nil {
		return errors.Wrap(err, "install app")
	}
	helmApp, err := a.rainbondClient.RainbondV1alpha1().HelmApps(tenant.Namespace).Get(ctx1, app.AppName, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return errors.Wrap(bcode.ErrApplicationNotFound, "install app")
		}
		return errors.Wrap(err, "install app")
	}

	ctx3, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	helmApp.Spec.Overrides = overrides
	helmApp.Spec.PreStatus = v1alpha1.HelmAppPreStatusConfigured
	_, err = a.rainbondClient.RainbondV1alpha1().HelmApps(tenant.Namespace).Update(ctx3, helmApp, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return errors.Wrap(err, "install app")
}

// ListServices returns the list of the application.
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
			Address:     service.Address,
		}

		svc.Pods = a.convertPods(service.Pods)
		svc.OldPods = a.convertPods(service.OldPods)
		svc.Ports = append(svc.Ports, service.Ports...)
		services = append(services, svc)
	}

	sort.Sort(model.ByServiceName(services))

	return services, nil
}

func (a *ApplicationAction) convertPods(pods []*pb.AppService_Pod) []*model.AppPod {
	var res []*model.AppPod
	for _, pod := range pods {
		res = append(res, &model.AppPod{
			PodName:   pod.Name,
			PodStatus: pod.Status,
		})
	}
	sort.Sort(model.ByPodName(res))
	return res
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

// ListHelmAppReleases returns the list of the helm app.
func (a *ApplicationAction) ListHelmAppReleases(ctx context.Context, app *dbmodel.Application) ([]*model.HelmAppRelease, error) {
	// only for helm app
	if app.AppType != model.AppTypeHelm {
		return nil, nil
	}

	nctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	releases, err := a.statusCli.ListHelmAppRelease(nctx, &pb.AppReq{
		AppId: app.AppID,
	})
	if err != nil {
		return nil, err
	}

	var result []*model.HelmAppRelease
	for _, rel := range releases.HelmAppRelease {
		result = append(result, &model.HelmAppRelease{
			Revision:    int(rel.Revision),
			Updated:     rel.Updated,
			Status:      rel.Status,
			Chart:       rel.Chart,
			AppVersion:  rel.AppVersion,
			Description: rel.Description,
		})
	}
	return result, nil
}

// SyncComponents -
func (a *ApplicationAction) SyncComponents(app *dbmodel.Application, components []*model.Component, deleteComponentIDs []string) error {
	return db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		if err := GetServiceManager().SyncComponentBase(tx, app, components); err != nil {
			return err
		}
		if err := GetGatewayHandler().SyncHTTPRules(tx, components); err != nil {
			return err
		}
		if err := GetGatewayHandler().SyncRuleConfigs(tx, components); err != nil {
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
		if err := GetServiceManager().SyncComponentEndpoints(tx, components); err != nil {
			return err
		}
		if err := GetServiceManager().SyncComponentK8sAttributes(tx, app, components); err != nil {
			return err
		}
		if len(deleteComponentIDs) != 0 {
			return a.deleteByComponentIDs(tx, app, deleteComponentIDs)
		}
		return nil
	})
}

func (a *ApplicationAction) deleteByComponentIDs(tx *gorm.DB, app *dbmodel.Application, componentIDs []string) error {
	if err := db.GetManager().TenantServiceDaoTransactions(tx).DeleteByComponentIDs(app.TenantID, app.AppID, componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().HTTPRuleDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TCPRuleDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServiceMonitorDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantPluginVersionConfigDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServicePluginRelationDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantPluginVersionENVDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServicesPortDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServiceRelationDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServiceEnvVarDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServiceMountRelationDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServiceConfigFileDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().ServiceProbeDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().AppConfigGroupServiceDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServiceLabelDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().ThirdPartySvcDiscoveryCfgDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	autoScaleRules, err := db.GetManager().TenantServceAutoscalerRulesDaoTransactions(tx).ListByComponentIDs(componentIDs)
	if err != nil {
		return err
	}
	var autoScaleRuleIDs []string
	for _, rule := range autoScaleRules {
		autoScaleRuleIDs = append(autoScaleRuleIDs, rule.RuleID)
	}
	if err = db.GetManager().TenantServceAutoscalerRulesDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err = db.GetManager().TenantServceAutoscalerRuleMetricsDaoTransactions(tx).DeleteByRuleIDs(autoScaleRuleIDs); err != nil {
		return err
	}
	return db.GetManager().ComponentK8sAttributeDaoTransactions(tx).DeleteByComponentIDs(componentIDs)
}

// ListAppStatuses -
func (a *ApplicationAction) ListAppStatuses(ctx context.Context, appIDs []string) ([]*model.AppStatus, error) {
	var resp []*model.AppStatus
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	appStatuses, err := a.statusCli.ListAppStatuses(ctx, &pb.AppStatusesReq{
		AppIds: appIDs,
	})
	if err != nil {
		return nil, err
	}
	for _, appStatus := range appStatuses.AppStatuses {
		diskUsage := a.getDiskUsage(appStatus.AppId)
		var cpu *int64
		if appStatus.SetCPU {
			cpu = commonutil.Int64(appStatus.Cpu)
		}
		var memory *int64
		if appStatus.SetMemory {
			memory = commonutil.Int64(appStatus.Memory)
		}
		resp = append(resp, &model.AppStatus{
			Status:    appStatus.Status,
			CPU:       cpu,
			Memory:    memory,
			Disk:      int64(diskUsage),
			Phase:     appStatus.Phase,
			Overrides: appStatus.Overrides,
			Version:   appStatus.Version,
			AppID:     appStatus.AppId,
			AppName:   appStatus.AppName,
		})
	}
	return resp, nil
}

// ListGovernanceMode -
func (a *ApplicationAction) ListGovernanceMode() ([]model.GovernanceMode, error) {
	governanceModes := []model.GovernanceMode{
		{
			Name:        dbmodel.GovernanceModeBuildInServiceMesh,
			IsDefault:   true,
			Description: dbmodel.GovernanceModeBuildInServiceMeshDesc,
		},
		{
			Name:        dbmodel.GovernanceModeKubernetesNativeService,
			IsDefault:   true,
			Description: dbmodel.GovernanceModeKubernetesNativeServiceDesc,
		},
	}

	var serviceMeshClassesResource = schema.GroupVersionResource{Group: "rainbond.io", Version: "v1alpha1", Resource: "servicemeshclasses"}
	list, err := a.dynamicClient.Resource(serviceMeshClassesResource).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Warning("list servicemeshclasses error", err.Error())
		return governanceModes, nil
	}
	for _, item := range list.Items {
		desc, _, _ := unstructured.NestedString(item.Object, "description")
		governanceModes = append(governanceModes, model.GovernanceMode{
			Name:        item.GetName(),
			Description: desc,
		})
	}
	return governanceModes, nil
}

// CheckGovernanceMode Check whether the governance mode can be switched
func (a *ApplicationAction) CheckGovernanceMode(ctx context.Context, governanceMode string) error {
	if !adaptor.IsGovernanceModeValid(governanceMode, a.dynamicClient) {
		return bcode.ErrInvalidGovernanceMode
	}
	return nil
}

//GetAndHandleOperatorManaged get operator managed component
func (a *ApplicationAction) GetAndHandleOperatorManaged(appID string) (*pb.OperatorManaged, error) {
	watchData, err := a.statusCli.GetOperatorWatchData(appID)
	if err != nil {
		return nil, err
	}
	return watchData, nil
}

// ChangeVolumes Since the component name supports modification, the storage directory of stateful components will change.
// This interface is used to modify the original directory name to the storage directory that will actually be used.
func (a *ApplicationAction) ChangeVolumes(app *dbmodel.Application) error {
	components, err := db.GetManager().TenantServiceDao().ListByAppID(app.AppID)
	if err != nil {
		return err
	}
	sharePath := os.Getenv("SHARE_DATA_PATH")
	if sharePath == "" {
		sharePath = "/grdata"
	}

	var componentIDs []string
	k8sComponentNames := make(map[string]string)
	for _, component := range components {
		if !component.IsState() {
			continue
		}
		componentIDs = append(componentIDs, component.ServiceID)
		k8sComponentNames[component.ServiceID] = component.K8sComponentName
	}
	volumes, err := db.GetManager().TenantServiceVolumeDao().ListVolumesByComponentIDs(componentIDs)
	if err != nil {
		return err
	}
	componentVolumes := make(map[string][]*dbmodel.TenantServiceVolume)
	for _, volume := range volumes {
		componentVolumes[volume.ServiceID] = append(componentVolumes[volume.ServiceID], volume)
	}

	for componentID, singleComponentVols := range componentVolumes {
		for _, componentVolume := range singleComponentVols {
			parentDir := fmt.Sprintf("%s/tenant/%s/service/%s%s", sharePath, app.TenantID, componentID, componentVolume.VolumePath)
			newPath := fmt.Sprintf("%s/%s-%s", parentDir, app.K8sApp, k8sComponentNames[componentID])
			if err := changeVolumeDirectoryNames(parentDir, newPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func changeVolumeDirectoryNames(parentDir, newPath string) error {
	files, _ := ioutil.ReadDir(parentDir)
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		isEndWithNumber, suffix := util.IsEndWithNumber(f.Name())
		if isEndWithNumber {
			oldPath := fmt.Sprintf("%s/%s", parentDir, f.Name())
			newVolPath := newPath + suffix
			if err := os.Rename(oldPath, newVolPath); err != nil {
				if err == os.ErrExist || strings.Contains(err.Error(), "file exists") {
					logrus.Infof("Ingore change volume path err: [%v]", err)
					continue
				}
				return err
			}
			logrus.Infof("Success change volume path [%s] to [%s]", oldPath, newVolPath)
		}
	}
	return nil
}
