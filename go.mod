module github.com/goodrain/rainbond

go 1.13

require (
	github.com/DATA-DOG/go-sqlmock v1.3.3
	github.com/Microsoft/go-winio v0.4.15-0.20200113171025-3fe6c5262873 // indirect
	github.com/Microsoft/hcsshim v0.8.7 // indirect
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/aliyun/aliyun-oss-go-sdk v2.1.4+incompatible
	github.com/aws/aws-sdk-go v1.34.17
	github.com/barnettZQG/gotty v1.0.1-0.20200904091006-a0a1f7d747dc
	github.com/beorn7/perks v1.0.1
	github.com/bitly/go-simplejson v0.5.0
	github.com/bluebreezecf/opentsdb-goclient v0.0.0-20190921120552-796138372df3
	github.com/cockroachdb/cmux v0.0.0-20170110192607-30d10be49292 // indirect
	github.com/containerd/continuity v0.0.0-20200228182428-0f16d7a0959c // indirect
	github.com/coreos/etcd v3.3.17+incompatible
	github.com/coreos/prometheus-operator v0.38.3
	github.com/docker/cli v0.0.0-20190711175710-5b38d82aa076
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.4.2-0.20200309214505-aa6a9891b09c
	github.com/docker/go-units v0.4.0
	github.com/docker/libcompose v0.4.1-0.20190808084053-143e0f3f1ab9
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/eapache/channels v1.1.0
	github.com/emicklei/go-restful v2.14.2+incompatible
	github.com/emicklei/go-restful-swagger12 v0.0.0-20170926063155-7524189396c6
	github.com/envoyproxy/go-control-plane v0.9.5
	github.com/fatih/color v1.9.0
	github.com/fatih/structs v1.1.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-chi/render v1.0.1
	github.com/go-kit/kit v0.10.0
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/go-playground/assert/v2 v2.0.1
	github.com/go-sql-driver/mysql v1.5.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/mock v1.4.3
	github.com/golang/protobuf v1.4.2
	github.com/goodrain/rainbond-operator v1.0.0
	github.com/google/uuid v1.1.1
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/gosuri/uitable v0.0.4
	github.com/howeyc/fsnotify v0.9.0
	github.com/imdario/mergo v0.3.11
	github.com/jinzhu/gorm v1.9.16
	github.com/json-iterator/go v1.1.10
	github.com/kr/pty v1.1.8
	github.com/mattn/go-runewidth v0.0.6
	github.com/mattn/go-shellwords v1.0.10 // indirect
	github.com/mitchellh/go-ps v1.0.0
	github.com/mitchellh/go-wordwrap v1.0.0
	github.com/mitchellh/mapstructure v1.3.3
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/ncabatoff/process-exporter v0.7.1
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/pborman/uuid v1.2.1
	github.com/pebbe/zmq4 v1.2.1
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.12.0
	github.com/pquerna/ffjson v0.0.0-20190930134022-aa0246cd15f7
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.10.0
	github.com/prometheus/node_exporter v1.0.1
	github.com/prometheus/procfs v0.1.3
	github.com/shirou/gopsutil v2.20.8+incompatible
	github.com/sirupsen/logrus v1.6.0
	github.com/smartystreets/goconvey v1.6.4
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/testcontainers/testcontainers-go v0.7.0
	github.com/thejerf/suture v3.0.3+incompatible
	github.com/tidwall/gjson v1.6.1
	github.com/twinj/uuid v1.0.0
	github.com/urfave/cli v1.22.2
	github.com/yudai/umutex v0.0.0-20150817080136-18216d265c6b
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	golang.org/x/sys v0.0.0-20200622214017-ed371f2e16b4
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	google.golang.org/grpc v1.29.0
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.0
	k8s.io/apiextensions-apiserver v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/apiserver v0.19.0
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/ingress-nginx v0.0.0-20200903213136-333288e755c1
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.19.0
)

// Pinned to kubernetes-1.16.2
replace (
	github.com/coreos/etcd => github.com/coreos/etcd v3.2.31+incompatible
	github.com/coreos/go-systemd => github.com/coreos/go-systemd v0.0.0-20161114122254-48702e0da86b
	github.com/docker/cli => github.com/docker/cli v0.0.0-20181026144139-6b71e84ec8bf
	github.com/docker/docker => github.com/docker/engine v0.0.0-20190725163905-fa8dd90ceb7b
	github.com/docker/libcompose => github.com/docker/libcompose v0.4.1-0.20181019154650-213509acef0f
	github.com/godbus/dbus/v5 => github.com/godbus/dbus/v5 v5.0.3
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.0
	// github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.8.1-0.20161124155732-575f371f7862
	github.com/xeipuuv/gojsonschema => github.com/xeipuuv/gojsonschema v0.0.0-20160323030313-93e72a773fad
	k8s.io/api => k8s.io/api v0.16.15
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.16.15
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.15
	k8s.io/apiserver => k8s.io/apiserver v0.16.15
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20191016114015-74ad18325ed5
	k8s.io/client-go => k8s.io/client-go v12.0.0+incompatible
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20191016115326-20453efc2458
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20191016115129-c07a134afb42
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20191004115455-8e001e5d1894
	k8s.io/component-base => k8s.io/component-base v0.0.0-20191016111319-039242c015a9
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190828162817-608eb1dad4ac
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20191016115521-756ffa5af0bd
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20191016112429-9587704a8ad4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20191016114939-2b2b218dc1df
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20191016114407-2e83b6f20229
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20191016114748-65049c67a58b
	k8s.io/kubectl => k8s.io/kubectl v0.0.0-20191016120415-2ed914427d51
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20191016114556-7841ed97f1b2
	k8s.io/kubernetes => k8s.io/kubernetes v1.16.15
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20191016115753-cf0698c3a16b
	k8s.io/metrics => k8s.io/metrics v0.0.0-20191016113814-3b1a734dba6e
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20191016112829-06bb3c9d77c9
)
