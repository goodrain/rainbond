
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

package controller

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/core"
	corenode "github.com/goodrain/rainbond/pkg/node/core/node"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/goodrain/rainbond/pkg/node/utils"
	"github.com/twinj/uuid"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	"github.com/go-chi/chi"
	//"github.com/gorilla/websocket"
	//"github.com/goodrain/rainbond/pkg/event"
	"strconv"

	"github.com/goodrain/rainbond/pkg/event"
)

type ProcFetchOptions struct {
	Groups  []string
	NodeIds []string
	JobIds  []string
}

func SubtractStringArray(a, b []string) (c []string) {
	c = []string{}

	for _, _a := range a {
		if !InStringArray(_a, b) {
			c = append(c, _a)
		}
	}

	return
}
func UniqueStringArray(a []string) []string {
	al := len(a)
	if al == 0 {
		return a
	}

	ret := make([]string, al)
	index := 0

loopa:
	for i := 0; i < al; i++ {
		for j := 0; j < index; j++ {
			if a[i] == ret[j] {
				continue loopa
			}
		}
		ret[index] = a[i]
		index++
	}

	return ret[:index]
}
func getStringArrayFromQuery(name, sep string, r *http.Request) (arr []string) {
	val := strings.TrimSpace(r.FormValue(name))
	if len(val) == 0 {
		return
	}

	return strings.Split(val, sep)
}

//func NewComputeNodeToInstall(w http.ResponseWriter, r *http.Request) {
//	nodeIP := strings.TrimSpace(chi.URLParam(r, "ip"))
//	j,err:=core.NewComputeNodeToInstall(nodeIP)
//	if err != nil {
//		outRespDetails(w,500,"reg jobs to node failed,details :"+err.Error(),"为内置任务注册新增节点失败",nil,nil)
//		return
//	}
//	outRespSuccess(w,j,nil)
//}

func writer(eventId string, nodeIp string, doneAll chan *core.JobList, doneOne chan *core.BuildInJob) {
	//defer func() {
	//	ws.Close()
	//}()
	done := make(chan int, 1)
	//核心逻辑写在这，有新的执行完了，就给一个channel add一个
	//eventId:=""

	go func(done chan int) {
		logrus.Infof("starting ping")

		for {
			select {
			case <-done:
				logrus.Infof("heart beat stopping")
				return
			default:
				{
					time.Sleep(5 * time.Second)
					if eventId == "" {
						logrus.Warnf("heart beat msg failed,because event id is null,caused by no job executed")
						continue
					}
					logrus.Infof("sending ping")
					logger := event.GetManager().GetLogger(eventId)
					logger.Info("ping", nil)
					event.GetManager().ReleaseLogger(logger)
				}
			}
		}
	}(done)

	for {
		select {
		case job := <-doneOne:
			logrus.Infof("job %s execute done", job.JobName)
			logger := event.GetManager().GetLogger(job.JobSEQ)
			logger.Info("one job done", map[string]string{"jobId": job.JobId, "status": strconv.Itoa(job.JobResult)})
			eventId = job.JobSEQ
			event.GetManager().ReleaseLogger(logger)
		case result := <-doneAll:

			logrus.Infof("job  execute done")
			logger := event.GetManager().GetLogger(eventId)
			time.Sleep(2 * time.Second)

			done <- 1
			logrus.Infof("stopping heart beat")

			logrus.Infof("send final message ,using eventID:%s", eventId)
			logger.Info("all job done", map[string]string{"step": "final", "status": strconv.FormatBool(result.Result)})
			event.GetManager().ReleaseLogger(logger)
		}
	}
}

func Ping(w http.ResponseWriter, r *http.Request) {
	outSuccess(w)
}
func GetALLGroup(w http.ResponseWriter, r *http.Request) {

	// swagger:operation GET /v2/job/group v2 GetAllGroup
	//
	// 获取所有job的group
	//
	// get all groups
	//
	// ---
	// produces:
	// - application/json
	//
	// Responses:
	//   '200':
	//    description: '["group1",...]'

	resp, err := store.DefalutClient.Get(conf.Config.Cmd, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}

	var cmdKeyLen = len(conf.Config.Cmd)
	var groupMap = make(map[string]bool, 8)

	for i := range resp.Kvs {
		ss := strings.Split(string(resp.Kvs[i].Key)[cmdKeyLen:], "/")
		groupMap[ss[0]] = true
	}

	var groupList = make([]string, 0, len(groupMap))
	for k := range groupMap {
		groupList = append(groupList, k)
	}

	sort.Strings(groupList)
	outJSON(w, groupList)
}
func JobExecute(w http.ResponseWriter, r *http.Request) {

	// swagger:operation PUT /v2/job/{group}-{id}/execute/{name} v2 JobExecute
	//
	// 立即在 node上 执行一次指定group/id 的job
	//
	// execute job
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: group
	//   in: path
	//   description: group name
	//   required: true
	//   type: string
	//   format: string
	// - name: id
	//   in: path
	//   description: job id
	//   required: true
	//   type: string
	//   format: string
	// - name: name
	//   in: path
	//   description: node name
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//    description: '{"ok":true}'

	group := strings.TrimSpace(chi.URLParam(r, "group"))
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if len(group) == 0 || len(id) == 0 {
		outJSONWithCode(w, http.StatusBadRequest, "Invalid job id or group.")
		return
	}

	//node := getStringVal("node", ctx.R)
	//node :=r.FormValue("node")
	node := chi.URLParam(r, "name")
	err := core.PutOnce(group, id, node)
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}
	outSuccess(w)
	//outJSONWithCode(w, http.StatusNoContent, nil)
}
func GetJobNodes(w http.ResponseWriter, r *http.Request) {

	// swagger:operation GET /v2/job/{group}-{id}/nodes v2 GetJobNodes
	//
	// 获取job的可执行节点
	//
	// get job runnable nodes
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: group
	//   in: path
	//   description: group name
	//   required: true
	//   type: string
	//   format: string
	// - name: id
	//   in: path
	//   description: job id
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//    description: '["10.1.1.2",...]'

	job, err := core.GetJob(chi.URLParam(r, "group"), chi.URLParam(r, "id"))
	var statusCode int
	if err != nil {
		if err == utils.ErrNotFound {
			statusCode = http.StatusNotFound
		} else {
			statusCode = http.StatusInternalServerError
		}
		outJSONWithCode(w, statusCode, err.Error())
		return
	}

	var nodes []string
	var exNodes []string
	groups, err := core.GetGroups("")
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range job.Rules {
		inNodes := append(nodes, job.Rules[i].NodeIDs...)
		for _, gid := range job.Rules[i].GroupIDs {
			if g, ok := groups[gid]; ok {
				inNodes = append(inNodes, g.NodeIDs...)
			}
		}
		exNodes = append(exNodes, job.Rules[i].ExcludeNodeIDs...)
		inNodes = SubtractStringArray(inNodes, exNodes)
		nodes = append(nodes, inNodes...)
	}

	outJSON(w, UniqueStringArray(nodes))
}
func DeleteJob(w http.ResponseWriter, r *http.Request) {

	// swagger:operation DELETE /v2/job/{group}-{id} v2 DeleteJob
	//
	// 删除 job
	//
	// delete job by group and id
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: group
	//   in: path
	//   description: group name
	//   required: true
	//   type: string
	//   format: string
	// - name: id
	//   in: path
	//   description: job id
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//    description: '{"ok":true}'

	_, err := core.DeleteJob(chi.URLParam(r, "group"), chi.URLParam(r, "id"))
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}
	outSuccess(w)
	//outJSONWithCode(w, http.StatusNoContent, nil)
}
func GetJob(w http.ResponseWriter, r *http.Request) {

	// swagger:operation GET /v2/job/{group}-{id} v2 GetJob
	//
	// 获取 job
	//
	// get job by group and id
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: group
	//   in: path
	//   description: group name
	//   required: true
	//   type: string
	//   format: string
	// - name: id
	//   in: path
	//   description: job id
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//    description: '{"id":"","kind":0,"name":"aac","group":"default","user":"","cmd":"echo \"hello \">/tmp/aac.txt","pause":true,"parallels":0,"timeout":0,"interval":0,"retry":0,"rules":[{"id":"NEW0.5930536330436825","nids":["172.16.0.118"],"timer":"* 5 * * * *","exclude_nids":["172.16.0.131"]}],"fail_notify":false,"to":[]}'

	job, err := core.GetJob(chi.URLParam(r, "group"), chi.URLParam(r, "id"))
	var statusCode int
	if err != nil {
		if err == utils.ErrNotFound {
			statusCode = http.StatusNotFound
		} else {
			statusCode = http.StatusInternalServerError
		}
		outJSONWithCode(w, statusCode, err.Error())
		return
	}

	outJSON(w, job)
}
func ChangeJobStatus(w http.ResponseWriter, r *http.Request) {

	// swagger:operation POST /v2/job/{group}-{id} v2 ChangeJobStatus
	//
	// 更改 job 状态
	//
	// change job status
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: group
	//   in: path
	//   description: group name
	//   required: true
	//   type: string
	//   format: string
	// - name: id
	//   in: path
	//   description: job id
	//   required: true
	//   type: string
	//   format: string
	// - name:
	//   in: body
	//   description: '{"id":"","kind":0,"name":"aac","group":"default","user":"","cmd":"echo \"hello \">/tmp/aac.txt","pause":true,"parallels":0,"timeout":0,"interval":0,"retry":0,"rules":[{"id":"NEW0.5930536330436825","nids":["172.16.0.118"],"timer":"* 5 * * * *","exclude_nids":["172.16.0.131"]}],"fail_notify":false,"to":[]}'
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//    description: '{"id":"","kind":0,"name":"aac","group":"default","user":"","cmd":"echo \"hello \">/tmp/aac.txt","pause":true,"parallels":0,"timeout":0,"interval":0,"retry":0,"rules":[{"id":"NEW0.5930536330436825","nids":["172.16.0.118"],"timer":"* 5 * * * *","exclude_nids":["172.16.0.131"]}],"fail_notify":false,"to":[]}'

	job := &core.Job{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&job)
	if err != nil {
		outJSONWithCode(w, http.StatusBadRequest, err.Error())
		return
	}
	defer r.Body.Close()

	originJob, rev, err := core.GetJobAndRev(chi.URLParam(r, "group"), chi.URLParam(r, "id"))
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}

	originJob.Pause = job.Pause
	b, err := json.Marshal(originJob)
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}

	_, err = store.DefalutClient.PutWithModRev(originJob.Key(), string(b), rev)
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}

	outJSON(w, originJob)
}
func GetExecutingJob(w http.ResponseWriter, r *http.Request) {

	opt := &ProcFetchOptions{
		Groups:  getStringArrayFromQuery("groups", ",", r),
		NodeIds: getStringArrayFromQuery("nodes", ",", r),
		JobIds:  getStringArrayFromQuery("jobs", ",", r),
	}

	gresp, err := store.DefalutClient.Get(conf.Config.Proc, clientv3.WithPrefix())
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}

	var list = make([]*core.Process, 0, 8)
	for i := range gresp.Kvs {
		proc, err := core.GetProcFromKey(string(gresp.Kvs[i].Key))
		if err != nil {
			logrus.Errorf("Failed to unmarshal Proc from key: %s", err.Error())
			continue
		}

		if !opt.Match(proc) {
			continue
		}
		proc.Time, _ = time.Parse(time.RFC3339, string(gresp.Kvs[i].Value))
		list = append(list, proc)
	}

	sort.Sort(ByProcTime(list))
	outJSON(w, list)
}
func InStringArray(k string, ss []string) bool {
	for i := range ss {
		if ss[i] == k {
			return true
		}
	}

	return false
}
func (opt *ProcFetchOptions) Match(proc *core.Process) bool {
	if len(opt.Groups) > 0 && !InStringArray(proc.Group, opt.Groups) {
		return false
	}

	if len(opt.JobIds) > 0 && !InStringArray(proc.JobID, opt.JobIds) {
		return false

	}

	if len(opt.NodeIds) > 0 && !InStringArray(proc.NodeID, opt.NodeIds) {
		return false
	}

	return true
}
func UpdateJob(w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /v2/job v2 UpdateJob
	//
	// 添加或者更新job
	//
	// add or update job
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: job
	//   in: body
	//   description: '{"id":"","kind":0,"name":"aac","oldGroup":"","group":"default","user":"","cmd":"echo \"hello \">/tmp/aac.txt","pause":true,"parallels":0,"timeout":0,"interval":0,"retry":0,"rules":[{"id":"NEW0.5930536330436825","nids":["172.16.0.118"],"timer":"* 5 * * * *","exclude_nids":["172.16.0.131"]}],"fail_notify":false,"to":[]}'
	//   required: true
	//   type: json
	//   format: string
	//
	// Responses:
	//   '200':
	//    description: '{"ok":true}'
	var job = &struct {
		*core.Job
		OldGroup string `json:"oldGroup"`
	}{}

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(&job)
	if err != nil {
		outJSONWithCode(w, http.StatusBadRequest, err.Error())
		return
	}
	r.Body.Close()

	if err = job.Check(); err != nil {
		outJSONWithCode(w, http.StatusBadRequest, err.Error())
		return
	}

	var deleteOldKey string
	if len(job.ID) == 0 {
		job.ID = uuid.NewV4().String()
	} else {
		job.OldGroup = strings.TrimSpace(job.OldGroup)
		if job.OldGroup != job.Group {
			deleteOldKey = core.JobKey(job.OldGroup, job.ID)
		}
	}

	b, err := json.Marshal(job)
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}

	// remove old key
	// it should be before the put method
	if len(deleteOldKey) > 0 {
		if _, err = store.DefalutClient.Delete(deleteOldKey); err != nil {
			logrus.Errorf("failed to remove old job key[%s], err: %s.", deleteOldKey, err.Error())
			outJSONWithCode(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	_, err = store.DefalutClient.Put(job.Key(), string(b))
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}
	outSuccess(w)
}

//todo response
func JobList(w http.ResponseWriter, r *http.Request) {

	// swagger:operation GET /v2/job v2 JobList
	//
	// 获取job列表
	//
	// get job list
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: node
	//   in: form
	//   description: node name
	//   required: false
	//   type: string
	//   format: string
	// - name: group
	//   in: form
	//   description: group name
	//   required: false
	//   type: string
	//   format: string
	// Responses:
	//   '200':
	//    description: '{"ok":true}'

	node := r.FormValue("node")
	group := r.FormValue("group")

	var prefix = conf.Config.Cmd
	if len(group) != 0 {
		prefix += group
	}

	type jobStatus struct {
		*core.Job
		LatestStatus *core.JobLatestLog `json:"latestStatus"`
	}

	resp, err := store.DefalutClient.Get(prefix, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nodeGroupMap map[string]*core.Group
	if len(node) > 0 {
		nodeGrouplist, err := corenode.GetNodeGroups()
		if err != nil {
			outJSONWithCode(w, http.StatusInternalServerError, err.Error())
			return
		}
		nodeGroupMap = map[string]*core.Group{}
		for i := range nodeGrouplist {
			nodeGroupMap[nodeGrouplist[i].ID] = nodeGrouplist[i]
		}
	}

	var jobIds []string
	var jobList = make([]*jobStatus, 0, resp.Count)
	for i := range resp.Kvs {
		job := core.Job{}
		err = json.Unmarshal(resp.Kvs[i].Value, &job)
		if err != nil {
			outJSONWithCode(w, http.StatusInternalServerError, err.Error())
			return
		}

		if len(node) > 0 && !job.IsRunOn(node, nodeGroupMap) {
			continue
		}
		jobList = append(jobList, &jobStatus{Job: &job})
		jobIds = append(jobIds, job.ID)
	}

	m, err := core.GetJobLatestLogListByJobIds(jobIds)
	if err != nil {
		logrus.Errorf("GetJobLatestLogListByJobIds error: %s", err.Error())
	} else {
		for i := range jobList {
			jobList[i].LatestStatus = m[jobList[i].ID]
		}
	}

	outJSON(w, jobList)
}

type ByProcTime []*core.Process

func (a ByProcTime) Len() int           { return len(a) }
func (a ByProcTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByProcTime) Less(i, j int) bool { return a[i].Time.After(a[j].Time) }
