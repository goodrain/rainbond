// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package controller

import (
	"context"
	"encoding/json"
	"github.com/bitly/go-simplejson"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/builder/discover"
	"github.com/goodrain/rainbond/builder/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/pkg/component/mq"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

// AddCodeCheck code check
func AddCodeCheck(w http.ResponseWriter, r *http.Request) {
	//b,_:=ioutil.ReadAll(r.Body)
	//{\"url_repos\": \"https://github.com/bay1ts/zk_cluster_mini.git\", \"check_type\": \"first_check\", \"code_from\": \"gitlab_manual\", \"service_id\": \"c24dea8300b9401b1461dd975768881a\", \"code_version\": \"master\", \"git_project_id\": 0, \"condition\": \"{\\\"language\\\":\\\"docker\\\",\\\"runtimes\\\":\\\"false\\\", \\\"dependencies\\\":\\\"false\\\",\\\"procfile\\\":\\\"false\\\"}\", \"git_url\": \"--branch master --depth 1 https://github.com/bay1ts/zk_cluster_mini.git\"}
	//logrus.Infof("request recive %s",string(b))
	result := new(model.CodeCheckResult)

	b, _ := ioutil.ReadAll(r.Body)
	j, err := simplejson.NewJson(b)
	if err != nil {
		logrus.Errorf("error decode json,details %s", err.Error())
		httputil.ReturnError(r, w, 400, "bad request")
		return
	}
	result.URLRepos, _ = j.Get("url_repos").String()
	result.CheckType, _ = j.Get("check_type").String()
	result.CodeFrom, _ = j.Get("code_from").String()
	result.ServiceID, _ = j.Get("service_id").String()
	result.CodeVersion, _ = j.Get("code_version").String()
	result.GitProjectId, _ = j.Get("git_project_id").String()
	result.Condition, _ = j.Get("condition").String()
	result.GitURL, _ = j.Get("git_url").String()

	defer r.Body.Close()

	dbmodel := convertModelToDB(result)
	//checkAndGet
	db.GetManager().CodeCheckResultDao().AddModel(dbmodel)
	httputil.ReturnSuccess(r, w, nil)
}

// Update update code check results
func Update(w http.ResponseWriter, r *http.Request) {
	serviceID := strings.TrimSpace(chi.URLParam(r, "serviceID"))
	result := new(model.CodeCheckResult)

	b, _ := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	logrus.Infof("update receive %s", string(b))
	j, err := simplejson.NewJson(b)
	if err != nil {
		logrus.Errorf("error decode json,details %s", err.Error())
		httputil.ReturnError(r, w, 400, "bad request")
		return
	}
	result.BuildImageName, _ = j.Get("image").String()
	portList, err := j.Get("port_list").Map()
	if err != nil {
		portList = make(map[string]interface{})
	}
	volumeList, err := j.Get("volume_list").StringArray()
	if err != nil {
		volumeList = nil
	}
	strMap := make(map[string]string)
	for k, v := range portList {
		strMap[k] = v.(string)
	}
	result.VolumeList = volumeList
	result.PortList = strMap
	result.ServiceID = serviceID
	dbmodel := convertModelToDB(result)
	dbmodel.DockerFileReady = true
	db.GetManager().CodeCheckResultDao().UpdateModel(dbmodel)
	httputil.ReturnSuccess(r, w, nil)
}

func convertModelToDB(result *model.CodeCheckResult) *dbmodel.CodeCheckResult {
	r := dbmodel.CodeCheckResult{}
	r.ServiceID = result.ServiceID
	r.CheckType = result.CheckType
	r.CodeFrom = result.CodeFrom
	r.CodeVersion = result.CodeVersion
	r.Condition = result.Condition
	r.GitProjectId = result.GitProjectId
	r.GitURL = result.GitURL
	r.URLRepos = result.URLRepos

	if result.Condition != "" {
		bs := []byte(result.Condition)
		l, err := simplejson.NewJson(bs)
		if err != nil {
			logrus.Errorf("error get condition,details %s", err.Error())
		}
		language, err := l.Get("language").String()
		if err != nil {
			logrus.Errorf("error get language,details %s", err.Error())
		}
		r.Language = language
	}
	r.BuildImageName = result.BuildImageName
	r.InnerPort = result.InnerPort
	pl, _ := json.Marshal(result.PortList)
	r.PortList = string(pl)
	vl, _ := json.Marshal(result.VolumeList)
	r.VolumeList = string(vl)
	r.VolumeMountPath = result.VolumeMountPath
	return &r
}

// GetCodeCheck Get the result of code check
func GetCodeCheck(w http.ResponseWriter, r *http.Request) {
	serviceID := strings.TrimSpace(chi.URLParam(r, "serviceID"))
	//findResultByServiceID
	cr, err := db.GetManager().CodeCheckResultDao().GetCodeCheckResult(serviceID)
	if err != nil {
		logrus.Errorf("error get check result,details %s", err.Error())
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, cr)
}

// CheckHealth Health probe
func CheckHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// 检查服务是否已经完全就绪
	if !discover.IsReady() {
		logrus.Warn("chaos service is not ready yet")
		httputil.ReturnError(r, w, 503, "service is starting, please retry later")
		return
	}

	mqClient := mq.Default().MqClient
	if mqClient == nil {
		httputil.ReturnError(r, w, 500, "task queue client not available")
		return
	}

	// 测试 MQ 的完整循环：Enqueue -> Dequeue
	// 这确保消费者循环已经在运行，避免 lost wakeup 问题
	taskBody, _ := json.Marshal("health check test")
	testTopic := client.BuilderHealth

	// 先尝试 Dequeue，清空可能存在的旧消息
	hostName, _ := os.Hostname()
	dequeueReq := &pb.DequeueRequest{
		Topic:      testTopic,
		ClientHost: hostName + "-health-check",
	}

	// 发送测试消息
	err := mqClient.SendBuilderTopic(client.TaskStruct{
		Topic:    testTopic,
		TaskType: "check_builder_health",
		TaskBody: taskBody,
		Arch:     "test",
	})
	if err != nil {
		logrus.Errorf("builder check send builder topic failure: %v", err)
		httputil.ReturnError(r, w, 500, "health check send failed")
		return
	}

	// 给消息一点时间传递和处理
	time.Sleep(100 * time.Millisecond)

	// 尝试接收测试消息，验证消费者循环正在运行
	for i := 0; i < 5; i++ {
		msg, err := mqClient.Dequeue(ctx, dequeueReq)
		if err != nil {
			logrus.Warnf("dequeue attempt %d failed: %v", i+1, err)
			time.Sleep(time.Second)
			continue
		}

		if msg != nil && len(msg.TaskBody) > 0 {
			// 成功接收到测试消息，说明 MQ 循环正常
			healthInfo := map[string]string{
				"status":  "ready",
				"message": "builder service is healthy and ready (MQ verified)",
			}
			httputil.ReturnSuccess(r, w, healthInfo)
			return
		}

		// 没收到消息，可能是 lost wakeup，等待一下再试
		logrus.Warnf("dequeue attempt %d returned empty, retrying...", i+1)
		time.Sleep(time.Second)
	}

	// 如果所有尝试都失败，返回未就绪状态
	logrus.Error("health check failed: unable to verify MQ consumer loop")
	httputil.ReturnError(r, w, 503, "service consumer loop not ready, please retry later")
}
