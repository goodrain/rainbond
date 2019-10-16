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
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/goodrain/rainbond/gateway/controller/openresty/nginxcmd"

	"github.com/Sirupsen/logrus"
	"github.com/golang/glog"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/goodrain/rainbond/gateway/controller/openresty/template"
	v1 "github.com/goodrain/rainbond/gateway/v1"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/cert"
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
	if err := o.configManage.NewNginxTemplate(nginx); err != nil {
		logrus.Errorf("init openresty config failure %s", err.Error())
		return err
	}
	if o.ocfg.EnableRbdEndpoints {
		if err := o.newRbdServers(); err != nil {
			showErr := fmt.Errorf("create rainbond default server config failure %s", err.Error())
			logrus.Error(showErr.Error())
			return showErr
		}
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

func (o *OrService) persistServerAndPool(servers []*model.Server, pools []*v1.Pool) error {
	if len(servers) == 0 {
		o.configManage.WriteServer(*o.ocfg, "stream", "", nil)
	}
	first := true
	for _, server := range servers {
		proxyPass := server.ProxyPass
		found := false
		for _, pool := range pools {
			logrus.Debugf("upstream.Name : %s", pool.Name)
			if proxyPass == pool.Name {
				found = true
				upstream := &model.Upstream{}
				upstream.Name = pool.Name
				upstream.UseLeastConn = pool.LeastConn
				var upstreamServers []model.UServer
				for _, node := range pool.Nodes {
					upstreamServer := model.UServer{
						Address: node.Host + ":" + fmt.Sprintf("%v", node.Port),
						Params: model.Params{
							Weight:      1,
							MaxFails:    node.MaxFails,
							FailTimeout: node.FailTimeout,
						},
					}
					logrus.Debugf("upstream.server address = %s", upstreamServer.Address)
					upstreamServers = append(upstreamServers, upstreamServer)
				}
				upstream.Servers = upstreamServers
				logrus.Debugf("first: %v, server: %v, upstream: %v", first, server, pool)
				if err := o.configManage.WriteServerAndUpstream(first, *o.ocfg, "stream", pool.Namespace, server, upstream); err != nil {
					logrus.Errorf("write server or upstream error: %v", err.Error())
				} else {
					first = false
				}
			}
		}
		if !found {
			logrus.Warnf("server %v do not found upstream by name %s, ignore it", server.ServerName, proxyPass)
		}
	}
	return nil
}

// PersistConfig persists ocfg
func (o *OrService) PersistConfig(conf *v1.Config) error {
	l7srv, l4srv := getNgxServer(conf)
	// http
	o.configManage.WriteServer(*o.ocfg, "http", "", l7srv...)
	// stream
	o.persistServerAndPool(l4srv, conf.TCPPools)

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
	var upstreams = make(map[string][]*model.Upstream)
	for _, pool := range pools {
		upstream := &model.Upstream{}
		upstream.Name = pool.Name
		upstream.UseLeastConn = pool.LeastConn
		var servers []model.UServer
		for _, node := range pool.Nodes {
			server := model.UServer{
				Address: node.Host + ":" + fmt.Sprintf("%v", node.Port),
				Params: model.Params{
					Weight:      1,
					MaxFails:    node.MaxFails,
					FailTimeout: node.FailTimeout,
				},
			}
			servers = append(servers, server)
		}
		upstream.Servers = servers
		upstreams[pool.Namespace] = append(upstreams[pool.Namespace], upstream)
	}
	for tenant, tupstreams := range upstreams {
		if err := o.configManage.WriteUpstream(*o.ocfg, tenant, tupstreams...); err != nil {
			logrus.Errorf("Fail to new nginx Upstream ocfg file: %v", err)
			return err
		}
	}
	return nil
}

func getNgxServer(conf *v1.Config) (l7srv []*model.Server, l4srv []*model.Server) {
	for _, vs := range conf.L7VS {
		server := &model.Server{
			Listen:     strings.Join(vs.Listening, " "),
			ServerName: strings.Replace(vs.ServerName, "tls", "", 1),
			// ForceSSLRedirect: vs.ForceSSLRedirect,
			OptionValue: map[string]string{
				"tenant_id":  vs.Namespace,
				"service_id": vs.ServiceID,
			},
		}
		if vs.SSLCert != nil {
			server.SSLCertificate = vs.SSLCert.CertificatePem
			server.SSLCertificateKey = vs.SSLCert.CertificatePem
		}
		for _, loc := range vs.Locations {
			location := &model.Location{
				DisableAccessLog: true,
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
			ProxyPass: vs.PoolName,
			OptionValue: map[string]string{
				"tenant_id":  vs.Namespace,
				"service_id": vs.ServiceID,
			},
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
	if len(tpools) > 0 {
		err := o.persistUpstreams(tpools)
		if err != nil {
			logrus.Warningf("error updating upstream.default.tcp.conf")
		}
		// reload nginx
		if err := nginxcmd.Reload(); err != nil {
			return fmt.Errorf("reload nginx config for update tcp upstream failure %s", err.Error())
		}
		logrus.Debug("Nginx reloads successfully for tcp pool.")
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
	logrus.Debug("dynamically update Upstream success")
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

// newRbdServers creates new configuration file for Rainbond servers
func (o *OrService) newRbdServers() error {
	o.configManage.ClearByTenant("rainbond")
	tenantConfigDir := path.Join(o.configManage.GetConfigFileDirPath(), "rainbond")
	// create cert
	err := createGoodRaindCert(tenantConfigDir, "goodrain.me")
	if err != nil {
		return err
	}
	if o.ocfg.EnableKApiServer {
		dummyUpstream := &model.Upstream{
			Name: "kube_apiserver",
			Servers: []model.UServer{
				{
					Address: "0.0.0.1:65535", // placeholder
					Params: model.Params{
						Weight: 1,
					},
				},
			},
		}
		if err := o.configManage.WriteUpstream(*o.ocfg, "rainbond", dummyUpstream); err != nil {
			logrus.Errorf("write kube api config upstream failure %s", err.Error())
			return err
		}
		ksrv := kubeApiserver(o.ocfg.KApiServerIP)
		if err := o.configManage.WriteServer(*o.ocfg, "stream", "rainbond", ksrv); err != nil {
			logrus.Errorf("write kube api config server failure %s", err.Error())
			return err
		}
	}
	var srv []*model.Server
	if o.ocfg.EnableLangGrMe {
		lesrv := langGoodrainMe(o.ocfg.LangGrMeIP)
		srv = append(srv, lesrv)
	}
	if o.ocfg.EnableMVNGrMe {
		mesrv := mavenGoodrainMe(o.ocfg.MVNGrMeIP)
		srv = append(srv, mesrv)
	}
	if o.ocfg.EnableGrMe {
		gesrv := goodrainMe(tenantConfigDir, o.ocfg.GrMeIP)
		srv = append(srv, gesrv)
	}
	if o.ocfg.EnableRepoGrMe {
		resrv := repoGoodrainMe(o.ocfg.RepoGrMeIP)
		srv = append(srv, resrv)
	}
	if err := o.configManage.WriteServer(*o.ocfg, "http", "rainbond", srv...); err != nil {
		logrus.Errorf("write kube api config server failure %s", err.Error())
		return err
	}
	return nil
}

func createGoodRaindCert(cfgPath string, cn string) error {
	p := path.Join(cfgPath, "ssl")
	crtexists, crterr := util.FileExists(fmt.Sprintf("%s/%s", p, "server.crt"))
	keyexists, keyerr := util.FileExists(fmt.Sprintf("%s/%s", p, "server.key"))
	if (crtexists && crterr == nil) && (keyexists && keyerr == nil) {
		logrus.Info("certificate for goodrain.me exists.")
		return nil
	}
	exists, err := util.FileExists(p)
	if !exists || err != nil {
		if e := os.MkdirAll(p, 0777); e != nil {
			return e
		}
	}
	baseinfo := cert.CertInformation{Country: []string{"CN"}, Organization: []string{"Goodrain"}, IsCA: true,
		OrganizationalUnit: []string{"Rainbond"}, EmailAddress: []string{"zengqg@goodrain.com"},
		Locality: []string{"BeiJing"}, Province: []string{"BeiJing"}, CommonName: cn,
		Domains: []string{"goodrain.me"},
		CrtName: fmt.Sprintf("%s/%s", cfgPath, "ssl/ca.pem"),
		KeyName: fmt.Sprintf("%s/%s", cfgPath, "ssl/ca.key")}

	if err := cert.CreateCRT(nil, nil, baseinfo); err != nil {
		logrus.Errorf("Create crt error: %s ", err.Error())
		return err
	}
	crtInfo := baseinfo
	crtInfo.IsCA = false
	crtInfo.CrtName = fmt.Sprintf("%s/%s", cfgPath, "ssl/server.crt")
	crtInfo.KeyName = fmt.Sprintf("%s/%s", cfgPath, "ssl/server.key")
	crtInfo.Names = []pkix.AttributeTypeAndValue{
		pkix.AttributeTypeAndValue{
			Type:  asn1.ObjectIdentifier{2, 1, 3},
			Value: "MAC_ADDR",
		},
	}

	crt, pri, err := cert.Parse(baseinfo.CrtName, baseinfo.KeyName)
	if err != nil {
		logrus.Errorf("Parse crt error,Error info: %s", err.Error())
		return err
	}
	err = cert.CreateCRT(crt, pri, crtInfo)
	if err != nil {
		logrus.Errorf("Create crt error,Error info: %s", err.Error())
		return err
	}
	logrus.Info("Create certificate for goodrain.me successfully")
	return nil
}
