package openresty

import (
	"github.com/goodrain/rainbond/gateway/annotations/rewrite"
	"fmt"
	"github.com/goodrain/rainbond/gateway/annotations/proxy"

	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/goodrain/rainbond/gateway/v1"
)

func langGoodrainMe(ip string) *model.Server {
	proxy := proxy.NewProxyConfig()
	proxy.ConnectTimeout = 60
	proxy.ReadTimeout = 600
	proxy.SendTimeout = 600
	svr := &model.Server{
		Listen:     fmt.Sprintf("%s:%d", ip, 80),
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
				Path:             "/",
				ProxyRedirect:    "off",
				EnableMetrics:    false,
				DisableAccessLog: true,
				Proxy:            proxy,
				NameCondition: map[string]*v1.Condition{
					"lang": {
						Type:  v1.DefaultType,
						Value: map[string]string{"1": "1"},
					},
				},
			},
		},
	}
	return svr
}

func mavenGoodrainMe(ip string) *model.Server {
	proxy := proxy.NewProxyConfig()
	proxy.ConnectTimeout = 60
	proxy.ReadTimeout = 600
	proxy.SendTimeout = 600
	svr := &model.Server{
		Listen:     fmt.Sprintf("%s:%d", ip, 80),
		ServerName: "maven.goodrain.me",
		Locations: []*model.Location{
			{
				EnableMetrics:    false,
				DisableAccessLog: true,
				Path:             "/",
				Rewrite: rewrite.Config{
					Rewrites: []*rewrite.Rewrite {
						{
							Regex:       "^/(.*)$",
							Replacement: "/artifactory/libs-release/$1",
							Flag:        "break",
						},
					},
				},
				ProxyRedirect: "off",
				Proxy:         proxy,
				NameCondition: map[string]*v1.Condition{
					"maven": {
						Type:  v1.DefaultType,
						Value: map[string]string{"1": "1"},
					},
				},
			},
			{
				Path:             "/monitor",
				Proxy:            proxy,
				Return:           model.Return{Code: 204},
				DisableProxyPass: true,
			},
		},
	}
	return svr
}

func goodrainMe(cfgPath string, ip string) *model.Server {
	proxy := proxy.NewProxyConfig()
	proxy.ReadTimeout = 900
	proxy.BodySize = 0
	proxy.SetHeaders["X-Forwarded-Proto"] = "https"
	svr := &model.Server{
		Listen:                  fmt.Sprintf("%s:%d %s", ip, 443, "ssl"),
		ServerName:              "goodrain.me",
		SSLCertificate:          fmt.Sprintf("%s/%s", cfgPath, "ssl/server.crt"),
		SSLCertificateKey:       fmt.Sprintf("%s/%s", cfgPath, "ssl/server.key"),
		ClientMaxBodySize:       model.Size{Num: 0, Unit: "k"},
		ChunkedTransferEncoding: true,
		Locations: []*model.Location{
			{
				EnableMetrics:    false,
				DisableAccessLog: true,
				Path:             "/v2/",
				Proxy: proxy,
				NameCondition: map[string]*v1.Condition{
					"registry": {
						Type:  v1.DefaultType,
						Value: map[string]string{"1": "1"},
					},
				},
			},
			{
				Path:             "/monitor",
				Proxy:            proxy,
				Return:           model.Return{Code: 200, Text: "ok"},
				DisableProxyPass: true,
			},
		},
	}
	return svr
}

func repoGoodrainMe(ip string) *model.Server {
	return &model.Server{
		Listen:     fmt.Sprintf("%s:%d", ip, 80),
		Root:       "/grdata/services/offline/pkgs/",
		ServerName: "repo.goodrain.me",
	}
}

func kubeApiserver(ip string) *model.Server {
	svr := &model.Server{
		Listen:    fmt.Sprintf("%s:%d", ip, 6443),
		ProxyPass: "kube_apiserver",
		ProxyTimeout: model.Time{
			Num:  10,
			Unit: "m",
		},
		ProxyConnectTimeout: model.Time{
			Num:  10,
			Unit: "m",
		},
	}

	return svr
}
