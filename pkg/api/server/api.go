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

package server

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/goodrain/rainbond/cmd/api/option"

	"github.com/goodrain/rainbond/pkg/api/apiRouters/doc"
	"github.com/goodrain/rainbond/pkg/api/apiRouters/license"

	"github.com/goodrain/rainbond/pkg/api/apiRouters/cloud"
	"github.com/goodrain/rainbond/pkg/api/apiRouters/version2"
	"github.com/goodrain/rainbond/pkg/api/apiRouters/websocket"

	apimiddleware "github.com/goodrain/rainbond/pkg/api/middleware"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

//Manager apiserver
type Manager struct {
	ctx      context.Context
	cancel   context.CancelFunc
	conf     option.Config
	stopChan chan struct{}
	r        *chi.Mux
}

//NewManager newManager
func NewManager(c option.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	//初始化 middleware
	apimiddleware.InitProxy(c)
	//controller.CreateV2RouterManager(c)
	r := chi.NewRouter()
	r.Use(middleware.RequestID) //每个请求的上下文中注册一个id
	//Sets a http.Request's RemoteAddr to either X-Forwarded-For or X-Real-IP
	r.Use(middleware.RealIP)
	//Logs the start and end of each request with the elapsed processing time
	r.Use(middleware.Logger)
	//Gracefully absorb panics and prints the stack trace
	r.Use(middleware.Recoverer)
	//request time out
	r.Use(middleware.Timeout(time.Second * 5))
	//simple authz
	if os.Getenv("TOKEN") != "" {
		r.Use(apimiddleware.Token)
	}
	//simple api version
	r.Use(apimiddleware.APIVersion)
	r.Use(apimiddleware.Proxy)
	return &Manager{
		ctx:      ctx,
		cancel:   cancel,
		conf:     c,
		stopChan: make(chan struct{}),
		r:        r,
	}
}

//Start manager
func (m *Manager) Start() error {
	go m.Do()
	logrus.Info("start api router success.")
	return nil
}

//Do do
func (m *Manager) Do() {
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
			m.Run()
		}
	}
}

//Stop manager
func (m *Manager) Stop() error {
	logrus.Info("api router is stopped.")
	m.cancel()
	return nil
}

//Run run
func (m *Manager) Run() {

	v2R := &version2.V2{}
	m.r.Mount("/v2", v2R.Routes())
	m.r.Mount("/cloud", cloud.Routes())
	m.r.Mount("/", doc.Routes())
	m.r.Mount("/license", license.Routes())
	//兼容老版docker
	m.r.Get("/v1/etcd/event-log/instances", m.EventLogInstance)
	//开启对浏览器的websocket服务和文件服务
	go func() {
		websocketRouter := chi.NewRouter()
		websocketRouter.Mount("/", websocket.Routes())
		websocketRouter.Mount("/logs", websocket.LogRoutes())
		if m.conf.WebsocketSSL {
			logrus.Infof("websocket listen on (HTTPs) 0.0.0.0%v", m.conf.WebsocketAddr)
			logrus.Fatal(http.ListenAndServeTLS(m.conf.WebsocketAddr, m.conf.WebsocketCertFile, m.conf.WebsocketKeyFile, websocketRouter))
		} else {
			logrus.Infof("websocket listen on (HTTP) 0.0.0.0%v", m.conf.WebsocketAddr)
			logrus.Fatal(http.ListenAndServe(m.conf.WebsocketAddr, websocketRouter))
		}
	}()
	if m.conf.APISSL {
		logrus.Infof("api listen on (HTTPs) 0.0.0.0%v", m.conf.APIAddr)
		logrus.Fatal(http.ListenAndServeTLS(m.conf.APIAddr, m.conf.APICertFile, m.conf.APIKeyFile, m.r))
	} else {
		logrus.Infof("api listen on (HTTP) 0.0.0.0%v", m.conf.APIAddr)
		logrus.Fatal(http.ListenAndServe(m.conf.APIAddr, m.r))
	}
}

//EventLogInstance 查询event server instance
func (m *Manager) EventLogInstance(w http.ResponseWriter, r *http.Request) {
	etcdclient, err := client.New(client.Config{
		Endpoints: m.conf.EtcdEndpoint,
	})
	if err != nil {
		w.WriteHeader(500)
		return
	}
	keyAPI := client.NewKeysAPI(etcdclient)
	res, err := keyAPI.Get(context.Background(), "/event/instance", &client.GetOptions{Recursive: true})
	if err != nil {
		w.WriteHeader(500)
		return
	}
	if res.Node != nil && res.Node.Nodes.Len() > 0 {
		result := `{"data":{"instance":[`
		for _, node := range res.Node.Nodes {
			result += node.Value + ","
		}
		result = result[:len(result)-1] + `]},"ok":true}`
		w.Write([]byte(result))
		w.WriteHeader(200)
		return
	}
	w.WriteHeader(404)
	return
}
