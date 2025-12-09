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
	"sync"
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

// EventTypeReadinessUnhealthy - Readiness probe unhealthy for extended period
var EventTypeReadinessUnhealthy EventType = "ReadinessUnhealthy"

// EventTypeLivenessRestart - Container restarted due to liveness probe failure
var EventTypeLivenessRestart EventType = "LivenessRestart"

// EventTypeStartupProbeFailure - Startup probe failure in CrashLoopBackOff
var EventTypeStartupProbeFailure EventType = "StartupProbeFailure"

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

// ContainerHealthState tracks the health state of a container over time
type ContainerHealthState struct {
	LastRestartCount        int32
	LastReadyState          bool
	ReadinessUnhealthySince time.Time
	LastLivenessFailTime    time.Time
	HasEverBeenReady        bool
	LastCheckTime           time.Time
}

// HealthStateCache manages container health states
type HealthStateCache struct {
	sync.RWMutex
	// key: pod_namespace/pod_name/container_name
	states map[string]*ContainerHealthState
}

// NewHealthStateCache creates a new health state cache
func NewHealthStateCache() *HealthStateCache {
	return &HealthStateCache{
		states: make(map[string]*ContainerHealthState),
	}
}

// Get retrieves the health state for a container
func (c *HealthStateCache) Get(key string) *ContainerHealthState {
	c.RLock()
	defer c.RUnlock()
	return c.states[key]
}

// Update updates the health state for a container
func (c *HealthStateCache) Update(key string, cs corev1.ContainerStatus) {
	c.Lock()
	defer c.Unlock()

	state, exists := c.states[key]
	if !exists {
		state = &ContainerHealthState{
			LastRestartCount: cs.RestartCount,
			LastReadyState:   cs.Ready,
			HasEverBeenReady: cs.Ready,
			LastCheckTime:    time.Now(),
		}
		c.states[key] = state
		return
	}

	// Update state
	state.LastRestartCount = cs.RestartCount
	state.LastReadyState = cs.Ready
	state.LastCheckTime = time.Now()

	// Track if container has ever been ready
	if cs.Ready {
		state.HasEverBeenReady = true
		// Reset readiness unhealthy timer when becoming ready
		state.ReadinessUnhealthySince = time.Time{}
	}
}

// Delete removes a container's health state
func (c *HealthStateCache) Delete(key string) {
	c.Lock()
	defer c.Unlock()
	delete(c.states, key)
}

// CleanupOldStates removes states that haven't been updated in a while
func (c *HealthStateCache) CleanupOldStates(maxAge time.Duration) {
	c.Lock()
	defer c.Unlock()

	now := time.Now()
	for key, state := range c.states {
		if now.Sub(state.LastCheckTime) > maxAge {
			delete(c.states, key)
		}
	}
}

// PodEvent -
type PodEvent struct {
	clientset        kubernetes.Interface
	stopCh           chan struct{}
	podEventCh       chan *corev1.Pod
	healthStateCache *HealthStateCache
	recentPods       sync.Map // map[string]*corev1.Pod, key: namespace/name
}

// New create a new PodEvent
func New(stopCh chan struct{}) *PodEvent {
	pe := &PodEvent{
		clientset:        k8s.Default().Clientset,
		stopCh:           stopCh,
		podEventCh:       make(chan *corev1.Pod, 100),
		healthStateCache: NewHealthStateCache(),
	}

	// Start periodic cleanup of old health states
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Clean up states older than 1 hour
				pe.healthStateCache.CleanupOldStates(1 * time.Hour)
			case <-stopCh:
				return
			}
		}
	}()

	return pe
}

// Handle -
func (p *PodEvent) Handle() {
	// Add periodic check for probe issues (every 30 seconds)
	// This ensures we detect issues even when pod state doesn't change
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case pod := <-p.podEventCh:
			// Extend monitoring window: record events from 5 seconds to 30 minutes after startup
			// This catches immediate failures faster and monitors long-running issues longer
			podAge := time.Now().Sub(pod.CreationTimestamp.Time)
			logrus.Infof("Received pod event: %s/%s, age: %.1fs, phase: %s",
				pod.Namespace, pod.Name, podAge.Seconds(), pod.Status.Phase)

			if podAge > 5*time.Second && podAge < 30*time.Minute {
				recordUpdateEvent(p.clientset, pod, defDetermineOptType)
				AbnormalEvent(p.clientset, pod)
				// Detect probe health issues (Readiness/Liveness/Startup)
				p.detectProbeIssues(pod)
			} else {
				logrus.Infof("Pod %s/%s outside monitoring window (age: %.1fs)",
					pod.Namespace, pod.Name, podAge.Seconds())
			}
		case <-ticker.C:
			// Periodic check: re-check all pods in cache for probe issues
			p.periodicProbeCheck()
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

		// Translate the error message to user-friendly format first
		userMsg := translateRuntimeError(optType.eventType.String(), optType.message)

		if evt == nil { // create event
			eventID, err = createSystemEvent(tenantID, serviceID, pod.GetName(), optType.eventType.String(), model.EventStatusFailure.String(), userMsg)
			if err != nil {
				logrus.Warningf("pod: %s; type: %s; error creating event: %v", pod.GetName(), optType.eventType.String(), err)
				return
			}
		} else {
			eventID = evt.EventID
		}

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
			// Don't auto-complete ContainerExitError events - preserve crash history
			// Users need to see that container crashed before, even if it recovered
			if evt.OptType != "ContainerExitError" {
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
			// Check both current and last terminated state for container exit errors
			// This ensures we catch exits even if container has already restarted
			var terminated *corev1.ContainerStateTerminated
			if containerStatus.State.Terminated != nil {
				terminated = containerStatus.State.Terminated
			} else if containerStatus.LastTerminationState.Terminated != nil {
				terminated = containerStatus.LastTerminationState.Terminated
			}

			// Container terminated with error (check before other states)
			if terminated != nil && terminated.ExitCode != 0 && terminated.Reason != "OOMKilled" {
				exitErrorEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, "ContainerExitError")
				if err != nil && err != gorm.ErrRecordNotFound {
					logrus.Warningf("error fetching container exit error event: %v", err)
					return
				}
				// Time-based deduplication: only skip if event was created within last 30 seconds
				// This allows recording repeated crashes while avoiding duplicate detection of same crash
				if exitErrorEvent != nil {
					eventTime, err := time.Parse(time.RFC3339, exitErrorEvent.CreatedAt)
					if err != nil {
						logrus.Warningf("failed to parse event time: %v", err)
						// If we can't parse the time, assume it's an old event and create a new one
					} else {
						timeSinceLastEvent := time.Now().Sub(eventTime)
						if timeSinceLastEvent < 30*time.Second {
							// Recent event exists, likely duplicate detection of same crash
							return
						}
					}
					// Old event exists, but this is a new crash - continue to create new event
				}

				// Determine if this is a startup failure or runtime crash
				// - Startup failure: container never successfully ran (RestartCount == 0 and ran < 10s)
				// - Runtime crash: container was running successfully, then exited
				var msg string
				wasRunning := false

				// Check if container ever ran successfully
				if containerStatus.RestartCount > 0 {
					// If restarted before, it means it was running successfully
					wasRunning = true
				} else if !terminated.StartedAt.IsZero() && !terminated.FinishedAt.IsZero() {
					// Check how long the container ran
					runDuration := terminated.FinishedAt.Sub(terminated.StartedAt.Time)
					if runDuration.Seconds() >= 10 {
						// Ran for at least 10 seconds, consider it was running
						wasRunning = true
					}
				}

				if wasRunning {
					msg = fmt.Sprintf("%s (exit code: %d)", util.Translation("Deployment failed: container crashed during runtime"), terminated.ExitCode)
				} else {
					msg = fmt.Sprintf("%s (exit code: %d)", util.Translation("Deployment failed: container startup failed"), terminated.ExitCode)
				}

				eventID, err := createSystemEvent(tenantID, serviceID, pod.Name, "ContainerExitError", model.EventStatusFailure.String(), msg)
				if err != nil {
					logrus.Warningf("pod: %s; type: ContainerExitError; error creating event: %v", pod.GetName(), err)
					return
				}
				// Delete old AbnormalExited event to avoid duplicates
				// (AbnormalExited is created by recordUpdateEvent, but ContainerExitError has more details)
				if err := db.GetManager().ServiceEventDao().DelAbnormalEvent(serviceID, "AbnormalExited"); err != nil && err != gorm.ErrRecordNotFound {
					logrus.Debugf("failed to delete AbnormalExited event: %v", err)
				}

				// Record container logs (300 lines)
				// If container has restarted, get previous container logs
				if containerStatus.State.Running != nil && containerStatus.RestartCount > 0 {
					// Container has restarted, get previous logs
					go recordPodLogsToEventWithPrevious(clientset, pod, eventID, containerStatus.Name)
				} else {
					// Container still in terminated state, get current logs
					go recordPodLogsToEvent(clientset, pod, eventID, containerStatus.Name)
				}
			}

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

			// Note: Container exit error is already handled above (line 386-442)
			// by checking both State.Terminated and LastTerminationState.Terminated
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

// getContainerSpec retrieves the container spec from pod by container name
func getContainerSpec(pod *corev1.Pod, containerName string) *corev1.Container {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == containerName {
			return &pod.Spec.Containers[i]
		}
	}
	return nil
}

// hasProbeFailureEvent checks if there's a probe failure event in recent pod events
func hasProbeFailureEvent(clientset kubernetes.Interface, pod *corev1.Pod, containerName, probeType string) bool {
	events := k8sutil.DefListEventsByPod(clientset, pod)
	if events == nil {
		return false
	}

	searchStr := fmt.Sprintf("%s probe failed", probeType)
	for _, evt := range events.Items {
		// Check if event is recent (within last 2 minutes)
		if time.Since(evt.LastTimestamp.Time) > 2*time.Minute {
			continue
		}
		if strings.Contains(evt.Message, searchStr) {
			// Optionally check if event mentions this specific container
			// (K8s events may not always include container name)
			return true
		}
	}
	return false
}

// hasLivenessFailureEvent checks specifically for liveness probe failures
func hasLivenessFailureEvent(clientset kubernetes.Interface, pod *corev1.Pod, containerName string) bool {
	return hasProbeFailureEvent(clientset, pod, containerName, "Liveness")
}

// hasReadinessFailureEvent checks specifically for readiness probe failures
func hasReadinessFailureEvent(clientset kubernetes.Interface, pod *corev1.Pod, containerName string) bool {
	return hasProbeFailureEvent(clientset, pod, containerName, "Readiness")
}

// hasStartupProbeFailureEvent checks specifically for startup probe failures
func hasStartupProbeFailureEvent(clientset kubernetes.Interface, pod *corev1.Pod, containerName string) bool {
	return hasProbeFailureEvent(clientset, pod, containerName, "Startup")
}

// Constants for probe health check thresholds
const (
	ReadinessUnhealthyThreshold = 1 * time.Minute // Alert if readiness unhealthy for > 1 minute
	LivenessRestartThreshold    = 3               // Alert if liveness caused > 3 restarts
	StartupProbeFailureMin      = 5               // Alert if startup probe failed > 5 times
)

// checkReadinessHealth detects containers running but not ready for extended periods
func (p *PodEvent) checkReadinessHealth(pod *corev1.Pod, cs corev1.ContainerStatus, container *corev1.Container, lastState *ContainerHealthState, tenantID, serviceID string) {
	// Only check if container has ReadinessProbe configured
	if container.ReadinessProbe == nil {
		return
	}

	// Container is running but not ready
	if cs.State.Running != nil && !cs.Ready {
		containerKey := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, cs.Name)

		// Get or initialize unhealthy start time
		var unhealthyStartTime time.Time
		if lastState != nil && !lastState.ReadinessUnhealthySince.IsZero() {
			// Already tracking this container as unhealthy
			unhealthyStartTime = lastState.ReadinessUnhealthySince
		} else {
			// First time seeing this container as not ready, initialize the time
			p.healthStateCache.Lock()
			state, exists := p.healthStateCache.states[containerKey]
			if !exists {
				state = &ContainerHealthState{
					ReadinessUnhealthySince: time.Now(),
					LastCheckTime:           time.Now(),
				}
				p.healthStateCache.states[containerKey] = state
				unhealthyStartTime = time.Now()
			} else if state.ReadinessUnhealthySince.IsZero() {
				state.ReadinessUnhealthySince = time.Now()
				unhealthyStartTime = time.Now()
			} else {
				unhealthyStartTime = state.ReadinessUnhealthySince
			}
			p.healthStateCache.Unlock()
		}

		// Check how long it's been unhealthy
		unhealthyDuration := time.Since(unhealthyStartTime)
		if unhealthyDuration > ReadinessUnhealthyThreshold {
			// Check if we already created an event for this
			existingEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, EventTypeReadinessUnhealthy.String())
			if err != nil && err != gorm.ErrRecordNotFound {
				logrus.Warnf("error fetching readiness unhealthy event: %v", err)
				return
			}
			if existingEvent != nil {
				// Check if the existing event is for the same pod
				if existingEvent.TargetID == pod.Name {
					// Event already exists for this pod, don't create duplicate
					return
				} else {
					// Event exists but for a different pod (old pod), delete it and create new one
					logrus.Infof("Found old ReadinessUnhealthy event for pod %s (current pod: %s), deleting old event",
						existingEvent.TargetID, pod.Name)
					if err := db.GetManager().ServiceEventDao().DelAbnormalEvent(serviceID, EventTypeReadinessUnhealthy.String()); err != nil && err != gorm.ErrRecordNotFound {
						logrus.Warnf("Failed to delete old event: %v", err)
					}
				}
			}

			logrus.Infof("Creating ReadinessUnhealthy event for pod %s/%s container %s...", pod.Namespace, pod.Name, cs.Name)

			// Create event message
			msg := fmt.Sprintf("容器 [%s] 运行正常但未通过就绪检查已持续 %.0f 分钟，流量已被移除。请检查健康检查配置或应用状态。",
				cs.Name, unhealthyDuration.Minutes())

			eventID, err := createSystemEvent(tenantID, serviceID, pod.Name, EventTypeReadinessUnhealthy.String(), model.EventStatusFailure.String(), msg)
			if err != nil {
				logrus.Warnf("pod: %s; type: ReadinessUnhealthy; error creating event: %v", pod.GetName(), err)
				return
			}

			logrus.Infof("✓ Created ReadinessUnhealthy event for pod %s/%s container %s (eventID: %s)",
				pod.Namespace, pod.Name, cs.Name, eventID)

			// Log probe details
			logger := event.GetManager().GetLogger(eventID)
			defer event.GetManager().ReleaseLogger(logger)
			logger.Info(fmt.Sprintf("Readiness Probe Configuration: %+v", container.ReadinessProbe),
				map[string]string{"step": "probe-check", "status": "info"})

			// Check K8s events for additional context
			if hasReadinessFailureEvent(p.clientset, pod, cs.Name) {
				logger.Info("Kubernetes events confirmed readiness probe failures",
					map[string]string{"step": "probe-check", "status": "info"})
			}
		}
	}
}

// checkLivenessRestart detects container restarts caused by liveness probe failures
func (p *PodEvent) checkLivenessRestart(pod *corev1.Pod, cs corev1.ContainerStatus, container *corev1.Container, lastState *ContainerHealthState, tenantID, serviceID string) {
	// Only check if container has LivenessProbe configured
	if container.LivenessProbe == nil {
		return
	}

	// Check if RestartCount increased
	if lastState != nil && cs.RestartCount > lastState.LastRestartCount {
		// Container restarted, check if it was due to liveness probe
		if cs.LastTerminationState.Terminated != nil {
			terminated := cs.LastTerminationState.Terminated
			// K8s kills containers with SIGKILL (exit code 137) when liveness probe fails
			if terminated.Reason == "Error" && terminated.ExitCode == 137 {
				// Verify with K8s events
				if hasLivenessFailureEvent(p.clientset, pod, cs.Name) {
					// Check restart count threshold
					if cs.RestartCount >= LivenessRestartThreshold {
						// Check if we already created an event recently
						existingEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, EventTypeLivenessRestart.String())
						if err != nil && err != gorm.ErrRecordNotFound {
							logrus.Warnf("error fetching liveness restart event: %v", err)
							return
						}

						// Check if event is for the same pod and recent
						if existingEvent != nil {
							if existingEvent.TargetID != pod.Name {
								// Old pod's event, delete it
								logrus.Infof("Found old LivenessRestart event for pod %s (current: %s), deleting",
									existingEvent.TargetID, pod.Name)
								db.GetManager().ServiceEventDao().DelAbnormalEvent(serviceID, EventTypeLivenessRestart.String())
							} else {
								// Same pod, check time-based deduplication
								eventTime, err := time.Parse(time.RFC3339, existingEvent.CreatedAt)
								if err == nil && time.Since(eventTime) < 30*time.Second {
									return
								}
							}
						}

						msg := fmt.Sprintf("容器 [%s] 因存活检查失败已重启 %d 次。最近一次失败：容器被 Kubernetes 强制终止 (Exit Code 137)。",
							cs.Name, cs.RestartCount)

						eventID, err := createSystemEvent(tenantID, serviceID, pod.Name, EventTypeLivenessRestart.String(), model.EventStatusFailure.String(), msg)
						if err != nil {
							logrus.Warnf("pod: %s; type: LivenessRestart; error creating event: %v", pod.GetName(), err)
							return
						}

						// Log probe details and recent logs
						logger := event.GetManager().GetLogger(eventID)
						defer event.GetManager().ReleaseLogger(logger)
						logger.Info(fmt.Sprintf("Liveness Probe Configuration: %+v", container.LivenessProbe),
							map[string]string{"step": "probe-check", "status": "info"})

						// Get logs from previous instance (before restart)
						go recordPodLogsToEventWithPrevious(p.clientset, pod, eventID, cs.Name)
					}
				}
			}
		}
	}
}

// checkStartupProbeFailure detects startup probe failures in CrashLoopBackOff
func (p *PodEvent) checkStartupProbeFailure(pod *corev1.Pod, cs corev1.ContainerStatus, container *corev1.Container, lastState *ContainerHealthState, tenantID, serviceID string) {
	// Only check if container has StartupProbe configured
	if container.StartupProbe == nil {
		return
	}

	// Container in CrashLoopBackOff or ImagePullBackOff with waiting state
	if cs.State.Waiting != nil && (cs.State.Waiting.Reason == "CrashLoopBackOff" ||
		cs.State.Waiting.Reason == "RunContainerError" || cs.State.Waiting.Reason == "CreateContainerError") {
		// Check if container has never been ready (startup phase failure)
		hasBeenReady := false
		if lastState != nil {
			hasBeenReady = lastState.HasEverBeenReady
		}

		if !hasBeenReady && cs.RestartCount >= StartupProbeFailureMin {
			// Check if we already created an event
			existingEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, EventTypeStartupProbeFailure.String())
			if err != nil && err != gorm.ErrRecordNotFound {
				logrus.Warnf("error fetching startup probe failure event: %v", err)
				return
			}
			if existingEvent != nil {
				// Check if event is for the same pod
				if existingEvent.TargetID != pod.Name {
					// Old pod's event, delete it
					logrus.Infof("Found old StartupProbeFailure event for pod %s (current: %s), deleting",
						existingEvent.TargetID, pod.Name)
					db.GetManager().ServiceEventDao().DelAbnormalEvent(serviceID, EventTypeStartupProbeFailure.String())
				} else {
					// Event already exists for this pod
					return
				}
			}

			// Check K8s events to confirm (optional, not required)
			hasK8sEvent := hasStartupProbeFailureEvent(p.clientset, pod, cs.Name)

			// Calculate backoff time (kubernetes uses exponential backoff)
			backoffSeconds := calculateBackoffDelay(cs.RestartCount)

			msg := fmt.Sprintf("容器 [%s] 启动阶段健康检查失败 %d 次，已进入退避重启（下次重试：约 %d 秒后）。请检查启动时间配置或初始化逻辑。",
				cs.Name, cs.RestartCount, backoffSeconds)

			eventID, err := createSystemEvent(tenantID, serviceID, pod.Name, EventTypeStartupProbeFailure.String(), model.EventStatusFailure.String(), msg)
			if err != nil {
				logrus.Warnf("pod: %s; type: StartupProbeFailure; error creating event: %v", pod.GetName(), err)
				return
			}

			// Log probe details
			logger := event.GetManager().GetLogger(eventID)
			defer event.GetManager().ReleaseLogger(logger)
			logger.Info(fmt.Sprintf("Startup Probe Configuration: %+v", container.StartupProbe),
				map[string]string{"step": "probe-check", "status": "info"})
			logger.Info(fmt.Sprintf("Container has restarted %d times and never became ready", cs.RestartCount),
				map[string]string{"step": "probe-check", "status": "info"})

			if hasK8sEvent {
				logger.Info("Kubernetes events confirmed startup probe failures",
					map[string]string{"step": "probe-check", "status": "info"})
			}

			// Get logs from previous instance
			go recordPodLogsToEventWithPrevious(p.clientset, pod, eventID, cs.Name)
		}
	}
}

// calculateBackoffDelay calculates the exponential backoff delay for CrashLoopBackOff
// Kubernetes uses: min(2^(restartCount-1) * 10s, 5 minutes)
func calculateBackoffDelay(restartCount int32) int {
	if restartCount <= 0 {
		return 10
	}
	// Calculate 2^(restartCount-1) * 10
	delay := 10
	for i := int32(1); i < restartCount; i++ {
		delay *= 2
		if delay > 300 { // Cap at 5 minutes
			return 300
		}
	}
	return delay
}

// detectProbeIssues performs comprehensive probe health detection
func (p *PodEvent) detectProbeIssues(pod *corev1.Pod) {
	// Non-platform created components do not log events
	tenantID, serviceID, _, _ := k8sutil.ExtractLabels(pod.GetLabels())
	if tenantID == "" || serviceID == "" {
		return
	}

	// Check pods in Running phase and also check containers in CrashLoopBackOff (for Startup Probe)
	// Don't skip pending pods as they might have probe issues
	if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
		return
	}

	// Store pod in recent pods map for periodic checking
	podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	p.recentPods.Store(podKey, pod)

	for _, cs := range pod.Status.ContainerStatuses {
		containerKey := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, cs.Name)
		lastState := p.healthStateCache.Get(containerKey)

		container := getContainerSpec(pod, cs.Name)
		if container == nil {
			continue
		}

		// Check readiness health
		if container.ReadinessProbe != nil {
			p.checkReadinessHealth(pod, cs, container, lastState, tenantID, serviceID)
		}

		// Check liveness restart
		if container.LivenessProbe != nil {
			p.checkLivenessRestart(pod, cs, container, lastState, tenantID, serviceID)
		}

		// Check startup probe failure
		if container.StartupProbe != nil {
			p.checkStartupProbeFailure(pod, cs, container, lastState, tenantID, serviceID)
		}

		// Update cache after checking
		p.healthStateCache.Update(containerKey, cs)
	}
}

// periodicProbeCheck periodically re-checks all recent pods for probe issues
// This ensures we catch issues even when pod state doesn't change
func (p *PodEvent) periodicProbeCheck() {
	// Track checked pods to avoid duplicates
	checkedPods := make(map[string]bool)

	// First, check pods in cache
	p.recentPods.Range(func(key, value interface{}) bool {
		// Re-fetch pod from Kubernetes to get latest state
		podKey := key.(string)
		parts := strings.Split(podKey, "/")
		if len(parts) != 2 {
			return true
		}
		namespace, name := parts[0], parts[1]

		// Get latest pod state
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		latestPod, err := p.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			// Pod might have been deleted, remove from cache
			p.recentPods.Delete(key)
			return true
		}

		// Check if pod is too old (> 30 minutes), remove from periodic check
		podAge := time.Since(latestPod.CreationTimestamp.Time)
		if podAge > 30*time.Minute {
			p.recentPods.Delete(key)
			return true
		}

		// Run probe detection
		p.detectProbeIssues(latestPod)
		checkedPods[podKey] = true
		return true
	})

	// Also actively scan for pods with health probes that might not be in cache
	// This handles cases where pod events weren't received
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// List all pods across all namespaces
	pods, err := p.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.Warnf("Failed to list pods for periodic check: %v", err)
		return
	}

	for _, pod := range pods.Items {
		// Skip if already checked from cache
		podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
		if checkedPods[podKey] {
			continue
		}

		// Check if pod is in monitoring window
		podAge := time.Since(pod.CreationTimestamp.Time)
		if podAge <= 5*time.Second || podAge >= 30*time.Minute {
			continue
		}

		// Check if pod is Running or Pending
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		// Check if any container has health probes
		hasProbes := false
		for _, container := range pod.Spec.Containers {
			if container.ReadinessProbe != nil || container.LivenessProbe != nil || container.StartupProbe != nil {
				hasProbes = true
				break
			}
		}

		if hasProbes {
			p.detectProbeIssues(&pod)
		}
	}
}
