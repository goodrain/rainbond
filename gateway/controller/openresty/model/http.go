package model

import "github.com/goodrain/rainbond/cmd/gateway/option"

type Http struct {
	DefaultType          string
	SendFile             bool
	KeepaliveTimeout     Time
	keepaliveRequests    int
	ClientMaxBodySize    Size
	ClientBodyBufferSize Size
	ProxyConnectTimeout  Time
	ProxySendTimeout     Time
	ProxyReadTimeout     Time
	ProxyBufferSize      Size
	ProxyBuffers         Size
	ProxyBusyBuffersSize Size

	Includes []string
}

type LogFormat struct {
	Name   string
	Format string
}

type AccessLog struct {
	Name string
	Path string
}

// NewHttp creates a new model.Http
func NewHttp(conf option.Config) *Http {
	return &Http{
		DefaultType: "text/html",
		SendFile:    true,
		KeepaliveTimeout: Time{
			Num:  conf.KeepaliveTimeout,
			Unit: "s",
		},
		keepaliveRequests: conf.KeepaliveRequests,
		ClientMaxBodySize: Size{
			Num:  10,
			Unit: "m",
		},
		ClientBodyBufferSize: Size{
			Num:  128,
			Unit: "k",
		},
		ProxyConnectTimeout: Time{
			Num:  75,
			Unit: "s",
		},
		ProxySendTimeout: Time{
			Num:  75,
			Unit: "s",
		},
		ProxyReadTimeout: Time{
			Num:  75,
			Unit: "s",
		},
		ProxyBufferSize: Size{
			Num:  8,
			Unit: "k",
		},
		ProxyBuffers: Size{
			Num:  32,
			Unit: "k",
		},
		Includes: []string{},
	}
}
