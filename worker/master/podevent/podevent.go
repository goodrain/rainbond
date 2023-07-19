package podevent

import (
	"context"
	"fmt"
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
func New(clientset kubernetes.Interface, stopCh chan struct{}) *PodEvent {
	return &PodEvent{
		clientset:  clientset,
		stopCh:     stopCh,
		podEventCh: make(chan *corev1.Pod, 100),
	}
}

// Handle -
func (p *PodEvent) Handle() {
	for {
		select {
		case pod := <-p.podEventCh:
			// do not record events that occur 10 minutes after startup
			if time.Now().Sub(pod.CreationTimestamp.Time) > 10*time.Minute {
				recordUpdateEvent(p.clientset, pod, defDetermineOptType)
			}
			if time.Now().Sub(pod.CreationTimestamp.Time) > 10*time.Second {
				AbnormalEvent(p.clientset, pod)
			}
		case <-p.stopCh:
			return
		}
	}
}

//GetChan get pod update chan
func (p *PodEvent) GetChan() chan<- *corev1.Pod {
	return p.podEventCh
}

//recordUpdateEvent -
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

		msg := fmt.Sprintf("image: %s; container: %s; state: %s; mesage: %s", optType.image, optType.containerID, optType.eventType.String(), optType.message)
		logger := event.GetManager().GetLogger(eventID)
		defer event.GetManager().ReleaseLogger(logger)
		logrus.Debugf("Service id: %s; %s.", serviceID, msg)
		logger.Error(msg, event.GetLoggerOption("failure"))
	} else if podstatus.Type == pb.PodStatus_RUNNING {
		if evt == nil {
			return
		}

		// running time
		var rtime time.Time
		for _, condition := range pod.Status.Conditions {
			if condition.Type != corev1.PodReady || condition.Status != corev1.ConditionTrue {
				continue
			}
			rtime = condition.LastTransitionTime.Time
		}

		// the container state of the pod in the PodStatus_Running must be running
		msg := fmt.Sprintf("state: running; started at: %s", rtime.Format(time.RFC3339))
		logger := event.GetManager().GetLogger(evt.EventID)
		defer event.GetManager().ReleaseLogger(logger)
		logrus.Debugf("Service id: %s; %s.", serviceID, msg)
		loggerOpt := event.GetLoggerOption("failure")

		if !rtime.IsZero() && time.Now().Sub(rtime) > 2*time.Minute {
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
				if strings.Contains(condition.Message, "affinity/selector") {
					msg = "不满足节点亲和性"
				}
				if strings.Contains(condition.Message, "cpu") {
					msg = "节点CPU不足"
				}
				if strings.Contains(condition.Message, "memory") {
					msg = "节点内存不足"
				}
				if strings.Contains(condition.Message, "PersistentVolumeClaims") {
					msg = "当前没有绑定 PVC"
				}
				if strings.Contains(condition.Message, "tolerate") {
					msg = "节点存在污点"
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
			FieldSelector: "metadata.namespace!=rbd-system",
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
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == EventTypeCrashLoopBackOff.String() {
				unSchedulableEvent, err := db.GetManager().ServiceEventDao().AbnormalEvent(serviceID, "CrashLoopBackOff")
				if err != nil && err != gorm.ErrRecordNotFound {
					logrus.Warningf("error fetching latest unfinished pod event: %v", err)
					return
				}
				if unSchedulableEvent != nil {
					return
				}
				_, err = createSystemEvent(tenantID, serviceID, pod.Name, containerStatus.State.Waiting.Reason, model.EventStatusFailure.String(), "服务运行异常，请检查容器日志")
				if err != nil {
					logrus.Warningf("pod: %s; type: %s; error creating event: %v", pod.GetName(), EventTypeAbnormalRecovery.String(), err)
					return
				}
			}
		}
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
	}
	if err = db.GetManager().ServiceEventDao().AddModel(et); err != nil {
		return
	}
	return
}
