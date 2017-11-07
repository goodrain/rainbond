
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

package core

import (
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/goodrain/rainbond/pkg/util"
	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/k8s"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	CanRunJob chan string
)

func PrepareState(loginInfo *model.Login) (*JobList, error) {
	cli, err := UnifiedLogin(loginInfo)
	if err != nil {
		logrus.Errorf("login to target host failed,details %s", err.Error())
		return nil, err
	}

	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath, clientv3.WithPrefix())
	if err != nil {
		logrus.Errorf("get acp_config from etcd failed,details %s", err.Error())
		return nil, err
	}
	netStatus := "online"
	for _, v := range resp.Kvs {
		logrus.Infof("get net state from db,now is %s", v.Key)
		if string(v.Key) == "netStatus" {
			netStatus = string(v.Value)
			break
		}
	}
	toInstall := ""
	if netStatus == "online" {
		sess, err := cli.NewSession()
		if err != nil {
			logrus.Errorf("get remote host ssh session failed,details %s", err.Error())
			return nil, err
		}
		buf := bytes.NewBuffer(nil)
		cmd := "bash -c   \"$(curl -s repo.goodrain.com/node_actions/compute/prepare/check.sh)\""

		sess.Stdout = buf
		logrus.Infof("prepare run check installation cmd ,details %s", cmd)
		err = sess.Run(cmd)
		sess.Close()
		if err != nil {
			logrus.Errorf("run command %s on host ssh session failed,details %s", cmd, err.Error())
			return nil, err
		}
		logrus.Infof("check result is %s", buf)
		n := strings.Split(buf.String(), "\n")[0]
		infos := strings.Split(n, ":")
		system := infos[0]
		host := loginInfo.HostPort[0:len(loginInfo.HostPort)]

		err = UpdateMultiConfig(SYSTEM, host, system)
		if err != nil {
			logrus.Warnf("update config %s to %s failed,details %s", SYSTEM, system, err.Error())
		}
		toInstall = infos[1]

		inited := inited(toInstall)
		logrus.Infof("target system is %s ,is this system inited result is %v,needed component is %v", system, inited, toInstall)
		if !inited {
			//执行init
			buf := bytes.NewBuffer(nil)
			mip, err := GetMIp()
			if err != nil {
				logrus.Errorf("error get master ip,details %s", err.Error())
				return nil, err
			}
			//mip:="10.0.55.72"
			cmd := "bash -c \"set " + system + " " + GetInstallTypeOrDefault() + " " + GetRepoVersionOrDefault() + " " + mip + ";$(curl -s repo.goodrain.com/node_actions/compute/init/init_compute.sh)\""
			//cmd := "bash -c \"set "+system+" "+"default"+" " +"3.4"+" "+mip+";$(curl -s repo.goodrain.com/node_actions/compute/init/init_compute.sh)\""

			logrus.Infof("executing init cmd,details %s", cmd)
			sess, err := cli.NewSession()
			if err != nil {
				logrus.Errorf("get remote host ssh session failed,details %s", err.Error())
				return nil, err
			}
			sess.Stdout = buf
			err = sess.Run(cmd)
			sess.Close()
			if err != nil {
				logrus.Errorf("run init script, run command %s on host ssh session failed,details %s", cmd, err.Error())
				return nil, err
			}
		}
		etcd, err := GetETCDIp()
		if err != nil {
			logrus.Errorf("err get etcd ip args,details %s", err.Error())
			return nil, err
		}
		cmd = "bash -c \"set " + system + " " + etcd + ";$(curl -s repo.goodrain.com/node_actions/compute/acp_node/acp_node.sh)\""
		logrus.Infof("installing acp_node, using cmd : %s", cmd)
		sess, err = cli.NewSession()
		bufACP := bytes.NewBuffer(nil)
		sess.Stdout = bufACP
		err = sess.Run(cmd)
		if err != nil {
			logrus.Errorf("install acp_node, run command %s on host ssh session failed,details %s", cmd, err.Error())
			return nil, err
		}
		logrus.Infof("install acp_node stdout is %s", bufACP.String())
		sess.Close()
		logrus.Infof("在线安装acp_node成功")
	} else {
		sess, err := cli.NewSession()
		if err != nil {
			logrus.Errorf("get remote host ssh session failed,details %s", err.Error())
			return nil, err
		}
		buf := bytes.NewBuffer(nil)
		cmd := "bash /usr/local/acp-node/compute/prepare/check.sh"
		sess.Stdout = buf
		logrus.Infof("prepare run check installation cmd ,details %s", cmd)
		err = sess.Run(cmd)
		if err != nil {
			logrus.Errorf("run command %s on host ssh session failed,details %s", cmd, err.Error())
			return nil, err
		}
		sess.Close()
		logrus.Infof("check result is %s", buf)
		infos := strings.Split(buf.String(), ":")
		system := infos[0]
		toInstall = infos[1]
		host := loginInfo.HostPort[0:len(loginInfo.HostPort)]
		//记录主机系统
		err = UpdateMultiConfig(SYSTEM, host, system)
		inited := inited(toInstall)
		logrus.Infof("target system is %s ,is this system inited result is %v,needed component is %v", system, inited, toInstall)
		if inited {
			//执行init
			mip, err := GetMIp()
			if err != nil {
				logrus.Errorf("error get master ip,details %s", err.Error())
				return nil, err
			}
			cmd = "bash /usr/local/acp-node/compute/init/init_compute.sh " + system + " " + GetInstallTypeOrDefault() + " " + GetRepoVersionOrDefault() + " " + mip
			logrus.Infof("locally executing init cmd,details %s", cmd)
			sess, err := cli.NewSession()
			if err != nil {
				logrus.Errorf("get remote host ssh session failed,details %s", err.Error())
				return nil, err
			}

			err = sess.Run(cmd)
			sess.Close()
			if err != nil {
				logrus.Errorf("run command %s on host ssh session failed,details %s", cmd, err.Error())
				return nil, err
			}
		}
		etcd, err := GetETCDIp()
		if err != nil {
			logrus.Errorf("err get etcd ip args,details %s", err.Error())
			return nil, err
		}
		cmd = "bash /usr/local/acp-node/compute/acp_node/acp_node.sh " + system + " " + etcd
		logrus.Infof("installing acp_node,using command %s", cmd)
		sess, err = cli.NewSession()
		err = sess.Run(cmd)
		if err != nil {
			logrus.Errorf("run command %s on host ssh session failed,details %s", cmd, err.Error())
			return nil, err
		}
		sess.Close()
		logrus.Infof("离线安装acp_node成功")
	}

	host := strings.Split(loginInfo.HostPort, ":")[0]

	toInstall = strings.Replace(toInstall, "\n", "", -1)
	toInstalls := strings.Split(toInstall, " ")
	eventId := util.NewUUID()
	logrus.Infof("need to install component :%v", toInstalls)
	//todo 注册需要安装的组件 初始状态注册到etcd中 done
	//todo 此处需要获取在线／离线 done
	jobs, err := GetBuildinJobs()
	for _, v := range jobs {
		v.JobSEQ = eventId
	}

	if err != nil {
		return nil, err
	}
	//注册需要运行的job
	err = UpdateNodeJobStatus(host, filterNeededJobs(jobs, netStatus, toInstalls))
	if err != nil {
		return nil, err
	}

	jl, err := GetBuildInJobWithStatusForNode(toInstalls, host)
	if err != nil {
		logrus.Warnf("error get build-in jobs for node %s ,details: %s", host, err.Error())
		return nil, err
	}
	jl.SEQ = eventId
	_, err = store.DefalutClient.Put(conf.Config.ConfigPath+"result_log/"+host, jl.SEQ)
	if err != nil {
		logrus.Warnf("can't set job's total done event id,details:%s", err.Error())
	}
	logrus.Infof("prepare return job list status ,details:%v", jl)
	return jl, nil
}
func inited(toInstall string) bool {
	inited := true
	toInstalls := strings.Split(toInstall, " ")
	for _, v := range toInstalls {
		if v == "init" {
			//说明还没有执行init
			inited = false
			break
		} else {
			//inited=true//更新状态表示已经执行了init
		}
	}
	return inited
}
func filterNeededJobs(all []*BuildInJob, net string, needs []string) []*BuildInJob {
	result := []*BuildInJob{}

	for _, v := range needs {
		v = strings.TrimSpace(v)
		for _, v2 := range all {
			logrus.Infof("needs %s,now is %s", v, v2.JobId)
			if v2.JobName == v && strings.Contains(v2.JobId, net) {
				result = append(result, v2)
				break
			}
		}
	}
	logrus.Infof("filter needed job result is :%v", result)
	return result
}
func CheckJobGetStatus(node string) (*JobList, error) {
	_, info, err := CheckJob(node)
	if err != nil {
		logrus.Errorf("check job failed,details :%s", err.Error())
		return nil, err
	}
	toInstall := strings.Split(info, " ")

	jl, err := GetBuildInJobWithStatusForNode(toInstall, node)
	if err != nil {
		logrus.Warnf("error get build-in jobs for node %s ,details: %s", node, err.Error())
		return nil, err
	}
	return jl, nil
}
func watchRealBuildInLog(ch chan *JobLog) {
	rch := WatchBuildInLog()
	joblog := &JobLog{}
onedone:
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch {
			case ev.IsCreate() || ev.IsModify():
				logrus.Infof("find a job executed,log created")
				json.Unmarshal(ev.Kv.Value, joblog)
				ch <- joblog

				break onedone
			}
		}
	}
	logrus.Infof("job done ,find log,break watch")
}
func watchBuildInJobLog(ch chan map[string]string) {
	rch := WatchBuildInLog()
	joblog := &JobLog{}
onedone:
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch {
			case ev.IsCreate() || ev.IsModify():
				logrus.Infof("find a job executed,log created")
				json.Unmarshal(ev.Kv.Value, joblog)
				a := make(map[string]string)

				a["output"] = joblog.Output
				a["name"] = joblog.Name
				a["result"] = strconv.FormatBool(joblog.Success)
				a["node"] = joblog.Node
				a["jobid"] = joblog.JobId
				logrus.Infof("a job execute done,job log is %v", a)
				ch <- a
				break onedone
			}
		}
	}
}
func RunBuildJobs(node string, done chan *JobList, doneOne chan *BuildInJob) error {
	//jl, err := CheckJobGetStatus(node)
	jl, err := GetJobStatusByNodeIP(node)
	jlb, _ := json.Marshal(jl)
	logrus.Infof("needed job list is %s", string(jlb))
	if err != nil {
		logrus.Warnf("error get build-in jobs for node %s ,details: %s", node, err.Error())
		return err
	}

	allSuccess := true
	for _, v := range jl.List {
		job := &Job{}
		jobId := v.JobId
		if v.JobResult == 1 {
			continue
		}
		logrus.Infof("geting buildin job from etcd by key %s", conf.Config.BuildIn+jobId)
		resp, err := store.DefalutClient.Get(conf.Config.BuildIn+jobId, clientv3.WithPrefix())
		if err != nil {
			logrus.Errorf("can't found build-in job by given key %s,details: %s", conf.Config.BuildIn+jobId, err.Error())
		}
		if !(resp.Count > 0) {
			logrus.Errorf("get nothing from etcd")
			return nil
		}
		if err = json.Unmarshal(resp.Kvs[0].Value, job); err != nil {
			logrus.Errorf("can't unmarshal build-in job by given key %s,details: %s", conf.Config.BuildIn+jobId, err.Error())
		}
		logger := event.GetManager().GetLogger(v.JobSEQ)
		updateBuildInJob(v, 3)

		err = UpdateNodeJobStatus(node, jl.List)
		logrus.Infof("update job to running")
		if err != nil {
			logrus.Warnf("update node build-in job status failed,details :%s", err.Error())
		}
		job.Init(node)

		netStatus := strings.Split(jobId, "_")[0]
		logrus.Infof("prepare to run build-in jobs for node %s,network status is %s,component is %s", node, netStatus, job.Name)

		ch := make(chan *JobLog)
		go watchRealBuildInLog(ch)
		logrus.Infof("wait for job %s done", jobId)
		//在这加入参数吧

		rawCmd, toAddArg, err := getOrderdArgsByJobName(node, job)
		if err != nil {
			logrus.Errorf("get necessary args failed,details %s", err.Error())
			removeCMDFromJob(job, rawCmd)
			return err
		}

		updateBuildInJobCMD(job, toAddArg)
		logger.Info("prepare running job "+v.JobName, map[string]string{"jobId": v.JobId, "status": strconv.Itoa(v.JobResult)})
		event.GetManager().ReleaseLogger(logger)

		if err = PutBuildIn(jobId, node); err != nil {
			logrus.Errorf("can't put job to a watched etcd path,details %s", err.Error())
		}

		joblog := <-ch
		logrus.Infof("job %s done", jobId)
		removeCMDFromJob(job, rawCmd)
		//这里应该去掉 加入的参数
		//要在job执行之前watch
		//watchBuildInJob,获取后判断success.success是不可靠的。应该判断return值//success是可靠的
		if !joblog.Success { //执行失败//检查job执行情况，执行成功才执行下一个，此处阻塞
			updateBuildInJob(v, 2)
			err = UpdateNodeJobStatus(node, jl.List)
			if err != nil {
				logrus.Warnf("update node build-in job status failed,details :%s", err.Error())
			}
			allSuccess = false
			//todo
			//logger:=event.GetManager().GetLogger(uuid.NewV4().String())
			//logger.Info("build-in job execute failed,break and exit",nil)
			//event.GetManager().ReleaseLogger(logger)
			logrus.Errorf("内置任务执行失败")
			doneOne <- v
			break
		} else { //执行成功
			updateBuildInJob(v, 1)
			err = UpdateNodeJobStatus(node, jl.List)
			if err != nil {
				logrus.Warnf("job execute success but update node build-in job status failed,details :%s", err.Error())
			}

			if job.Name == "network" {
				//todo 不用传ip
				err := addToNet(node)
				if err != nil {
					logrus.Errorf("error reg network midoman,details %s", err.Error())
					allSuccess = false
					break
				}
			}
			if job.Name == "kubelet" {
				err := addToKubernetes(node)
				if err != nil {
					logrus.Errorf("error reg kubelet,details %s", err.Error())
					allSuccess = false
					break
				}
			}
			allSuccess = true
			doneOne <- v
		}
		//更新前端列表 websocket会监听到，监听到了会朝前台response
		logrus.Infof("jon one done prepare to next")
	}

	if allSuccess {
		jl.Result = true
		//todo
		err := updateNodeDB(node, "running")
		if err != nil {
			logrus.Errorf("error occured while update node info,details %s", err.Error())
		}
		err = markSuccess(node)
		if err != nil {
			logrus.Errorf("error occured while mark node install success,details %s", err.Error())
		}
		err = removeNode(node)
		if err != nil {
			logrus.Errorf("error occured while remove job's executable node ,details %s", err.Error())
		}
		logrus.Infof("delete node %s 's runnable ", node)
		store.DefalutClient.DelRunnable("/acp_node/runnable/" + node)
	} else {
		jl.Result = false
		err := updateNodeDB(node, "failed")
		if err != nil {
			logrus.Errorf("error occured while update node info,details %s", err.Error())
		}
	}
	done <- jl
	logrus.Infof("result :%v", jl.Result)
	return nil
}
func removeNode(node string) error {
	jobs, err := GetBuildinJobs() //状态为未安装
	if err != nil {
		return err
	}

	return removeNodeFromJob(jobs, node)
}

func removeNodeFromJob(jobs []*BuildInJob, node string) error {
	for _, v := range jobs {
		coreJobId := v.JobId

		resp, err := store.DefalutClient.Get(conf.Config.BuildIn+coreJobId, clientv3.WithPrefix())
		if err != nil {
			return err
		}
		job := &Job{}
		err = json.Unmarshal(resp.Kvs[0].Value, job)
		if err != nil {
			return err
		}
		for _, rv := range job.Rules {
			nids := []string{}
			for _, nid := range rv.NodeIDs {
				if nid == node {
					continue
				} else {
					nids = append(nids, nid)
				}
				rv.NodeIDs = nids
			}
		}
		if err != nil {
			return err
		}

		body, err := json.Marshal(job)
		if err != nil {
			return err
		}
		store.DefalutClient.Put(conf.Config.BuildIn+coreJobId, string(body))
	}
	return nil
}

func markSuccess(node string) error {
	loginInfo, err := GetLoginInfoByNode(node)

	cli, err := UnifiedLogin(loginInfo)
	if err != nil {
		logrus.Errorf("login remote host failed,details %s", err.Error())
		return err
	}
	sess, err := cli.NewSession()
	if err != nil {
		logrus.Errorf("get remote host ssh session failed,details %s", err.Error())
		return err
	}
	defer sess.Close()
	buf := bytes.NewBuffer(nil)
	sess.Stdout = buf
	err = sess.Run("echo " + loginInfo.HostType + conf.Config.InstalledMarker)
	if err != nil {
		logrus.Errorf("error run cmd %s ,details %s", "echo 'success' > '/etc/acp/install/status'", err.Error())
		return err
	}
	return nil
}

func addToKubernetes(uid string) error {
	cnode, err := k8s.GetSource(conf.Config.K8SNode + uid)
	if err != nil {
		logrus.Errorf("error get node info from etcd by key %s", conf.Config.K8SNode+uid)
		return err
	}
	newk8sNode, err := k8s.CreateK8sNode(cnode)
	if err != nil {
		logrus.Errorf("generating k8s code failed details %s", err.Error())
		cnode.Status = "failed"
		return err
	}
	realK8SNode, err := k8s.K8S.Core().Nodes().Create(newk8sNode)
	logrus.Infof("creating k8s node ,get node uid is %s", string(realK8SNode.UID))
	if err != nil {
		logrus.Errorf("create k8s code failed details %s", err.Error())

		return err
	}
	//rawUUID:=cnode.UUID
	data, _ := json.Marshal(cnode)
	logrus.Infof("adding node :%s online ,updated to %s ", string(realK8SNode.UID), string(data))
	cnode.UUID = string(realK8SNode.UID)
	//防止下面更新内存cpu时获取不到
	err = k8s.AddSource(conf.Config.K8SNode+uid, cnode)
	if err != nil {
		return err
	}
	//err = DeleteSource(conf.Config.K8SNode + uid)
	return err
}
func addToNet(node string) error {

	login, err := GetLoginInfoByNode(node)
	if err != nil {
		logrus.Errorf("error get login info")
		return err
	}
	args, err := getNetWorkArgs(login)
	if err != nil {
		logrus.Errorf("error get network args")
		return err
	}
	lastArgs := "set"
	for _, v := range args {
		lastArgs += " " + v
	}
	lastArgs += ";"

	resp, err := http.Get("http://repo.goodrain.com/node_actions/compute/network/set_midonet_cni.sh")
	//

	if err != nil {
		logrus.Infof("download shell failed,details %s", err.Error())
		return err
	}
	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)
	lastArgs = lastArgs + string(b)
	logrus.Infof("reg network using cmd %s", lastArgs)
	cmd := exec.Command("bash", "-c", lastArgs)
	buf := bytes.NewBuffer(nil)
	cmd.Stdout = buf
	cmd.Stderr = buf
	err = cmd.Run()
	if err != nil {
		logrus.Infof("error install midoman network,details %s", err.Error())
		return err
	}
	logrus.Infof("reg std out/err is %s", buf.String())
	return nil
}
func GetLoginInfoByNode(node string) (*model.Login, error) {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + "login/" + node)
	if err != nil {
		logrus.Errorf("error get response by key %s", conf.Config.ConfigPath+"login/"+node)
		return nil, err
	}
	if resp.Count <= 0 {
		return nil, errors.New("get nothing from etcd")
	}
	v := resp.Kvs[0].Value
	result := &model.Login{}
	err = json.Unmarshal(v, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
func getNetWorkArgs(loginInfo *model.Login) ([]string, error) {

	cli, err := UnifiedLogin(loginInfo)
	if err != nil {
		logrus.Errorf("login remote host failed,details %s", err.Error())

		return nil, err
	}
	sess, err := cli.NewSession()
	if err != nil {
		logrus.Errorf("get remote host ssh session failed,details %s", err.Error())
		return nil, err
	}
	defer sess.Close()
	buf := bytes.NewBuffer(nil)
	sess.Stdout = buf
	err = sess.Run("cat /etc/midonet_host_id.properties | grep 'host_uuid' | awk -F '=' '{print $2}'")
	if err != nil {
		logrus.Errorf("error run cmd %s ,details %s", "cat /etc/midonet_host_id.properties | grep 'host_uuid' | awk -F '=' '{print $2}'", err.Error())
		return nil, err
	}
	uids := buf.String()

	logrus.Infof("get midonet host id is %s", uids)
	hostUid := uids
	hostIp := strings.Split(loginInfo.HostPort, ":")[0]
	hostUid = strings.Split(hostUid, "\n")[0]
	mysqlIp, err := GetMysqlIp()
	if err != nil {
		logrus.Errorf("error get config mysql ip details %s", err.Error())
		return nil, err
	}
	mysqlUser, err := GetMysqlUser()
	if err != nil {
		logrus.Errorf("error get config mysql ip details %s", err.Error())
		return nil, err
	}
	mysqlPwd, err := GetMySqlPwd()
	if err != nil {
		logrus.Errorf("error get config mysql ip details %s", err.Error())
		return nil, err
	}
	return []string{hostUid, hostIp, mysqlIp, mysqlUser, mysqlPwd}, nil
}
func updateNodeDB(node, engStatus string) error {

	cnode, err := k8s.GetSource(conf.Config.K8SNode + node)
	if err != nil {
		logrus.Infof("get source from etcd failed,details %s", err)
		return err
	}
	loginInfo := new(model.Login)
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + "login/" + node)
	if err != nil {
		logrus.Errorf("prepare stage  failed,get login info failed,details %s", err.Error())

		return err
	}
	if resp.Count > 0 {
		err := json.Unmarshal(resp.Kvs[0].Value, loginInfo)
		if err != nil {
			logrus.Errorf("decode request failed,details %s", err.Error())

			return err
		}
	} else {
		logrus.Errorf("prepare stage failed,get login info failed,details %s", err.Error())

		return err
	}

	cli, err := UnifiedLogin(loginInfo)
	if err != nil {
		logrus.Errorf("login to target host failed,details %s", err.Error())
		return err
	}
	sess, err := cli.NewSession()
	if err != nil {
		logrus.Errorf("get remote host ssh session failed,details %s", err.Error())
		return err
	}
	buf := bytes.NewBuffer(nil)
	cmd := "free -mt | grep 'Mem' | awk '{print $2/1000 }'"

	sess.Stdout = buf
	logrus.Infof("prepare run check mem cmd ,details %s", cmd)
	err = sess.Run(cmd)
	sess.Close()
	if err != nil {
		logrus.Errorf("run command %s on host ssh session failed,details %s", cmd, err.Error())
		return err
	}
	res := buf.String()

	gb, err := strconv.Atoi(strings.Split(res, ".")[0])
	logrus.Infof("get remote node memory gb size is %s", buf.String())

	sessc, err := cli.NewSession()
	if err != nil {
		logrus.Errorf("get remote host ssh session failed,details %s", err.Error())
		return err
	}
	bufcpu := bytes.NewBuffer(nil)
	cmdcpu := "cat /proc/cpuinfo| grep 'processor'| wc -l"

	sessc.Stdout = bufcpu
	logrus.Infof("prepare run check mem cmd ,details %s", cmd)
	err = sessc.Run(cmdcpu)
	sessc.Close()
	if err != nil {
		logrus.Errorf("run command %s on host ssh session failed,details %s", cmd, err.Error())
		return err
	}
	resc := bufcpu.String()
	cpu, err := strconv.Atoi(strings.Replace(resc, "\n", "", -1))
	logrus.Infof("get remote node memory kb size is %s", resc)
	if err != nil {
		logrus.Infof("error get remote mem info")
		return err
	}
	cnode.AvailableMemory = int64(gb - 5)
	cnode.AvailableCPU = int64(cpu)
	cnode.Status = engStatus
	logrus.Infof("update node mem to %d,status to %s", cnode.AvailableMemory, cnode.Status)

	if engStatus == "running" {
		kn, err := k8s.K8S.Core().Nodes().Get(node, v1.GetOptions{})
		if err != nil {
			logrus.Errorf("error get kubernetes node ")
		}
		uid := string(kn.UID)
		cnode.UUID = uid
		err = k8s.AddSource(conf.Config.K8SNode+uid, cnode)
		if err != nil {
			logrus.Errorf("add source to db failed,details %s", err.Error())
			cnode.Status = "failed"
			return err
		}
		k8s.DeleteSource(conf.Config.K8SNode + node)
	} else {
		err = k8s.AddSource(conf.Config.K8SNode+node, cnode)
		if err != nil {
			logrus.Errorf("add source to db failed,details %s", err.Error())
			cnode.Status = "failed"
			return err
		}
	}

	return nil
	//cnode.UUID = string(realK8SNode.UID)

	//更改状态

}

func updateBuildInJobCMD(job *Job, newCmd string) {
	logrus.Infof("update build in job to %s", newCmd)
	job.Command = newCmd
	jobMd, _ := json.Marshal(job)
	_, err := store.DefalutClient.Put(conf.Config.BuildIn+job.ID, string(jobMd))
	if err != nil {
		return
	}
}
func removeCMDFromJob(job *Job, rawCmd string) {
	job.Command = rawCmd
	jobMd, _ := json.Marshal(job)
	store.DefalutClient.Put(conf.Config.BuildIn+job.ID, string(jobMd))
}
func getOrderdArgsByJobName(node string, job *Job) (string, string, error) {
	//todo
	rawCmd := job.Command
	res, err := store.DefalutClient.Get(conf.Config.ConfigPath+SYSTEM+"/"+node, clientv3.WithPrefix())
	logrus.Infof("geting system info from etcd by key %s", conf.Config.ConfigPath+SYSTEM+"/"+node)
	if err != nil {
		logrus.Errorf("get node %s's system failed,details,%s", node, err.Error())
		return "", "", err
	}
	if res.Count <= 0 {
		logrus.Infof("get nothing from etcd")
		return "", "", errors.New("get system info failed")
	}
	system := string(res.Kvs[0].Value)
	if strings.Contains(job.ID, "online") {
		resp, err := http.Get(job.Command)
		if err != nil {
			logrus.Infof("download shell failed,details %s", err.Error())
			return "", "", err
		}
		defer resp.Body.Close()
		b, _ := ioutil.ReadAll(resp.Body)
		if job.Name == JOB_NFS {
			nfsHost, err := GetNFSHost()
			if err != nil {
				logrus.Errorf("err get nfs host args,details %s", err.Error())
				return "", "", err
			}
			//nfsDest,err:=GetNFSDest()
			//if err!=nil {
			//	logrus.Errorf("err get nfs dest args,details %s",err.Error())
			//	return "","",err
			//}
			nfsEndPoint, err := GetNFSEndPoint()
			if err != nil {
				logrus.Errorf("err get nfs endpoint args,details %s", err.Error())
				return "", "", err
			}
			args := []string{system, GetStorageModeOrDefault(), nfsHost, nfsEndPoint}

			argStr := appendArgs(args, b)
			return rawCmd, argStr, nil
		} else if job.Name == JOB_NETWORK {

			//CASSANDRA_IP=$4 #cassandra service eg: 10.0.1.14 所有cassandra,逗号分隔
			zkHosts, err := GetZKHosts()
			if err != nil {
				logrus.Errorf("err get zk host args,details %s", err.Error())
				return "", "", err
			}
			cassandraIp, err := GetCASSANDRAIP()
			if err != nil {
				logrus.Errorf("err get cassandra ip args,details %s", err.Error())
				return "", "", err
			}
			etcdIp, err := GetETCDIp()
			if err != nil {
				logrus.Errorf("err get etcd ip args,details %s", err.Error())
				return "", "", err
			}
			mode := GetNetWorkMode()
			args := []string{}
			args = []string{system, node, mode, zkHosts, cassandraIp, etcdIp}

			argStr := appendArgs(args, b)
			return rawCmd, argStr, nil
		} else if job.Name == JOB_DOCKER {
			args := []string{system}
			argStr := appendArgs(args, b)
			//logrus.Infof("add args to command,before is %s,after is %s",rawCmd,argStr)
			return rawCmd, argStr, nil
		} else if job.Name == JOB_SYNC_IMAGES {
			//INSTALL_TYPE=$1
			//ACP_VERSION=$2 # ACP版本
			args := []string{GetInstallTypeOrDefault(), getACPVersionOrDefault()}
			argStr := appendArgs(args, b)
			//logrus.Infof("add args to command,before is %s,after is %s",rawCmd,argStr)
			return rawCmd, argStr, nil
		} else if job.Name == JOB_TENGINE {
			//OS_TYPE=$1
			//HOST_IP=$2
			//DNS=$3
			ver := getACPVersionOrDefault()
			etcd, err := GetETCDIp()
			if err != nil {
				logrus.Errorf("err get dns ip args,details %s", err.Error())
				return "", "", err
			}
			k8sapi, err := GetK8SIp()
			if err != nil {
				logrus.Errorf("err get k8sapi args,details %s", err.Error())
				return "", "", err
			}

			masterip, err := GetMIp()
			if err != nil {
				logrus.Errorf("err get master ip args,details %s", err.Error())
				return "", "", err
			}

			args := []string{ver, etcd, k8sapi, masterip}

			argStr := appendArgs(args, b)
			//logrus.Infof("add args to command,before is %s,after is %s",rawCmd,argStr)
			return rawCmd, argStr, nil
		} else if job.Name == JOB_KUBELET {
			//OS_TYPE=$1
			//HOST_IP=$2
			//DNS=$3
			dns, err := GetDNS()
			if err != nil {
				logrus.Errorf("err get dns ip args,details %s", err.Error())
				return "", "", err
			}
			args := []string{system, node, dns}

			argStr := appendArgs(args, b)
			//logrus.Infof("add args to command,before is %s,after is %s",rawCmd,argStr)
			return rawCmd, argStr, nil
		} else if job.Name == JOB_PRISM {
			//ACP_VERSION=$1
			//ZMQ_SUB=$2
			//ZMQ_TO=$3 #dalaran_cep host:port
			zmqSub, err := GetZMQSub()
			if err != nil {
				logrus.Errorf("err get dns ip args,details %s", err.Error())
				return "", "", err
			}
			zmqTo, err := GetZMQTo()
			if err != nil {
				logrus.Errorf("err get dns ip args,details %s", err.Error())
				return "", "", err
			}
			args := []string{getACPVersionOrDefault(), zmqSub, zmqTo}
			argStr := appendArgs(args, b)
			//logrus.Infof("add args to command,before is %s,after is %s",rawCmd,argStr)
			return rawCmd, argStr, nil
		}
	} else {
		if job.Name == JOB_NFS {
			nfsHost, err := GetNFSHost()
			if err != nil {
				logrus.Errorf("err get nfs host args,details %s", err.Error())
				return "", "", err
			}
			nfsEndPoint, err := GetNFSEndPoint()
			if err != nil {
				logrus.Errorf("err get nfs endpoint args,details %s", err.Error())
				return "", "", err
			}

			args := []string{system, GetStorageModeOrDefault(), nfsHost, nfsEndPoint}

			argStr := stringArgs(args)
			argStr = "bash " + job.Command + argStr
			//logrus.Infof("add args to command,before is %s,after is %s",rawCmd,argStr)
			return rawCmd, argStr, nil
		} else if job.Name == JOB_NETWORK {
			//CASSANDRA_IP=$4 #cassandra service eg: 10.0.1.14 所有cassandra,逗号分隔
			zkHosts, err := GetZKHosts()
			if err != nil {
				logrus.Errorf("err get zk host args,details %s", err.Error())
				return "", "", err
			}
			cassandraIp, err := GetCASSANDRAIP()
			if err != nil {
				logrus.Errorf("err get cassandra ip args,details %s", err.Error())
				return "", "", err
			}

			args := []string{system, node, GetNetWorkMode(), zkHosts, cassandraIp}
			argStr := stringArgs(args)
			argStr = "bash " + job.Command + argStr
			//logrus.Infof("add args to command,before is %s,after is %s",rawCmd,argStr)
			return rawCmd, argStr, nil
		} else if job.Name == JOB_DOCKER {
			args := []string{system}
			argStr := stringArgs(args)
			argStr = "bash " + job.Command + argStr
			//logrus.Infof("add args to command,before is %s,after is %s",rawCmd,argStr)
			return rawCmd, argStr, nil
		} else if job.Name == JOB_SYNC_IMAGES {
			//INSTALL_TYPE=$1
			//ACP_VERSION=$2 # ACP版本

			args := []string{GetInstallTypeOrDefault(), getACPVersionOrDefault()}
			argStr := stringArgs(args)
			argStr = "bash " + job.Command + argStr
			//logrus.Infof("add args to command,before is %s,after is %s",rawCmd,argStr)
			return rawCmd, argStr, nil
		} else if job.Name == JOB_KUBELET {
			//OS_TYPE=$1
			//HOST_IP=$2
			//DNS=$3
			dns, err := GetDNS()
			if err != nil {
				logrus.Errorf("err get dns ip args,details %s", err.Error())
				return "", "", err
			}
			args := []string{system, node, dns}
			argStr := stringArgs(args)
			argStr = "bash " + job.Command + argStr
			//logrus.Infof("add args to command,before is %s,after is %s",rawCmd,argStr)
			return rawCmd, argStr, nil
		} else if job.Name == JOB_PRISM {
			//ACP_VERSION=$1
			//ZMQ_SUB=$2
			//ZMQ_TO=$3 #dalaran_cep host:port
			zmqSub, err := GetZMQSub()
			if err != nil {
				logrus.Errorf("err get dns ip args,details %s", err.Error())
				return "", "", err
			}
			zmqTo, err := GetZMQTo()
			if err != nil {
				logrus.Errorf("err get dns ip args,details %s", err.Error())
				return "", "", err
			}
			args := []string{getACPVersionOrDefault(), zmqSub, zmqTo}
			argStr := stringArgs(args)
			argStr = "bash " + job.Command + argStr
			//logrus.Infof("add args to command,before is %s,after is %s",rawCmd,argStr)
			return rawCmd, argStr, nil
		}
	}
	return "", "", nil

}
func appendArgs(args []string, body []byte) string {
	argStr := "set"
	for _, v := range args {
		argStr += "_*"
		argStr += v
	}
	argStr += ";"
	argStr += string(body)
	argStr = "bash -c " + argStr
	//bash -c set_*2_*3_*4;jkljlk
	return argStr
}
func stringArgs(args []string) string {
	argStr := " "
	for _, v := range args {
		argStr += v
		argStr += "_*"
	}
	argStr = argStr[0 : len(argStr)-2]
	return argStr
}
func updateBuildInJob(buildinJob *BuildInJob, status int) {
	buildinJob.JobResult = status
	//save to etcd state
}
func RegWorkerInstallJobs() error {
	//ids := []string{ JOB_NFS, JOB_DOCKER,JOB_NETWORK ,JOB_KUBELET, JOB_PRISM}
	ids := []string{JOB_NFS, JOB_DOCKER, JOB_NETWORK, JOB_TENGINE, JOB_SYNC_IMAGES, JOB_KUBELET, JOB_PRISM}
	//exist, err := checkBuildInJobsExists()
	//if err != nil {
	//	logrus.Warnf("error get resp from etcd with given key: %s", conf.Config.BuildIn)
	//	return err
	//}
	logrus.Infof("first master node,registing build-in job to etcd")
	//event.GetManager().GetLogger(uuid.NewV4().String()).Info("master registing build-in jobs ",nil)
	m := make(map[string]string)
	//map中key,value顺序随意，key要和ids相同
	//value为安装此模块要执行的命令

	m[JOB_NFS] = "http://repo.goodrain.com/node_actions/compute/nfs/set_mount.sh"
	m[JOB_NETWORK] = "http://repo.goodrain.com/node_actions/compute/network/set_network_node.sh"
	m[JOB_DOCKER] = "http://repo.goodrain.com/node_actions/compute/docker/install_docker.sh"
	m[JOB_TENGINE] = "http://repo.goodrain.com/node_actions/compute/tengine/compute_tengine.sh"
	m[JOB_SYNC_IMAGES] = "http://repo.goodrain.com/node_actions/compute/sync_images/sync_images.sh"
	m[JOB_KUBELET] = "http://repo.goodrain.com/node_actions/compute/kubelet/kubelet.sh"
	m[JOB_PRISM] = "http://repo.goodrain.com/node_actions/compute/prism/prism.sh"
	for _, v := range ids {
		c := m[v]
		j, err := makeJob("online_"+v, v, c)
		if err != nil {
			return err
		}
		v, _ := json.Marshal(j)
		logrus.Infof("making new job ,details %s", v)
	}
	m[JOB_NFS] = "/usr/local/acp-node/nfs/set_mount.sh"
	m[JOB_NETWORK] = "/usr/local/acp-node/network/set_network_node.sh"
	m[JOB_DOCKER] = "/usr/local/acp-node/compute/docker/install_docker.sh"
	m[JOB_TENGINE] = "/usr/local/acp-node/compute/tengine/compute_tengine.sh"
	m[JOB_SYNC_IMAGES] = "/usr/local/acp-node/compute/sync_images/sync_images.sh"
	m[JOB_KUBELET] = "/usr/local/acp-node/compute/kubelet/kubelet.sh"
	m[JOB_PRISM] = "/usr/local/acp-node/compute/prism/prism.sh"

	for _, v := range ids {
		c := m[v]
		_, err := makeJob("offline_"+v, v, c)
		if err != nil {
			return err
		}
	}
	return nil
}

func makeJob(id, name, command string) (*Job, error) {
	job1 := &Job{}
	job1.ID = id
	job1.Name = name
	job1.Group = "build_in"
	job1.Kind = 0
	job1.Pause = true
	job1.FailNotify = false
	job1.Parallels = 0
	job1.Timeout = 0
	job1.Interval = 0
	job1.Retry = 0
	job1.Command = command
	job1.To = []string{}
	rule := &JobRule{}
	rule.ID = ""
	rule.GroupIDs = []string{}
	rule.NodeIDs = []string{}
	rule.ExcludeNodeIDs = []string{}
	rule.Timer = "0 0/60 * * ?"
	rules := []*JobRule{rule}
	job1.Rules = rules
	err := job1.Check()
	if err != nil {
		logrus.Warnf("job check failed,details: %s", err.Error())
		return nil, err
	}
	//这里应该只放进去，而不被watch
	if err = putBuildInJob(id, job1); err != nil {
		return nil, err
	}
	return job1, nil
}

func putBuildInJob(jobId string, job *Job) error {
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	resp, err := store.DefalutClient.Get(conf.Config.BuildIn + jobId)
	if err != nil {
		logrus.Errorf("error get info from etcd by key %s", conf.Config.BuildIn+jobId)
	}
	if resp.Count > 0 {
		return nil
	}
	_, err = store.DefalutClient.Put(conf.Config.BuildIn+jobId, string(b))
	if err != nil {
		return err
	}
	return nil
}

//toInstall为需要安装的
func GetBuildinJobs() ([]*BuildInJob, error) {
	netStatus := ""
	resp, err := store.DefalutClient.Get(conf.Config.BuildIn, clientv3.WithPrefix())
	if err != nil {
		logrus.Warnf("get build-in jobs failed,details: %s", err.Error())
		return nil, err
	}
	coreJob := []*Job{}
	logrus.Infof("get build-in jobs count:%v", resp.Count)
	for _, v := range resp.Kvs {
		job := &Job{}
		err = json.Unmarshal(v.Value, job)
		if err != nil {
			return nil, err
		}
		if strings.Contains(job.Name, netStatus) {
			coreJob = append(coreJob, job)
		}
	}
	buildinJobs := []*BuildInJob{}
	for _, v := range coreJob {
		job := &BuildInJob{
			JobName:   v.Name,
			JobId:     v.ID,
			JobResult: 0,
			JobSEQ:    "", //todo
			CnName:    getCnName(v.Name),
			Describe:  getDescribe(v.Name),
		}

		buildinJobs = append(buildinJobs, job)
	}
	d, _ := json.Marshal(buildinJobs)
	logrus.Infof("get all build-in jobs details :%s", string(d))
	return buildinJobs, nil
}
func getCnName(name string) string {

	switch name {
	case JOB_NFS:
		return "存储"
	case JOB_DOCKER:
		return "Docker"
	case JOB_NETWORK:
		return "网络"
	case JOB_TENGINE:
		return "代理"
	case JOB_SYNC_IMAGES:
		return "镜像"
	case JOB_KUBELET:
		return "Kubernetes-Kubelet"
	case JOB_PRISM:
		return "监控"
	}
	return "组件"
}
func getDescribe(name string) string {
	switch name {
	case JOB_NFS:
		return "正在安装存储"
	case JOB_DOCKER:
		return "正在安装Docker"
	case JOB_NETWORK:
		return "正在安装网络"
	case JOB_TENGINE:
		return "正在安装代理"
	case JOB_SYNC_IMAGES:
		return "正在同步镜像"
	case JOB_KUBELET:
		return "正在安装Kubernetes-Kubelet"
	case JOB_PRISM:
		return "正在安装监控"
	}
	return "正在处理组件"
}
func GetBuildInJobWithStatusForNode(toInstall []string, node string) (*JobList, error) {
	install := make(map[string]string)
	resp, err := store.DefalutClient.Get(conf.Config.CompJobStatus+node, clientv3.WithPrefix())
	if err != nil {
		logrus.Warnf("err getting resp from etcd with given key: %s", conf.Config.CompJobStatus+node)
		return nil, err
	}
	for _, v := range toInstall {
		install[v] = v
	}
	logrus.Infof("generating jobStatusList ,total job:%v", install)
	result := &JobList{}
	jobs := []*BuildInJob{}
	for _, v := range resp.Kvs {
		BIJob := &BuildInJob{}

		err = json.Unmarshal(v.Value, BIJob)
		if err != nil {
			logrus.Warnf("err unmarshal build in job ,details : %s", err.Error())
			return nil, err
		}

		_, ok := install[BIJob.JobName]
		logrus.Infof("get node's build-in job,now is %s ,is this job needed: %v", BIJob.JobId, ok)
		if ok {
			jobs = append(jobs, BIJob)
		}
	}
	result.List = jobs
	result.Result = false
	return result, nil
}
func GetJobStatusByNodeIP(ip string) (*JobList, error) {
	status := 0
	resp, err := store.DefalutClient.Get(conf.Config.CompJobStatus+ip, clientv3.WithPrefix())
	if err != nil {
		logrus.Warnf("err getting resp from etcd with given key: %s", conf.Config.CompJobStatus+ip)
		return nil, err
	}
	result := &JobList{}
	jobs := []*BuildInJob{}
	for _, v := range resp.Kvs {
		BIJob := &BuildInJob{}
		err = json.Unmarshal(v.Value, BIJob)
		if err != nil {
			logrus.Warnf("err unmarshal build in job ,details : %s", err.Error())
			return nil, err
		}
		jobs = append(jobs, BIJob)
	}
	ids := []string{JOB_NFS, JOB_DOCKER, JOB_NETWORK, JOB_TENGINE, JOB_SYNC_IMAGES, JOB_KUBELET, JOB_PRISM}
	r := []*BuildInJob{}
	for _, v := range ids {
		for _, v2 := range jobs {
			if v == v2.JobName {

				r = append(r, v2)
			}
		}
	}
	result.List = r
	b := false
	for _, v := range result.List {
		if v.JobResult == 1 {
			b = true
		} else {
			b = false
			break
		}
	}

	if len(result.List) == 0 {
		b = true
		status = 4
	} else {
		if b {
			status = 1
		}
	}
	for _, v := range result.List {
		if v.JobResult == 3 {
			status = 3
			break
		}
		if v.JobResult == 2 {
			status = 2
			break
		}
	}

	result.Result = b
	result.Status = status
	return result, nil
}

//为build-in job添加 nid(新添加的计算节点，使node可以在上面执行）
func AddNewNodeToJobs(jobs []*BuildInJob, node string) error {
	for _, v := range jobs {
		coreJobId := v.JobId
		err := changeBIJobNID(coreJobId, node)
		if err != nil {
			return err
		}
	}
	return nil
}
func RemoveRepByMap(slc []string) []string {
	result := []string{}
	tempMap := map[string]byte{} // 存放不重复主键
	for _, e := range slc {
		l := len(tempMap)
		tempMap[e] = 0
		if len(tempMap) != l { // 加入map后，map长度变化，则元素不重复
			result = append(result, e)
		}
	}
	return result
}

//为job添加新的nid
//因为结构已经变了，并不是buildin/jobid了
func changeBIJobNID(jobId, node string) error {
	resp, err := store.DefalutClient.Get(conf.Config.BuildIn+jobId, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	job := &Job{}
	err = json.Unmarshal(resp.Kvs[0].Value, job)
	if err != nil {
		return err
	}
	for _, rv := range job.Rules {
		rv.NodeIDs = append(rv.NodeIDs, node)
		rv.NodeIDs = RemoveRepByMap(rv.NodeIDs)
	}
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}
	store.DefalutClient.Put(conf.Config.BuildIn+jobId, string(body))
	return nil
}

func CheckJob(node string) (bool, string, error) {
	//在这里检测 哪些组件已经安装了

	info, err := RunCheckInstallJob(node, "check")
	if err != nil {
		logrus.Debugf("check installed component failed,details :%s", err.Error())
		return false, "", err
	}
	logrus.Infof("prepare info is %s", info)
	//logrus.Infof("installing node %s ,need to install component :%s",node,unInstalled)
	infos := strings.Split(info, ":")
	if infos == nil {
		logrus.Infof("get check job result failed,no formated output!")
		return false, "", err
	}
	var online bool
	if infos[0] == "online" {
		online = true
	} else {
		online = false
	}

	logrus.Infof("is node % online? %t", node, online)
	return online, infos[1], nil
}

//每安装一个节点，都需要将其 build in jobs 的执行状态保留下来,返回给前端初始状态。等待前端执行信号
//func NewComputeNodeToInstall(node string) (*JobList, error) {
//
//	jobs, err := GetBuildinJobs( "online") //状态为未安装
//	if err != nil {
//		return nil, err
//	}
//	d, _ := json.Marshal(jobs)
//	logrus.Infof("prepare to install node %s's jobs,details: %s", node, d)
//	err = AddNewNodeToJobs(jobs, node)
//	if err != nil {
//		return nil, err
//	}
//
//	//todo add args to jobs
//	//check args
//	err = UpdateNodeJobStatus(node, jobs)
//	if err != nil {
//		return nil, err
//	}
//
//	j, err := GetBuildInJobWithStatusForNode(node)
//	if err != nil {
//		return nil, err
//	}
//	return j, nil
//}
func RunCheckInstallJob(node, jobId string) (string, error) {

	ch := make(chan map[string]string)
	go watchBuildInJobLog(ch)

	//todo 这里是执行
	if err := PutBuildIn(jobId, node); err != nil {
		logrus.Errorf("can't put job to a watched etcd path,details %s", err.Error())
	}
	//此处获得job的output
	result := <-ch
	logrus.Infof("job output result is,%s", result)
	return result["output"], nil
}

func UpdateNodeJobStatus(node string, jobs []*BuildInJob) error {
	for _, v := range jobs {
		//此时 所有build-in job 已经更新了nid,可以在新的node上执行了
		jobB, err := json.Marshal(v)
		if err != nil {
			return err
		}
		//为新的计算节点注册 build-in job状态表   /&*^%/128.3.4.5/id1/job1
		_, err = store.DefalutClient.Put(conf.Config.CompJobStatus+node+"/"+v.JobId, string(jobB))
		logrus.Infof("update job to %s", string(jobB))
		if err != nil {
			return err
		}
	}
	return nil
}

type JobList struct {
	List   []*BuildInJob
	SEQ    string
	Result bool
	Status int
}
type BuildInJob struct {
	JobSEQ    string //执行顺序
	JobId     string //key 用于查找
	JobName   string //可读名
	CnName    string
	Describe  string
	JobResult int //最终结果  0 1 ing,2 succ,3 failed
}
