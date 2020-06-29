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
	HTTPListen           int
	HTTPSListen          int
	AccessLogPath        string
	DisableAccessLog     bool
	AccessLogFormat      string
}

// LogFormat -
type LogFormat struct {
	Name   string
	Format string
}

// AccessLog -
type AccessLog struct {
	Name string
	Path string
}

// NewHTTP creates a new model.HTTP
func NewHTTP(conf *option.Config) *HTTP {
	return &HTTP{
		HTTPListen:    conf.ListenPorts.HTTP,
		HTTPSListen:   conf.ListenPorts.HTTPS,
		DefaultType:   "text/html",
		SendFile:      true,
		StatusPort:    conf.ListenPorts.Status,
		AccessLogPath: conf.AccessLogPath,
		AccessLogFormat: func() string {
			if conf.AccessLogFormat == "" {
				return `$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent" $request_length $request_time $upstream_addr $upstream_response_length $upstream_response_time $upstream_status`
			}
			return conf.AccessLogFormat
		}(),
		DisableAccessLog: conf.AccessLogPath == "",
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
			Num:  int(conf.ShareMemory),
			Unit: "m",
		},
	}
}
