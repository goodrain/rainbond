package rbdcomponent

import (
	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/api/eventlog/conf"
	utils "github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/thejerf/suture"
)

// APIConfig config
type APIConfig struct {
	APIAddr                string
	APIHealthzAddr         string
	APIAddrSSL             string
	EventLogEndpoints      []string
	NodeAPI                []string
	BuilderAPI             []string
	V1API                  string
	APISSL                 bool
	APICertFile            string
	APIKeyFile             string
	APICaFile              string
	Opentsdb               string
	RegionTag              string
	EnableFeature          []string
	Debug                  bool
	MinExtPort             int // minimum external port
	KuberentesDashboardAPI string
	GrctlImage             string
	RegionName             string
	RegionSN               string
	StartRegionAPI         bool
}

func AddAPIFlags(fs *pflag.FlagSet, apic *APIConfig) {
	fs.StringVar(&apic.APIAddr, "api-addr", "0.0.0.0:8888", "the api server listen address")
	fs.StringVar(&apic.APIHealthzAddr, "api-healthz-addr", "0.0.0.0:8889", "the api server health check listen address")
	fs.StringVar(&apic.APIAddrSSL, "api-addr-ssl", "0.0.0.0:8443", "the api server listen address")
	fs.BoolVar(&apic.APISSL, "api-ssl-enable", false, "whether to enable websocket  SSL")
	fs.StringVar(&apic.APICaFile, "client-ca-file", "", "api ssl ca file")
	fs.StringVar(&apic.APICertFile, "api-ssl-certfile", "", "api ssl cert file")
	fs.StringVar(&apic.APIKeyFile, "api-ssl-keyfile", "", "api ssl cert file")
	fs.BoolVar(&apic.StartRegionAPI, "start", false, "Whether to start region old api")
	fs.StringVar(&apic.V1API, "v1-api", "127.0.0.1:8887", "the region v1 api")
	fs.StringSliceVar(&apic.BuilderAPI, "builder-api", []string{"rbd-chaos:3228"}, "the builder api")
	fs.StringVar(&apic.Opentsdb, "opentsdb", "127.0.0.1:4242", "opentsdb server config")
	fs.StringVar(&apic.RegionTag, "region-tag", "test-ali", "region tag setting")
	fs.BoolVar(&apic.Debug, "debug", false, "open debug will enable pprof")
	fs.IntVar(&apic.MinExtPort, "min-ext-port", 0, "minimum external port")
	fs.StringArrayVar(&apic.EnableFeature, "enable-feature", []string{}, "List of special features supported, such as `windows`")
	fs.StringVar(&apic.KuberentesDashboardAPI, "k8s-dashboard-api", "kubernetes-dashboard."+utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)+":443", "The service DNS name of Kubernetes dashboard. Default to kubernetes-dashboard.kubernetes-dashboard")
	fs.StringVar(&apic.GrctlImage, "shell-image", "registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-shell:latest", "use shell image")
	fs.StringSliceVar(&apic.NodeAPI, "node-api", []string{"rbd-node:6100"}, "the rbd-node server api")
	fs.StringSliceVar(&apic.EventLogEndpoints, "event-log", []string{"local=>rbd-eventlog:6363"}, "event log websocket address")
}

type EventLogConfig struct {
	Conf   conf.Conf
	Entry  *Entry
	Logger *logrus.Logger
}

type Entry struct {
	supervisor *suture.Supervisor
	log        *logrus.Entry
	conf       EntryConf
}

type EntryConf struct {
	EventLogServer              EventLogServerConf
	DockerLogServer             DockerLogServerConf
	MonitorMessageServer        MonitorMessageServerConf
	NewMonitorMessageServerConf NewMonitorMessageServerConf
}

type EventLogServerConf struct {
	BindIP           string
	BindPort         int
	CacheMessageSize int
}

// MonitorMessageServerConf monitor message server conf
type MonitorMessageServerConf struct {
	SubAddress       []string
	SubSubscribe     string
	CacheMessageSize int
}

// NewMonitorMessageServerConf new monitor message server conf
type NewMonitorMessageServerConf struct {
	ListenerHost string
	ListenerPort int
}

// DockerLogServerConf docker log server conf
type DockerLogServerConf struct {
	BindIP           string
	BindPort         int
	CacheMessageSize int
	Mode             string
}

func AddEventLogFlags(fs *pflag.FlagSet, elc *EventLogConfig) {
	fs.StringVar(&elc.Conf.Entry.EventLogServer.BindIP, "eventlog.bind.ip", "0.0.0.0", "Collect the log service to listen the IP")
	fs.IntVar(&elc.Conf.Entry.EventLogServer.BindPort, "eventlog.bind.port", 6366, "Collect the log service to listen the Port")
	fs.IntVar(&elc.Conf.Entry.EventLogServer.CacheMessageSize, "eventlog.cache", 100, "the event log server cache the receive message size")
	fs.StringVar(&elc.Conf.Entry.DockerLogServer.BindIP, "dockerlog.bind.ip", "0.0.0.0", "Collect the log service to listen the IP")
	fs.StringVar(&elc.Conf.Entry.DockerLogServer.Mode, "dockerlog.mode", "stream", "the server mode zmq or stream")
	fs.IntVar(&elc.Conf.Entry.DockerLogServer.BindPort, "dockerlog.bind.port", 6362, "Collect the log service to listen the Port")
	fs.IntVar(&elc.Conf.Entry.DockerLogServer.CacheMessageSize, "dockerlog.cache", 200, "the docker log server cache the receive message size")
	fs.StringSliceVar(&elc.Conf.Entry.MonitorMessageServer.SubAddress, "monitor.subaddress", []string{"tcp://127.0.0.1:9442"}, "monitor message source address")
	fs.IntVar(&elc.Conf.Entry.MonitorMessageServer.CacheMessageSize, "monitor.cache", 200, "the monitor sub server cache the receive message size")
	fs.StringVar(&elc.Conf.Entry.MonitorMessageServer.SubSubscribe, "monitor.subscribe", "ceptop", "the monitor message sub server subscribe info")
	fs.StringVar(&elc.Conf.Cluster.Discover.InstanceIP, "cluster.instance.ip", "", "The current instance IP in the cluster can be communications.")
	fs.StringVar(&elc.Conf.Cluster.Discover.Type, "discover.type", "etcd", "the instance in cluster auto discover way.")
	fs.StringVar(&elc.Conf.Cluster.Discover.HomePath, "discover.etcd.homepath", "/event", "etcd home key")
	fs.StringVar(&elc.Conf.Cluster.PubSub.PubBindIP, "cluster.bind.ip", "0.0.0.0", "Cluster communication to listen the IP")
	fs.IntVar(&elc.Conf.Cluster.PubSub.PubBindPort, "cluster.bind.port", 6365, "Cluster communication to listen the Port")
	fs.StringVar(&elc.Conf.EventStore.MessageType, "message.type", "json", "Receive and transmit the log message type.")
	fs.StringVar(&elc.Conf.EventStore.GarbageMessageSaveType, "message.garbage.save", "file", "garbage message way of storage")
	fs.StringVar(&elc.Conf.EventStore.GarbageMessageFile, "message.garbage.file", "/var/log/envent_garbage_message.log", "save garbage message file path when save type is file")
	fs.Int64Var(&elc.Conf.EventStore.PeerEventMaxLogNumber, "message.max.number", 100000, "the max number log message for peer event")
	fs.IntVar(&elc.Conf.EventStore.PeerEventMaxCacheLogNumber, "message.cache.number", 256, "Maintain log the largest number in the memory peer event")
	fs.Int64Var(&elc.Conf.EventStore.PeerDockerMaxCacheLogNumber, "dockermessage.cache.number", 128, "Maintain log the largest number in the memory peer docker service")
	fs.IntVar(&elc.Conf.EventStore.HandleMessageCoreNumber, "message.handle.core.number", 2, "The number of concurrent processing receive log data.")
	fs.IntVar(&elc.Conf.EventStore.HandleSubMessageCoreNumber, "message.sub.handle.core.number", 3, "The number of concurrent processing receive log data. more than message.handle.core.number")
	fs.IntVar(&elc.Conf.EventStore.HandleDockerLogCoreNumber, "message.dockerlog.handle.core.number", 2, "The number of concurrent processing receive log data. more than message.handle.core.number")
	fs.StringVar(&elc.Conf.WebSocket.BindIP, "websocket.bind.ip", "0.0.0.0", "the bind ip of websocket for push event message")
	fs.IntVar(&elc.Conf.WebSocket.BindPort, "websocket.bind.port", 6363, "the bind port of websocket for push event message")
	fs.IntVar(&elc.Conf.WebSocket.SSLBindPort, "websocket.ssl.bind.port", 6364, "the ssl bind port of websocket for push event message")
	fs.BoolVar(&elc.Conf.WebSocket.EnableCompression, "websocket.compression", true, "weither enable compression for web socket")
	fs.IntVar(&elc.Conf.WebSocket.ReadBufferSize, "websocket.readbuffersize", 4096, "the readbuffersize of websocket for push event message")
	fs.IntVar(&elc.Conf.WebSocket.WriteBufferSize, "websocket.writebuffersize", 4096, "the writebuffersize of websocket for push event message")
	fs.IntVar(&elc.Conf.WebSocket.MaxRestartCount, "websocket.maxrestart", 5, "the max restart count of websocket for push event message")
	fs.BoolVar(&elc.Conf.WebSocket.SSL, "websocket.ssl", false, "whether to enable websocket  SSL")
	fs.StringVar(&elc.Conf.WebSocket.CertFile, "websocket.certfile", "/etc/ssl/goodrain.com/goodrain.com.crt", "websocket ssl cert file")
	fs.StringVar(&elc.Conf.WebSocket.KeyFile, "websocket.keyfile", "/etc/ssl/goodrain.com/goodrain.com.key", "websocket ssl cert file")
	fs.StringVar(&elc.Conf.WebSocket.TimeOut, "websocket.timeout", "1m", "Keep websocket service the longest time when without message ")
	fs.StringVar(&elc.Conf.WebSocket.PrometheusMetricPath, "monitor-path", "/metrics", "promethesu monitor metrics path")
	fs.StringVar(&elc.Conf.Entry.NewMonitorMessageServerConf.ListenerHost, "monitor.udp.host", "0.0.0.0", "receive new monitor udp server host")
	fs.IntVar(&elc.Conf.Entry.NewMonitorMessageServerConf.ListenerPort, "monitor.udp.port", 6166, "receive new monitor udp server port")
	fs.StringVar(&elc.Conf.EventStore.StorageHomePath, "docker.log.homepath", "/grdata/logs/", "container log persistent home path")
}
