package openresty

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/goodrain/rainbond/gateway/controller/openresty/template"
	"github.com/goodrain/rainbond/gateway/v1"
	"strings"
)

type OpenrestyService struct{}

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

	httpServers, tcpServers := getNgxServer(conf)
	var ngxIncludes []string
	// http
	if len(httpServers) > 0 { // TODO: 是否需要一个server一个文件
		serverFilename := "Servers-http.conf"
		if err := template.NewServerTemplate(httpServers, serverFilename); err != nil {
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
	if len(tcpServers) > 0 {
		serverFilename := "Servers-tcp.conf"
		if err := template.NewServerTemplate(tcpServers, serverFilename); err != nil {
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

func getNgxServer(conf *v1.Config) (httpServers []*model.Server, tcpServers []*model.Server) {
	for _, vs := range conf.VirtualServices {
		if vs.Protocol == "stream" {
			server := &model.Server{
				ProxyPass: vs.PoolName,
			}
			server.Listen = strings.Join(vs.Listening, ",")
			tcpServers = append(tcpServers, server)
		} else {
			server := &model.Server{
				ServerName: vs.ServerName,
			}
			if vs.SSLCert != nil {
				server.SSLCertificate = vs.SSLCert.CertificatePem
				server.SSLCertificateKey = vs.SSLCert.CertificatePem
			}
			for _, loc := range vs.Locations {
				location := model.Location{
					Path:      loc.Path,
					ProxyPass: fmt.Sprintf("%s%s", "http://", loc.PoolName), // TODO https
				}
				server.Locations = append(server.Locations, location)
			}
			httpServers = append(httpServers, server)
		}
	}

	return httpServers, tcpServers
}
