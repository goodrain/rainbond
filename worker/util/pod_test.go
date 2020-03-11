package util

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/goodrain/rainbond/worker/server/pb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

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

func TestDescribePodStatus(t *testing.T) {
	tests := []struct {
		name, podfilename, eventsfilename string
		expstatus                         pb.PodStatus_Type
	}{
		{name: "insufficient-memory", podfilename: "testdata/insufficient-memory.json", expstatus: pb.PodStatus_SCHEDULING},
		{name: "containercreating", podfilename: "testdata/containercreating.json", expstatus: pb.PodStatus_NOTREADY},
		{name: "crashloopbackoff", podfilename: "testdata/crashloopbackoff.json", expstatus: pb.PodStatus_ABNORMAL},
		{name: "initiating", podfilename: "testdata/initiating.json", expstatus: pb.PodStatus_INITIATING},
		{name: "notready", podfilename: "testdata/notready.json", expstatus: pb.PodStatus_NOTREADY},
		{name: "liveness", podfilename: "testdata/liveness.json", eventsfilename: "testdata/livenessprobefailed.json", expstatus: pb.PodStatus_UNHEALTHY},
		{name: "readiness", podfilename: "testdata/readiness.json", eventsfilename: "testdata/readinessprobefailed.json", expstatus: pb.PodStatus_UNHEALTHY},
		{name: "initc-notready-mainc-ready", podfilename: "testdata/initc-notready-mainc-ready.json", expstatus: pb.PodStatus_RUNNING},
	}
	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			jsonfile, err := ioutil.ReadFile(tc.podfilename)
			if err != nil {
				t.Errorf("failed to read file '%s': %v", tc.podfilename, err)
			}

			var pod corev1.Pod
			if err := json.Unmarshal(jsonfile, &pod); err != nil {
				t.Fatalf("file: %s; failed to unmarshalling json: %v", tc.podfilename, err)
			}

			listEventsByPodFunc := func(clientset kubernetes.Interface, pod *corev1.Pod) *corev1.EventList {
				if tc.eventsfilename != "" {
					events := eventsFromJSONFile(t, tc.eventsfilename)
					return events
				}
				return nil
			}

			podStatus := &pb.PodStatus{}
			DescribePodStatus(nil, &pod, podStatus, listEventsByPodFunc)
			if podStatus.Type != tc.expstatus {
				t.Errorf("Expected %s for pod status type, but returned %s", tc.expstatus.String(), podStatus.Type.String())
			}
		})
	}
}
