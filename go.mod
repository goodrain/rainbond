module github.com/goodrain/rainbond

go 1.15

require (
	cuelang.org/go v0.2.2
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/alecthomas/units v0.0.0-20201120081800-1786d5ef83d4 // indirect
	github.com/aliyun/aliyun-oss-go-sdk v2.1.5+incompatible
	github.com/atcdot/gorm-bulk-upsert v1.0.0
	github.com/aws/aws-sdk-go v1.36.15
	github.com/barnettZQG/gotty v1.0.1-0.20200904091006-a0a1f7d747dc
	github.com/beorn7/perks v1.0.1
	github.com/bitly/go-simplejson v0.5.0
	github.com/bluebreezecf/opentsdb-goclient v0.0.0-20190921120552-796138372df3
	github.com/cockroachdb/cmux v0.0.0-20170110192607-30d10be49292 // indirect
	github.com/coreos/etcd v3.3.17+incompatible
	github.com/creack/pty v1.1.11 // indirect
	github.com/crossplane/crossplane-runtime v0.10.0
	github.com/docker/cli v20.10.3+incompatible
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.2+incompatible
	github.com/docker/go-units v0.4.0
	github.com/docker/libcompose v0.4.1-0.20190808084053-143e0f3f1ab9
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
	github.com/go-playground/validator/v10 v10.4.1
	github.com/go-sql-driver/mysql v1.5.0
	github.com/godbus/dbus v4.1.0+incompatible // indirect
	github.com/gofrs/flock v0.8.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.4.3
	github.com/goodrain/rainbond-oam v0.0.0-20210721020036-158e1be667dc
	github.com/goodrain/rainbond-operator v1.3.1-0.20210401055914-f8fe4bf89a21
	github.com/google/go-cmp v0.5.4 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/gosuri/uitable v0.0.4
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/howeyc/fsnotify v0.9.0
	github.com/imdario/mergo v0.3.11
	github.com/jinzhu/gorm v1.9.16
	github.com/json-iterator/go v1.1.10
	github.com/kr/pretty v0.2.1 // indirect
	github.com/kr/pty v1.1.8
	github.com/mattn/go-runewidth v0.0.6
	github.com/mitchellh/go-ps v1.0.0
	github.com/mitchellh/go-wordwrap v1.0.0
	github.com/mitchellh/mapstructure v1.3.3
	github.com/ncabatoff/process-exporter v0.7.1
	github.com/oam-dev/kubevela v1.1.0-alpha.4.0.20210625105426-e176fcfc56f0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.3
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/runc v1.0.0-rc91.0.20200707015106-819fcc687efb // indirect
	github.com/pborman/uuid v1.2.1
	github.com/pebbe/zmq4 v1.2.1
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.12.0
	github.com/pquerna/ffjson v0.0.0-20190930134022-aa0246cd15f7
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.45.0
	github.com/prometheus-operator/prometheus-operator/pkg/client v0.45.0
	github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.15.0
	github.com/prometheus/node_exporter v1.0.1
	github.com/prometheus/procfs v0.2.0
	github.com/shirou/gopsutil v3.21.3+incompatible
	github.com/sirupsen/logrus v1.7.0
	github.com/smartystreets/goconvey v1.6.4
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.8.0
	github.com/thejerf/suture v3.0.3+incompatible
	github.com/tidwall/gjson v1.6.8
	github.com/twinj/uuid v1.0.0
	github.com/urfave/cli v1.22.2
	github.com/yudai/umutex v0.0.0-20150817080136-18216d265c6b
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/net v0.0.0-20201224014010-6772e930b67b
	golang.org/x/oauth2 v0.0.0-20201208152858-08078c50e5b5 // indirect
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324
	golang.org/x/tools v0.0.0-20201228162255-34cd474b9958 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20201201144952-b05cb90ed32e // indirect
	google.golang.org/grpc v1.33.2
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.4.0
	helm.sh/helm/v3 v3.5.4
	k8s.io/api v0.20.4
	k8s.io/apiextensions-apiserver v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/apiserver v0.20.4
	k8s.io/cli-runtime v0.20.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.20.4
	sigs.k8s.io/controller-runtime v0.7.0
	sigs.k8s.io/yaml v1.2.0
)

// Pinned to kubernetes-1.20.0
replace (
	github.com/atcdot/gorm-bulk-upsert => github.com/goodrain/gorm-bulk-upsert v1.0.1-0.20210608013724-7e7870d16357
	github.com/coreos/etcd => github.com/coreos/etcd v3.2.31+incompatible
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/docker/docker v17.12.0-ce-rc1.0.20200916142827-bd33bbf0497b+incompatible
	github.com/godbus/dbus => github.com/godbus/dbus/v5 v5.0.4
	google.golang.org/grpc => google.golang.org/grpc v1.29.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.0
	k8s.io/apiserver => k8s.io/apiserver v0.20.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.20.0
	k8s.io/client-go => k8s.io/client-go v0.20.0
	k8s.io/code-generator => k8s.io/code-generator v0.20.0
	k8s.io/component-base => k8s.io/component-base v0.20.0
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.6.2
)
