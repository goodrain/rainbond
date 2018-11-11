package template

import (
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"testing"
)

func TestNewNginxTemplate(t *testing.T) {
	data := &model.Nginx{
		WorkerProcesses: 2,
		Includes:        []string{"/Users/abe/nginx/conf/vhosts/http/*.conf"},
	}

	err := NewNginxTemplate(data)
	if err != nil {
		t.Errorf("fail to new nginx config file: %v", err)
	}
}

func TestNewHttpTemplate(t *testing.T) {
	data := &model.Http{
		DefaultType: "text/html",
		SendFile:    true,
		KeepaliveTimeout: model.Time{
			Num:  70,
			Unit: "s",
		},
		ClientMaxBodySize: model.Size{
			Num:  10,
			Unit: "m",
		},
		ClientBodyBufferSize: model.Size{
			Num:  128,
			Unit: "k",
		},
		ProxyConnectTimeout: model.Time{
			Num:  75,
			Unit: "s",
		},
		ProxySendTimeout: model.Time{
			Num:  75,
			Unit: "s",
		},
		ProxyReadTimeout: model.Time{
			Num:  75,
			Unit: "s",
		},
		ProxyBufferSize: model.Size{
			Num:  8,
			Unit: "k",
		},
		ProxyBuffers: model.Size{
			Num:  32,
			Unit: "k",
		},
		Includes: []string{
			"/Users/abe/nginx/conf/vhosts/upstreams/*.conf",
			"/Users/abe/nginx/conf/vhosts/servers/*.conf",
		},
	}

	err := NewHttpTemplate(data, "/tmp/rbd-gateway/template/http.conf")
	if err != nil {
		t.Errorf("fail to new nginx http config file: %v", err)
	}
}

func TestNewServerTemplate(t *testing.T) {
	location := model.Location{
		Path:      "hello",
		ProxyPass: "http://endpoints",
	}

	data := &model.Server{
		Listen:     "80",
		ServerName: "dev-goodrain.com",
		KeepaliveTimeout: model.Time{
			Num:  70,
			Unit: "s",
		},
		DefaultType: "text/html",
		Charset:     "utf8",
		Locations: []model.Location{
			location,
		},
	}

	err := NewServerTemplate([]*model.Server{data}, "servers.conf")
	if err != nil {
		t.Errorf("fail to new nginx server config file: %v", err)
	}
}

func TestNewUpstreamTemplate(t *testing.T) {
	upstream := model.Upstream{
		Name: "dyn-upstream",
		Servers: []model.UServer{
			{
				Address: "localhost:7777",
				Params: model.Params{
					Weight: 1,
				},
			},
			{
				Address: "localhost:8888",
				Params: model.Params{
					Weight: 1,
				},
			},
			{
				Address: "localhost:9999",
				Params: model.Params{
					Weight: 1,
				},
			},
		},
	}

	err := NewUpstreamTemplate([]model.Upstream{upstream}, "dyn-upstreams.conf")
	if err != nil {
		t.Errorf("fail to new nginx config file: %v", err)
	}
}
