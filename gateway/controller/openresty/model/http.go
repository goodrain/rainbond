package model

import "github.com/goodrain/rainbond/cmd/gateway/option"

// HTTP contains data for nginx http configuration
type HTTP struct {
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
	StatusPort           int
	UpstreamsDict        Size
}

type LogFormat struct {
	Name   string
	Format string
}

type AccessLog struct {
	Name string
	Path string
}

// NewHTTP creates a new model.HTTP
func NewHTTP(conf *option.Config) *HTTP {
	return &HTTP{
		DefaultType: "text/html",
		SendFile:    true,
		StatusPort:  conf.ListenPorts.Status,
		KeepaliveTimeout: Time{
			Num:  30,
			Unit: "s",
		},
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
		UpstreamsDict: Size{
			Num:  128,
			Unit: "k",
		},
	}
}
