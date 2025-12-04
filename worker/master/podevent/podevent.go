package podevent

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	utils "github.com/goodrain/rainbond/util"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sort"
	"strings"
	"time"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/goodrain/rainbond/worker/server/pb"
	wutil "github.com/goodrain/rainbond/worker/util"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// EventType -
type EventType string

// String -
func (p EventType) String() string {
	return string(p)
}

// EventTypeOOMKilled -
var EventTypeOOMKilled EventType = "OOMKilled"

// EventTypeCrashLoopBackOff -
var EventTypeCrashLoopBackOff EventType = "CrashLoopBackOff"

// EventTypeAbnormalExited container exits abnormally
var EventTypeAbnormalExited EventType = "AbnormalExited"

// EventTypeLivenessProbeFailed -
var EventTypeLivenessProbeFailed EventType = "LivenessProbeFailed"

// EventTypeReadinessProbeFailed -
var EventTypeReadinessProbeFailed EventType = "ReadinessProbeFailed"

// EventTypeAbnormalRecovery -
var EventTypeAbnormalRecovery EventType = "AbnormalRecovery"

// SortableEventType implements sort.Interface for []EventType
type SortableEventType []EventType

var eventTypeTbl = map[EventType]int{
	EventTypeLivenessProbeFailed:  0,
	EventTypeReadinessProbeFailed: 1,
	EventTypeOOMKilled:            2,
	EventTypeAbnormalExited:       3,
}

func (s SortableEventType) Len() int {
	return len(s)
}
func (s SortableEventType) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s SortableEventType) Less(i, j int) bool {
	return eventTypeTbl[s[i]] > eventTypeTbl[s[j]]
}

type optType struct {
	eventType   EventType
	containerID string
	image       string
	message     string
}

// PodEvent -
type PodEvent struct {
	clientset  kubernetes.Interface
	stopCh     chan struct{}
	podEventCh chan *corev1.Pod
}

// New create a new PodEvent
func New(stopCh chan struct{}) *PodEvent {
	return &PodEvent{
		clientset:  k8s.Default().Clientset,
		stopCh:     stopCh,
		podEventCh: make(chan *corev1.Pod, 100),
	}
}

// Handle -
func (p *PodEvent) Handle() {
	for {
		select {
		case pod := <-p.podEventCh:
			// Extend monitoring window: record events from 5 seconds to 30 minutes after startup
			// This catches immediate failures faster and monitors long-running issues longer
			podAge := time.Now().Sub(pod.CreationTimestamp.Time)
			if podAge > 5*time.Second && podAge < 30*time.Minute {
				recordUpdateEvent(p.clientset, pod, defDetermineOptType)
				AbnormalEvent(p.clientset, pod)
			}
		case <-p.stopCh:
			return
		}
	}
}

// GetChan get pod update chan
func (p *PodEvent) GetChan() chan<- *corev1.Pod {
	return p.podEventCh
}

// recordUpdateEvent -
func recordUpdateEvent(clientset kubernetes.Interface, pod *corev1.Pod, f determineOptType) {
	evt, err := db.GetManager().ServiceEventDao().LatestFailurePodEvent(pod.GetName())
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.Warningf("error fetching latest unfinished pod event: %v", err)
		return
	}
	podstatus := new(pb.PodStatus)
	// Non-platform created components do not log events
	tenantID, serviceID, _, _ := k8sutil.ExtractLabels(pod.GetLabels())
	if tenantID == "" || serviceID == "" {
		logrus.Debugf("pod: %s; tenantID or serviceID is empty", pod.GetName())
		return
	}
	wutil.DescribePodStatus(clientset, pod, podstatus, k8sutil.DefListEventsByPod)
	// the pod in the pending status has no start time and container statuses
	if podstatus.Type == pb.PodStatus_ABNORMAL || podstatus.Type == pb.PodStatus_NOTREADY || podstatus.Type == pb.PodStatus_UNHEALTHY {
		var eventID string
		// determine the type of exception event that occurs by the state of multiple containers
		optType := f(clientset, pod, k8sutil.DefListEventsByPod)
		if optType == nil {
			return
		}

		if evt == nil { // create event
			eventID, err = createSystemEvent(tenantID, serviceID, pod.GetName(), optType.eventType.String(), model.EventStatusFailure.String(), optType.message)
			if err != nil {
				logrus.Warningf("pod: %s; type: %s; error creating event: %v", pod.GetName(), optType.eventType.String(), err)
				return
			}
		} else {
			eventID = evt.EventID
		}

		// Translate the error message to user-friendly format
		userMsg := translateRuntimeError(optType.eventType.String(), optType.message)
		detailMsg := fmt.Sprintf("image: %s; container: %s; state: %s; message: %s", optType.image, optType.containerID, optType.eventType.String(), userMsg)
		logger := event.GetManager().GetLogger(eventID)
		defer event.GetManager().ReleaseLogger(logger)
		logrus.Debugf("Service id: %s; %s.", serviceID, detailMsg)
		logger.Error(userMsg, event.GetLoggerOption("failure"))
	} else if podstatus.Type == pb.PodStatus_RUNNING {
		if evt == nil {
			return
		}

		// running time
		var rtime metav1.Time
		for _, condition := range pod.Status.Conditions {
			if condition.Type != corev1.PodReady || condition.Status != corev1.ConditionTrue {
				continue
			}
			if condition.LastTransitionTime.IsZero() {
				continue
			}
			rtime = condition.LastTransitionTime
		}

		// the container state of the pod in the PodStatus_Running must be running
		msg := fmt.Sprintf("state: running; started at: %s", rtime.Format(time.RFC3339))
		logger := event.GetManager().GetLogger(evt.EventID)
		defer event.GetManager().ReleaseLogger(logger)
		logrus.Debugf("Service id: %s; %s.", serviceID, msg)
		loggerOpt := event.GetLoggerOption("failure")

		if !rtime.IsZero() && time.Now().Sub(rtime.Time) > 2*time.Minute {
			evt.FinalStatus = model.EventFinalStatusEmptyComplete.String()
			if err := db.GetManager().ServiceEventDao().UpdateModel(evt); err != nil {
				logrus.Warningf("event id: %s; failed to update service event: %v", evt.EventID, err)
			} else {
				loggerOpt = event.GetCallbackLoggerOption()
				_, err := createSystemEvent(tenantID, serviceID, pod.GetName(), EventTypeAbnormalRecovery.String(), model.EventStatusSuccess.String(), "")
				if err != nil {
					logrus.Warningf("pod: %s; type: %s; error creating event: %v", pod.GetName(), EventTypeAbnormalRecovery.String(), err)
					return
				}
			}
		}
		logger.Info(msg, loggerOpt)
	}
}

// AbnormalEvent -
func AbnormalEvent(clientset kubernetes.Interface, pod *corev1.Pod) {
	// Non-platform created components do not log events
	tenantID, serviceID, _, _ := k8sutil.ExtractLabels(pod.GetLabels())
	if tenantID == "" || serviceID == "" {
		logrus.Debugf("pod: %s; tenantID or serviceID is empty", pod.GetName())
		return
	}
	if pod != nil && pod.Status.Phase == corev1.PodPending {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodScheduled && condition.Status == "False" {
				var msg string
				// Use more detailed and user-friendly messages with internationalization support
				if strings.Contains(condition.Message, "affinity/selector") {
					msg = util.Translation("Deployment failed: node affinity not satisfied")
				} else if strings.Contains(condition.Message, "Insufficient cpu") {
					msg = util.Translation("Deployment failed: insufficient CPU resources")
				} else if strings.Contains(condition.Message, "Insufficient memory") {
					msg = util.Translation("Deployment failed: insufficient memory resources")
				} else if strings.Contains(condition.Message, "cpu") {
					msg = util.Translation("Deployment failed: insufficient CPU resources")
				} else if strings.Contains(condition.Message, "memory") {
					msg = util.Translation("Deployment failed: insufficient memory resources")
				} else if strings.Contains(condition.Message, "PersistentVolumeClaim") || strings.Contains(condition.Message, "persistentvolumeclaim") {
					msg = util.Translation("Deployment failed: persistent volume claim is pending")
				} else if strings.Contains(condition.Message, "tolerate") || strings.Contains(condition.Message, "taint") {
					msg = util.Translation("Deployment failed: node has taints")
				} else if strings.Contains(condition.Message, "node(s)") && strings.Contains(condition.Message, "0") {
					msg = util.Translation("Deployment failed: no nodes available for scheduling")
				} else if strings.Contains(condition.Message, "Insufficient") {
					msg = util.Translation("Deployment failed: insufficient storage resources")
				} else {
					// For other scheduling failures, provide the original message
					msg = fmt.Sprintf("%s: %s", util.Translation("Pod scheduling failed"), condition.Message)
				}
				unSchedulableEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, "Unschedulable")
				if err != nil && err != gorm.ErrRecordNotFound {
					logrus.Warningf("error fetching latest unfinished pod event: %v", err)
					return
				}
				if unSchedulableEvent != nil {
					return
				}
				_, err = createSystemEvent(tenantID, serviceID, pod.Name, condition.Reason, model.EventStatusFailure.String(), msg)
				if err != nil {
					logrus.Warningf("pod: %s; type: %s; error creating event: %v", pod.GetName(), EventTypeAbnormalRecovery.String(), err)
					return
				}
			}
			if condition.Type == corev1.PodInitialized && condition.Status == "False" {
				serviceRelations, err := db.GetManager().TenantServiceRelationDao().GetTenantServiceRelations(serviceID)
				if err != nil && err != gorm.ErrRecordNotFound {
					logrus.Warningf("error fetching service relations: %v", err)
					return
				}
				var serviceIDs []string
				if serviceRelations != nil {
					for _, serviceRelation := range serviceRelations {
						serviceIDs = append(serviceIDs, serviceRelation.DependServiceID)
					}
				} else {
					return
				}
				services, err := db.GetManager().TenantServiceDao().GetServiceByIDs(serviceIDs)
				if err != nil {
					logrus.Warningf("get service relations ids: %v", err)
					return
				}
				var serviceNames []string
				for _, service := range services {
					tenant, err := db.GetManager().TenantDao().GetTenantByUUID(service.TenantID)
					if err != nil {
						logrus.Warningf("get service event tenant info error: %v", err)
					}
					servicePods, err := clientset.CoreV1().Pods(tenant.Namespace).List(context.Background(), metav1.ListOptions{
						LabelSelector: fields.SelectorFromSet(map[string]string{
							"service_id": service.ServiceID,
						}).String(),
					})
					if err != nil {
						logrus.Warningf("get service relations pods error: %v", err)
						return
					}
					for _, servicePod := range servicePods.Items {
						if servicePod.Status.Phase == corev1.PodPending {
							serviceNames = append(serviceNames, service.ServiceAlias)
						}
					}
				}
				if serviceNames == nil {
					return
				}
				msg := strings.Join(serviceNames, ",")
				// Update component Waiting for startup events
				initEvents, err := db.GetManager().ServiceEventDao().GetAppointEvent(serviceID, "failure", "INITIATING")
				if err != nil && err != gorm.ErrRecordNotFound {
					logrus.Warningf("get start time out event error: %v", err)
				}
				if initEvents != nil {
					initEvents.Message = msg
					err = db.GetManager().ServiceEventDao().UpdateModel(initEvents)
					if err != nil {
						logrus.Warningf("update start time out event error: %v", err)
					}
					return
				}
				_, err = createSystemEvent(tenantID, serviceID, pod.Name, "INITIATING", model.EventStatusFailure.String(), msg)
				if err != nil {
					logrus.Warningf("pod: %s; type: %s; error creating event: %v", pod.GetName(), EventTypeAbnormalRecovery.String(), err)
					return
				}
			}
		}
	}
	if pod != nil && pod.Status.Phase == corev1.PodRunning {
		servicePods, err := clientset.CoreV1().Pods(pod.GetNamespace()).List(context.Background(), metav1.ListOptions{
			FieldSelector: "metadata.namespace!=" + utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace),
			LabelSelector: fields.SelectorFromSet(map[string]string{
				"service_id": serviceID,
			}).String(),
		})
		if err != nil {
			return
		}
		allPodRunning := true
		allCtrRunning := true
		for _, servicePod := range servicePods.Items {
			if servicePod.Status.Phase != corev1.PodRunning {
				allPodRunning = false
			}
			for _, containerStatus := range servicePod.Status.ContainerStatuses {
				if containerStatus.State.Running == nil {
					allCtrRunning = false
				}
			}
		}
		if allPodRunning {
			err = db.GetManager().ServiceEventDao().DelAbnormalEvent(serviceID, "Unschedulable")
			if err != nil {
				logrus.Warningf("Delete component scheduling pod event: %v", err)
			}
			// Update last startup component timeout events
			startEvents, err := db.GetManager().ServiceEventDao().GetAppointEvent(serviceID, "timeout", "start-service")
			if err != nil && err != gorm.ErrRecordNotFound {
				logrus.Warningf("get start time out event error: %v", err)
			}
			if startEvents != nil {
				startEvents.Message = "Start service success"
				startEvents.Status = "success"
				err = db.GetManager().ServiceEventDao().UpdateModel(startEvents)
				if err != nil {
					logrus.Warningf("update start time out event error: %v", err)
				}
			}
			// delete Waiting for start event
			err = db.GetManager().ServiceEventDao().DelAbnormalEvent(serviceID, "INITIATING")
			if err != nil && err != gorm.ErrRecordNotFound {
				logrus.Warningf("delete INITIATING event error: %v", err)
				return
			}
		}
		if allCtrRunning {
			err := db.GetManager().ServiceEventDao().DelAbnormalEvent(serviceID, "CrashLoopBackOff")
			if err != nil {
				logrus.Warningf("Delete component scheduling pod event: %v", err)
			}
		}
		// Check for runtime container issues
		for _, containerStatus := range pod.Status.ContainerStatuses {
			// CrashLoopBackOff - container keeps crashing
			if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == EventTypeCrashLoopBackOff.String() {
				unSchedulableEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, "CrashLoopBackOff")
				if err != nil && err != gorm.ErrRecordNotFound {
					logrus.Warningf("error fetching latest unfinished pod event: %v", err)
					return
				}
				if unSchedulableEvent != nil {
					return
				}
				msg := util.Translation("Deployment failed: container is being terminated repeatedly")
				eventID, err := createSystemEvent(tenantID, serviceID, pod.Name, containerStatus.State.Waiting.Reason, model.EventStatusFailure.String(), msg)
				if err != nil {
					logrus.Warningf("pod: %s; type: %s; error creating event: %v", pod.GetName(), EventTypeAbnormalRecovery.String(), err)
					return
				}
				// 记录 Pod 容器最后 300 行日志（CrashLoopBackOff 需要获取上一个实例的日志）
				go recordPodLogsToEventWithPrevious(clientset, pod, eventID, containerStatus.Name)
			}

			// ImagePullBackOff - cannot pull image during runtime
			if containerStatus.State.Waiting != nil && (containerStatus.State.Waiting.Reason == "ImagePullBackOff" || containerStatus.State.Waiting.Reason == "ErrImagePull") {
				imagePullEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, "ImagePullBackOff")
				if err != nil && err != gorm.ErrRecordNotFound {
					logrus.Warningf("error fetching image pull event: %v", err)
					return
				}
				if imagePullEvent != nil {
					return
				}
				msg := util.Translation("Deployment failed: image pull failed")
				if strings.Contains(containerStatus.State.Waiting.Message, "not found") {
					msg = util.Translation("Deployment failed: image not found")
				} else if strings.Contains(containerStatus.State.Waiting.Message, "unauthorized") {
					msg = util.Translation("Deployment failed: image pull authentication failed")
				}
				_, err = createSystemEvent(tenantID, serviceID, pod.Name, "ImagePullBackOff", model.EventStatusFailure.String(), msg)
				if err != nil {
					logrus.Warningf("pod: %s; type: ImagePullBackOff; error creating event: %v", pod.GetName(), err)
					return
				}
			}

			// CreateContainerConfigError - container config issue
			if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CreateContainerConfigError" {
				configErrorEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, "CreateContainerConfigError")
				if err != nil && err != gorm.ErrRecordNotFound {
					logrus.Warningf("error fetching container config error event: %v", err)
					return
				}
				if configErrorEvent != nil {
					return
				}
				msg := util.Translation("Deployment failed: container configuration error")
				_, err = createSystemEvent(tenantID, serviceID, pod.Name, "CreateContainerConfigError", model.EventStatusFailure.String(), msg)
				if err != nil {
					logrus.Warningf("pod: %s; type: CreateContainerConfigError; error creating event: %v", pod.GetName(), err)
					return
				}
			}

			// OOMKilled - out of memory during runtime
			if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.Reason == "OOMKilled" {
				oomEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, "OOMKilled")
				if err != nil && err != gorm.ErrRecordNotFound {
					logrus.Warningf("error fetching OOM event: %v", err)
					return
				}
				if oomEvent != nil {
					return
				}
				msg := util.Translation("Deployment failed: container out of memory killed")
				eventID, err := createSystemEvent(tenantID, serviceID, pod.Name, "OOMKilled", model.EventStatusFailure.String(), msg)
				if err != nil {
					logrus.Warningf("pod: %s; type: OOMKilled; error creating event: %v", pod.GetName(), err)
					return
				}
				// 记录 Pod 容器最后 300 行日志
				go recordPodLogsToEvent(clientset, pod, eventID, containerStatus.Name)
			}

			// Container terminated with error
			if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 && containerStatus.State.Terminated.Reason != "OOMKilled" {
				exitErrorEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, "ContainerExitError")
				if err != nil && err != gorm.ErrRecordNotFound {
					logrus.Warningf("error fetching container exit error event: %v", err)
					return
				}
				if exitErrorEvent != nil {
					return
				}
				msg := fmt.Sprintf("%s (exit code: %d)", util.Translation("Deployment failed: container startup failed"), containerStatus.State.Terminated.ExitCode)
				eventID, err := createSystemEvent(tenantID, serviceID, pod.Name, "ContainerExitError", model.EventStatusFailure.String(), msg)
				if err != nil {
					logrus.Warningf("pod: %s; type: ContainerExitError; error creating event: %v", pod.GetName(), err)
					return
				}
				// 记录 Pod 容器最后 300 行日志
				go recordPodLogsToEvent(clientset, pod, eventID, containerStatus.Name)
			}
		}

		// Check for Pod-level issues
		// Pod Evicted - node ran out of resources
		if pod.Status.Reason == "Evicted" {
			evictedEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, "Evicted")
			if err != nil && err != gorm.ErrRecordNotFound {
				logrus.Warningf("error fetching evicted event: %v", err)
				return
			}
			if evictedEvent != nil {
				return
			}
			var msg string
			if strings.Contains(pod.Status.Message, "memory") {
				msg = util.Translation("Deployment failed: container out of memory killed")
			} else if strings.Contains(pod.Status.Message, "disk") {
				msg = util.Translation("Deployment failed: insufficient storage resources")
			} else {
				msg = fmt.Sprintf("%s: %s", util.Translation("Pod scheduling failed"), pod.Status.Message)
			}
			_, err = createSystemEvent(tenantID, serviceID, pod.Name, "Evicted", model.EventStatusFailure.String(), msg)
			if err != nil {
				logrus.Warningf("pod: %s; type: Evicted; error creating event: %v", pod.GetName(), err)
				return
			}
		}
	}
}

// translateRuntimeError translates runtime event types to user-friendly messages
func translateRuntimeError(eventType, message string) string {
	switch eventType {
	case "OOMKilled":
		return util.Translation("Deployment failed: container out of memory killed")
	case "CrashLoopBackOff":
		return util.Translation("Deployment failed: container is being terminated repeatedly")
	case "AbnormalExited":
		return util.Translation("Deployment failed: container startup failed")
	case "LivenessProbeFailed":
		return util.Translation("Deployment failed: liveness probe failed")
	case "ReadinessProbeFailed":
		return util.Translation("Deployment failed: readiness probe failed")
	case "ImagePullBackOff":
		return util.Translation("Deployment failed: image pull failed")
	case "CreateContainerConfigError":
		return util.Translation("Deployment failed: container configuration error")
	case "Evicted":
		return util.Translation("Deployment failed: insufficient storage resources")
	case "ContainerExitError":
		return util.Translation("Deployment failed: container startup failed")
	default:
		if message != "" {
			return message
		}
		return eventType
	}
}

// determine the type of exception
type determineOptType func(clientset kubernetes.Interface, pod *corev1.Pod, f k8sutil.ListEventsByPod) *optType

// defDetermineOptType -
func defDetermineOptType(clientset kubernetes.Interface, pod *corev1.Pod, f k8sutil.ListEventsByPod) *optType {
	oneContainerOptType := func(state corev1.ContainerState) (EventType, string) {
		if state.Terminated != nil {
			if state.Terminated.Reason == EventTypeOOMKilled.String() {
				return EventTypeOOMKilled, state.Terminated.Reason
			}
			if state.Terminated.ExitCode != 0 {
				return EventTypeAbnormalExited, state.Terminated.Reason
			}
		}
		events := f(clientset, pod)
		if events != nil {
			for _, evt := range events.Items {
				if strings.Contains(evt.Message, "Liveness probe failed") && state.Waiting != nil {
					return EventTypeLivenessProbeFailed, evt.Message
				}
				if strings.Contains(evt.Message, "Readiness probe failed") {
					return EventTypeReadinessProbeFailed, evt.Message
				}
			}
		}
		return "", ""
	}

	var optTypes []*optType
	for _, cs := range pod.Status.ContainerStatuses {
		eventType, reason := oneContainerOptType(cs.State)
		if eventType == "" {
			continue
		}
		optTypes = append(optTypes, &optType{
			eventType:   eventType,
			containerID: cs.ContainerID,
			image:       cs.Image,
			message:     reason,
		})
	}

	if len(optTypes) == 0 {
		return nil
	}

	// sorts data
	keys := make([]EventType, 0, len(optTypes))
	optTypeMap := make(map[EventType]*optType)
	for _, optType := range optTypes {
		keys = append(keys, optType.eventType)
		// conflict with same event type
		optTypeMap[optType.eventType] = optType
	}
	sort.Sort(SortableEventType(keys))

	return optTypeMap[keys[0]]
}

// createSystemEvent -
func createSystemEvent(tenantID, serviceID, targetID, optType, status, msg string) (eventID string, err error) {
	eventID = util.NewUUID()
	et := &model.ServiceEvent{
		EventID:     eventID,
		TenantID:    tenantID,
		ServiceID:   serviceID,
		Target:      model.TargetTypePod,
		TargetID:    targetID,
		UserName:    model.UsernameSystem,
		OptType:     optType,
		Status:      status,
		FinalStatus: model.EventFinalStatusEmpty.String(),
		Message:     msg,
		CreatedAt:   time.Now().Format(time.RFC3339),
		StartTime:   time.Now().Format(time.RFC3339),
	}
	if err = db.GetManager().ServiceEventDao().AddModel(et); err != nil {
		return
	}
	return
}

// getPodContainerLogs 获取 Pod 容器的最后 N 行日志
func getPodContainerLogs(clientset kubernetes.Interface, pod *corev1.Pod, containerName string, tailLines int64) (string, error) {
	if containerName == "" {
		// 如果没有指定容器名，使用第一个容器
		if len(pod.Spec.Containers) > 0 {
			containerName = pod.Spec.Containers[0].Name
		} else {
			return "", fmt.Errorf("no containers found in pod")
		}
	}

	// 设置日志选项
	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
	}

	// 获取日志
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOptions)
	podLogs, err := req.Stream(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer podLogs.Close()

	// 读取日志内容
	body, err := io.ReadAll(podLogs)
	if err != nil {
		return "", fmt.Errorf("failed to read pod logs: %w", err)
	}

	return string(body), nil
}

// getPodContainerLogsWithPrevious 获取 Pod 容器的最后 N 行日志（包括上一个实例）
func getPodContainerLogsWithPrevious(clientset kubernetes.Interface, pod *corev1.Pod, containerName string, tailLines int64) (string, error) {
	if containerName == "" {
		// 如果没有指定容器名，使用第一个容器
		if len(pod.Spec.Containers) > 0 {
			containerName = pod.Spec.Containers[0].Name
		} else {
			return "", fmt.Errorf("no containers found in pod")
		}
	}

	// 设置日志选项 - 获取上一个容器实例的日志
	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
		Previous:  true, // ← 获取上一个容器实例的日志
	}

	// 获取日志
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOptions)
	podLogs, err := req.Stream(context.Background())
	if err != nil {
		// 如果获取上一个实例失败，尝试获取当前实例
		logrus.Debugf("failed to get previous container logs, trying current: %v", err)
		return getPodContainerLogs(clientset, pod, containerName, tailLines)
	}
	defer podLogs.Close()

	// 读取日志内容
	body, err := io.ReadAll(podLogs)
	if err != nil {
		return "", fmt.Errorf("failed to read pod logs: %w", err)
	}

	return string(body), nil
}

// recordPodLogsToEvent 记录 Pod 容器日志到 event 日志中
func recordPodLogsToEvent(clientset kubernetes.Interface, pod *corev1.Pod, eventID string, containerName string) {
	// 获取最后 300 行日志
	logs, err := getPodContainerLogs(clientset, pod, containerName, 300)
	if err != nil {
		logrus.Warnf("failed to get pod logs for %s/%s: %v", pod.Namespace, pod.Name, err)
		return
	}

	if logs == "" {
		logrus.Debugf("no logs available for pod %s/%s container %s", pod.Namespace, pod.Name, containerName)
		return
	}

	// 获取 logger 并记录日志
	logger := event.GetManager().GetLogger(eventID)
	defer event.GetManager().ReleaseLogger(logger)

	// 记录日志头部信息
	logger.Info(fmt.Sprintf("==================== Pod Container Logs (Last 300 lines) ===================="),
		map[string]string{"step": "pod-logs", "status": "info"})
	logger.Info(fmt.Sprintf("Pod: %s/%s", pod.Namespace, pod.Name),
		map[string]string{"step": "pod-logs", "status": "info"})
	logger.Info(fmt.Sprintf("Container: %s", containerName),
		map[string]string{"step": "pod-logs", "status": "info"})
	logger.Info("================================================================================",
		map[string]string{"step": "pod-logs", "status": "info"})

	// 逐行记录日志
	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		if line != "" {
			logger.Info(line, map[string]string{"step": "pod-logs", "status": "info"})
		}
	}

	logger.Info("==================== End of Pod Container Logs ====================",
		map[string]string{"step": "pod-logs", "status": "info"})
}

// recordPodLogsToEventWithPrevious 记录 Pod 容器日志到 event 日志中（包括上一个实例的日志）
func recordPodLogsToEventWithPrevious(clientset kubernetes.Interface, pod *corev1.Pod, eventID string, containerName string) {
	// 获取最后 300 行日志（尝试获取上一个实例）
	logs, err := getPodContainerLogsWithPrevious(clientset, pod, containerName, 300)
	if err != nil {
		logrus.Warnf("failed to get pod logs for %s/%s: %v", pod.Namespace, pod.Name, err)
		return
	}

	if logs == "" {
		logrus.Debugf("no logs available for pod %s/%s container %s", pod.Namespace, pod.Name, containerName)
		return
	}

	// 获取 logger 并记录日志
	logger := event.GetManager().GetLogger(eventID)
	defer event.GetManager().ReleaseLogger(logger)

	// 记录日志头部信息
	logger.Info(fmt.Sprintf("==================== Pod Container Logs (Last 300 lines from previous instance) ===================="),
		map[string]string{"step": "pod-logs", "status": "info"})
	logger.Info(fmt.Sprintf("Pod: %s/%s", pod.Namespace, pod.Name),
		map[string]string{"step": "pod-logs", "status": "info"})
	logger.Info(fmt.Sprintf("Container: %s (crashed instance)", containerName),
		map[string]string{"step": "pod-logs", "status": "info"})
	logger.Info("================================================================================",
		map[string]string{"step": "pod-logs", "status": "info"})

	// 逐行记录日志
	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		if line != "" {
			logger.Info(line, map[string]string{"step": "pod-logs", "status": "info"})
		}
	}

	logger.Info("==================== End of Pod Container Logs ====================",
		map[string]string{"step": "pod-logs", "status": "info"})
}
