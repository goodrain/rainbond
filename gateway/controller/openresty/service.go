package openresty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/golang/glog"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/goodrain/rainbond/gateway/controller/openresty/template"
	"github.com/goodrain/rainbond/gateway/v1"
	"k8s.io/ingress-nginx/ingress/controller/process"
)

// OrService handles the business logic of OpenrestyService
type OrService struct {
	AuxiliaryPort  int
	IsShuttingDown *bool

	// stopLock is used to enforce that only a single call to Stop send at
	// a given time. We allow stopping through an HTTP endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock *sync.Mutex
	config   *option.Config
}

//CreateOpenrestyService create openresty service
func CreateOpenrestyService(config *option.Config, isShuttingDown *bool) *OrService {
	gws := &OrService{
		AuxiliaryPort:  config.ListenPorts.AuxiliaryPort,
		IsShuttingDown: isShuttingDown,
		config:         config,
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
func (osvc *OrService) Start(errCh chan error) {
	// generate default nginx.conf
	nginx := model.NewNginx(*osvc.config, template.CustomConfigPath)
	nginx.HTTP = model.NewHTTP(osvc.config)
	if err := template.NewNginxTemplate(nginx, defaultNginxConf); err != nil {
		logrus.Fatalf("Can't not new nginx config: %v", err) // TODO: send err to errCh???
	}

	if osvc.config.EnableRbdEndpoints {
		if err := osvc.newRbdServers(); err != nil {
			errCh <- err // TODO: consider if it is right
		}
	}

	cmd := nginxExecCommand()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		glog.Fatalf("NGINX error: %v", err)
		errCh <- err
		return
	}

	go func() {
		errCh <- cmd.Wait()
	}()
}

// Stop gracefully stops the NGINX master process.
func (osvc *OrService) Stop() error {
	// send stop signal to NGINX
	logrus.Info("Stopping NGINX process")
	cmd := nginxExecCommand("-s", "quit")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	// wait for the NGINX process to terminate
	timer := time.NewTicker(time.Second * 1)
	for range timer.C {
		if !process.IsNginxRunning() {
			logrus.Info("NGINX process has stopped")
			timer.Stop()
			break
		}
	}

	return nil
}

// PersistConfig persists config
func (osvc *OrService) PersistConfig(conf *v1.Config) error {
	// delete the old configuration
	if err := os.RemoveAll(template.CustomConfigPath); err != nil {
		logrus.Errorf("Cant not remove directory(%s): %v", template.CustomConfigPath, err)
	}

	if err := osvc.persistUpstreams(conf.HTTPPools, "upstreams-http.tmpl", template.CustomConfigPath, "http/upstreams.conf"); err != nil {
		logrus.Errorf("fail to persist http upstreams.conf ")
	}
	if err := osvc.persistUpstreams(conf.TCPPools, "upstreams-tcp.tmpl", template.CustomConfigPath, "stream/upstreams.conf"); err != nil {
		logrus.Errorf("fail to persist tcp upstreams.conf")
	}
	pools := append(conf.HTTPPools, conf.TCPPools...)
	if err := osvc.persistUpstreams(pools, "update-ups.tmpl", "/run/nginx/","update-ups.conf"); err != nil {
		logrus.Errorf("fail to persist tcp upstreams.conf")
	}

	l7srv, l4srv := getNgxServer(conf)
	// http
	if len(l7srv) > 0 {
		filename := "http/servers.conf"
		if err := template.NewServerTemplate(l7srv, filename); err != nil {
			logrus.Errorf("Fail to new nginx Server config file: %v", err)
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
func (osvc *OrService) persistUpstreams(pools []*v1.Pool, tmpl string, path string, filename string) error {
	var upstreams []*model.Upstream
	for _, pool := range pools {
		upstream := &model.Upstream{}
		upstream.Name = pool.Name
		var servers []model.UServer
		for _, node := range pool.Nodes {
			server := model.UServer{
				Address: node.Host + ":" + fmt.Sprintf("%v", node.Port),
				Params: model.Params{
					Weight: 1,
				},
			}
			servers = append(servers, server)
		}
		upstream.Servers = servers
		upstreams = append(upstreams, upstream)
	}
	if len(upstreams) > 0 {
		if err := template.NewUpstreamTemplateWithCfgPath(upstreams, tmpl, path, filename); err != nil {
			logrus.Errorf("Fail to new nginx Upstream config file: %v", err)
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
func (osvc *OrService) UpdatePools(pools []*v1.Pool) error {
	if len(pools) == 0 {
		return nil
	}
	var upstreams []*Upstream
	for _, pool := range pools {
		upstream := &Upstream{}
		upstream.Name = pool.Name
		for _, node := range pool.Nodes {
			server := &Server{
				Host:   node.Host,
				Port:   node.Port,
				Weight: node.Weight,
			}
			upstream.Servers = append(upstream.Servers, server)
		}
		upstreams = append(upstreams, upstream)
	}
	return osvc.updateUpstreams(upstreams)
}

// updateUpstreams updates the upstreams in ngx.shared.dict by post
func (osvc *OrService) updateUpstreams(upstream []*Upstream) error {
	url := fmt.Sprintf("http://127.0.0.1:%v/update-upstreams", osvc.AuxiliaryPort)
	data, _ := json.Marshal(upstream)
	logrus.Debugf("request contest of update-upstreams is %v", string(data))

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		logrus.Errorf("fail to update upstreams: %v", err)
		return err
	}
	defer resp.Body.Close()

	logrus.Debugf("the status of dynamically updating upstreams is %v.", resp.Status)
	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("dynamically update Upstream, error is %v", err.Error())
		return err
	}

	logrus.Infof("dynamically update Upstream, response is %v", string(res))
	return nil
}

// DeletePools deletes pools
func (osvc *OrService) DeletePools(pools []*v1.Pool) error {
	if len(pools) == 0 {
		return nil
	}
	var data []string
	for _, pool := range pools {
		data = append(data, pool.Name)
	}
	return osvc.deletePools(data)
}

func (osvc *OrService) deletePools(names []string) error {
	url := fmt.Sprintf("http://127.0.0.1:%v/delete-upstreams", osvc.AuxiliaryPort)
	data, _ := json.Marshal(names)
	logrus.Debugf("request content of delete-upstreams is %v", string(data))

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		logrus.Errorf("fail to delete upstreams: %v", err)
		return err
	}
	defer resp.Body.Close()

	logrus.Debugf("the status of dynamically deleting upstreams is %v.", resp.Status)
	return nil
}

// WaitPluginReady waits for nginx to be ready.
func (osvc *OrService) WaitPluginReady() {
	url := fmt.Sprintf("http://127.0.0.1:%v/healthz", osvc.AuxiliaryPort)
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
func (osvc *OrService) newRbdServers() error {
	cfgPath := "/run/nginx/rainbond"
	// delete the old configuration
	if err := os.RemoveAll(cfgPath); err != nil {
		logrus.Errorf("Cant not remove directory(%s): %v", cfgPath, err)
		return err
	}

	lesrv, leus := langGoodrainMe()
	mesrv, meus := mavenGoodrainMe()
	gesrv, geus := goodrainMe()
	if err := template.NewServerTemplateWithCfgPath([]*model.Server{
		lesrv,
		mesrv,
		gesrv,
	}, cfgPath, "servers.default.http.conf"); err != nil {
		return err
	}

	// upstreams
	if err := template.NewUpstreamTemplateWithCfgPath([]*model.Upstream{
		leus,
		meus,
		geus,
	}, "upstreams-http.tmpl", cfgPath, "upstreams.default.http.conf"); err != nil {
		return err
	}
	return nil
}
