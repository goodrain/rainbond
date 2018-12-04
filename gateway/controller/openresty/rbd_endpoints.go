package openresty

import (
	"fmt"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/goodrain/rainbond/gateway/v1"
)

func langGoodrainMe(ip string) (*model.Server, *model.Upstream) {
	svr := &model.Server{
		Listen:     fmt.Sprintf("%s:%d", ip, 80),  // TODO: change ip address
		ServerName: "lang.goodrain.me",
		Rewrites: []model.Rewrite{
			{
				Regex:       "^/(.*)$",
				Replacement: "/artifactory/pkg_lang/$1",
				Flag:        "break",
			},
		},
		Locations: []*model.Location{
			{
				Path: "/",
				ProxyRedirect: "off",
				ProxyConnectTimeout: model.Time{Num: 60, Unit: "s"},
				ProxyReadTimeout: model.Time{Num: 600, Unit: "s"},
				ProxySendTimeout: model.Time{Num: 600, Unit: "s"},
				ProxySetHeaders: []*model.ProxySetHeader{
					{Field: "Host", Value: "$http_host"},
					{Field: "X-Real-IP", Value: "$remote_addr"},
					{Field: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
				},
				NameCondition: map[string]*v1.Condition{
					"lang0": {
						Type:  v1.DefaultType,
						Value: map[string]string{"1": "1"},
					},
				},
			},
		},
	}
	us := &model.Upstream{
		Name: "lang",
	}
	return svr, us
}

func mavenGoodrainMe(ip string) (*model.Server, *model.Upstream) {
	svr := &model.Server{
		Listen:     fmt.Sprintf("%s:%d", ip, 80),
		ServerName: "maven.goodrain.me",
		Locations: []*model.Location{
			{
				Path: "/",
				Rewrites: []model.Rewrite{
					{
						Regex:       "^/(.*)$",
						Replacement: "/artifactory/libs-release/$1",
						Flag:        "break",
					},
				},
				ProxyRedirect: "off",
				ProxyConnectTimeout: model.Time{Num: 60, Unit: "s"},
				ProxyReadTimeout: model.Time{Num: 600, Unit: "s"},
				ProxySendTimeout: model.Time{Num: 600, Unit: "s"},
				ProxySetHeaders: []*model.ProxySetHeader{
					{Field: "Host", Value: "$http_host"},
					{Field: "X-Real-IP", Value: "$remote_addr"},
					{Field: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
				},
				NameCondition: map[string]*v1.Condition{
					"maven0": {
						Type:  v1.DefaultType,
						Value: map[string]string{"1": "1"},
					},
				},
			},
			{
				Path: "/monitor",
				Return: model.Return{Code: 204},
				DisableProxyPass: true,
			},
		},
	}
	us := &model.Upstream{
		Name: "maven",
	}
	return svr, us
}

func goodrainMe(cfgPath string, ip string) (*model.Server, *model.Upstream) {
	svr := &model.Server{
		Listen:     fmt.Sprintf("%s:%d %s", ip, 443, "ssl"),
		ServerName: "goodrain.me",
		SSLCertificate: fmt.Sprintf("%s/%s", cfgPath, "ssl/server.crt"),
		SSLCertificateKey: fmt.Sprintf("%s/%s", cfgPath, "ssl/server.key"),
		ClientMaxBodySize: model.Size{Num:0, Unit:"k"},
		ChunkedTransferEncoding: true,
		Locations: []*model.Location{
			{
				Path: "/v2/",
				ProxySetHeaders: []*model.ProxySetHeader{
					{Field: "Host", Value: "$http_host"},
					{Field: "X-Real-IP", Value: "$remote_addr"},
					{Field: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
					{Field: "X-Forwarded-Proto", Value: "$scheme"},
				},
				ProxyReadTimeout: model.Time{
					Num: 900,
					Unit: "s",
				},
				NameCondition: map[string]*v1.Condition{
					"registry0": {
						Type:  v1.DefaultType,
						Value: map[string]string{"1": "1"},
					},
				},
			},
		},
	}
	us := &model.Upstream{
		Name: "registry",
	}
	return svr, us
}
