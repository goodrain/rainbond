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

package server

import (
	"fmt"
	"os"
	"syscall"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api/controller"
	"github.com/goodrain/rainbond/node/core/job"
	"github.com/goodrain/rainbond/node/core/k8s"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/node/masterserver"
	"github.com/goodrain/rainbond/node/monitormessage"
	"github.com/goodrain/rainbond/node/nodeserver"
	"github.com/goodrain/rainbond/node/statsd"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/pkg/api/v1"

	"github.com/Sirupsen/logrus"

	eventLog "github.com/goodrain/rainbond/event"

	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/node/api"
)

//Run start run
func Run(c *option.Conf) error {
	errChan := make(chan error, 3)
	err := eventLog.NewManager(eventLog.EventConfig{
		EventLogServers: c.EventLogServer,
		DiscoverAddress: c.Etcd.Endpoints,
	})
	if err != nil {
		logrus.Errorf("error creating eventlog manager")
		return nil
	}
	defer eventLog.CloseManager()

	// init etcd client
	if err = store.NewClient(c); err != nil {
		return fmt.Errorf("Connect to ETCD %s failed: %s",
			c.Etcd.Endpoints, err)
	}
	stop := make(chan struct{})
	if err := k8s.NewK8sClient(c); err != nil {
		return fmt.Errorf("Connect to K8S %s failed: %s",
			c.K8SConfPath, err)
	}
	sharedInformers := informers.NewSharedInformerFactory(k8s.K8S, c.MinResyncPeriod)
	sharedInformers.Core().V1().Services().Informer()
	sharedInformers.Core().V1().Endpoints().Informer()
	sharedInformers.Start(stop)
	defer close(stop)

	s, err := nodeserver.NewNodeServer(c) //todo 配置文件 done
	if err != nil {
		return err
	}
	if err := s.Run(errChan); err != nil {
		logrus.Errorf(err.Error())
		return err
	}
	defer s.Stop(nil)
	//master服务在node服务之后启动
	var ms *masterserver.MasterServer
	if c.RunMode == "master" {
		ms, err = masterserver.NewMasterServer(s.HostNode, k8s.K8S.Clientset)
		if err != nil {
			logrus.Errorf(err.Error())
			return err
		}
		if !s.HostNode.Role.HasRule("compute") {
			getInfoForMaster(s)
		}
		ms.Cluster.UpdateNode(s.HostNode)
		if err := ms.Start(errChan); err != nil {
			logrus.Errorf(err.Error())
			return err
		}
		defer ms.Stop(nil)
	}
	//statsd exporter
	registry := prometheus.NewRegistry()
	exporter := statsd.CreateExporter(c.StatsdConfig, registry)
	if err := exporter.Start(); err != nil {
		logrus.Errorf("start statsd exporter server error,%s", err.Error())
		return err
	}
	meserver := monitormessage.CreateUDPServer("0.0.0.0", 6666, c.Etcd.Endpoints)
	if err := meserver.Start(); err != nil {
		return err
	}
	//启动API服务
	apiManager := api.NewManager(*s.Conf, s.HostNode, ms, exporter, sharedInformers)
	if err := apiManager.Start(errChan); err != nil {
		return err
	}
	defer apiManager.Stop()

	defer job.Exit(nil)
	defer controller.Exist(nil)
	defer option.Exit(nil)
	//step finally: listen Signal
	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	select {
	case <-term:
		logrus.Warn("Received SIGTERM, exiting gracefully...")
	case err := <-errChan:
		logrus.Errorf("Received a error %s, exiting gracefully...", err.Error())
	}
	logrus.Info("See you next time!")
	return nil
}
func getInfoForMaster(s *nodeserver.NodeServer) {
	resp, err := http.Get("http://repo.goodrain.com/release/3.4.1/gaops/jobs/cron/check/manage/sys.sh")
	if err != nil {
		logrus.Errorf("error get sysinfo script,details %s", err.Error())
		return
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("error get response from sysinfo script,details %s", err.Error())
		return
	}
	cmd := exec.Command("bash", "-c", string(b))

	//cmd := exec.Command("bash", "/usr/share/gr-rainbond-node/gaops/jobs/install/manage/tasks/ex_domain.sh")
	outbuf := bytes.NewBuffer(nil)
	cmd.Stderr = outbuf
	err = cmd.Run()
	if err != nil {
		logrus.Infof("err run command ,details %s", err.Error())
		return
	}
	result := make(map[string]string)

	out := outbuf.Bytes()
	logrus.Infof("get system info is %s ", string(out))
	err = json.Unmarshal(out, &result)
	if err != nil {
		logrus.Infof("err unmarshal shell output ,details %s", err.Error())
		return
	}
	s.HostNode.NodeStatus = &v1.NodeStatus{
		NodeInfo: v1.NodeSystemInfo{
			KernelVersion:   result["KERNEL"],
			Architecture:    result["PLATFORM"],
			OperatingSystem: result["OS"],
			KubeletVersion:  "N/A",
		},
	}
	if cpuStr, ok := result["LOGIC_CORES"]; ok {
		if cpu, err := strconv.Atoi(cpuStr); err == nil {
			logrus.Infof("server cpu is %v", cpu)
			s.HostNode.AvailableCPU = int64(cpu)
			s.HostNode.NodeStatus.Allocatable.Cpu().Set(int64(cpu))
		}
	}

	if memStr, ok := result["MEMORY"]; ok {
		memStr = strings.Replace(memStr, " ", "", -1)
		memStr = strings.Replace(memStr, "G", "", -1)
		memStr = strings.Replace(memStr, "B", "", -1)
		if mem, err := strconv.ParseFloat(memStr, 64); err == nil {
			s.HostNode.AvailableMemory = int64(mem * 1024 * 1024 * 1024)
			s.HostNode.NodeStatus.Allocatable.Memory().SetScaled(int64(mem*1024*1024*1024), 0)
		} else {
			logrus.Warnf("get master memory info failed ,details %s", err.Error())
		}
	}
	logrus.Infof("memory is %v", s.HostNode.AvailableMemory)
	s.Update()

}
