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
	"github.com/goodrain/rainbond/util/cert"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/goodrain/rainbond/util"

	"github.com/Sirupsen/logrus"
	"github.com/golang/glog"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/goodrain/rainbond/gateway/controller/openresty/template"
	"github.com/goodrain/rainbond/gateway/v1"
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
func (o *OrService) Start(errCh chan error) {
	defaultNginxConf = path.Join(template.CustomConfigPath, "nginx.conf")
	// delete the old configuration
	if !util.DirIsEmpty(template.CustomConfigPath) {
		dirs, _ := util.GetDirNameList(template.CustomConfigPath, 1)
		for _, dir := range dirs {
			err := os.RemoveAll(fmt.Sprintf("%s/%s", template.CustomConfigPath, dir))
			if err != nil {
				logrus.Warningf("error removing %s: %v", dir, err)
			}
		}
		os.RemoveAll(defaultNginxConf)
	}
	// generate default nginx.conf
	nginx := model.NewNginx(*o.ocfg)
	nginx.HTTP = model.NewHTTP(o.ocfg)
	if err := template.NewNginxTemplate(nginx, defaultNginxConf); err != nil {
		errCh <- fmt.Errorf("Can't not new nginx ocfg: %s", err.Error())
		return
	}
	if o.ocfg.EnableRbdEndpoints {
		if err := o.newRbdServers(); err != nil {
			errCh <- err // TODO: consider if it is right
		}
	}
	cmd := nginxExecCommand()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		logrus.Errorf("NGINX start error: %v", err)
		errCh <- err
		return
	}
	o.nginxProgress = cmd.Process
	go func() {
		if err := cmd.Wait(); err != nil {
			errCh <- err
		}
		//errCh <- fmt.Errorf("nginx process is exit")
	}()
}

// Stop gracefully stops the NGINX master process.
func (o *OrService) Stop() error {
	// send stop signal to NGINX
	logrus.Info("Stopping NGINX process")
	if o.nginxProgress != nil {
		if err := o.nginxProgress.Signal(syscall.SIGTERM); err != nil {
			return err
		}
	}
	return nil
}

// PersistConfig persists ocfg
func (o *OrService) PersistConfig(conf *v1.Config) error {
	if err := o.persistUpstreams(conf.TCPPools, "upstreams-tcp.tmpl", template.CustomConfigPath, "stream/upstreams.conf"); err != nil {
		logrus.Errorf("fail to persist tcp upstreams.conf")
	}

	l7srv, l4srv := getNgxServer(conf)
	// http
	if len(l7srv) > 0 {
		filename := "http/servers.conf"
		if err := template.NewServerTemplate(l7srv, filename); err != nil {
			logrus.Errorf("Fail to new nginx Server ocfg file: %v", err)
			return err
		}
	}

	// stream
	if len(l4srv) > 0 {
		filename := "stream/servers.conf"
		if err := template.NewServerTemplate(l4srv, filename); err != nil {
			logrus.Errorf("Fail to new nginx Server file: %v", err)
			return err
		}
	}

	// check nginx configuration
	if out, err := nginxExecCommand("-t").CombinedOutput(); err != nil {
		return fmt.Errorf("%v\n%v", err, string(out))
	}
	logrus.Debug("Nginx configuration is ok.")

	// reload nginx
	if out, err := nginxExecCommand("-s", "reload").CombinedOutput(); err != nil {
		return fmt.Errorf("%v\n%v", err, string(out))
	}
	logrus.Debug("Nginx reloads successfully.")

	return nil
}

// persistUpstreams persists upstreams
func (o *OrService) persistUpstreams(pools []*v1.Pool, tmpl string, path string, filename string) error {
	var upstreams []*model.Upstream
	for _, pool := range pools {
		upstream := &model.Upstream{}
		upstream.Name = pool.Name
		upstream.UseLeastConn = pool.LeastConn
		var servers []model.UServer
		for _, node := range pool.Nodes {
			server := model.UServer{
				Address: node.Host + ":" + fmt.Sprintf("%v", node.Port),
				Params: model.Params{
					Weight: 1,
					MaxFails: node.MaxFails,
					FailTimeout: node.FailTimeout,
				},
			}
			servers = append(servers, server)
		}
		upstream.Servers = servers
		upstreams = append(upstreams, upstream)
	}
	if len(upstreams) > 0 {
		if err := template.NewUpstreamTemplateWithCfgPath(upstreams, tmpl, path, filename); err != nil {
			logrus.Errorf("Fail to new nginx Upstream ocfg file: %v", err)
			return err
		}
	}
	return nil
}

func getNgxServer(conf *v1.Config) (l7srv []*model.Server, l4srv []*model.Server) {
	for _, vs := range conf.L7VS {
		server := &model.Server{
			Listen:           strings.Join(vs.Listening, " "),
			ServerName:       vs.ServerName,
			ForceSSLRedirect: vs.ForceSSLRedirect,
		}
		if vs.SSLCert != nil {
			server.SSLCertificate = vs.SSLCert.CertificatePem
			server.SSLCertificateKey = vs.SSLCert.CertificatePem
		}
		for _, loc := range vs.Locations {
			location := &model.Location{
				Path:          loc.Path,
				NameCondition: loc.NameCondition,
				ProxySetHeaders: []*model.ProxySetHeader{
					{"Host", "$host"},
					{"X-Real-IP", "$remote_addr"},
					{"X-Forwarded-For", "$proxy_add_x_forwarded_for"},
				},
			}
			server.Locations = append(server.Locations, location)
		}
		l7srv = append(l7srv, server)
	}

	for _, vs := range conf.L4VS {
		server := &model.Server{
			ProxyPass: vs.PoolName,
		}
		server.Listen = strings.Join(vs.Listening, " ")
		l4srv = append(l4srv, server)
	}

	return l7srv, l4srv
}

// UpdatePools updates http upstreams dynamically.
func (o *OrService) UpdatePools(hpools []*v1.Pool, tpools []*v1.Pool) error {
	if len(tpools) > 0 {
		err := o.persistUpstreams(tpools, "upstreams-tcp.tmpl", "/run/nginx/rainbond/stream",
			"upstream.default.tcp.conf")
		if err != nil {
			logrus.Warningf("error updating upstream.default.tcp.conf")
		}
		// check nginx configuration
		if out, err := nginxExecCommand("-t").CombinedOutput(); err != nil {
			return fmt.Errorf("%v\n%v", err, string(out))
		}
		logrus.Debug("Nginx configuration is ok.")

		// reload nginx
		if out, err := nginxExecCommand("-s", "reload").CombinedOutput(); err != nil {
			return fmt.Errorf("%v\n%v", err, string(out))
		}
		logrus.Debug("Nginx reloads successfully.")
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
	logrus.Infof("dynamically update Upstream success")
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
	cfgPath := "/run/nginx/rainbond"
	httpCfgPath := fmt.Sprintf("%s/%s", cfgPath, "http") // http config path
	tcpCfgPath := fmt.Sprintf("%s/%s", cfgPath, "stream") // tcp config path
	// delete the old configuration
	if err := os.RemoveAll(httpCfgPath); err != nil {
		logrus.Errorf("Cant not remove directory(%s): %v", httpCfgPath, err)
		return err
	}
	if err := os.RemoveAll(tcpCfgPath); err != nil {
		logrus.Errorf("Cant not remove directory(%s): %v", tcpCfgPath, err)
		return err
	}

	// create cert
	err := createCert(cfgPath, "goodrain.me")
	if err != nil {
		return err
	}

	if o.ocfg.EnableKApiServer {
		ksrv := kubeApiserver(o.ocfg.KApiServerIP)
		if err := template.NewServerTemplateWithCfgPath(
			[]*model.Server{
				ksrv,
			}, tcpCfgPath, "server.default.tcp.conf"); err != nil {
			return err
		}
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
		if err := template.NewUpstreamTemplateWithCfgPath(
			[]*model.Upstream{dummyUpstream},
			"upstreams-tcp.tmpl",
			tcpCfgPath,
			"upstream.default.tcp.conf"); err != nil {
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
		gesrv := goodrainMe(cfgPath, o.ocfg.GrMeIP)
		srv = append(srv, gesrv)
	}
	if o.ocfg.EnableRepoGrMe {
		resrv := repoGoodrainMe(o.ocfg.RepoGrMeIP)
		srv = append(srv, resrv)
	}

	if err := template.NewServerTemplateWithCfgPath(srv, httpCfgPath,
		"servers.default.http.conf"); err != nil {
		return err
	}

	return nil
}

func createCert(cfgPath string, cn string) error {
	p := fmt.Sprintf("%s/%s", cfgPath, "ssl")
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
		logrus.Errorf("Create crt error: ", err)
		return err
	}
	crtInfo := baseinfo
	crtInfo.IsCA = false
	crtInfo.CrtName = fmt.Sprintf("%s/%s", cfgPath, "ssl/server.crt")
	crtInfo.KeyName = fmt.Sprintf("%s/%s", cfgPath, "ssl/server.key")
	crtInfo.Names = []pkix.AttributeTypeAndValue{{asn1.ObjectIdentifier{2, 1, 3}, "MAC_ADDR"}}

	crt, pri, err := cert.Parse(baseinfo.CrtName, baseinfo.KeyName)
	if err != nil {
		logrus.Errorf("Parse crt error,Error info:", err)
		return err
	}
	err = cert.CreateCRT(crt, pri, crtInfo)
	if err != nil {
		logrus.Errorf("Create crt error,Error info:", err)
		return err
	}

	logrus.Info("Create certificate for goodrain.me successfully")

	return nil
}
