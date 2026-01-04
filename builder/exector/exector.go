// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package exector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/builder/job"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/pkg/component/mq"
	"github.com/goodrain/rainbond/util"

	dbmodel "github.com/goodrain/rainbond/db/model"
	mqclient "github.com/goodrain/rainbond/mq/client"
	workermodel "github.com/goodrain/rainbond/worker/discover/model"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"

	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// MetricTaskNum task number
var MetricTaskNum float64

// MetricErrorTaskNum error run task number
var MetricErrorTaskNum float64

// MetricBackTaskNum back task number
var MetricBackTaskNum float64

// Manager 任务执行管理器
type Manager interface {
	GetMaxConcurrentTask() float64
	GetCurrentConcurrentTask() float64
	AddTask(*pb.TaskMessage) error
	SetReturnTaskChan(func(*pb.TaskMessage))
	Start() error
	Stop() error
	GetImageClient() sources.ImageClient
}

// NewManager new manager
func NewManager() (Manager, error) {
	configDefault := configs.Default()
	imageClient, err := sources.NewImageClient()
	if err != nil {
		return nil, err
	}
	containerdClient := imageClient.GetContainerdClient()
	if containerdClient == nil && configDefault.ChaosConfig.ContainerRuntime == sources.ContainerRuntimeContainerd {
		return nil, fmt.Errorf("containerd client is nil")
	}
	var restConfig *rest.Config // TODO fanyangyang use k8sutil.NewRestConfig
	if configDefault.K8SConfig.KubeConfigPath != "" {
		restConfig, err = clientcmd.BuildConfigFromFlags("", configDefault.K8SConfig.KubeConfigPath)
	} else {
		restConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	rainbondClient, err := versioned.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	numCPU := runtime.NumCPU()
	// 示例逻辑：根据 CPU 核数设置基准最大并发数，实际中可以加上内存的判断
	maxConcurrentTask := numCPU * 2
	stop := make(chan struct{})
	if err := job.InitJobController(configDefault.PublicConfig.RbdNamespace, stop, kubeClient); err != nil {
		cancel()
		return nil, err
	}
	logrus.Infof("The maximum number of concurrent build tasks supported by the current node is %d", maxConcurrentTask)

	return &exectorManager{
		BuildKitImage:     configDefault.ChaosConfig.BuildKitImage,
		BuildKitArgs:      strings.Split(configDefault.ChaosConfig.BuildKitArgs, "&"),
		BuildKitCache:     configDefault.ChaosConfig.BuildKitCache,
		KubeClient:        kubeClient,
		RainbondClient:    rainbondClient,
		mqClient:          mq.Default().MqClient,
		tasks:             make(chan *pb.TaskMessage, maxConcurrentTask),
		maxConcurrentTask: maxConcurrentTask,
		ctx:               ctx,
		cancel:            cancel,
		imageClient:       imageClient,
	}, nil
}

type exectorManager struct {
	BuildKitImage     string
	BuildKitArgs      []string
	BuildKitCache     bool
	KubeClient        kubernetes.Interface
	RainbondClient    versioned.Interface
	tasks             chan *pb.TaskMessage
	callback          func(*pb.TaskMessage)
	maxConcurrentTask int
	mqClient          mqclient.MQClient
	ctx               context.Context
	cancel            context.CancelFunc
	runningTask       sync.Map
	imageClient       sources.ImageClient
}

// TaskWorker worker interface
type TaskWorker interface {
	Run(timeout time.Duration) error
	GetLogger() event.Logger
	Name() string
	Stop() error
	//ErrorCallBack if run error will callback
	ErrorCallBack(err error)
}

var workerCreaterList = make(map[string]func([]byte, *exectorManager) (TaskWorker, error))

// RegisterWorker register worker creator
func RegisterWorker(name string, fun func([]byte, *exectorManager) (TaskWorker, error)) {
	workerCreaterList[name] = fun
}

// ErrCallback do not handle this task
var ErrCallback = fmt.Errorf("callback task to mq")

func (e *exectorManager) SetReturnTaskChan(re func(*pb.TaskMessage)) {
	e.callback = re
}

// TaskType:
// build_from_image build app from docker image
// build_from_source_code build app from source code
// build_from_market_slug build app from app market by download slug
// service_check check service source info
// plugin_image_build build plugin from image
// plugin_dockerfile_build build plugin from dockerfile
// share-slug share app with slug
// share-image share app with image
// build_from_kubeblocks build app from kubeblocks, actually no build action, workload was managed by block-mechanica
func (e *exectorManager) AddTask(task *pb.TaskMessage) error {
	if task.TaskType == "" {
		return nil
	}
	if e.callback != nil && len(e.tasks) > e.maxConcurrentTask {
		e.callback(task)
		time.Sleep(time.Second * 2)
		MetricBackTaskNum++
		return nil
	}
	if e.callback != nil && task.Arch != "" && task.Arch != runtime.GOARCH {
		e.callback(task)
		for len(e.tasks) >= e.maxConcurrentTask {
			time.Sleep(time.Second * 2)
		}
		MetricBackTaskNum++
		return nil
	}
	select {
	case e.tasks <- task:
		MetricTaskNum++
		e.RunTask(task)
		return nil
	default:
		logrus.Infof("The current number of parallel builds exceeds the maximum")
		if e.callback != nil {
			e.callback(task)
			//Wait a while
			//It's best to wait until the current controller can continue adding tasks
			for len(e.tasks) >= e.maxConcurrentTask {
				time.Sleep(time.Second * 2)
			}
			MetricBackTaskNum++
			return nil
		}
		return ErrCallback
	}
}
func (e *exectorManager) runTask(f func(task *pb.TaskMessage), task *pb.TaskMessage, concurrencyControl bool) {
	logrus.Infof("Build task %s in progress", task.TaskId)
	e.runningTask.LoadOrStore(task.TaskId, task)
	if !concurrencyControl {
		<-e.tasks
	} else {
		defer func() { <-e.tasks }()
	}
	f(task)
	e.runningTask.Delete(task.TaskId)
	logrus.Infof("Build task %s is completed", task.TaskId)
}

func (e *exectorManager) runTaskWithErr(f func(task *pb.TaskMessage) error, task *pb.TaskMessage, concurrencyControl bool) {
	if task.TaskType == "" || task.TaskId == "" {
		return
	}
	logrus.Infof("Build task %s in progress", task.TaskId)
	logrus.Infof("[runTask] Starting task execution: task_id=%s, task_type=%s", task.TaskId, task.TaskType)
	e.runningTask.LoadOrStore(task.TaskId, task)
	//Remove a task that is being executed, not necessarily a task that is currently completed
	if !concurrencyControl {
		<-e.tasks
	} else {
		defer func() { <-e.tasks }()
	}
	if err := f(task); err != nil {
		logrus.Errorf("[runTask] Task execution failed: task_id=%s, error=%s", task.TaskId, err.Error())
	}
	e.runningTask.Delete(task.TaskId)
	logrus.Infof("[runTask] Task completed: task_id=%s", task.TaskId)
}
func (e *exectorManager) RunTask(task *pb.TaskMessage) {
	logrus.Infof("[RunTask] Received task from MQ: task_id=%s, task_type=%s", task.TaskId, task.TaskType)

	switch task.TaskType {
	case "build_from_image":
		go e.runTask(e.buildFromImage, task, false)
	case "build_from_vm":
		go e.runTask(e.buildFromVM, task, false)
	case "build_from_source_code":
		go e.runTask(e.buildFromSourceCode, task, true)
	case "build_from_market_slug":
		//deprecated
		go e.runTask(e.buildFromMarketSlug, task, false)
	case "service_check":
		go e.runTask(e.serviceCheck, task, true)
	case "plugin_image_build":
		go e.runTask(e.pluginImageBuild, task, false)
	case "plugin_dockerfile_build":
		go e.runTask(e.pluginDockerfileBuild, task, true)
	case "share-slug":
		//deprecated
		go e.runTask(e.slugShare, task, false)
	case "share-image":
		go e.runTask(e.imageShare, task, false)
	case "load-tar-image":
		logrus.Infof("[RunTask] Dispatching load-tar-image task to handler")
		go e.runTask(e.loadTarImage, task, false)
	case "garbage-collection":
		go e.runTask(e.garbageCollection, task, false)
	case "build_from_kubeblocks":
		go e.runTask(e.buildFromKubeBlocks, task, false)
	case "warmup":
		// 预热任务，用于确保消费循环已经启动，避免 lost wakeup 问题
		// 直接忽略，从任务队列中移除即可
		logrus.Info("[RunTask] Received warmup task, consumer loop is active")
		<-e.tasks // 从队列中移除
	default:
		logrus.Warnf("[RunTask] Unknown task type: %s, using default handler", task.TaskType)
		go e.runTaskWithErr(e.exec, task, false)
	}
}

func (e *exectorManager) exec(task *pb.TaskMessage) error {
	creator, ok := workerCreaterList[task.TaskType]
	if !ok {
		return fmt.Errorf("`%s` tasktype can't support", task.TaskType)
	}
	worker, err := creator(task.TaskBody, e)
	if err != nil {
		logrus.Errorf("create worker for builder error.%s", err)
		return err
	}
	defer event.GetManager().ReleaseLogger(worker.GetLogger())
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			debug.PrintStack()
			worker.GetLogger().Error(util.Translation("Please try again or contact customer service"), map[string]string{"step": "callback", "status": "failure"})
			worker.ErrorCallBack(fmt.Errorf("%s", r))
		}
	}()
	if err := worker.Run(time.Minute * 10); err != nil {
		logrus.Errorf("task type: %s; body: %s; run task: %+v", task.TaskType, task.TaskBody, err)
		MetricErrorTaskNum++
		worker.ErrorCallBack(err)
	}
	return nil
}

// buildFromImage build app from docker image
func (e *exectorManager) buildFromImage(task *pb.TaskMessage) {
	i := NewImageBuildItem(task.TaskBody)
	i.ImageClient = e.imageClient
	i.Logger.Info("Start with the image build application task", map[string]string{"step": "builder-exector", "status": "starting"})
	defer event.GetManager().ReleaseLogger(i.Logger)
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			debug.PrintStack()
			i.Logger.Error(util.Translation("Back end service drift. Please check the rbd-chaos log"), map[string]string{"step": "callback", "status": "failure"})
		}
	}()
	start := time.Now()
	defer func() {
		logrus.Debugf("complete build from source code, consuming time %s", time.Since(start).String())
	}()
	for n := 0; n < 2; n++ {
		err := i.Run(time.Minute * 30)
		if err != nil {
			logrus.Errorf("build from image error: %s", err.Error())
			if n < 1 {
				i.Logger.Error("The application task to build from the mirror failed to execute，will try", map[string]string{"step": "build-exector", "status": "failure"})
			} else {
				MetricErrorTaskNum++
				i.Logger.Error(i.FailCause, map[string]string{"step": "callback", "status": "failure"})
				if err := i.UpdateVersionInfo("failure"); err != nil {
					logrus.Debugf("update version Info error: %s", err.Error())
				}
			}
		} else {
			var configs = make(map[string]string, len(i.Configs))
			for k, v := range i.Configs {
				configs[k] = v.String()
			}
			if err := e.UpdateDeployVersion(i.ServiceID, i.DeployVersion); err != nil {
				logrus.Errorf("Update app service deploy version failure %s, service %s do not auto upgrade", err.Error(), i.ServiceID)
				break
			}
			err = e.sendAction(i.TenantID, i.ServiceID, i.EventID, i.DeployVersion, i.Action, configs, i.Logger)
			if err != nil {
				i.Logger.Error("Send upgrade action failed", map[string]string{"step": "callback", "status": "failure"})
			}
			break
		}
	}
}

// buildFromSourceCode build app from source code
// support git repository
func (e *exectorManager) buildFromSourceCode(task *pb.TaskMessage) {
	// Check if this is a callback from source-scan
	sourceScanCompleted := gjson.GetBytes(task.TaskBody, "source_scan_completed").Bool()
	if sourceScanCompleted {
		// Get scan event logger
		scanEventID := gjson.GetBytes(task.TaskBody, "source_scan_event_id").String()
		scanLogger := event.GetManager().GetLogger(scanEventID)
		defer event.GetManager().ReleaseLogger(scanLogger)

		sourceScanPassed := gjson.GetBytes(task.TaskBody, "source_scan_passed").Bool()

		if sourceScanPassed {
			logrus.Info("Source scan passed, creating build-service event and continuing with build")
			scanLogger.Info("源码安全检测通过", map[string]string{"step": "source-scan-complete", "status": "success"})
			// Update scan event to complete
			if err := e.updateSourceScanEvent(scanEventID, "success", "complete", "源码安全检测通过"); err != nil {
				logrus.Errorf("Failed to update source scan event: %v", err)
			}

			// NOW create the build-service event since scan passed
			buildEventID := gjson.GetBytes(task.TaskBody, "event_id").String()
			tenantID := gjson.GetBytes(task.TaskBody, "tenant_id").String()
			serviceID := gjson.GetBytes(task.TaskBody, "service_id").String()
			operator := gjson.GetBytes(task.TaskBody, "operator").String()

			if err := e.createBuildServiceEvent(buildEventID, tenantID, serviceID, operator); err != nil {
				logrus.Errorf("Failed to create build-service event: %v", err)
				// Continue anyway - event creation failure shouldn't stop build
			} else {
				logrus.Infof("Created build-service event: %s", buildEventID)
			}

			// Continue to build (fall through)
		} else {
			logrus.Warn("Source scan failed, stopping build")
			scanLogger.Error("源码安全检测未通过，请查看检测报告", map[string]string{"step": "source-scan-complete", "status": "failure"})
			// Update scan event to failure
			if err := e.updateSourceScanEvent(scanEventID, "failure", "complete", "源码安全检测未通过"); err != nil {
				logrus.Errorf("Failed to update source scan event: %v", err)
			}
			return
		}
	} else {
		// Check if source code scanning is required
		if e.shouldPerformSourceScan(task.TaskBody) {
			// Create a new source scan event
			scanEventID := util.NewUUID()
			tenantID := gjson.GetBytes(task.TaskBody, "tenant_id").String()
			serviceID := gjson.GetBytes(task.TaskBody, "service_id").String()
			operator := gjson.GetBytes(task.TaskBody, "operator").String()

			logrus.Info("Source code scan required, creating source scan event (build event will be created after scan)")

			// Create source scan event in database
			if err := e.createSourceScanEvent(scanEventID, tenantID, serviceID, operator); err != nil {
				logrus.Errorf("Failed to create source scan event: %v", err)
			}

			// Get logger for the scan event
			scanLogger := event.GetManager().GetLogger(scanEventID)
			defer event.GetManager().ReleaseLogger(scanLogger)
			scanLogger.Info("开始源码安全检测，请稍候...", map[string]string{"step": "source-scan-start", "status": "starting"})

			if err := e.forwardToSourceScan(task, scanEventID); err != nil {
				logrus.Errorf("Failed to forward task to source-scan: %v", err)
				scanLogger.Error("转发源码检测任务失败，将创建构建事件并继续正常构建流程", map[string]string{"step": "source-scan-start", "status": "failure"})
				if err := e.updateSourceScanEvent(scanEventID, "failure", "complete", "转发失败"); err != nil {
					logrus.Errorf("Failed to update source scan event: %v", err)
				}

				// Create build event since scan failed and we'll continue with build
				buildEventID := gjson.GetBytes(task.TaskBody, "event_id").String()
				if err := e.createBuildServiceEvent(buildEventID, tenantID, serviceID, operator); err != nil {
					logrus.Errorf("Failed to create build-service event: %v", err)
				} else {
					logrus.Infof("Created build-service event after scan failure: %s", buildEventID)
				}
				// Continue with normal build if forwarding fails
			} else {
				logrus.Info("Task successfully forwarded to source-scan topic, waiting for scan result")
				// Stop here, wait for scan result
				return
			}
		} else {
			// Source scan not needed, create build event if it was deferred
			buildEventID := gjson.GetBytes(task.TaskBody, "event_id").String()
			tenantID := gjson.GetBytes(task.TaskBody, "tenant_id").String()
			serviceID := gjson.GetBytes(task.TaskBody, "service_id").String()
			operator := gjson.GetBytes(task.TaskBody, "operator").String()

			// Check if event was deferred (plugin installed but this component doesn't need scan)
			// In this case, we still need to create the build event
			if err := e.createBuildServiceEventIfNotExists(buildEventID, tenantID, serviceID, operator); err != nil {
				logrus.Debugf("Build event may already exist or creation not needed: %v", err)
			}
		}
	}

	i := NewSouceCodeBuildItem(task.TaskBody)
	i.ImageClient = e.imageClient
	i.BuildKitImage = e.BuildKitImage
	i.BuildKitArgs = e.BuildKitArgs
	i.BuildKitCache = e.BuildKitCache
	i.KubeClient = e.KubeClient
	i.Ctx = e.ctx
	i.Arch = task.Arch
	i.Logger.Info("Build app version from source code start", map[string]string{"step": "builder-exector", "status": "starting"})
	start := time.Now()
	defer event.GetManager().ReleaseLogger(i.Logger)
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			debug.PrintStack()
			i.Logger.Error(util.Translation("Back end service drift. Please check the rbd-chaos log"), map[string]string{"step": "callback", "status": "failure"})
		}
	}()
	defer func() {
		logrus.Debugf("Complete build from source code, consuming time %s", time.Now().Sub(start).String())
	}()
	err := i.Run(time.Minute * 30)
	if err != nil {
		logrus.Errorf("build from source code error: %s", err.Error())
		i.Logger.Error(i.FailCause, map[string]string{"step": "callback", "status": "failure"})
		vi := &dbmodel.VersionInfo{
			FinalStatus: "failure",
			EventID:     i.EventID,
			CodeBranch:  i.CodeSouceInfo.Branch,
			CodeVersion: i.commit.Hash,
			CommitMsg:   i.commit.Message,
			Author:      i.commit.Author,
			FinishTime:  time.Now(),
		}
		if err := i.UpdateVersionInfo(vi); err != nil {
			logrus.Errorf("update version Info error: %s", err.Error())
			i.Logger.Error(fmt.Sprintf("error updating version info: %v", err), event.GetCallbackLoggerOption())
		}
	} else {
		var configs = make(map[string]string, len(i.Configs))
		for k, v := range i.Configs {
			configs[k] = v.String()
		}
		if err := e.UpdateDeployVersion(i.ServiceID, i.DeployVersion); err != nil {
			logrus.Errorf("Update app service deploy version failure %s, service %s do not auto upgrade", err.Error(), i.ServiceID)
			return
		}
		err = e.sendAction(i.TenantID, i.ServiceID, i.EventID, i.DeployVersion, i.Action, configs, i.Logger)
		if err != nil {
			i.Logger.Error("Send upgrade action failed", map[string]string{"step": "callback", "status": "failure"})
		}
	}
}

// buildFromVM build app from vm
func (e *exectorManager) buildFromVM(task *pb.TaskMessage) {
	v := NewVMBuildItem(task.TaskBody)
	v.ImageClient = e.imageClient
	v.BuildKitImage = e.BuildKitImage
	v.BuildKitArgs = e.BuildKitArgs
	v.BuildKitCache = e.BuildKitCache
	v.kubeClient = e.KubeClient
	v.Logger.Info("Start with the vm build application task", map[string]string{"step": "builder-exector", "status": "starting"})
	defer event.GetManager().ReleaseLogger(v.Logger)
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			v.Logger.Error(util.Translation("Back end service drift. Please check the rbd-chaos log"), map[string]string{"step": "builder-exector", "status": "starting"})
		}
	}()
	start := time.Now()
	defer func() {
		logrus.Debugf("complete build from source code, consuming time %s", time.Since(start).String())
	}()
	if v.VMImageSource != "" {
		err := v.RunVMBuild()
		if err != nil {
			logrus.Errorf("failure")
		}
	}
	var configs = make(map[string]string, len(v.Configs))
	for k, u := range v.Configs {
		configs[k] = u.String()
	}
	if err := e.UpdateDeployVersion(v.ServiceID, v.DeployVersion); err != nil {
		logrus.Errorf("Update app service deploy version failure %s, service %s do not auto upgrade", err.Error(), v.ServiceID)
	}
	err := e.sendAction(v.TenantID, v.ServiceID, v.EventID, v.DeployVersion, v.Action, configs, v.Logger)
	if err != nil {
		v.Logger.Error("Send upgrade action failed", map[string]string{"step": "callback", "status": "failure"})
	}
}

// buildFromMarketSlug build app from market slug
func (e *exectorManager) buildFromMarketSlug(task *pb.TaskMessage) {
	eventID := gjson.GetBytes(task.TaskBody, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("Build app version from market slug start", map[string]string{"step": "builder-exector", "status": "starting"})
	i, err := NewMarketSlugItem(task.TaskBody)
	if err != nil {
		logrus.Error("create build from market slug task error.", err.Error())
		return
	}
	go func() {
		start := time.Now()
		defer event.GetManager().ReleaseLogger(i.Logger)
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r)
				debug.PrintStack()
				i.Logger.Error(util.Translation("Back end service drift. Please check the rbd-chaos log"), map[string]string{"step": "callback", "status": "failure"})
			}
		}()
		defer func() {
			logrus.Debugf("complete build from market slug consuming time %s", time.Now().Sub(start).String())
		}()
		for n := 0; n < 2; n++ {
			err := i.Run()
			if err != nil {
				logrus.Errorf("image share error: %s", err.Error())
				if n < 1 {
					i.Logger.Error("Build app version from market slug failure, will try", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					MetricErrorTaskNum++
					i.Logger.Error("Build app version from market slug failure", map[string]string{"step": "callback", "status": "failure"})
				}
			} else {
				if err := e.UpdateDeployVersion(i.ServiceID, i.DeployVersion); err != nil {
					logrus.Errorf("Update app service deploy version failure %s, service %s do not auto upgrade", err.Error(), i.ServiceID)
					break
				}
				err = e.sendAction(i.TenantID, i.ServiceID, i.EventID, i.DeployVersion, i.Action, i.Configs, i.Logger)
				if err != nil {
					i.Logger.Error("Send upgrade action failed", map[string]string{"step": "callback", "status": "failure"})
				}
				break
			}
		}
	}()

}

// buildFromKubeBlocks builds app from KubeBlocks components
func (e *exectorManager) buildFromKubeBlocks(task *pb.TaskMessage) {
	// 从 task.TaskBody 解析参数
	serviceID := gjson.GetBytes(task.TaskBody, "service_id").String()
	tenantID := gjson.GetBytes(task.TaskBody, "tenant_id").String()
	eventID := gjson.GetBytes(task.TaskBody, "event_id").String()
	deployVersion := gjson.GetBytes(task.TaskBody, "deploy_version").String()
	action := gjson.GetBytes(task.TaskBody, "action").String()

	// 获取日志记录器
	logger := event.GetManager().GetLogger(eventID)

	defer event.GetManager().ReleaseLogger(logger)
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			debug.PrintStack()
			logger.Error("KubeBlocks build task failed", map[string]string{"step": "callback", "status": "failure"})
		}
	}()

	// 参数校验
	if serviceID == "" {
		logger.Error("Service ID is required for KubeBlocks component", map[string]string{"step": "builder-exector", "status": "failure"})
		return
	}
	if deployVersion == "" {
		logger.Error("Deploy version is required for KubeBlocks component", map[string]string{"step": "builder-exector", "status": "failure"})
		return
	}

	var configs = make(map[string]string)
	if configsJson := gjson.GetBytes(task.TaskBody, "configs"); configsJson.Exists() {
		configsJson.ForEach(func(key, value gjson.Result) bool {
			configs[key.String()] = value.String()
			return true
		})
	}

	if err := e.UpdateDeployVersion(serviceID, deployVersion); err != nil {
		logger.Error("Update KubeBlocks deploy version failed", map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("Update KubeBlocks service deploy version failure %s", err.Error())
		return
	}

	err := e.sendAction(tenantID, serviceID, eventID, deployVersion, action, configs, logger)
	if err != nil {
		logger.Error("Send KubeBlocks deployment action failed", map[string]string{"step": "callback", "status": "failure"})
		logrus.Errorf("Send KubeBlocks action failed: %s", err.Error())
	} else {
		logger.Info("KubeBlocks component deployment triggered successfully", map[string]string{"step": "last", "status": "success"})
	}
}

// rollingUpgradeTaskBody upgrade message body type
type rollingUpgradeTaskBody struct {
	TenantID  string   `json:"tenant_id"`
	ServiceID string   `json:"service_id"`
	EventID   string   `json:"event_id"`
	Strategy  []string `json:"strategy"`
}

func (e *exectorManager) sendAction(tenantID, serviceID, eventID, newVersion, actionType string, configs map[string]string, logger event.Logger) error {
	// update build event complete status
	logger.Info("Build success", map[string]string{"step": "last", "status": "success"})
	switch actionType {
	case "upgrade":
		//add upgrade event
		event := &dbmodel.ServiceEvent{
			EventID:   util.NewUUID(),
			TenantID:  tenantID,
			ServiceID: serviceID,
			CreatedAt: time.Now().Format(time.RFC3339),
			StartTime: time.Now().Format(time.RFC3339),
			OptType:   "upgrade",
			Target:    "service",
			TargetID:  serviceID,
			UserName:  "",
			SynType:   dbmodel.ASYNEVENTTYPE,
		}
		if err := db.GetManager().ServiceEventDao().AddModel(event); err != nil {
			logrus.Errorf("create upgrade event failure %s, service %s do not auto upgrade", err.Error(), serviceID)
			return nil
		}
		body := workermodel.RollingUpgradeTaskBody{
			TenantID:         tenantID,
			ServiceID:        serviceID,
			NewDeployVersion: newVersion,
			EventID:          event.EventID,
			Configs:          configs,
		}
		if err := e.mqClient.SendBuilderTopic(mqclient.TaskStruct{
			Topic:    mqclient.WorkerTopic,
			TaskType: "rolling_upgrade", // TODO(huangrh 20190816): Separate from build
			TaskBody: body,
		}); err != nil {
			return err
		}
		return nil
	default:
	}
	return nil
}

// slugShare share app of slug
func (e *exectorManager) slugShare(task *pb.TaskMessage) {
	i, err := NewSlugShareItem(task.TaskBody)
	if err != nil {
		logrus.Error("create share image task error.", err.Error())
		return
	}
	i.Logger.Info("开始分享应用", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	go func() {
		defer event.GetManager().ReleaseLogger(i.Logger)
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(r)
				debug.PrintStack()
				i.Logger.Error("后端服务开小差，请重试或联系客服", map[string]string{"step": "callback", "status": "failure"})
			}
		}()
		for n := 0; n < 2; n++ {
			err := i.ShareService()
			if err != nil {
				logrus.Errorf("image share error: %s", err.Error())
				if n < 1 {
					i.Logger.Error("应用分享失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
				} else {
					MetricErrorTaskNum++
					i.Logger.Error("分享应用任务执行失败", map[string]string{"step": "builder-exector", "status": "failure"})
					status = "failure"
				}
			} else {
				status = "success"
				break
			}
		}
		if err := i.UpdateShareStatus(status); err != nil {
			logrus.Debugf("Add image share result error: %s", err.Error())
		}
	}()
}

// imageShare share app of docker image
func (e *exectorManager) imageShare(task *pb.TaskMessage) {
	i, err := NewImageShareItem(task.TaskBody, e.imageClient)
	if err != nil {
		logrus.Error("create share image task error.", err.Error())
		i.Logger.Error(util.Translation("create share image task error"), map[string]string{"step": "builder-exector", "status": "failure"})
		return
	}
	i.Logger.Info("开始分享应用", map[string]string{"step": "builder-exector", "status": "starting"})
	status := "success"
	defer event.GetManager().ReleaseLogger(i.Logger)
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			i.Logger.Error("后端服务开小差，请重试或联系客服", map[string]string{"step": "callback", "status": "failure"})
		}
	}()
	for n := 0; n < 2; n++ {
		err := i.ShareService()
		if err != nil {
			logrus.Errorf("image share error: %s", err.Error())
			if n < 1 {
				i.Logger.Error("应用分享失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
			} else {
				MetricErrorTaskNum++
				i.Logger.Error("分享应用任务执行失败", map[string]string{"step": "builder-exector", "status": "failure"})
				status = "failure"
			}
		} else {
			status = "success"
			break
		}
	}
	if err := i.UpdateShareStatus(status); err != nil {
		logrus.Debugf("Add image share result error: %s", err.Error())
	}
}

func (e *exectorManager) garbageCollection(task *pb.TaskMessage) {
	gci, err := NewGarbageCollectionItem(task.TaskBody)
	if err != nil {
		logrus.Warningf("create a new GarbageCollectionItem: %v", err)
	}

	go func() {
		// delete docker log file and event log file
		gci.delLogFile()
		// volume data
		gci.delVolumeData()
	}()
}

func (e *exectorManager) Start() error {
	return nil
}
func (e *exectorManager) Stop() error {
	e.cancel()
	logrus.Info("Waiting for all threads to exit.")
	//Recycle all ongoing tasks
	e.runningTask.Range(func(k, v interface{}) bool {
		task := v.(*pb.TaskMessage)
		e.callback(task)
		return true
	})
	logrus.Info("All threads is exited.")
	return nil
}

func (e *exectorManager) GetImageClient() sources.ImageClient {
	return e.imageClient
}

func (e *exectorManager) GetMaxConcurrentTask() float64 {
	return float64(e.maxConcurrentTask)
}

func (e *exectorManager) GetCurrentConcurrentTask() float64 {
	return float64(len(e.tasks))
}

func (e *exectorManager) UpdateDeployVersion(serviceID, newVersion string) error {
	return db.GetManager().TenantServiceDao().UpdateDeployVersion(serviceID, newVersion)
}

// shouldPerformSourceScan checks if source code scanning should be performed
func (e *exectorManager) shouldPerformSourceScan(taskBody []byte) bool {
	// Check if source scan plugin is installed
	sourceScanURL := e.getSourceScanPluginURL()
	if sourceScanURL == "" {
		logrus.Debug("Source scan plugin not installed, skipping source scan")
		return false
	}

	// Check if source scan has already been completed
	sourceScanCompleted := gjson.GetBytes(taskBody, "source_scan_completed").Bool()
	if sourceScanCompleted {
		logrus.Debug("Source scan already completed, skipping")
		return false
	}

	// Get service ID from task body
	serviceID := gjson.GetBytes(taskBody, "service_alias").String()
	if serviceID == "" {
		logrus.Warn("Service ID not found in task body, skipping source scan")
		return false
	}

	// Make HTTP request to check if scanning is enabled
	checkURL := fmt.Sprintf("%s/api/v1/components/%s", sourceScanURL, serviceID)
	resp, err := e.httpGet(checkURL)
	if err != nil {
		logrus.Warnf("Failed to check source scan status: %v, skipping source scan", err)
		return false
	}

	// Parse response to check if enabled
	enabled := gjson.Get(resp, "data.enabled").Bool()
	return enabled
}

// getSourceScanPluginURL retrieves the source scan plugin URL from rbdplugin
// Returns empty string if plugin is not installed or Backend is empty
func (e *exectorManager) getSourceScanPluginURL() string {
	const pluginName = "rainbond-sourcescan"

	// Try to get plugin from rbdplugin
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
	defer cancel()

	plugin, err := e.RainbondClient.RainbondV1alpha1().RBDPlugins(metav1.NamespaceAll).Get(ctx, pluginName, metav1.GetOptions{})
	if err != nil {
		logrus.Debugf("Source scan plugin %s not found: %v", pluginName, err)
		return ""
	}

	// Get Backend URL from plugin spec
	if plugin.Spec.Backend == "" {
		logrus.Debugf("Source scan plugin %s found but Backend is empty", pluginName)
		return ""
	}

	logrus.Infof("Using source scan URL from rbdplugin: %s", plugin.Spec.Backend)
	return plugin.Spec.Backend
}

// createSourceScanEvent creates a new source scan event in database
func (e *exectorManager) createSourceScanEvent(eventID, tenantID, serviceID, operator string) error {
	now := time.Now().Format(time.RFC3339)
	event := &dbmodel.ServiceEvent{
		EventID:     eventID,
		TenantID:    tenantID,
		ServiceID:   serviceID,
		Target:      dbmodel.TargetTypeService,
		TargetID:    serviceID,
		UserName:    operator,
		StartTime:   now,
		CreatedAt:   now,
		SynType:     dbmodel.ASYNEVENTTYPE,
		OptType:     "source-scan",
		FinalStatus: "",
		Status:      "",
	}

	return db.GetManager().ServiceEventDao().AddModel(event)
}

// updateSourceScanEvent updates source scan event status
func (e *exectorManager) updateSourceScanEvent(eventID, status, finalStatus, message string) error {
	logrus.Infof("Updating source scan event %s: status=%s, finalStatus=%s, message=%s", eventID, status, finalStatus, message)

	// Use Updates() instead of UpdateModel to only update specified fields
	// This avoids overwriting other fields with zero values
	updates := map[string]interface{}{
		"end_time":     time.Now().Format(time.RFC3339),
		"status":       status,
		"final_status": finalStatus,
		"message":      message,
	}

	err := db.GetManager().DB().Model(&dbmodel.ServiceEvent{}).
		Where("event_id = ?", eventID).
		Updates(updates).Error

	if err != nil {
		logrus.Errorf("Failed to update source scan event %s: %v", eventID, err)
	} else {
		logrus.Infof("Successfully updated source scan event %s", eventID)

		// Verify the update
		updatedEvent, verifyErr := db.GetManager().ServiceEventDao().GetEventByEventID(eventID)
		if verifyErr != nil {
			logrus.Warnf("Failed to verify event update: %v", verifyErr)
		} else if updatedEvent != nil {
			logrus.Infof("Verified event %s: OptType=%s, Status=%s, FinalStatus=%s",
				eventID, updatedEvent.OptType, updatedEvent.Status, updatedEvent.FinalStatus)
		} else {
			logrus.Warnf("Event %s not found after update!", eventID)
		}
	}

	return err
}

// createBuildServiceEvent creates a new build-service event in database
func (e *exectorManager) createBuildServiceEvent(eventID, tenantID, serviceID, operator string) error {
	now := time.Now().Format(time.RFC3339)
	event := &dbmodel.ServiceEvent{
		EventID:     eventID,
		TenantID:    tenantID,
		ServiceID:   serviceID,
		Target:      dbmodel.TargetTypeService,
		TargetID:    serviceID,
		UserName:    operator,
		StartTime:   now,
		CreatedAt:   now,
		SynType:     dbmodel.ASYNEVENTTYPE,
		OptType:     "build-service",
		FinalStatus: "",
		Status:      "",
	}

	return db.GetManager().ServiceEventDao().AddModel(event)
}

// createBuildServiceEventIfNotExists creates build-service event only if it doesn't already exist
func (e *exectorManager) createBuildServiceEventIfNotExists(eventID, tenantID, serviceID, operator string) error {
	// Check if event already exists
	existingEvent, err := db.GetManager().ServiceEventDao().GetEventByEventID(eventID)
	if err == nil && existingEvent != nil {
		logrus.Debugf("Build-service event %s already exists, skipping creation", eventID)
		return nil
	}

	// Event doesn't exist, create it
	logrus.Infof("Creating build-service event %s (was deferred)", eventID)
	return e.createBuildServiceEvent(eventID, tenantID, serviceID, operator)
}

// forwardToSourceScan forwards the build task to source-scan topic
func (e *exectorManager) forwardToSourceScan(task *pb.TaskMessage, scanEventID string) error {
	logrus.Infof("Forwarding task to source-scan, original TaskBody length: %d bytes", len(task.TaskBody))

	// Unmarshal TaskBody to avoid double serialization
	var taskBodyMap map[string]interface{}
	if err := json.Unmarshal(task.TaskBody, &taskBodyMap); err != nil {
		logrus.Errorf("Failed to unmarshal task body: %v", err)
		return fmt.Errorf("failed to unmarshal task body: %w", err)
	}

	// Add source scan event ID
	taskBodyMap["source_scan_event_id"] = scanEventID

	// Log key fields being forwarded
	serviceID, _ := taskBodyMap["service_id"].(string)
	repoURL, _ := taskBodyMap["repo_url"].(string)
	branch, _ := taskBodyMap["branch"].(string)
	logrus.Infof("Forwarding source scan task - service_id: %s, repo_url: %s, branch: %s, scan_event_id: %s", serviceID, repoURL, branch, scanEventID)

	// Create source scan task
	sourceScanTask := mqclient.TaskStruct{
		Topic:    mqclient.SourceScanTopic,
		TaskType: "source_code_scan",
		TaskBody: taskBodyMap,
		Arch:     task.Arch,
	}

	// Send to source-scan topic
	if err := e.mqClient.SendBuilderTopic(sourceScanTask); err != nil {
		logrus.Errorf("Failed to send to source-scan topic: %v", err)
		return fmt.Errorf("failed to send task to source-scan topic: %w", err)
	}

	logrus.Infof("Successfully forwarded task to source-scan topic")
	return nil
}

// httpGet performs an HTTP GET request and returns the response body as string
func (e *exectorManager) httpGet(url string) (string, error) {
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
