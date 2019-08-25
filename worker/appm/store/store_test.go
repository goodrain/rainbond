// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package store

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/eapache/channels"
	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func TestAppRuntimeStore_GetTenantResource(t *testing.T) {
	ocfg := option.Config{
		DBType:                  "mysql",
		MysqlConnectionInfo:     "ree8Ai:Een3meeY@tcp(192.168.1.152:3306)/region",
		EtcdEndPoints:           []string{"http://192.168.1.152:2379"},
		EtcdTimeout:             5,
		KubeConfig:              "/opt/rainbond/etc/kubernetes/kubecfg/admin.kubeconfig",
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
	ocfg.KubeClient = clientset
	startCh := channels.NewRingChannel(1024)
	probeCh := channels.NewRingChannel(1024)
	store := NewStore(clientset, db.GetManager(), ocfg, startCh, probeCh)
	if err := store.Start(); err != nil {
		t.Fatalf("error starting store: %v", err)
	}

	tenantID := "d22797956503441abce65e40705aac29"
	resource := store.GetTenantResource(tenantID)
	t.Logf("%+v", resource)
}

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

func TestRecordUpdateEvent(t *testing.T) {
	tests := []struct {
		name, oldPodFile, newPodFile                   string
		eventID, tenantID, targetID, optType, username string
		finalStatus                                    model.EventFinalStatus
		eventErr                                       error
		explevel, expstatus                            string
	}{
		{
			name:        "OOMkilled",
			oldPodFile:  "testdata/pod-pending.json",
			newPodFile:  "testdata/pod-oom-killed.json",
			eventID:     "event id",
			tenantID:    "6e22adb70c114b1d9a46d17d8146ba37",
			targetID:    "135c3e10e3be34337bde752449a07e4c",
			optType:     "OOMKilled",
			username:    model.UsernameSystem,
			finalStatus: model.EventFinalStatusRunning,
			eventErr:    gorm.ErrRecordNotFound,
			explevel:    "error",
			expstatus:   "failure",
		},
		{
			name:        "duplicated OOMkilled",
			oldPodFile:  "testdata/pod-pending.json",
			newPodFile:  "testdata/pod-oom-killed.json",
			eventID:     "event id",
			tenantID:    "6e22adb70c114b1d9a46d17d8146ba37",
			targetID:    "135c3e10e3be34337bde752449a07e4c",
			optType:     "OOMKilled",
			username:    model.UsernameSystem,
			finalStatus: model.EventFinalStatusRunning,
			eventErr:    nil,
			explevel:    "error",
			expstatus:   "failure",
		},
		{
			name:       "running temporarily",
			oldPodFile: "testdata/pod-oom-killed.json",
			newPodFile: "testdata/pod-running-temporarily.json",
			eventID:    "event id",
			tenantID:   "6e22adb70c114b1d9a46d17d8146ba37",
			targetID:   "135c3e10e3be34337bde752449a07e4c",
			optType:    "OOMKilled",
			username:   model.UsernameSystem,
			eventErr:   nil,
			explevel:   "error",
			expstatus:  "failure",
		},
		{
			name:       "running",
			oldPodFile: "testdata/pod-oom-killed.json",
			newPodFile: "testdata/pod-running.json",
			eventID:    "event id",
			tenantID:   "6e22adb70c114b1d9a46d17d8146ba37",
			targetID:   "135c3e10e3be34337bde752449a07e4c",
			optType:    "OOMKilled",
			username:   model.UsernameSystem,
			eventErr:   nil,
			explevel:   "error",
			expstatus:  "success",
		},
	}

	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			stopCh := make(chan struct{})

			old := podFromJSONFile(t, tc.oldPodFile)
			new := podFromJSONFile(t, tc.newPodFile)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// mock db
			dbmanager := db.NewMockManager(ctrl)
			db.SetManager(dbmanager)
			serviceEventDao := dao.NewMockEventDao(ctrl)
			dbmanager.EXPECT().ServiceEventDao().AnyTimes().Return(serviceEventDao)
			var evt *model.ServiceEvent
			if tc.eventErr == nil {
				evt = &model.ServiceEvent{
					EventID:     tc.eventID,
					TenantID:    tc.tenantID,
					TargetID:    tc.targetID,
					UserName:    tc.username,
					OptType:     tc.optType,
					FinalStatus: model.EventFinalStatusRunning.String(),
				}
				evt.CreatedAt = new.Status.StartTime.Time
			}
			serviceEventDao.EXPECT().AddModel(gomock.Any()).AnyTimes().Return(nil)
			serviceEventDao.EXPECT().GetByTargetIDTypeUser(tc.targetID, tc.optType, tc.username).Return(evt, tc.eventErr)

			// mock event manager
			lm := event.NewMockManager(ctrl)
			event.SetManager(lm)
			sendCh := make(chan []byte)
			l := event.NewLogger(tc.eventID, sendCh)
			lm.EXPECT().GetLogger(gomock.Any()).Return(l).AnyTimes()
			lm.EXPECT().ReleaseLogger(l)

			// receive message from logger
			go func(sendCh chan []byte) {
				for {
					select {
					case msg := <-sendCh:
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

			recordUpdateEvent(old, new)
			<-stopCh
		})
	}
}
