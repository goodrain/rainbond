package podevent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/goodrain/rainbond/worker/server/pb"
	wutil "github.com/goodrain/rainbond/worker/util"
	"github.com/jinzhu/gorm"
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
			recordUpdateEvent(p.clientset, pod, defDetermineOptType)
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
	for _, cs := range pod.Status.ContainerStatuses {
		state := cs.State
		if podstatus.Type == pb.PodStatus_ABNORMAL || podstatus.Type == pb.PodStatus_NOTREADY || podstatus.Type == pb.PodStatus_UNHEALTHY {
			var eventID string
			optType, message := f(clientset, pod, &state, k8sutil.DefListEventsByPod)
			if optType == "" {
				continue
			}
			if evt == nil { // create event
				eventID, err = createSystemEvent(tenantID, serviceID, pod.GetName(), optType.String(), model.EventStatusFailure.String())
				if err != nil {
					logrus.Warningf("pod: %s; type: %s; error creating event: %v", pod.GetName(), optType.String(), err)
					continue
				}
			} else {
				eventID = evt.EventID
			}

			msg := fmt.Sprintf("container: %s; state: %s; mesage: %s", cs.Name, optType.String(), message)
			logger := event.GetManager().GetLogger(eventID)
			defer event.GetManager().ReleaseLogger(logger)
			logrus.Debugf("Service id: %s; %s.", serviceID, msg)
			logger.Error(msg, event.GetLoggerOption("failure"))
		} else if podstatus.Type == pb.PodStatus_RUNNING {
			if evt == nil {
				continue
			}
			// the container state of the pod in the PodStatus_Running must be running
			msg := fmt.Sprintf("container: %s; state: running; started at: %s", cs.Name, state.Running.StartedAt.Time.Format(time.RFC3339))
			logger := event.GetManager().GetLogger(evt.EventID)
			defer event.GetManager().ReleaseLogger(logger)
			logrus.Debugf("Service id: %s; %s.", serviceID, msg)
			loggerOpt := event.GetLoggerOption("failure")
			if time.Now().Sub(state.Running.StartedAt.Time) > 2*time.Minute {
				loggerOpt = event.GetCallbackLoggerOption()
				_, err := createSystemEvent(tenantID, serviceID, pod.GetName(), EventTypeAbnormalRecovery.String(), model.EventStatusSuccess.String())
				if err != nil {
					logrus.Warningf("pod: %s; type: %s; error creating event: %v", pod.GetName(), EventTypeAbnormalRecovery.String(), err)
					continue
				}
			}
			logger.Info(msg, loggerOpt)
		}
	}
}

// determine the type of exception
type determineOptType func(clientset kubernetes.Interface, pod *corev1.Pod, state *corev1.ContainerState, f k8sutil.ListEventsByPod) (EventType, string)

func defDetermineOptType(clientset kubernetes.Interface, pod *corev1.Pod, state *corev1.ContainerState, f k8sutil.ListEventsByPod) (EventType, string) {
	if state.Terminated != nil {
		if state.Terminated.Reason == EventTypeOOMKilled.String() {
			return EventTypeOOMKilled, state.Terminated.Reason
		}
		if state.Terminated.ExitCode != 0 {
			return EventTypeAbnormalExited, state.Terminated.Reason
		}
	}
	events := f(clientset, pod)
	for _, evt := range events.Items {
		if strings.Contains(evt.Message, "Liveness probe failed") && state.Waiting != nil {
			return EventTypeLivenessProbeFailed, evt.Message
		}
		if strings.Contains(evt.Message, "Readiness probe failed") {
			return EventTypeReadinessProbeFailed, evt.Message
		}
	}

	b, _ := json.Marshal(pod)
	logrus.Debugf("unrecognized operation type; pod info: %s", string(b))
	return "", ""
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
