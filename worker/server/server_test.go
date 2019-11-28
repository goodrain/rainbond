package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/eapache/channels"
	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/worker/appm/store"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/server/pb"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

func TestListEvents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	storer := store.NewMockStorer(ctrl)
	c, err := clientcmd.BuildConfigFromFlags("", "/Users/abe/go/src/github.com/goodrain/rainbond/test/admin.kubeconfig")
	if err != nil {
		t.Fatalf("read kube config file error: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatalf("create kube api client error: %v", err)
	}
	rserver := &RuntimeServer{
		store:     storer,
		clientset: clientset,
	}
	as := &v1.AppService{}
	storer.EXPECT().GetAppService("sid").Return(as)

	name := "88d8c4c55657217522f3bb86cfbded7e-deployment-647b84b467-kd6zc"
	namespace := "c1a29fe4d7b0413993dc859430cf743d"
	podEvents := rserver.listPodEventsByName(name, namespace)
	t.Logf("pod events: %v", podEvents)
}

func TestGetStorageClass(t *testing.T) {
	c, err := clientcmd.BuildConfigFromFlags("", "/Users/fanyangyang/Documents/company/goodrain/admin.kubeconfig")
	if err != nil {
		t.Fatalf("read kube config file error: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatalf("create kube api client error: %v", err)
	}
	s := store.NewStore(clientset, nil, option.Config{}, nil, nil)
	stes := s.GetStorageClasses()
	storageclasses := new(pb.StorageClasses)
	if stes != nil {
		for _, st := range stes {
			var allowTopologies []*pb.TopologySelectorTerm
			for _, topologySelectorTerm := range st.AllowedTopologies {
				var expressions []*pb.TopologySelectorLabelRequirement
				for _, value := range topologySelectorTerm.MatchLabelExpressions {
					expressions = append(expressions, &pb.TopologySelectorLabelRequirement{Key: value.Key, Values: value.Values})
				}
				allowTopologies = append(allowTopologies, &pb.TopologySelectorTerm{MatchLabelExpressions: expressions})
			}

			var allowVolumeExpansion bool
			if st.AllowVolumeExpansion == nil {
				allowVolumeExpansion = false
			} else {
				allowVolumeExpansion = *st.AllowVolumeExpansion
			}
			storageclasses.List = append(storageclasses.List, &pb.StorageClassDetail{
				Name:                 st.Name,
				Provisioner:          st.Provisioner,
				ReclaimPolicy:        st.ReclaimPolicy,
				AllowVolumeExpansion: allowVolumeExpansion,
				VolumeBindingMode:    st.VolumeBindingMode,
				AllowedTopologies:    allowTopologies,
			})
			t.Logf("allowVolumeExpansion is : %v", allowVolumeExpansion)
		}
	}
}

func TestGetAppVolumeStatus(t *testing.T) {
	ocfg := option.Config{
		DBType:                  "mysql",
		MysqlConnectionInfo:     "oc6Poh:noot6Mea@tcp(192.168.2.203:3306)/region",
		EtcdEndPoints:           []string{"http://192.168.2.203:2379"},
		EtcdTimeout:             5,
		KubeConfig:              "/Users/fanyangyang/Documents/company/goodrain/admin.kubeconfig",
		LeaderElectionNamespace: "rainbond",
	}

	dbconfig := config.Config{
		DBType:              ocfg.DBType,
		MysqlConnectionInfo: ocfg.MysqlConnectionInfo,
		EtcdEndPoints:       ocfg.EtcdEndPoints,
		EtcdTimeout:         ocfg.EtcdTimeout,
	}
	//step 1:db manager init ,event log client init
	if err := db.CreateManager(dbconfig); err != nil {
		t.Fatalf("error creating db manager: %v", err)
	}
	defer db.CloseManager()

	c, err := clientcmd.BuildConfigFromFlags("", ocfg.KubeConfig)
	if err != nil {
		t.Fatalf("read kube config file error: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatalf("create kube api client error: %v", err)
	}
	startCh := channels.NewRingChannel(1024)
	probeCh := channels.NewRingChannel(1024)
	storer := store.NewStore(clientset, db.GetManager(), option.Config{LeaderElectionNamespace: ocfg.LeaderElectionNamespace, KubeClient: clientset}, startCh, probeCh)
	if err := storer.Start(); err != nil {
		t.Fatalf("error starting store: %v", err)
	}
	server := &RuntimeServer{
		store:     storer,
		clientset: clientset,
	}
	statusList, err := server.GetAppVolumeStatus(context.Background(), &pb.ServiceRequest{ServiceId: "69123df08744e36800c29c91574370d5"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(statusList.GetStatus())
	time.Sleep(20 * time.Second) // db woulld close
}
