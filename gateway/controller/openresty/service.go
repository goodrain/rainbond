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

package openresty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/goodrain/rainbond/gateway/controller/openresty/nginxcmd"

	"github.com/golang/glog"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/goodrain/rainbond/gateway/controller/openresty/template"
	v1 "github.com/goodrain/rainbond/gateway/v1"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

// OrService handles the business logic of OpenrestyService
type OrService struct {
	IsShuttingDown *bool
	// stopLock is used to enforce that only a single call to Stop send at
	// a given time. We allow stopping through an HTTP endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock      *sync.Mutex
	ocfg          *option.Config
	nginxProgress *os.Process
	configManage  *template.NginxConfigFileTemplete
}

//CreateOpenrestyService create openresty service
func CreateOpenrestyService(config *option.Config, isShuttingDown *bool) *OrService {
	gws := &OrService{
		IsShuttingDown: isShuttingDown,
		ocfg:           config,
	}
	return gws
}

// Upstream defines a group of servers. Servers can listen on different ports
type Upstream struct {
	Name    string
	Servers []*Server
}

// Server belongs to Upstream
type Server struct {
	Host   string
	Port   int32
	Weight int
}

// Start starts nginx
func (o *OrService) Start(errCh chan error) error {
	logrus.Infof("openresty server starting")
	templete, err := template.NewNginxConfigFileTemplete()
	if err != nil {
		logrus.Errorf("create config template manage failure %s", err.Error())
		return err
	}
	o.configManage = templete
	defaultNginxConf := path.Join(o.configManage.GetConfigFileDirPath(), "nginx.conf")
	nginxcmd.SetDefaultNginxConf(defaultNginxConf)
	// delete the old configuration
	if !util.DirIsEmpty(o.configManage.GetConfigFileDirPath()) {
		dirs, _ := util.GetDirNameList(o.configManage.GetConfigFileDirPath(), 1)
		for _, dir := range dirs {
			path := fmt.Sprintf("%s/%s", o.configManage.GetConfigFileDirPath(), dir)
			err := os.RemoveAll(path)
			if err != nil {
				logrus.Warningf("error removing %s: %v", path, err)
			} else {
				logrus.Debugf("remove old dir %s", path)
			}
		}
		os.RemoveAll(defaultNginxConf)
	}
	// generate default nginx.conf
	nginx := model.NewNginx(*o.ocfg)
	nginx.HTTP = model.NewHTTP(o.ocfg)
	nginx.Stream = model.NewStream(o.ocfg)
	if err := o.configManage.NewNginxTemplate(nginx); err != nil {
		logrus.Errorf("init openresty config failure %s", err.Error())
		return err
	}
	logrus.Infof("init openresty config success")
	go func() {
		for {
			logrus.Infof("start openresty progress")
			cmd := nginxcmd.CreateNginxCommand()
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				logrus.Errorf("openresty start error: %v", err)
				errCh <- err
				return
			}
			o.nginxProgress = cmd.Process
			if err := cmd.Wait(); err != nil {
				errCh <- err
			}
		}
	}()
	return nil
}

// Stop gracefully stops the openresty master process.
func (o *OrService) Stop() error {
	// send stop signal to openresty
	logrus.Info("Stopping openresty process")
	if o.nginxProgress != nil {
		if err := o.nginxProgress.Signal(syscall.SIGTERM); err != nil {
			return err
		}
	}
	return nil
}

// PersistConfig persists ocfg
func (o *OrService) PersistConfig(conf *v1.Config) error {
	l7srv, l4srv := o.getNgxServer(conf)
	// http server
	o.configManage.WriteServer(*o.ocfg, "http", "", l7srv...)
	// tcp and udp server
	o.configManage.WriteServer(*o.ocfg, "stream", "", l4srv...)

	// reload nginx
	if err := nginxcmd.Reload(); err != nil {
		logrus.Errorf("Nginx reloads falure %s", err.Error())
		return err
	}
	logrus.Debug("Nginx reloads successfully.")
	return nil
}

// persistUpstreams persists upstreams
func (o *OrService) persistUpstreams(pools []*v1.Pool) error {
	streams := make([]model.Backend, 0)
	for _, pool := range pools {
		var endpoints []model.Endpoint
		for _, node := range pool.Nodes {
			endpoints = append(endpoints, model.Endpoint{
				Address: node.Host,
				Port:    strconv.Itoa(int(node.Port)),
			})
		}
		streams = append(streams, model.Backend{
			Name:      pool.Name,
			Endpoints: endpoints,
		})
	}

	buf, err := json.Marshal(streams)
	if err != nil {
		return err
	}

	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%v", o.ocfg.ListenPorts.Stream))
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write(buf)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(conn, "\r\n")
	if err != nil {
		return err
	}
	logrus.Debug("dynamically update tcp and udp Upstream success")
	return nil
}

func (o *OrService) getNgxServer(conf *v1.Config) (l7srv []*model.Server, l4srv []*model.Server) {
	for _, vs := range conf.L7VS {
		server := &model.Server{
			Listen:     strings.Join(vs.Listening, " "),
			Protocol:   "HTTP",
			ServerName: strings.Replace(vs.ServerName, "tls", "", 1),
			// ForceSSLRedirect: vs.ForceSSLRedirect,
			OptionValue: map[string]string{
				"tenant_id":  vs.Namespace,
				"service_id": vs.ServiceID,
			},
		}
		if vs.SSLCert != nil {
			server.SSLProtocols = vs.SSlProtocols
			server.SSLCertificate = vs.SSLCert.CertificatePem
			server.SSLCertificateKey = vs.SSLCert.CertificatePem
			server.EnableSSLStapling = o.ocfg.EnableSSLStapling

		}
		for _, loc := range vs.Locations {
			location := &model.Location{
				DisableAccessLog: o.ocfg.AccessLogPath == "",
				// TODO: Distinguish between server output logs
				AccessLogPath:    o.ocfg.AccessLogPath,
				EnableMetrics:    true,
				Path:             loc.Path,
				NameCondition:    loc.NameCondition,
				Proxy:            loc.Proxy,
				Rewrite:          loc.Rewrite,
				PathRewrite:      false,
				DisableProxyPass: loc.DisableProxyPass,
			}
			server.Locations = append(server.Locations, location)
		}
		l7srv = append(l7srv, server)
	}

	for _, vs := range conf.L4VS {
		server := &model.Server{
			Protocol: string(vs.Protocol),
			OptionValue: map[string]string{
				"tenant_id":  vs.Namespace,
				"service_id": vs.ServiceID,
			},
			UpstreamName: vs.PoolName,
		}
		server.Listen = strings.Join(vs.Listening, " ")
		l4srv = append(l4srv, server)
	}

	return l7srv, l4srv
}

// UpdatePools updates http upstreams dynamically.
func (o *OrService) UpdatePools(hpools []*v1.Pool, tpools []*v1.Pool) error {
	var lock sync.Mutex
	lock.Lock()
	defer lock.Unlock()
	logrus.Debugf("start update pools(tcp pools count %d, http pool count %d)", len(tpools), len(hpools))
	if len(tpools) > 0 {
		err := o.persistUpstreams(tpools)
		if err != nil {
			logrus.Warningf("error updating upstream.default.tcp.conf")
		}
	}
	if hpools == nil || len(hpools) == 0 {
		return nil
	}
	var backends []*model.Backend
	for _, pool := range hpools {
		backends = append(backends, model.CreateBackendByPool(pool))
	}
	return o.updateBackends(backends)
}

// updateUpstreams updates the upstreams in ngx.shared.dict by post
func (o *OrService) updateBackends(backends []*model.Backend) error {
	url := fmt.Sprintf("http://127.0.0.1:%v/config/backends", o.ocfg.ListenPorts.Status)
	if err := post(url, backends); err != nil {
		return err
	}
	logrus.Debug("dynamically update http Upstream success")
	return nil
}

func post(url string, data interface{}) error {
	buf, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			glog.Warningf("Error while closing response body:\n%v", err)
		}
	}()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected error code: %d", resp.StatusCode)
	}

	return nil
}

// WaitPluginReady waits for nginx to be ready.
func (o *OrService) WaitPluginReady() {
	url := fmt.Sprintf("http://127.0.0.1:%v/%s", o.ocfg.ListenPorts.Status, o.ocfg.HealthPath)
	for {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			logrus.Info("Nginx is ready")
			break
		}
		logrus.Infof("Nginx is not ready yet: %v", err)
		time.Sleep(1 * time.Second)
	}
}
