// Copyright (C) 2nilfmt.Errorf("a")4-2nilfmt.Errorf("a")8 Goodrain Co., Ltd.
// RAINBOND, component Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package handle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/config/configs/rbdcomponent"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm/controller"
	"github.com/goodrain/rainbond/worker/appm/conversion"
	"github.com/goodrain/rainbond/worker/appm/store"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/discover/model"
	"github.com/goodrain/rainbond/worker/gc"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	yamlt "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

// Manager manager
type Manager struct {
	ctx               context.Context
	store             store.Storer
	dbmanager         db.Manager
	controllerManager *controller.Manager
	garbageCollector  *gc.GarbageCollector
	k8sComponent      *k8s.Component
	workerConfig      *rbdcomponent.WorkerConfig
}

// NewManager now handle
func NewManager(ctx context.Context,
	store store.Storer,
	controllerManager *controller.Manager,
	garbageCollector *gc.GarbageCollector,
) *Manager {
	return &Manager{
		ctx:               ctx,
		dbmanager:         db.GetManager(),
		store:             store,
		controllerManager: controllerManager,
		garbageCollector:  garbageCollector,
		k8sComponent:      k8s.Default(),
		workerConfig:      configs.Default().WorkerConfig,
	}
}

// ErrCallback do not handle this task
var ErrCallback = fmt.Errorf("callback task to mq")

const (
	k8sResourceDeleteLeaseDuration  = 30 * time.Second
	k8sResourceDeleteRequestTimeout = 20 * time.Second
	k8sResourceDeleteRetryLimit     = 6
	k8sResourceDeletePollAttempts   = 5
	k8sResourceDeletePollInterval   = time.Second
	k8sResourceDeleteConcurrency    = 8
)

type k8sResourceDeleteIdentityError struct {
	message string
}

func (e *k8sResourceDeleteIdentityError) Error() string {
	return e.message
}

type k8sResourceDeleteFinalizerPolicyError struct {
	origin string
}

func (e *k8sResourceDeleteFinalizerPolicyError) Error() string {
	return fmt.Sprintf("finalizer removal is not allowed for k8s resource origin %q", e.origin)
}

func (m *Manager) checkCount() bool {
	return m.controllerManager.GetControllerSize() > m.workerConfig.MaxTasks
}

// AnalystToExec analyst exec
func (m *Manager) AnalystToExec(task *model.Task) error {
	if task == nil {
		return nil
	}
	//max worker count check
	if m.checkCount() {
		return ErrCallback
	}
	if !m.store.Ready() {
		return ErrCallback
	}
	switch task.Type {
	case "start":
		logrus.Info("start a 'start' task worker")
		return m.startExec(task)
	case "stop":
		logrus.Info("start a 'stop' task worker")
		return m.stopExec(task)
	case "restart":
		logrus.Info("start a 'restart' task worker")
		return m.restartExec(task)
	case "horizontal_scaling":
		logrus.Info("start a 'horizontal_scaling' task worker")
		return m.horizontalScalingExec(task)
	case "vertical_scaling":
		logrus.Info("start a 'vertical_scaling' task worker")
		return m.verticalScalingExec(task)
	case "rolling_upgrade":
		logrus.Info("start a 'rolling_upgrade' task worker")
		return m.rollingUpgradeExec(task)
	case "apply_rule":
		logrus.Info("start a 'apply_rule' task worker")
		return m.applyRuleExec(task)
	case "apply_plugin_config":
		logrus.Info("start a 'apply_plugin_config' task worker")
		return m.applyPluginConfig(task)
	case "service_gc":
		logrus.Info("start the 'service_gc' task")
		return m.ExecServiceGCTask(task)
	case "delete_tenant":
		logrus.Info("start a 'delete_tenant' task worker")
		return m.deleteTenant(task)
	case "refreshhpa":
		logrus.Info("start a 'refreshhpa' task worker")
		return m.ExecRefreshHPATask(task)
	case "apply_registry_auth_secret":
		logrus.Info("start a 'apply_registry_auth_secret' task worker")
		return m.ExecApplyRegistryAuthSecretTask(task)
	case "delete_k8s_resource":
		logrus.Info("start a 'delete_k8s_resource' task worker")
		return m.DeleteK8sResource(task)
	case "build_from_kubeblocks":
		logrus.Info("start a 'build_from_kubeblocks' task worker")
		return m.buildFromKubeBlocksExec(task)
	default:
		if task.Type != "" {
			logrus.Warning("task can not execute because no type is identified ->", task.Type)
		}
		return nil
	}
}

// startExec exec start service task
func (m *Manager) startExec(task *model.Task) error {
	body, ok := task.Body.(model.StartTaskBody)
	if !ok {
		logrus.Errorf("start body convert to taskbody error")
		return fmt.Errorf("start body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	appService := m.store.GetAppService(body.ServiceID)
	if appService != nil && !appService.IsClosed() {
		logger.Info("component is not closed, can not start", event.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	newAppService, err := conversion.InitAppService(false, m.dbmanager, body.ServiceID, body.Configs)
	if err != nil {
		logrus.Errorf("component init create failure:%s", err.Error())
		logger.Error(util.Translation("component init create failure"), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("application init create failure")
	}
	newAppService.Logger = logger
	//regist new app service
	m.store.RegistAppService(newAppService)
	err = m.controllerManager.StartController(controller.TypeStartController, *newAppService)
	if err != nil {
		logrus.Errorf("component run start controller failure:%s", err.Error())
		logger.Error(util.Translation("component run start controller failure"), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("component start failure")
	}
	logrus.Infof("component(%s) %s working is running.", body.ServiceID, "start")
	return nil
}

func (m *Manager) stopExec(task *model.Task) error {
	body, ok := task.Body.(model.StopTaskBody)
	if !ok {
		logrus.Errorf("stop body convert to taskbody error")
		return fmt.Errorf("stop body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	appService := m.store.GetAppService(body.ServiceID)
	if appService == nil {
		logger.Info("component is closed, can not stop", event.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	appService.Logger = logger
	for k, v := range body.Configs {
		appService.ExtensionSet[k] = v
	}
	err := m.controllerManager.StartController(controller.TypeStopController, *appService)
	if err != nil {
		logrus.Errorf("component run  stop controller failure:%s", err.Error())
		logger.Info("component run stop controller failure", event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("component stop failure")
	}
	logrus.Infof("service(%s) %s working is running.", body.ServiceID, "stop")
	return nil
}

func (m *Manager) restartExec(task *model.Task) error {
	body, ok := task.Body.(model.RestartTaskBody)
	if !ok {
		logrus.Errorf("stop body convert to taskbody error")
		return fmt.Errorf("stop body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	appService := m.store.GetAppService(body.ServiceID)
	if appService == nil {
		logger.Info("component is closed, can not stop", event.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	appService.Logger = logger
	for k, v := range body.Configs {
		appService.ExtensionSet[k] = v
	}
	//first stop app
	err := m.controllerManager.StartController(controller.TypeRestartController, *appService)
	if err != nil {
		logrus.Errorf("component run restart controller failure:%s", err.Error())
		logger.Info("component run restart controller failure", event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("component restart failure")
	}
	logrus.Infof("service(%s) %s working is running.", body.ServiceID, "restart")
	return nil
}

func (m *Manager) horizontalScalingExec(task *model.Task) (err error) {
	body, ok := task.Body.(model.HorizontalScalingTaskBody)
	if !ok {
		logrus.Errorf("horizontal_scaling body convert to taskbody error")
		err = fmt.Errorf("a")
		return
	}

	logger := event.GetManager().GetLogger(body.EventID)
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(body.ServiceID)
	if err != nil {
		logger.Error("Get app base info failure", event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		logrus.Errorf("horizontal_scaling get rc error. %v", err)
		err = fmt.Errorf("a")
		return
	}
	appService := m.store.GetAppService(body.ServiceID)
	if appService == nil || appService.IsClosed() {
		logger.Info("service is closed, no need handle", event.GetLastLoggerOption())
		return
	}
	oldReplicas, newReplicas := appService.Replicas, service.Replicas

	defer func() {
		desc := "the replicas is scaling from %d to %d successfully"
		desc = fmt.Sprintf(desc, oldReplicas, newReplicas)
		reason := "SuccessfulRescale"
		if err != nil {
			desc = "the replicas is scaling from %d to %d: %v"
			desc = fmt.Sprintf(desc, oldReplicas, newReplicas, err)
			reason = "FailedRescale"
		}
		scalingRecord := &dbmodel.TenantServiceScalingRecords{
			ServiceID:   body.ServiceID,
			EventName:   util.NewUUID(),
			RecordType:  "manual",
			Reason:      reason,
			Count:       1,
			Description: desc,
			Operator:    body.Username,
			LastTime:    time.Now(),
		}
		if err := db.GetManager().TenantServiceScalingRecordsDao().AddModel(scalingRecord); err != nil {
			logrus.Warningf("save scaling record: %v", err)
		}
	}()

	appService.Logger = logger
	appService.Replicas = service.Replicas
	err = m.controllerManager.StartController(controller.TypeScalingController, *appService)
	if err != nil {
		logrus.Errorf("component run  scaling controller failure:%s", err.Error())
		logger.Info("component run scaling controller failure", event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return
	}
	logrus.Infof("service(%s) %s working is running.", body.ServiceID, "scaling")
	return nil
}

func (m *Manager) verticalScalingExec(task *model.Task) error {
	body, ok := task.Body.(model.VerticalScalingTaskBody)
	if !ok {
		logrus.Errorf("vertical_scaling body convert to taskbody error")
		return fmt.Errorf("vertical_scaling body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(body.ServiceID)
	if err != nil {
		logrus.Errorf("vertical_scaling get rc error. %v", err)
		logger.Error("Get app base info failure", event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("vertical_scaling get rc error. %v", err)
	}
	appService := m.store.GetAppService(body.ServiceID)
	if appService == nil || appService.IsClosed() {
		logger.Info("service is closed,no need handle", event.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}
	appService.ContainerCPU = service.ContainerCPU
	appService.ContainerMemory = service.ContainerMemory
	appService.ContainerGPU = service.ContainerGPU
	appService.Logger = logger
	newAppService, err := conversion.InitAppService(false, m.dbmanager, body.ServiceID, nil)
	if err != nil {
		logrus.Errorf("component init create failure:%s", err.Error())
		logger.Error(util.Translation("component init create failure"), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("application init create failure")
	}
	newAppService.Logger = logger
	appService.SetUpgradePatch(newAppService)
	err = m.controllerManager.StartController(controller.TypeUpgradeController, *newAppService)
	if err != nil {
		logrus.Errorf("component run  vertical scaling(upgrade) controller failure:%s", err.Error())
		logger.Info("component run vertical scaling(upgrade) controller failure", event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("application vertical scaling(upgrade) failure")
	}
	logrus.Infof("service(%s) %s working is running.", body.ServiceID, "vertical scaling")
	return nil
}

// TODO kb
func (m *Manager) rollingUpgradeExec(task *model.Task) error {
	body, ok := task.Body.(model.RollingUpgradeTaskBody)
	if !ok {
		logrus.Error("rolling_upgrade body convert to taskbody error", task.Body)
		return fmt.Errorf("rolling_upgrade body convert to taskbody error")
	}
	logger := event.GetManager().GetLogger(body.EventID)
	newAppService, err := conversion.InitAppService(body.DryRun, m.dbmanager, body.ServiceID, body.Configs)
	if err != nil {
		logrus.Errorf("component init create failure:%s", err.Error())
		logger.Error(util.Translation("component init create failure"), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("component init create failure")
	}
	newAppService.Logger = logger
	oldAppService := m.store.GetAppService(body.ServiceID)
	// if service not deploy,start it
	if oldAppService == nil || oldAppService.IsClosed() {
		//regist new app service
		m.store.RegistAppService(newAppService)
		if body.DryRun {
			err = m.controllerManager.ExportController(body.AppName, body.AppVersion, body.EventIDs, body.End, *newAppService)
		} else {
			err = m.controllerManager.StartController(controller.TypeStartController, *newAppService)
		}
		if err != nil {
			logrus.Errorf("component run  start controller failure:%s", err.Error())
			logger.Info("component run start controller failure", event.GetCallbackLoggerOption())
			event.GetManager().ReleaseLogger(logger)
			return fmt.Errorf("component start failure")
		}
		logrus.Infof("service(%s) %s working is running.", body.ServiceID, "start")
		return nil
	}
	if err := oldAppService.SetUpgradePatch(newAppService); err != nil {
		if err.Error() == "no upgrade" {
			logger.Info("component no change no need upgrade.", event.GetLastLoggerOption())
			return nil
		}
		logrus.Errorf("component get upgrade info error:%s", err.Error())
		logger.Error(fmt.Sprintf("component get upgrade info error:%s", err.Error()), event.GetCallbackLoggerOption())
		return nil
	}
	//if service already deploy,upgrade it:
	err = m.controllerManager.StartController(controller.TypeUpgradeController, *newAppService)
	if err != nil {
		logrus.Errorf("component run  upgrade controller failure:%s", err.Error())
		logger.Info("component run upgrade controller failure", event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("component upgrade failure")
	}
	logrus.Infof("service(%s) %s working is running.", body.ServiceID, "upgrade")
	return nil
}

func (m *Manager) applyRuleExec(task *model.Task) error {
	body, ok := task.Body.(*model.ApplyRuleTaskBody)
	if !ok {
		logrus.Errorf("Can't convert %s to *model.ApplyRuleTaskBody", reflect.TypeOf(task.Body))
		return fmt.Errorf("can't convert %s to *model.ApplyRuleTaskBody", reflect.TypeOf(task.Body))
	}
	svc, err := db.GetManager().TenantServiceDao().GetServiceByID(body.ServiceID)
	if err != nil {
		logrus.Errorf("error get TenantServices: %v", err)
		return fmt.Errorf("error get TenantServices: %v", err)
	}
	logger := event.GetManager().GetLogger(body.EventID)
	oldAppService := m.store.GetAppService(body.ServiceID)
	logrus.Debugf("body action: %s", body.Action)
	if svc.Kind != dbmodel.ServiceKindThirdParty.String() && !strings.HasPrefix(body.Action, "port") {
		if oldAppService == nil || oldAppService.IsClosed() {
			logrus.Debugf("service is closed, no need handle")
			logger.Info("service is closed,no need handle", event.GetLastLoggerOption())
			event.GetManager().ReleaseLogger(logger)
			return nil
		}
	}
	var newAppService *v1.AppService
	if svc.Kind == dbmodel.ServiceKindThirdParty.String() {
		newAppService, err = conversion.InitAppService(false, m.dbmanager, body.ServiceID, nil,
			"ServiceSource", "TenantServiceBase", "TenantServiceRegist")
	} else {
		newAppService, err = conversion.InitAppService(false, m.dbmanager, body.ServiceID, nil)
	}
	if err != nil {
		logrus.Errorf("component init create failure:%s", err.Error())
		logger.Error(util.Translation("component init create failure"), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("component init create failure")
	}
	newAppService.Logger = logger
	newAppService.SetDeletedResources(m.store.GetAppService(body.ServiceID))
	// update k8s resources
	newAppService.CustomParams = body.Limit
	err = m.controllerManager.StartController(controller.TypeApplyRuleController, *newAppService)
	if err != nil {
		logrus.Errorf("component apply rule controller failure:%s", err.Error())
		return fmt.Errorf("component apply rule controller failure:%s", err.Error())
	}

	return nil
}

// applyPluginConfig apply service plugin config
func (m *Manager) applyPluginConfig(task *model.Task) error {
	body, ok := task.Body.(*model.ApplyPluginConfigTaskBody)
	if !ok {
		logrus.Errorf("Can't convert %s to *model.ApplyPluginConfigTaskBody", reflect.TypeOf(task.Body))
		return fmt.Errorf("can't convert %s to *model.ApplyPluginConfigTaskBody", reflect.TypeOf(task.Body))
	}
	oldAppService := m.store.GetAppService(body.ServiceID)
	if oldAppService == nil || oldAppService.IsClosed() {
		logrus.Debugf("service is closed,no need handle")
		return nil
	}
	newApp, err := conversion.InitAppService(false, m.dbmanager, body.ServiceID, nil, "ServiceSource", "TenantServiceBase", "TenantServicePlugin")
	if err != nil {
		logrus.Errorf("component apply plugin config controller failure:%s", err.Error())
		return err
	}
	err = m.controllerManager.StartController(controller.TypeApplyConfigController, *newApp)
	if err != nil {
		logrus.Errorf("component apply plugin config controller failure:%s", err.Error())
		return fmt.Errorf("component apply plugin config controller failure:%s", err.Error())
	}
	return nil
}

// ExecServiceGCTask executes the 'service_gc' task
func (m *Manager) ExecServiceGCTask(task *model.Task) error {
	serviceGCReq, ok := task.Body.(model.ServiceGCTaskBody)
	if !ok {
		return fmt.Errorf("can not convert the request body to 'ServiceGCTaskBody'")
	}

	m.garbageCollector.DelLogFile(serviceGCReq)
	m.garbageCollector.DelPvPvcByServiceID(serviceGCReq)
	m.garbageCollector.DelVolumeData(serviceGCReq)
	m.garbageCollector.DelKubernetesObjects(serviceGCReq)
	m.garbageCollector.DelComponentPkg(serviceGCReq)
	m.garbageCollector.DelShellPod()
	return nil
}

func (m *Manager) deleteTenant(task *model.Task) (err error) {
	body, ok := task.Body.(*model.DeleteTenantTaskBody)
	if !ok {
		logrus.Errorf("can't convert %s to *model.DeleteTenantTaskBody", reflect.TypeOf(task.Body))
		err = fmt.Errorf("can't convert %s to *model.DeleteTenantTaskBody", reflect.TypeOf(task.Body))
		return
	}

	defer func() {
		if err == nil {
			return
		}
		logrus.Errorf("failed to delete tenant: %v", err)
		var tenant *dbmodel.Tenants
		tenant, err = db.GetManager().TenantDao().GetTenantByUUID(body.TenantID)
		if err != nil {
			err = fmt.Errorf("tenant id: %s; find tenant: %v", body.TenantID, err)
			return
		}
		tenant.Status = dbmodel.TenantStatusDeleteFailed.String()
		err := db.GetManager().TenantDao().UpdateModel(tenant)
		if err != nil {
			logrus.Errorf("update tenant_status to '%s': %v", tenant.Status, err)
			return
		}
	}()
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(body.TenantID)
	if err != nil {
		err = fmt.Errorf("tenant id: %s; find tenant: %v", body.TenantID, err)
		return
	}
	if err = m.k8sComponent.Clientset.CoreV1().Namespaces().Delete(context.Background(), tenant.Namespace, metav1.DeleteOptions{
		GracePeriodSeconds: util.Int64(0),
	}); err != nil && !k8sErrors.IsNotFound(err) {
		err = fmt.Errorf("delete namespace: %v", err)
		return
	}

	err = db.GetManager().TenantDao().DelByTenantID(body.TenantID)
	if err != nil {
		err = fmt.Errorf("delete tenant: %v", err)
		return
	}

	return
}

// ExecRefreshHPATask executes a 'refresh hpa' task.
func (m *Manager) ExecRefreshHPATask(task *model.Task) error {
	body, ok := task.Body.(*model.RefreshHPATaskBody)
	if !ok {
		logrus.Errorf("exec task 'refreshhpa'; wrong type: %v", reflect.TypeOf(task))
		return fmt.Errorf("exec task 'refreshhpa': wrong input")
	}

	logger := event.GetManager().GetLogger(body.EventID)

	oldAppService := m.store.GetAppService(body.ServiceID)
	if oldAppService != nil && oldAppService.IsClosed() {
		logger.Info("application is closed, ignore task 'refreshhpa'", event.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}

	newAppService, err := conversion.InitAppService(false, m.dbmanager, body.ServiceID, nil)
	if err != nil {
		logrus.Errorf("component init create failure:%s", err.Error())
		logger.Error(util.Translation("component init create failure"), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("component init create failure")
	}
	newAppService.Logger = logger
	newAppService.SetDeletedResources(oldAppService)

	err = m.controllerManager.StartController(controller.TypeControllerRefreshHPA, *newAppService)
	if err != nil {
		logrus.Errorf("component run  refreshhpa controller failure: %s", err.Error())
		logger.Error(util.Translation("refresh hpa failure"), event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("refresh hpa: %v", err)
	}

	logrus.Infof("rule id: %s; successfully refresh hpa", body.RuleID)
	return nil
}

// ExecApplyRegistryAuthSecretTask executes a 'apply registry auth secret' task.
func (m *Manager) ExecApplyRegistryAuthSecretTask(task *model.Task) error {
	body, ok := task.Body.(*model.ApplyRegistryAuthSecretTaskBody)
	if !ok {
		return fmt.Errorf("can't convert %s to *model.ApplyRegistryAuthSecretTaskBody", reflect.TypeOf(task.Body))
	}
	tenant, err := m.dbmanager.TenantDao().GetTenantByUUID(body.TenantID)
	if err != nil {
		logrus.Debugf("cant get tenant by uuid: %s", body.TenantID)
		return err
	}

	secretNameFrom := func(secretID string) string {
		return fmt.Sprintf("rbd-registry-auth-%s", secretID)
	}

	secret, err := m.k8sComponent.Clientset.CoreV1().Secrets(tenant.Namespace).Get(m.ctx, secretNameFrom(body.SecretID), metav1.GetOptions{})
	switch body.Action {
	case "apply":
		if err != nil {
			if k8sErrors.IsNotFound(err) {
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretNameFrom(body.SecretID),
						Namespace: tenant.Namespace,
						Labels: map[string]string{
							"tenant_id":                        tenant.UUID,
							"tenant_name":                      tenant.Name,
							"creator":                          "Rainbond",
							"rainbond.io/registry-auth-secret": "true",
						},
					},
					Data: map[string][]byte{
						"Domain":   []byte(body.Domain),
						"Username": []byte(body.Username),
						"Password": []byte(body.Password),
					},
					Type: corev1.SecretTypeOpaque,
				}
				_, err = m.k8sComponent.Clientset.CoreV1().Secrets(tenant.Namespace).Create(m.ctx, secret, metav1.CreateOptions{})
			} else {
				logrus.Errorf("get secret failure: %s", err.Error())
				return err
			}
		} else {
			secret.Data["Domain"] = []byte(body.Domain)
			secret.Data["Username"] = []byte(body.Username)
			secret.Data["Password"] = []byte(body.Password)
			_, err = m.k8sComponent.Clientset.CoreV1().Secrets(tenant.Namespace).Update(m.ctx, secret, metav1.UpdateOptions{})
		}
		if err != nil {
			logrus.Errorf("apply secret failure: %s", err.Error())
			return err
		}
	case "delete":
		err := m.k8sComponent.Clientset.CoreV1().Secrets(tenant.Namespace).Delete(m.ctx, secretNameFrom(body.SecretID), metav1.DeleteOptions{})
		if err != nil {
			logrus.Debugf("delete secret: %s", err.Error())
		}
		return err
	}
	return nil
}

// RefreshMapper -
func (m *Manager) RefreshMapper() error {
	gr, err := restmapper.GetAPIGroupResources(m.k8sComponent.Clientset)
	if err != nil {
		return err
	}
	m.k8sComponent.Mapper = restmapper.NewDiscoveryRESTMapper(gr)
	return nil
}

// DeleteK8sResource deletes persisted Region resources. A duplicate MQ message
// is harmless: a worker first claims the row's exact deletion generation lease.
// Per-resource failures are persisted and retried by the recovery scanner, so
// returning an error here must never be used as the reliability mechanism.
func (m *Manager) DeleteK8sResource(task *model.Task) error {
	body, ok := task.Body.(*model.DeleteK8sResourceTaskBody)
	if !ok {
		return fmt.Errorf("can't convert task body to *model.DeleteK8sResourceTaskBody")
	}
	if len(body.Resources) == 0 {
		return nil
	}
	dc, err := dynamic.NewForConfig(m.k8sComponent.RestConfig)
	if err != nil {
		logrus.Errorf("create dynamic client for k8s resource deletion: %v", err)
		return err
	}

	resolver := &k8sResourceDeleteMapperResolver{
		mapper: m.k8sComponent.Mapper,
		refresh: func() (meta.RESTMapper, error) {
			if err := m.RefreshMapper(); err != nil {
				return nil, err
			}
			return m.k8sComponent.Mapper, nil
		},
	}
	workerCount := k8sResourceDeleteWorkerCount(len(body.Resources))
	if workerCount == 0 {
		return nil
	}
	jobs := make(chan model.K8sResourceDeleteTaskTarget)
	var waitGroup sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for target := range jobs {
				m.deletePersistedK8sResource(dc, resolver, target)
			}
		}()
	}
	for _, target := range uniqueK8sResourceDeleteTargets(body.Resources) {
		jobs <- target
	}
	close(jobs)
	waitGroup.Wait()
	return nil
}

func k8sResourceDeleteWorkerCount(targetCount int) int {
	if targetCount <= 0 {
		return 0
	}
	if targetCount < k8sResourceDeleteConcurrency {
		return targetCount
	}
	return k8sResourceDeleteConcurrency
}

type k8sResourceDeleteMapperResolver struct {
	mu      sync.Mutex
	mapper  meta.RESTMapper
	refresh func() (meta.RESTMapper, error)
}

func (r *k8sResourceDeleteMapperResolver) resolve(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return resolveK8sResourceMapping(r.mapper, gk, version, func() (meta.RESTMapper, error) {
		refreshed, err := r.refresh()
		if err != nil {
			return nil, err
		}
		if refreshed != nil {
			r.mapper = refreshed
		}
		return refreshed, nil
	})
}

// resolveK8sResourceMapping retries the actual RESTMapping after discovery has
// been refreshed. The persisted YAML may name a CRD version that is no longer
// served (for example, KubeBlocks v1alpha1 after an upgrade), so after that
// retry a versionless GroupKind lookup selects the currently served version.
// The old deletion path accidentally returned the successful refresh error
// (nil) instead of continuing to map the resource.
func resolveK8sResourceMapping(mapper meta.RESTMapper, gk schema.GroupKind, version string, refresh func() (meta.RESTMapper, error)) (*meta.RESTMapping, error) {
	if mapper == nil {
		return nil, fmt.Errorf("k8s REST mapper is unavailable")
	}
	mapping, err := mapper.RESTMapping(gk, version)
	if err == nil || !meta.IsNoMatchError(err) {
		return mapping, err
	}
	refreshed, refreshErr := refresh()
	if refreshErr != nil {
		return nil, refreshErr
	}
	if refreshed == nil {
		return nil, fmt.Errorf("k8s REST mapper refresh returned nil")
	}
	mapping, err = refreshed.RESTMapping(gk, version)
	if err == nil || !meta.IsNoMatchError(err) {
		return mapping, err
	}
	return refreshed.RESTMapping(gk)
}

func uniqueK8sResourceDeleteTargets(targets []model.K8sResourceDeleteTaskTarget) []model.K8sResourceDeleteTaskTarget {
	seen := make(map[string]struct{}, len(targets))
	unique := make([]model.K8sResourceDeleteTaskTarget, 0, len(targets))
	for _, target := range targets {
		if target.ResourceID == 0 || target.DeleteGeneration <= 0 {
			continue
		}
		key := fmt.Sprintf("%d/%d", target.ResourceID, target.DeleteGeneration)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, target)
	}
	return unique
}

func (m *Manager) deletePersistedK8sResource(dc dynamic.Interface, resolver *k8sResourceDeleteMapperResolver, target model.K8sResourceDeleteTaskTarget) {
	now := time.Now()
	lease, err := m.dbmanager.K8sResourceDao().ClaimK8sResourceDeleteLease(target.ResourceID, target.DeleteGeneration, now, now.Add(k8sResourceDeleteLeaseDuration))
	if err != nil {
		logrus.Errorf("claim k8s resource deletion lease for %d/%d: %v", target.ResourceID, target.DeleteGeneration, err)
		return
	}
	if lease == nil {
		return
	}
	resource, err := m.dbmanager.K8sResourceDao().GetK8sResourceForDeletion(target.ResourceID, target.DeleteGeneration, lease.DeleteLeaseToken)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			logrus.Errorf("load claimed k8s resource deletion %d/%d: %v", target.ResourceID, target.DeleteGeneration, err)
		}
		return
	}
	updated, err := m.dbmanager.K8sResourceDao().IncreaseK8sResourceDeleteAttempts(resource.ID, target.DeleteGeneration, lease.DeleteLeaseToken)
	if err != nil || !updated {
		if err != nil {
			logrus.Errorf("record k8s resource deletion attempt for %d/%d: %v", resource.ID, target.DeleteGeneration, err)
		}
		return
	}
	// A create/update error does not prove Kubernetes created nothing: a client
	// timeout may arrive after the API server accepted the create. Never remove
	// Region state without an actual GET NotFound verification.
	if resource.State != apimodel.CreateSuccess && resource.State != apimodel.UpdateSuccess {
		m.finishK8sResourceDeleteFailure(resource, target, lease.DeleteLeaseToken, fmt.Errorf("resource create/update did not complete; physical identity cannot be verified"), true)
		return
	}

	deleteContext, cancel := context.WithTimeout(m.ctx, k8sResourceDeleteRequestTimeout)
	defer cancel()
	obj, gvk, err := decodeK8sResourceForDeletion(resource.Content)
	if err != nil {
		m.finishK8sResourceDeleteFailure(resource, target, lease.DeleteLeaseToken, err, true)
		return
	}
	mapping, err := resolver.resolve(gvk.GroupKind(), gvk.Version)
	if err != nil {
		m.finishK8sResourceDeleteFailure(resource, target, lease.DeleteLeaseToken, err, false)
		return
	}
	physicalIdentity := dbmodel.K8sResourcePhysicalIdentity(mapping.Resource.Group, mapping.Resource.Resource, obj.GetNamespace(), obj.GetName())
	if resource.ResourceIdentity != "" && resource.ResourceIdentity != physicalIdentity {
		m.finishK8sResourceDeleteFailure(resource, target, lease.DeleteLeaseToken, &k8sResourceDeleteIdentityError{message: fmt.Sprintf("persisted Kubernetes identity %q does not match saved content identity %q", resource.ResourceIdentity, physicalIdentity)}, true)
		return
	}
	var resourceClient dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		resourceClient = dc.Resource(mapping.Resource).Namespace(obj.GetNamespace())
	} else {
		resourceClient = dc.Resource(mapping.Resource)
	}
	if resource.ResourceUID == "" || resource.ResourceIdentity == "" {
		observed, getErr := resourceClient.Get(deleteContext, obj.GetName(), metav1.GetOptions{})
		if k8sErrors.IsNotFound(getErr) {
			m.completeK8sResourceDeletion(resource, target, lease.DeleteLeaseToken)
			return
		}
		if getErr != nil {
			m.finishK8sResourceDeleteFailure(resource, target, lease.DeleteLeaseToken, getErr, false)
			return
		}
		observedUID := string(observed.GetUID())
		if observedUID == "" {
			m.finishK8sResourceDeleteFailure(resource, target, lease.DeleteLeaseToken, &k8sResourceDeleteIdentityError{message: "exact Kubernetes object has no UID; refusing deletion"}, true)
			return
		}
		if resource.ResourceUID != "" && resource.ResourceUID != observedUID {
			m.finishK8sResourceDeleteFailure(resource, target, lease.DeleteLeaseToken, &k8sResourceDeleteIdentityError{message: fmt.Sprintf("persisted Kubernetes UID %q does not match observed object UID %q", resource.ResourceUID, observedUID)}, true)
			return
		}
		adopted, adoptErr := m.dbmanager.K8sResourceDao().AdoptK8sResourceDeleteIdentity(resource.ID, target.DeleteGeneration, lease.DeleteLeaseToken, observedUID, physicalIdentity)
		if adoptErr != nil {
			m.finishK8sResourceDeleteFailure(resource, target, lease.DeleteLeaseToken, adoptErr, false)
			return
		}
		if !adopted {
			logrus.Debugf("k8s resource deletion identity %d/%d was superseded before UID adoption", resource.ID, target.DeleteGeneration)
			return
		}
		resource.ResourceUID = observedUID
		resource.ResourceIdentity = physicalIdentity
	}
	if err := resourceClient.Delete(deleteContext, obj.GetName(), forceDeleteOptions(types.UID(resource.ResourceUID))); err != nil && !k8sErrors.IsNotFound(err) {
		m.finishK8sResourceDeleteFailure(resource, target, lease.DeleteLeaseToken, err, k8sErrors.IsConflict(err))
		return
	}

	deleted, err := waitForK8sResourceDeletion(deleteContext, resourceClient, obj.GetName(), types.UID(resource.ResourceUID), resource.ResourceOrigin, resource.AllowsForceFinalizerRemoval())
	if err == nil && deleted {
		m.completeK8sResourceDeletion(resource, target, lease.DeleteLeaseToken)
		return
	}
	if err != nil {
		m.finishK8sResourceDeleteFailure(resource, target, lease.DeleteLeaseToken, err, isPermanentK8sResourceDeleteError(err))
		return
	}
	m.finishK8sResourceDeleteFailure(resource, target, lease.DeleteLeaseToken, fmt.Errorf("resource %s is still terminating after force deletion", obj.GetName()), false)
}

// completeK8sResourceDeletion removes Region state only after a Kubernetes GET
// for the exact persisted identity has returned NotFound.
func (m *Manager) completeK8sResourceDeletion(resource dbmodel.K8sResource, target model.K8sResourceDeleteTaskTarget, leaseToken string) {
	if removed, deleteErr := m.dbmanager.K8sResourceDao().DeleteK8sResourceByIDAndGeneration(resource.ID, target.DeleteGeneration, leaseToken); deleteErr != nil {
		logrus.Errorf("remove verified k8s resource deletion row %d/%d: %v", resource.ID, target.DeleteGeneration, deleteErr)
	} else if !removed {
		logrus.Debugf("k8s resource deletion row %d/%d was superseded before physical removal", resource.ID, target.DeleteGeneration)
	}
}

func decodeK8sResourceForDeletion(resourceYAML string) (*unstructured.Unstructured, schema.GroupVersionKind, error) {
	decoder := yamlt.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(resourceYAML)), 1000)
	var objects []*unstructured.Unstructured
	var groupVersionKinds []schema.GroupVersionKind
	for {
		var rawObj runtime.RawExtension
		err := decoder.Decode(&rawObj)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, schema.GroupVersionKind{}, err
		}
		if len(rawObj.Raw) == 0 {
			continue
		}
		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return nil, schema.GroupVersionKind{}, err
		}
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, schema.GroupVersionKind{}, err
		}
		objects = append(objects, &unstructured.Unstructured{Object: unstructuredMap})
		groupVersionKinds = append(groupVersionKinds, *gvk)
	}
	if len(objects) != 1 {
		return nil, schema.GroupVersionKind{}, fmt.Errorf("expected exactly one persisted k8s resource object, got %d", len(objects))
	}
	if objects[0].GetName() == "" {
		return nil, schema.GroupVersionKind{}, fmt.Errorf("persisted k8s resource has no metadata.name")
	}
	return objects[0], groupVersionKinds[0], nil
}

func forceDeleteOptions(resourceUID types.UID) metav1.DeleteOptions {
	gracePeriodSeconds := int64(0)
	propagationPolicy := metav1.DeletePropagationBackground
	return metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
		PropagationPolicy:  &propagationPolicy,
		Preconditions:      &metav1.Preconditions{UID: &resourceUID},
	}
}

func waitForK8sResourceDeletion(ctx context.Context, resourceClient dynamic.ResourceInterface, name string, expectedUID types.UID, resourceOrigin string, allowFinalizerRemoval bool) (bool, error) {
	for attempt := 0; attempt < k8sResourceDeletePollAttempts; attempt++ {
		resource, err := resourceClient.Get(ctx, name, metav1.GetOptions{})
		if k8sErrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if resource.GetUID() != expectedUID {
			return false, &k8sResourceDeleteIdentityError{message: fmt.Sprintf("persisted Kubernetes UID %q does not match current object UID %q", expectedUID, resource.GetUID())}
		}
		if resource.GetDeletionTimestamp() != nil && len(resource.GetFinalizers()) > 0 {
			if !allowFinalizerRemoval {
				return false, &k8sResourceDeleteFinalizerPolicyError{origin: resourceOrigin}
			}
			finalizerPatch, _ := json.Marshal([]map[string]interface{}{
				{"op": "test", "path": "/metadata/uid", "value": string(expectedUID)},
				{"op": "test", "path": "/metadata/resourceVersion", "value": resource.GetResourceVersion()},
				{"op": "replace", "path": "/metadata/finalizers", "value": []string{}},
			})
			if _, err := resourceClient.Patch(ctx, name, types.JSONPatchType, finalizerPatch, metav1.PatchOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
				return false, err
			}
		}
		if attempt == k8sResourceDeletePollAttempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(k8sResourceDeletePollInterval):
		}
	}
	return false, nil
}

func isPermanentK8sResourceDeleteError(err error) bool {
	switch err.(type) {
	case *k8sResourceDeleteIdentityError, *k8sResourceDeleteFinalizerPolicyError:
		return true
	default:
		return false
	}
}

func (m *Manager) finishK8sResourceDeleteFailure(resource dbmodel.K8sResource, target model.K8sResourceDeleteTaskTarget, leaseToken string, deleteErr error, permanent bool) {
	deleteError := sanitizeK8sResourceDeleteError(deleteErr)
	if permanent || resource.DeleteAttempts+1 >= k8sResourceDeleteRetryLimit {
		if updated, err := m.dbmanager.K8sResourceDao().MarkK8sResourceDeleteFailed(resource.ID, target.DeleteGeneration, leaseToken, deleteError); err != nil {
			logrus.Errorf("mark k8s resource deletion %d/%d failed: %v", resource.ID, target.DeleteGeneration, err)
		} else if !updated {
			logrus.Debugf("k8s resource deletion failure %d/%d was superseded", resource.ID, target.DeleteGeneration)
		}
		return
	}
	delay := k8sResourceDeleteRetryDelay(resource.DeleteAttempts + 1)
	if updated, err := m.dbmanager.K8sResourceDao().ScheduleK8sResourceDeleteRetry(resource.ID, target.DeleteGeneration, leaseToken, time.Now().Add(delay), deleteError); err != nil {
		logrus.Errorf("schedule k8s resource deletion %d/%d retry: %v", resource.ID, target.DeleteGeneration, err)
	} else if !updated {
		logrus.Debugf("k8s resource deletion retry %d/%d was superseded", resource.ID, target.DeleteGeneration)
	}
}

func k8sResourceDeleteRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second * time.Duration(1<<(uint(attempt)-1))
	if delay > time.Minute {
		return time.Minute
	}
	return delay
}

func sanitizeK8sResourceDeleteError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 1024 {
		return message[:1024]
	}
	return message
}

// buildFromKubeBlocksExec KubeBlocks组件构建执行方法
// KubeBlocks组件不需要实际的构建过程，直接启动Service
func (m *Manager) buildFromKubeBlocksExec(task *model.Task) error {
	body, ok := task.Body.(model.BuildFromKubeBlocksTaskBody)
	if !ok {
		logrus.Errorf("build_from_kubeblocks body convert to taskbody error")
		return fmt.Errorf("build_from_kubeblocks body convert to taskbody error")
	}

	logger := event.GetManager().GetLogger(body.EventID)

	// 检查服务是否已经启动
	appService := m.store.GetAppService(body.ServiceID)
	if appService != nil && !appService.IsClosed() {
		logger.Info("KubeBlocks component is not closed, can not start", event.GetLastLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return nil
	}

	// 初始化AppService，跳过镜像构建阶段
	newAppService, err := conversion.InitAppService(false, m.dbmanager, body.ServiceID, body.Configs)
	if err != nil {
		logrus.Errorf("KubeBlocks component init create failure:%s", err.Error())
		logger.Error("KubeBlocks component init create failure", event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("KubeBlocks component init create failure")
	}

	newAppService.Logger = logger

	// 注册新的应用服务
	m.store.RegistAppService(newAppService)

	// KubeBlocks组件使用启动控制器，跳过镜像构建过程直接部署K8s资源
	err = m.controllerManager.StartController(controller.TypeStartController, *newAppService)
	if err != nil {
		logrus.Errorf("KubeBlocks component run start controller failure:%s", err.Error())
		logger.Error("KubeBlocks component run start controller failure", event.GetCallbackLoggerOption())
		event.GetManager().ReleaseLogger(logger)
		return fmt.Errorf("KubeBlocks component start failure")
	}

	logrus.Infof("KubeBlocks component(%s) %s working is running.", body.ServiceID, "build_from_kubeblocks")
	return nil
}
