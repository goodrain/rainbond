package podevent

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func podFromJSONFile(t *testing.T, filename string) *corev1.Pod {
	jsonfile, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read file '%s': %v", filename, err)
	}

	var pod corev1.Pod
	if err := json.Unmarshal(jsonfile, &pod); err != nil {
		t.Fatalf("file: %s; failed to unmarshalling json: %v", filename, err)
	}

	return &pod
}

func eventsFromJSONFile(t *testing.T, filename string) *corev1.EventList {
	jsonfile, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read file '%s': %v", filename, err)
	}

	var result corev1.EventList
	if err := json.Unmarshal(jsonfile, &result); err != nil {
		t.Fatalf("file: %s; failed to unmarshalling json: %v", filename, err)
	}

	return &result
}

func TestRecordUpdateEvent(t *testing.T) {
	tests := []struct {
		name, oldPodFile, newPodFile string
		eventID, tenantID, serviceID string
		finalStatus                  model.EventFinalStatus
		eventErr                     error
		explevel, expstatus, message string
		optType                      EventType
		startTime                    time.Time
	}{
		{
			name:       "running",
			newPodFile: "testdata/pod-running.json",
			eventID:    "event id",
			tenantID:   "6e22adb70c114b1d9a46d17d8146ba37",
			serviceID:  "135c3e10e3be34337bde752449a07e4c",
			eventErr:   nil,
			explevel:   "info",
			expstatus:  "success",
		},
		{
			name:       "running temporarily",
			newPodFile: "testdata/pod-running-temporarily.json",
			eventID:    "event id",
			tenantID:   "6e22adb70c114b1d9a46d17d8146ba37",
			serviceID:  "135c3e10e3be34337bde752449a07e4c",
			eventErr:   nil,
			explevel:   "info",
			expstatus:  "failure",
			startTime:  time.Now(),
		},
		{
			name:       "oom killed",
			newPodFile: "testdata/pod-oom-killed.json",
			eventID:    "event id",
			tenantID:   "6e22adb70c114b1d9a46d17d8146ba37",
			serviceID:  "135c3e10e3be34337bde752449a07e4c",
			optType:    EventTypeOOMKilled,
			eventErr:   nil,
			explevel:   "error",
			expstatus:  "failure",
			message:    "OOMKilled",
		},
		{
			name:       "oom killed without event",
			newPodFile: "testdata/pod-oom-killed.json",
			eventID:    "event id",
			tenantID:   "6e22adb70c114b1d9a46d17d8146ba37",
			serviceID:  "135c3e10e3be34337bde752449a07e4c",
			optType:    EventTypeOOMKilled,
			eventErr:   gorm.ErrRecordNotFound,
			explevel:   "error",
			expstatus:  "failure",
		},
		{
			name:       "liveness",
			newPodFile: "testdata/pod-liveness.json",
			eventID:    "event id",
			tenantID:   "f614a5eddea546c2bbaeb67d381599ee",
			serviceID:  "fa9c83c9198bfee9325804d3b4e7ff06",
			optType:    EventTypeLivenessProbeFailed,
			eventErr:   nil,
			explevel:   "error",
			expstatus:  "failure",
		},
		{
			name:       "readiness",
			newPodFile: "testdata/pod-readiness.json",
			eventID:    "event id",
			tenantID:   "f614a5eddea546c2bbaeb67d381599ee",
			serviceID:  "0c3a85977aab7adcc8b3451472d3ee94",
			optType:    EventTypeReadinessProbeFailed,
			eventErr:   nil,
			explevel:   "error",
			expstatus:  "failure",
		},
	}

	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			stopCh := make(chan struct{})
			pod := podFromJSONFile(t, tc.newPodFile)
			if !tc.startTime.IsZero() {
				pod.Status.ContainerStatuses[0].State.Running.StartedAt.Time = tc.startTime
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// mock db
			dbmanager := db.NewMockManager(ctrl)
			db.SetTestManager(dbmanager)
			serviceEventDao := dao.NewMockEventDao(ctrl)
			dbmanager.EXPECT().ServiceEventDao().AnyTimes().Return(serviceEventDao)
			serviceEventDao.EXPECT().AddModel(gomock.Any()).AnyTimes().Return(nil)

			// mock event manager
			lm := event.NewMockManager(ctrl)
			event.NewTestManager(lm)
			sendCh := make(chan []byte)
			l := event.NewLogger(tc.eventID, sendCh)
			lm.EXPECT().GetLogger(gomock.Any()).Return(l).AnyTimes()
			lm.EXPECT().ReleaseLogger(l)

			// receive message from logger
			go func(sendCh chan []byte) {
				for {
					select {
					case msg := <-sendCh:
						t.Logf("Recevied message: %s", string(msg))
						var data map[string]string
						if err := json.Unmarshal(msg, &data); err != nil {
							t.Logf("Recevied message: %s", string(msg))
						}
						level := data["level"]
						status := data["status"]
						if level == "" || status == "" {
							t.Errorf("Recevied wrong message: %s; expected field 'level' and 'status'", string(msg))
						} else {
							if level != tc.explevel {
								t.Errorf("Expected %s for level, but returned %s", tc.explevel, level)
							}
							if status != tc.expstatus {
								t.Errorf("Expected %s for status, but returned %s\n", tc.expstatus, status)
							}
						}

						close(stopCh)
					}
				}
			}(sendCh)

			testDetermineOptType := func(clientset kubernetes.Interface, pod *corev1.Pod, f k8sutil.ListEventsByPod) *optType {
				return &optType{
					eventType: tc.optType,
					message:   tc.message,
				}
			}

			recordUpdateEvent(nil, pod, testDetermineOptType)
			<-stopCh
		})
	}
}

func TestDetermineOptType(t *testing.T) {
	listEventsByPod := func(clientset kubernetes.Interface, pod *corev1.Pod) *corev1.EventList {
		if pod == nil {
			return nil
		}
		filename := fmt.Sprintf("testdata/%s-events.json", pod.GetName())
		return eventsFromJSONFile(t, filename)
	}
	tests := []struct {
		podfile      string
		podEventType EventType
	}{
		{"testdata/pod-readiness.json", EventTypeReadinessProbeFailed},
		{"testdata/pod-liveness.json", EventTypeLivenessProbeFailed},
		{"testdata/pod-oom-killed.json", EventTypeOOMKilled},
	}
	for idx := range tests {
		tc := tests[idx]
		pod := podFromJSONFile(t, tc.podfile)
		optType := defDetermineOptType(nil, pod, listEventsByPod)
		if optType.eventType != tc.podEventType {
			t.Errorf("Expected %s for opt type, but returned %s", tc.podEventType.String(), optType.eventType.String())
		}
	}
}
