package store

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
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// PodEventType -
type PodEventType string

// String -
func (p PodEventType) String() string {
	return string(p)
}

// PodEventTypeOOMKilled -
var PodEventTypeOOMKilled PodEventType = "OOMKilled"

// PodEventTypeLivenessProbeFailed -
var PodEventTypeLivenessProbeFailed PodEventType = "LivenessProbeFailed"

// PodEventTypeReadinessProbeFailed -
var PodEventTypeReadinessProbeFailed PodEventType = "ReadinessProbeFailed"

//Storer is the interface that wraps the required methods to gather information
type Storer interface {
	// Run initiates the synchronization of the controllers
	Run(stopCh chan struct{})
}

type k8sStore struct {
	// informer contains the cache Informers
	informers      *Informer
	sharedInformer informers.SharedInformerFactory
}

// New creates a new Storer
func New(clientset kubernetes.Interface) Storer {
	store := &k8sStore{
		informers: &Informer{},
	}

	// create informers factory, enable and assign required informers
	store.sharedInformer = k8sutil.NewRainbondFilteredSharedInformerFactory(clientset)

	store.informers.Pod = store.sharedInformer.Core().V1().Pods().Informer()

	store.informers.Pod.AddEventHandler(podEventHandler(clientset))

	return store
}

// Run initiates the synchronization of the informers.
func (s *k8sStore) Run(stopCh chan struct{}) {
	// start informers
	s.informers.Run(stopCh)
}

func podEventHandler(clientset kubernetes.Interface) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
		},
		DeleteFunc: func(obj interface{}) {
		},
		UpdateFunc: func(old, cur interface{}) {
			opod := old.(*corev1.Pod)
			cpod := cur.(*corev1.Pod)

			// judge the state of the event
			oldPodstatus := &pb.PodStatus{}
			wutil.DescribePodStatus(opod, oldPodstatus)
			curPodstatus := &pb.PodStatus{}
			wutil.DescribePodStatus(cpod, curPodstatus)
			if oldPodstatus.Type == curPodstatus.Type {
				return
			}

			// extract the service information from the pod
			// _, serviceID, _, _ := k8sutil.ExtractLabels(cpod.GetLabels())
			// // ignore user actions
			// if hasUnfinishedUserActions(serviceID) {
			// 	logrus.Debugf("service id: %s; has unfinished user actions.", serviceID)
			// 	return
			// }

			recordUpdateEvent(clientset, cpod, defDetermineOptType)
		},
	}
}

func hasUnfinishedUserActions(serviceID string) bool {
	usrActs := []string{
		"rollback-service", "build-service", "update-service", "start-service", "stop-service",
		"restart-service", "vertical-service", "horizontal-service", "upgrade-service",
	}
	events, err := db.GetManager().ServiceEventDao().UnfinishedEvents(model.TargetTypeService, serviceID, usrActs...)
	if err != nil {
		logrus.Warningf("error listing unfinished events: %v", err)
	}
	return len(events) > 0
}

func recordUpdateEvent(clientset kubernetes.Interface, pod *corev1.Pod, f determineOptType) {
	evt, err := db.GetManager().ServiceEventDao().LatestFailurePodEvent(pod.GetName())
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.Warningf("error fetching latest unfinished pod event: %v", err)
		return
	}
	podstatus := new(pb.PodStatus)
	wutil.DescribePodStatus(pod, podstatus)
	tenantID, serviceID, _, _ := k8sutil.ExtractLabels(pod.GetLabels())
	// the pod in the pending status has no start time and container statuses
	for _, cs := range pod.Status.ContainerStatuses {
		state := cs.State
		if podstatus.Type == pb.PodStatus_ABNORMAL { // TODO: not ready
			var eventID string
			optType, message := f(clientset, pod, &state, k8sutil.DefListEventsByPod)
			if optType == "" {
				continue
			}
			if evt == nil { // create event
				eventID, err = createSystemEvent(tenantID, serviceID, pod.GetName(), optType.String())
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
				evt.FinalStatus = model.EventFinalStatusComplete.String()
				loggerOpt = event.GetLastLoggerOption()
			}
			logger.Info(msg, loggerOpt)
		}
	}
}

func createSystemEvent(tenantID, serviceID, targetID, optType string) (eventID string, err error) {
	eventID = util.NewUUID()
	et := &model.ServiceEvent{
		EventID:     eventID,
		TenantID:    tenantID,
		ServiceID:   serviceID,
		Target:      model.TargetTypePod,
		TargetID:    targetID,
		UserName:    model.UsernameSystem,
		OptType:     optType,
		Status:      model.EventStatusFailure.String(),
		FinalStatus: model.EventFinalStatusEmpty.String(),
	}
	if err = db.GetManager().ServiceEventDao().AddModel(et); err != nil {
		return
	}
	return
}

// determine the type of exception
type determineOptType func(clientset kubernetes.Interface, pod *corev1.Pod, state *corev1.ContainerState, f k8sutil.ListEventsByPod) (PodEventType, string)

func defDetermineOptType(clientset kubernetes.Interface, pod *corev1.Pod, state *corev1.ContainerState, f k8sutil.ListEventsByPod) (PodEventType, string) {
	if state.Terminated != nil && state.Terminated.Reason == PodEventTypeOOMKilled.String() {
		return PodEventTypeOOMKilled, state.Terminated.Reason
	}
	events := f(clientset, pod)
	for _, evt := range events.Items {
		if strings.Contains(evt.Message, "Liveness probe failed") && state.Waiting != nil {
			return PodEventTypeLivenessProbeFailed, evt.Message
		}
		if strings.Contains(evt.Message, "Readiness probe failed") && state.Running != nil {
			return PodEventTypeReadinessProbeFailed, evt.Message
		}
	}

	b, _ := json.Marshal(pod)
	logrus.Debugf("unrecognized operation type; pod info: %s", string(b))
	return "", ""
}
