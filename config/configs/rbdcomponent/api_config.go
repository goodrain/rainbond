package rbdcomponent

import (
	"github.com/goodrain/rainbond-operator/util/constants"
	utils "github.com/goodrain/rainbond/util"
	"github.com/spf13/pflag"
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
	LicensePath            string
	LicSoPath              string
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
	fs.StringVar(&apic.LicensePath, "license-path", "/opt/rainbond/etc/license/license.yb", "the license path of the enterprise version.")
	fs.StringVar(&apic.LicSoPath, "license-so-path", "/opt/rainbond/etc/license/license.so", "Dynamic library file path for parsing the license.")
	fs.StringVar(&apic.KuberentesDashboardAPI, "k8s-dashboard-api", "kubernetes-dashboard."+utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)+":443", "The service DNS name of Kubernetes dashboard. Default to kubernetes-dashboard.kubernetes-dashboard")
	fs.StringVar(&apic.GrctlImage, "shell-image", "registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-shell:v5.13.0-release", "use shell image")
	fs.StringSliceVar(&apic.NodeAPI, "node-api", []string{"rbd-node:6100"}, "the rbd-node server api")
	fs.StringSliceVar(&apic.EventLogEndpoints, "event-log", []string{"local=>rbd-eventlog:6363"}, "event log websocket address")
}
