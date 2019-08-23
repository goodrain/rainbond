package util

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/goodrain/rainbond/worker/server/pb"
	corev1 "k8s.io/api/core/v1"
)

func TestDescribePodStatus(t *testing.T) {
	tests := []struct {
		filename  string
		expstatus pb.PodStatus_Type
	}{
		{filename: "testdata/insufficient-memory.json", expstatus: pb.PodStatus_SCHEDULING},
		{filename: "testdata/abnormal.json", expstatus: pb.PodStatus_ABNORMAL},
		{filename: "testdata/initiating.json", expstatus: pb.PodStatus_INITIATING},
	}
	for idx := range tests {
		tc := tests[idx]
		jsonfile, err := ioutil.ReadFile(tc.filename)
		if err != nil {
			t.Errorf("failed to read file '%s': %v", tc.filename, err)
		}

		var pod corev1.Pod
		if err := json.Unmarshal(jsonfile, &pod); err != nil {
			t.Fatalf("file: %s; failed to unmarshalling json: %v", tc.filename, err)
		}

		podStatus := &pb.PodStatus{}
		DescribePodStatus(&pod, podStatus)
		if podStatus.Type != tc.expstatus {
			t.Errorf("Expected %s for pod status type, but returned %s", tc.expstatus.String(), podStatus.Type.String())
		}
	}
}
