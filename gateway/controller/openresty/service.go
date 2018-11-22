package openresty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/goodrain/rainbond/gateway/controller/openresty/template"
	"github.com/goodrain/rainbond/gateway/v1"
	"k8s.io/ingress-nginx/ingress/controller/process"
)

type OpenrestyService struct {
	AuxiliaryPort  int
	IsShuttingDown *bool

	// stopLock is used to enforce that only a single call to Stop send at
	// a given time. We allow stopping through an HTTP endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock *sync.Mutex
	config   *option.Config
}

//CreateOpenrestyService create openresty service
func CreateOpenrestyService(config *option.Config, isShuttingDown *bool) *OpenrestyService {
	gws := &OpenrestyService{
		AuxiliaryPort:  config.ListenPorts.AuxiliaryPort,
		IsShuttingDown: isShuttingDown,
		config:         config,
	}
	return gws
}

type Upstream struct {
	Name    string
	Servers []*Server
}

type Server struct {
	Host string
	Port int32
}

//Start start
func (osvc *OpenrestyService) Start() error {
	nginx := model.NewNginx(*osvc.config, template.CustomConfigPath)
	if err := template.NewNginxTemplate(nginx, defaultNginxConf); err != nil {
		return err
	}
	o, err := nginxExecCommand().CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v\n%v", err, string(o))
	}
	return nil
}

// Starts starts nginx
func (osvc *OpenrestyService) Starts(errCh chan error) {
	nginx := model.NewNginx(*osvc.config, template.CustomConfigPath)
	nginx.HTTP = model.NewHTTP(osvc.config)
	if err := template.NewNginxTemplate(nginx, defaultNginxConf); err != nil {
		logrus.Fatalf("Can't not new nginx config: %v", err)
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
func (osvc *OpenrestyService) Stop() error {
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
func (osvc *OpenrestyService) PersistConfig(conf *v1.Config) error {
	if err := osvc.PersistUpstreams(conf.HttpPools, "upstreams-http.tmpl", "http/upstreams.conf"); err != nil {
		logrus.Errorf("fail to persist http upstreams.conf ")
	}

	if err := osvc.PersistUpstreams(conf.TCPPools, "upstreams-tcp.tmpl", "stream/upstreams.conf"); err != nil {
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

// PersistUpstreams persists upstreams
func (osvc *OpenrestyService) PersistUpstreams(pools []*v1.Pool, tmpl string, filename string) error {
	var upstreams []model.Upstream
	for _, pool := range pools {
		upstream := model.Upstream{}
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
		if err := template.NewUpstreamTemplate(upstreams, tmpl, filename); err != nil {
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
func (osvc *OpenrestyService) UpdatePools(pools []*v1.Pool) error {
	if len(pools) == 0 {
		return nil
	}
	var upstreams []*Upstream
	for _, pool := range pools {
		upstream := &Upstream{}
		upstream.Name = pool.Name
		for _, node := range pool.Nodes {
			server := &Server{
				Host: node.Host,
				Port: node.Port,
			}
			upstream.Servers = append(upstream.Servers, server)
		}
		upstreams = append(upstreams, upstream)
	}
	return osvc.updateUpstreams(upstreams)
}

// updateUpstreams updates the upstreams in ngx.shared.dict by post
func (osvc *OpenrestyService) updateUpstreams(upstream []*Upstream) error {
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

func (osvc *OpenrestyService) DeletePools(pools []*v1.Pool) error {
	if len(pools) == 0 {
		return nil
	}
	var data []string
	for _, pool := range pools {
		data = append(data, pool.Name)
	}
	return osvc.deletePools(data)
}
func (osvc *OpenrestyService) deletePools(names []string) error {
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
func (osvc *OpenrestyService) WaitPluginReady() {
	url := fmt.Sprintf("http://127.0.0.1:%v/healthz", osvc.AuxiliaryPort)
	for {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			logrus.Info("Nginx is ready")
			break
		}
		logrus.Info("Nginx is not ready yet: %v", err)
		time.Sleep(1 * time.Second)
	}
}
