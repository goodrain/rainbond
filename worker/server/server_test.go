package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/server/pb"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getReplicaSet() *appsv1.ReplicaSet {
	layout := "2006-01-02T15:04:05Z"
	creation, err := time.Parse(layout, "2019-08-15T12:15:56Z")
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.NewTime(creation),
		},
	}
}

func getPods(filename string) ([]*corev1.Pod, error) {
	pfile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening '%s': %v", filename, err.Error())
	}

	var pods []*corev1.Pod
	if err := json.Unmarshal([]byte(pfile), &pods); err != nil {
		return nil, fmt.Errorf("error unmarshaling pfile: %v", err.Error())
	}

	return pods, nil
}

func serviceAppPodListEqual(a, b *pb.ServiceAppPodList) bool {
	if a == b {
		return true
	}
	podequal := func(m, n []*pb.ServiceAppPod) bool {
		if len(m) != len(n) {
			return false
		}
		return true
	}
	if podequal(a.OldPods, b.OldPods) && podequal(a.NewPods, b.NewPods) {
		return true
	}
	return false
}

func TestRuntimeServer_GetAppPods(t *testing.T) {
	replicaset := getReplicaSet()
	newpods, err := getPods("newpods.json")
	if err != nil {
		t.Errorf("error getting new pods: %s", err.Error())
	}
	oldpods, err := getPods("oldpods.json")
	if err != nil {
		t.Errorf("error getting old pods: %s", err.Error())
	}

	tests := []struct {
		name, sid string
		as        *v1.AppService
		expres    *pb.ServiceAppPodList
		experr    bool
	}{
		{name: "empty result", sid: "dummy service id 1", as: nil, expres: nil, experr: false},
		{name: "only new pods", sid: "dummy service id 2", as: func() *v1.AppService {
			as := &v1.AppService{}
			as.SetReplicaSets(replicaset)
			for _, pod := range newpods {
				as.SetPods(pod)
			}
			return as
		}(), expres: &pb.ServiceAppPodList{
			NewPods: []*pb.ServiceAppPod{{}, {}, {}},
		}, experr: false},
		{name: "old and new pods", sid: "dummy service id 4", as: func() *v1.AppService {
			as := &v1.AppService{}
			as.SetReplicaSets(replicaset)
			pods := append(oldpods, newpods...)
			for _, pod := range pods {
				as.SetPods(pod)
			}
			return as
		}(), expres: &pb.ServiceAppPodList{
			OldPods: []*pb.ServiceAppPod{{}, {}, {}},
			NewPods: []*pb.ServiceAppPod{{}, {}, {}},
		}, experr: false},
	}

	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			storer := store.NewMockStorer(ctrl)
			rserver := &RuntimeServer{
				store: storer,
			}
			storer.EXPECT().GetAppService(tc.sid).Return(tc.as)

			sreq := &pb.ServiceRequest{ServiceId: tc.sid}
			res, err := rserver.GetAppPods(context.Background(), sreq)
			if tc.experr != (err != nil) {
				t.Errorf("Unexpected error: %v", err)
			}
			if !serviceAppPodListEqual(res, tc.expres) {
				t.Errorf("Expected %+v for pods\n, but returned %v", tc.expres, res)
			}
		})
	}
}
