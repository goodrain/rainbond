package openresty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/goodrain/rainbond/gateway/controller/openresty/template"
	"github.com/goodrain/rainbond/gateway/v1"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type OpenrestyService struct{}

type Upstream struct {
	Name    string
	Servers []*Server
}

type Server struct {
	Host string
	Port int32
}

func (osvc *OpenrestyService) Start() error {
	o, err := nginxExecCommand().CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v\n%v", err, string(o))
	}
	return nil
}

func (osvc *OpenrestyService) PersistConfig(conf *v1.Config) error {
	upsHttp := "upstreams-http.conf"
	if err := osvc.PersistUpstreams(conf.HttpPools, "upstreams-http.tmpl", upsHttp); err != nil {
		logrus.Errorf("fail to persist %s", upsHttp)
	}

	upsTcp := "upstreams-tcp.conf"
	if err := osvc.PersistUpstreams(conf.TCPPools, "upstreams-tcp.tmpl", upsTcp); err != nil {
		logrus.Errorf("fail to persist %s", upsHttp)
	}

	l7srv, l4srv := getNgxServer(conf)
	var ngxIncludes []string
	// http
	if len(l7srv) > 0 {
		serverFilename := "servers-http.conf"
		if err := template.NewServerTemplate(l7srv, serverFilename); err != nil {
			logrus.Errorf("Fail to new nginx Server config file: %v", err)
			return err
		}

		httpData := model.NewHttp()
		httpData.Includes = []string{
			serverFilename,
			upsHttp,
		}
		httpFilename := "http.conf"
		if err := template.NewHttpTemplate(httpData, httpFilename); err != nil {
			logrus.Fatalf("Fail to new nginx http template: %v", err)
			return nil
		}
		ngxIncludes = append(ngxIncludes, httpFilename)
	}

	// stream
	if len(l4srv) > 0 {
		serverFilename := "servers-tcp.conf"
		if err := template.NewServerTemplate(l4srv, serverFilename); err != nil {
			logrus.Errorf("Fail to new nginx Server file: %v", err)
			return err
		}
		streamData := model.NewStream()
		streamData.Includes = []string{
			serverFilename,
			upsTcp,
		}
		streamFilename := "stream.conf"
		if err := template.NewStreamTemplate(streamData, streamFilename); err != nil {
			logrus.Fatalf("Fail to new nginx stream template: %v", err)
			return nil
		}
		ngxIncludes = append(ngxIncludes, streamFilename)
	}

	nginx := model.NewNginx()
	nginx.Includes = ngxIncludes
	if err := template.NewNginxTemplate(nginx); err != nil {
		return err
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
				Path:      loc.Path,
				ProxyPass: loc.PoolName,
				Header:    loc.Header,
				Cookie:    loc.Cookie,
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
	return updateUpstreams(upstreams)
}

// updateUpstreams updates the upstreams in ngx.shared.dict by post
func updateUpstreams(upstream []*Upstream) error {
	url := "http://localhost:33333/update-upstreams" // TODO
	json, _ := json.Marshal(upstream)
	logrus.Debugf("request contest of update-upstreams is %v", string(json))

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(json))
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
	return deletePools(data)
}
func deletePools(data []string) error {
	url := "http://localhost:33333/delete-upstreams" // TODO
	json, _ := json.Marshal(data)
	logrus.Errorf("request content of delete-upstreams is %v", string(json))

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(json))
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
	url := "http://localhost:33333/healthy" // TODO
	for {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			logrus.Info("Nginx is ready")
			break
		}
		time.Sleep(200 * time.Microsecond)
	}
}
