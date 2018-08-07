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
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/goodrain/rainbond/api/handler"

	"github.com/goodrain/rainbond/util"

	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/api/option"

	"github.com/goodrain/rainbond/api/apiRouters/doc"
	"github.com/goodrain/rainbond/api/apiRouters/license"
	"github.com/goodrain/rainbond/api/proxy"

	"github.com/goodrain/rainbond/api/apiRouters/cloud"
	"github.com/goodrain/rainbond/api/apiRouters/version2"
	"github.com/goodrain/rainbond/api/apiRouters/websocket"

	apimiddleware "github.com/goodrain/rainbond/api/middleware"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

//Manager apiserver
type Manager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	conf            option.Config
	stopChan        chan struct{}
	r               *chi.Mux
	prometheusProxy proxy.Proxy
}

//NewManager newManager
func NewManager(c option.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	//controller.CreateV2RouterManager(c)
	r := chi.NewRouter()
	r.Use(middleware.RequestID) //每个请求的上下文中注册一个id
	//Sets a http.Request's RemoteAddr to either X-Forwarded-For or X-Real-IP
	r.Use(middleware.RealIP)
	//Logs the start and end of each request with the elapsed processing time
	if c.LoggerFile != "" {
		logerFile, err := os.OpenFile(c.LoggerFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			logrus.Errorf("open logger file %s error %s", c.LoggerFile, err.Error())
			r.Use(middleware.DefaultLogger)
		} else {
			requestLog := middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: log.New(logerFile, "", log.LstdFlags)})
			r.Use(requestLog)
		}
	} else {
		r.Use(middleware.DefaultLogger)
	}
	//Gracefully absorb panics and prints the stack trace
	r.Use(middleware.Recoverer)
	//request time out
	r.Use(middleware.Timeout(time.Second * 5))
	//simple authz
	if os.Getenv("TOKEN") != "" {
		r.Use(apimiddleware.FullToken)
	}
	//simple api version
	r.Use(apimiddleware.APIVersion)
	r.Use(apimiddleware.Proxy)

	if c.Debug {
		util.ProfilerSetup(r)
	}

	r.Get("/monitor", func(res http.ResponseWriter, req *http.Request) {
		res.Write([]byte("ok"))
	})

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

	//prometheus单节点代理
	m.r.Get("/api/v1/query", m.PrometheusAPI)
	m.r.Get("/api/v1/query_range", m.PrometheusAPI)
	//开启对浏览器的websocket服务和文件服务
	go func() {
		websocketRouter := chi.NewRouter()
		websocketRouter.Mount("/", websocket.Routes())
		websocketRouter.Mount("/logs", websocket.LogRoutes())
		websocketRouter.Mount("/app", websocket.AppRoutes())
		if m.conf.WebsocketSSL {
			logrus.Infof("websocket listen on (HTTPs) 0.0.0.0%v", m.conf.WebsocketAddr)
			logrus.Fatal(http.ListenAndServeTLS(m.conf.WebsocketAddr, m.conf.WebsocketCertFile, m.conf.WebsocketKeyFile, websocketRouter))
		} else {
			logrus.Infof("websocket listen on (HTTP) 0.0.0.0%v", m.conf.WebsocketAddr)
			logrus.Fatal(http.ListenAndServe(m.conf.WebsocketAddr, websocketRouter))
		}
	}()
	if m.conf.APISSL {
		go func() {
			pool := x509.NewCertPool()
			caCrt, err := ioutil.ReadFile(m.conf.APICaFile)
			if err != nil {
				logrus.Fatal("ReadFile ca err:", err)
				return
			}
			pool.AppendCertsFromPEM(caCrt)
			s := &http.Server{
				Addr:    m.conf.APIAddrSSL,
				Handler: m.r,
				TLSConfig: &tls.Config{
					ClientCAs:  pool,
					ClientAuth: tls.RequireAndVerifyClientCert,
				},
			}
			logrus.Infof("api listen on (HTTPs) 0.0.0.0%v", m.conf.APIAddrSSL)
			logrus.Fatal(s.ListenAndServeTLS(m.conf.APICertFile, m.conf.APIKeyFile))
		}()
	}
	logrus.Infof("api listen on (HTTP) 0.0.0.0%v", m.conf.APIAddr)
	logrus.Fatal(http.ListenAndServe(m.conf.APIAddr, m.r))
}

//EventLogInstance 查询event server instance
func (m *Manager) EventLogInstance(w http.ResponseWriter, r *http.Request) {
	etcdclient, err := clientv3.New(clientv3.Config{
		Endpoints: m.conf.EtcdEndpoint,
	})
	if err != nil {
		w.WriteHeader(500)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := etcdclient.Get(ctx, "/event/instance", clientv3.WithPrefix())
	if err != nil {
		w.WriteHeader(500)
		return
	}
	if res.Kvs != nil && len(res.Kvs) > 0 {
		result := `{"data":{"instance":[`
		for _, kv := range res.Kvs {
			result += string(kv.Value) + ","
		}
		result = result[:len(result)-1] + `]},"ok":true}`
		w.Write([]byte(result))
		w.WriteHeader(200)
		return
	}
	w.WriteHeader(404)
	return
}

//PrometheusAPI prometheus api 代理
func (m *Manager) PrometheusAPI(w http.ResponseWriter, r *http.Request) {
	handler.GetPrometheusProxy().Proxy(w, r)
}
