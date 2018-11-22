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
	"net/http"

	"github.com/goodrain/rainbond/api/proxy"

	"github.com/Sirupsen/logrus"
)

//AcpNodeStruct acp node struct
type AcpNodeStruct struct {
	HTTPProxy proxy.Proxy
}

//Nodes trans to node
func (a *AcpNodeStruct) Nodes(w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /v2/nodes/login v2 login
	//
	// 尝试通过SSH链接远程服务器
	//
	// try to connect to ssh
	//
	// ---
	// produces:
	// - application/json
	//
	// Responses:
	//   '200':
	//     description: '{"bean":{"hostport":"10.0.55.73:22","type":true,"result":"success"}}'

	// swagger:operation PUT /v2/nodes/{ip}/init v2 init
	//
	// 完成基础初始化
	//
	// try to basicly init
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: ip
	//   in: path
	//   description: '10.0.55.73'
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//     description: '{"bean":{"List":[{"JobSEQ":"asdfsa-sdfasdf-asdf","JobId":"online_init","JobName":"init","JobResult":0}],"Result":false}}'

	// swagger:operation GET /v2/nodes/{ip}/install/status v2 CheckJobGetStatus
	//
	// 检查job状态
	//
	// check job status
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: ip
	//   in: path
	//   description: '10.0.55.73'
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//     description: '{"bean":{"List":[{"JobSEQ":"asdfsa-sdfasdf-asdf","JobId":"online_init","JobName":"init","JobResult":0}],"Result":false}}'

	// swagger:operation GET /v2/nodes/{ip}/install v2 StartBuildInJobs
	//
	// 开始执行内置任务
	//
	// start build-in jobs
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: ip
	//   in: path
	//   description: '10.0.55.73'
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//     description: '{"bean":{"List":[{"JobSEQ":"asdfsa-sdfasdf-asdf","JobId":"online_init","JobName":"init","JobResult":3}],"Result":false}}'

	// swagger:operation PUT /v2/nodes/{ip}/install v2 StartBuildInJobs
	//
	// 开始安装
	//
	// check job status
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: ip
	//   in: path
	//   description: '10.0.55.73'
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//     description: '{"bean":{"List":[{"JobSEQ":"asdfsa-sdfasdf-asdf","JobId":"online_init","JobName":"init","JobResult":3}],"Result":false}}'

	// swagger:operation PUT /v2/nodes/{node}/unschedulable v2 Cordon
	//
	// 使节点不可调度
	//
	// make node unschedulable
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: node
	//   in: path
	//   description: nodeuid
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//     description: '{"code":200,"msg":"success","msgcn":"成功","body":nil}'

	// swagger:operation PUT /v2/nodes/{node}/reschedulable v2 UnCordon
	//
	// 使节点可调度
	//
	// make node schedulable
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: node
	//   in: path
	//   description: nodeuid
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//     description: '{"code":200,"msg":"success","msgcn":"成功","body":nil}'

	// swagger:operation DELETE /v2/nodes/{node} v2 DeleteFromDB
	//
	// 从etcd 删除计算节点
	//
	// delete node from etcd
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: node
	//   in: path
	//   description: nodeuid
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//     description: '{"code":200,"msg":"success","msgcn":"成功","body":nil}'

	// swagger:operation POST /v2/nodes/{node}/down v2 DeleteNode
	//
	// 下线计算节点
	//
	// offline node
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: node
	//   in: path
	//   description: nodeuid
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//     description: '{"code":200,"msg":"success","msgcn":"成功","body":nil}'

	// swagger:operation POST /v2/nodes v2 New
	//
	// 添加新节点到etcd
	//
	// add new node info to etcd
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name:
	//   in: body
	//   description: '{"uuid":"757755e4-99e4-11e7-bab9-00163e020ab5","host_name":"10.0.55.73","internal_ip":"10.0.55.73","external_ip":"10.0.55.73","available_memory":16,"available_cpu":4,"role":"","status":"offline","labels":{"key1":"value1"},"unschedulable":false}'
	//   required: true
	//   type: string
	//   format: json
	// Responses:
	//   '200':
	//     description: '{"code":200,"msg":"success","msgcn":"成功","body":nil}'

	// swagger:operation GET /v2/nodes//{node}/basic v2 GetNodeBasic
	//
	// 从服务器获取节点基本信息
	//
	// get node basic info from etcd
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: node
	//   in: path
	//   description: nodeuid
	//   required: true
	//   type: string
	//   format: string
	// Responses:
	//   '200':
	//     description: '{"code":200,"msg":"success","msgcn":"成功","body":{"list":null,"bean":{{"uuid":"757755e4-99e4-11e7-bab9-00163e020ab5","host_name":"10.0.55.73","internal_ip":"10.0.55.73","external_ip":"10.0.55.73","available_memory":16,"available_cpu":4,"role":"","status":"offline","labels":{"key1":"value1"},"unschedulable":false}}}}'

	// swagger:operation GET /v2/nodes/{node}/details v2 GetNodeDetails
	//
	// 从服务器获取节点详细信息
	//
	// get node details info from k8s
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: node
	//   in: path
	//   description: nodeuid
	//   required: true
	//   type: string
	//   format: string
	// Responses:
	//   '200':
	//     description: '{"code":200,"msg":"success","msgcn":"成功","body":{"list":null,"bean":{"name":"10.0.55.73","role":"","labels":{"key1":"value1"},"annotations":{"node.alpha.kubernetes.io/ttl":"0","volumes.kubernetes.io/controller-managed-attach-detach":"true"},"creationtimestamp":"2017-09-19 11:23:09 +0800 CST","conditions":[{"type":"OutOfDisk","status":"False","lastHeartbeatTime":"2017-09-20T01:37:25Z","lastTransitionTime":"2017-09-19T03:23:18Z","reason":"KubeletHasSufficientDisk","message":"kubelet has sufficient disk space available"},{"type":"MemoryPressure","status":"False","lastHeartbeatTime":"2017-09-20T01:37:25Z","lastTransitionTime":"2017-09-19T03:23:18Z","reason":"KubeletHasSufficientMemory","message":"kubelet has sufficient memory available"},{"type":"DiskPressure","status":"False","lastHeartbeatTime":"2017-09-20T01:37:25Z","lastTransitionTime":"2017-09-19T03:23:18Z","reason":"KubeletHasNoDiskPressure","message":"kubelet has no disk pressure"},{"type":"Ready","status":"True","lastHeartbeatTime":"2017-09-20T01:37:25Z","lastTransitionTime":"2017-09-19T10:59:55Z","reason":"KubeletReady","message":"kubelet is posting ready status"}],"addresses":{"Hostname":"10.0.55.73","InternalIP":"10.0.55.73","LegacyHostIP":"10.0.55.73"},"capacity":{"cpu":"4","memory":"15886","pods":"10k"},"allocatable":{"cpu":"4","memory":"15786","pods":"10k"},"systeminfo":{"machineID":"df39b3efbcee4d5c84b2feab34009235","systemUUID":"08A7D00A-C4B6-4AB7-8D8D-DA14C104A4DC","bootID":"b4cd3c6c-e18d-43ae-9f11-3125110e9179","kernelVersion":"3.10.0-514.21.2.el7.x86_64","osImage":"CentOS Linux 7 (Core)","containerRuntimeVersion":"docker://1.12.6","kubeletVersion":"v1.6.4-66+4fd15729100998-dirty","kubeProxyVersion":"v1.6.4-66+4fd15729100998-dirty","operatingSystem":"linux","architecture":"amd64"},"externalid":"10.0.55.73","nonterminatedpods":[{"namespace":"1b56aa1ed27b4289b6f108e551545f29","id":"gr04bff0","name":"c0dd44112bd39ff7dda3ac3a47e619ea-3m1m8","cpurequest":"80m","cpurequestr":"2","cpulimits":"120m","cpulimitsr":"3","memoryrequests":"268435456","memoryrequestsr":"0","memorylimits":"268435456","memorylimitsr":"0"},{"namespace":"1b56aa1ed27b4289b6f108e551545f29","id":"gr04bff0","name":"c0dd44112bd39ff7dda3ac3a47e619ea-bxfmc","cpurequest":"80m","cpurequestr":"2","cpulimits":"120m","cpulimitsr":"3","memoryrequests":"268435456","memoryrequestsr":"0","memorylimits":"268435456","memorylimitsr":"0"},{"namespace":"1b56aa1ed27b4289b6f108e551545f29","id":"","name":"web-0","cpurequest":"0","cpurequestr":"0","cpulimits":"0","cpulimitsr":"0","memoryrequests":"0","memoryrequestsr":"0","memorylimits":"0","memorylimitsr":"0"},{"namespace":"1b56aa1ed27b4289b6f108e551545f29","id":"","name":"web-1","cpurequest":"0","cpurequestr":"0","cpulimits":"0","cpulimitsr":"0","memoryrequests":"0","memoryrequestsr":"0","memorylimits":"0","memorylimitsr":"0"},{"namespace":"232bd923d3794b979974bb21b863608b","id":"gr184a0b","name":"bc74e3d2cc734ce1a81f0a1903286e35-g4p31","cpurequest":"480m","cpurequestr":"12","cpulimits":"1780m","cpulimitsr":"44","memoryrequests":"2Gi","memoryrequestsr":"12","memorylimits":"2Gi","memorylimitsr":"12"},{"namespace":"232bd923d3794b979974bb21b863608b","id":"gr868196","name":"d2fa251096c9452ba25d081d861265d9-mdqln","cpurequest":"120m","cpurequestr":"3","cpulimits":"640m","cpulimitsr":"16","memoryrequests":"512Mi","memoryrequestsr":"3","memorylimits":"512Mi","memorylimitsr":"3"}],"allocatedresources":{"CPULimits":"2660","CPULimitsR":"66","CPURequests":"760","CPURequestsR":"19","MemoryLimits":"3072","MemoryLimitsR":"15","MemoryRequests":"3072","MemoryRequestsR":"15"},"events":null}}}'

	// swagger:operation GET /v2/nodes v2 GetNodeList
	//
	// 从etcd获取节点简单列表信息
	//
	// get node list info from etcd
	//
	// ---
	// produces:
	// - application/json
	//
	// Responses:
	//   '200':
	//    description: '{"code":200,"msg":"success","msgcn":"成功","body":{"list":[{"uuid":"915f2c95-b723-4c88-9860-dbe33f4f51ac","host_name":"10.0.55.73","internal_ip":"10.0.55.73","external_ip":"10.0.55.73","available_memory":16267956,"available_cpu":4,"role":"compute","status":"create","labels":{"key1":"value1"},"unschedulable":false},{"uuid":"1234c95-b723-4c88-9860-dbe33f4f51ac","host_name":"10.0.55.73","internal_ip":"10.0.55.73","external_ip":"10.0.55.73","available_memory":16267956,"available_cpu":4,"role":"","status":"running","labels":{"key1":"value4"},"unschedulable":false}]}}'

	// swagger:operation POST /v2/nodes/{node} v2 UpdateNode
	//
	// 更新node
	//
	// update
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: node
	//   in: path
	//   description: nodeuid
	//   required: true
	//   type: string
	//   format: string
	// - name:
	//   in: body
	//   description: '{"uuid": "ccc", "Status":"create","host_name": "10.0.55.73", "internal_ip": "10.0.55.73", "external_ip": "10.0.55.73", "available_memory": 16267956, "available_cpu": 4, "role": "", "labels": {"key1": "value1"}, "unschedulable": false}'
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//    description: '{"code":200,"msg":"success","msgcn":"成功","body":{"list":[{"uuid":"1234c95-b723-4c88-9860-dbe33f4f51ac","host_name":"10.0.55.73","internal_ip":"10.0.55.73","external_ip":"10.0.55.73","available_memory":16267956,"available_cpu":4,"role":"","status":"running","labels":{"key1":"value4"},"unschedulable":false}]}}'

	// swagger:operation PUT /v2/nodes/{node} v2 AddNode
	//
	// 重新上线计算节点
	//
	// add node
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: node
	//   in: path
	//   description: nodeuid
	//   required: true
	//   type: string
	//   format: string
	//
	// Responses:
	//   '200':
	//    description: '{"code":200,"msg":"success","msgcn":"成功","body":nil}'

	// swagger:operation POST /v2/nodes/{node}/label v2 labels
	//
	// 为node添加label
	//
	// add label to node
	//
	// ---
	// produces:
	// - application/json
	// Responses:
	//   '200':
	//    description: '{"code":200,"msg":"success","msgcn":"成功","body":nil}'
	logrus.Debugf("proxy acp_node api %s", r.RequestURI)
	a.HTTPProxy.Proxy(w, r)
}

//Apps trans to app
func (a *AcpNodeStruct) Apps(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/apps/{app_name}/discover v2 GetServiceEndPoints
	//
	// 获取指定服务endpoints
	//
	// get endpoints of app_name
	//
	// ---
	// produces:
	// - application/json
	//
	// Responses:
	//   '200':
	//    description: '{"list":[{"name":"d275f5b34faf","url":"10.0.55.72:6363","weight":0}]}'
}

//Jobs trans to job
func (a *AcpNodeStruct) Jobs(w http.ResponseWriter, r *http.Request) {
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

	// swagger:operation PUT /v2/job/{id}/group/{group}/node/{name} v2 JobExecute
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

	// swagger:operation GET /v2/job/{id}/groups/{group}/nodes v2 GetJobNodes
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

	// swagger:operation DELETE /v2/job/{id}/group/{group} v2 DeleteJob
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

	// swagger:operation GET /v2/job/{id}/group/{group} v2 GetJob
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

	// swagger:operation POST /v2/job/{id}/group/{group} v2 ChangeJobStatus
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
	logrus.Debugf("proxy acp_node api %s", r.RequestURI)
	a.HTTPProxy.Proxy(w, r)
}
