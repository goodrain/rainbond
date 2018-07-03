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

package api

import (
	"net/http"

	"github.com/goodrain/rainbond/cmd/entrance/option"
	"github.com/goodrain/rainbond/entrance/api/controller"
	"github.com/goodrain/rainbond/entrance/api/model"
	apistore "github.com/goodrain/rainbond/entrance/api/store"
	"github.com/goodrain/rainbond/entrance/core"
	"github.com/goodrain/rainbond/entrance/core/monitor"
	"github.com/goodrain/rainbond/entrance/store"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"golang.org/x/net/context"

	_ "net/http/pprof"

	"github.com/Sirupsen/logrus"
	restful "github.com/emicklei/go-restful"
	swagger "github.com/emicklei/go-restful-swagger12"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
)

type Manager struct {
	container       *restful.Container
	ctx             context.Context
	cancel          context.CancelFunc
	conf            option.Config
	server          *http.Server
	coreManager     core.Manager
	apiStoreManager *apistore.Manager
}

//NewManager api manager
func NewManager(c option.Config, coreManager core.Manager, readStore store.ReadStore) *Manager {
	wsContainer := restful.NewContainer()
	ctx, cancel := context.WithCancel(context.Background())
	server := &http.Server{Addr: c.APIAddr, Handler: wsContainer}
	apiStore, err := apistore.NewManager(c)
	if err != nil {
		logrus.Error("create api store manager error.", err.Error())
	}
	//register modle type
	apiStore.Register("domain", &model.Domain{})
	apiStore.Register("host_node", &model.HostNode{})
	//create k8s api client
	kubeconfig := c.K8SConfPath
	conf, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logrus.Error(err)
	}
	conf.QPS = 50
	conf.Burst = 100
	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		logrus.Error(err)
	}
	controller.Register(wsContainer, coreManager, readStore, apiStore, clientset)
	return &Manager{
		container:       wsContainer,
		ctx:             ctx,
		cancel:          cancel,
		conf:            c,
		server:          server,
		coreManager:     coreManager,
		apiStoreManager: apiStore,
	}
}

//Start 启动
func (m *Manager) Start(errChan chan error) {
	logrus.Infof("api server start listening on %s", m.conf.APIAddr)
	m.doc()
	m.prometheus()
	go func() {
		if err := m.server.ListenAndServe(); err != nil {
			logrus.Error("entrance api listen error.", err.Error())
			errChan <- err
		}
	}()

	if m.conf.Debug {
		go func() {
			if err := http.ListenAndServe(":6101", nil); err != nil {
				logrus.Error("entrance api listen error.", err.Error())
				errChan <- err
			}
		}()
	}
}

func (m *Manager) doc() {
	// Optionally, you can install the Swagger Service which provides a nice Web UI on your REST API
	// You need to download the Swagger HTML5 assets and change the FilePath location in the config below.
	// Open http://localhost:8080/apidocs and enter http://localhost:8080/swagger.json in the api input field.
	config := swagger.Config{
		WebServices: m.container.RegisteredWebServices(), // you control what services are visible
		ApiPath:     "/swagger.json",

		// Optionally, specify where the UI is located
		SwaggerPath: "/apidocs/",
		Info: swagger.Info{
			Title: "goodrain entrance api doc.",
		},
		ApiVersion:      "1.0",
		SwaggerFilePath: "./api/dist"}
	swagger.RegisterSwaggerService(config, m.container)

}

//Stop 停止
func (m *Manager) Stop() error {
	logrus.Info("api server is stoping.")
	m.cancel()
	return nil
}

func (m *Manager) prometheus() {
	prometheus.MustRegister(version.NewCollector("acp_entrance"))
	exporter := monitor.NewExporter(m.coreManager)
	prometheus.MustRegister(exporter)
	m.container.Handle(m.conf.PrometheusMetricPath, promhttp.Handler())
}
