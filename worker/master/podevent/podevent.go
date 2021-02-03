package podevent

import (
	"fmt"
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
		case <-p.stopCh:
			return
		}
	}
}

//GetChan get pod update chan
func (p *PodEvent) GetChan() chan<- *corev1.Pod {
	return p.podEventCh
}
func recordUpdateEvent(clientset kubernetes.Interface, pod *corev1.Pod, f determineOptType) {
	evt, err := db.GetManager().ServiceEventDao().LatestFailurePodEvent(pod.GetName())
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.Warningf("error fetching latest unfinished pod event: %v", err)
		return
	}
	podstatus := new(pb.PodStatus)
	wutil.DescribePodStatus(clientset, pod, podstatus, k8sutil.DefListEventsByPod)
	tenantID, serviceID, _, _ := k8sutil.ExtractLabels(pod.GetLabels())
	// the pod in the pending status has no start time and container statuses
	if podstatus.Type == pb.PodStatus_ABNORMAL || podstatus.Type == pb.PodStatus_NOTREADY || podstatus.Type == pb.PodStatus_UNHEALTHY {
		var eventID string
		// determine the type of exception event that occurs by the state of multiple containers
		optType := f(clientset, pod, k8sutil.DefListEventsByPod)
		if optType == nil {
			return
		}

		if evt == nil { // create event
			eventID, err = createSystemEvent(tenantID, serviceID, pod.GetName(), optType.eventType.String(), model.EventStatusFailure.String())
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
				_, err := createSystemEvent(tenantID, serviceID, pod.GetName(), EventTypeAbnormalRecovery.String(), model.EventStatusSuccess.String())
				if err != nil {
					logrus.Warningf("pod: %s; type: %s; error creating event: %v", pod.GetName(), EventTypeAbnormalRecovery.String(), err)
					return
				}
			}
		}
		logger.Info(msg, loggerOpt)
	}
}

// determine the type of exception
type determineOptType func(clientset kubernetes.Interface, pod *corev1.Pod, f k8sutil.ListEventsByPod) *optType

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

func createSystemEvent(tenantID, serviceID, targetID, optType, status string) (eventID string, err error) {
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
	}
	if err = db.GetManager().ServiceEventDao().AddModel(et); err != nil {
		return
	}
	return
}
