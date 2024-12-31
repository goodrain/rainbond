module github.com/goodrain/rainbond

go 1.19

require (
	cuelang.org/go v0.2.2
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/aliyun/aliyun-oss-go-sdk v2.1.5+incompatible
	github.com/atcdot/gorm-bulk-upsert v1.0.0
	github.com/aws/aws-sdk-go v1.44.116
	github.com/barnettZQG/gotty v1.0.1-0.20200904091006-a0a1f7d747dc
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bitly/go-simplejson v0.5.0
	github.com/bluebreezecf/opentsdb-goclient v0.0.0-20190921120552-796138372df3
	github.com/containerd/containerd v1.6.3
	github.com/containerd/typeurl v1.0.2
	github.com/crossplane/crossplane-runtime v0.14.1-0.20210722005935-0b469fcc77cd
	github.com/docker/cli v20.10.16+incompatible
	github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/docker v20.10.16+incompatible
	github.com/docker/go-metrics v0.0.1
	github.com/docker/go-units v0.4.0 // indirect
	github.com/docker/libcompose v0.4.1-0.20190808084053-143e0f3f1ab9
	github.com/eapache/channels v1.1.0
	github.com/emicklei/go-restful v2.15.0+incompatible
	github.com/envoyproxy/go-control-plane v0.10.3
	github.com/fatih/color v1.15.0 // indirect
	github.com/fatih/structs v1.1.0
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-chi/render v1.0.1
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/go-playground/validator/v10 v10.14.0
	github.com/go-sql-driver/mysql v1.7.1
	github.com/gofrs/flock v0.8.1
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.0.0
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.3
	github.com/goodrain/rainbond-oam v0.0.0-20221115150510-dd668a6d6765
	github.com/goodrain/rainbond-operator v1.3.1-0.20210401055914-f8fe4bf89a21
	github.com/gorilla/websocket v1.5.0
	github.com/gosuri/uitable v0.0.4
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/jinzhu/gorm v1.9.16
	github.com/json-iterator/go v1.1.12
	github.com/kr/pty v1.1.8
	github.com/mattn/go-runewidth v0.0.9
	github.com/melbahja/got v0.5.0
	github.com/mitchellh/go-ps v1.0.0
	github.com/mitchellh/go-wordwrap v1.0.0
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/ncabatoff/process-exporter v0.7.1
	github.com/oam-dev/kubevela v1.1.0-alpha.4.0.20210625105426-e176fcfc56f0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.27.10
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pebbe/zmq4 v1.2.11
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.1
	github.com/pquerna/ffjson v0.0.0-20190930134022-aa0246cd15f7
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.45.0
	github.com/prometheus-operator/prometheus-operator/pkg/client v0.45.0
	github.com/prometheus/client_golang v1.16.0
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0
	github.com/prometheus/procfs v0.11.1 // indirect
	github.com/shirou/gopsutil/v4 v4.24.9
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.9.0
	github.com/thejerf/suture v3.0.3+incompatible
	github.com/tidwall/gjson v1.9.3
	github.com/twinj/uuid v1.0.0
	github.com/urfave/cli v1.22.4
	golang.org/x/crypto v0.14.0
	golang.org/x/net v0.17.0
	golang.org/x/sys v0.25.0
	golang.org/x/time v0.3.0
	google.golang.org/grpc v1.57.0
	google.golang.org/protobuf v1.31.0
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.4.0
	helm.sh/helm/v3 v3.8.2
	k8s.io/api v0.28.3
	k8s.io/apiextensions-apiserver v0.28.3
	k8s.io/apimachinery v0.28.3
	k8s.io/apiserver v0.28.3
	k8s.io/cli-runtime v0.26.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.28.1
	k8s.io/component-base v0.28.3
	k8s.io/cri-api v0.23.1
	k8s.io/kubernetes v1.23.12
	sigs.k8s.io/controller-runtime v0.16.1
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/apache/apisix-ingress-controller v1.7.1
	github.com/coreos/etcd v3.3.13+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/gin-gonic/gin v1.9.1
	github.com/google/go-containerregistry v0.5.1
	github.com/google/uuid v1.3.0
	github.com/grafana/pyroscope-go v1.2.0
	github.com/helm/helm v2.17.0+incompatible
	github.com/openkruise/kruise-api v1.4.0-rc.0
	golang.org/x/sync v0.3.0
	k8s.io/klog/v2 v2.100.1
	k8s.io/metrics v0.26.4
	kubevirt.io/api v1.1.0
	kubevirt.io/client-go v1.1.0
	sigs.k8s.io/gateway-api v0.8.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/BurntSushi/toml v1.0.0 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/semver/v3 v3.1.1 // indirect
	github.com/Masterminds/sprig/v3 v3.2.2 // indirect
	github.com/Masterminds/squirrel v1.5.2 // indirect
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/Microsoft/hcsshim v0.9.4 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/cncf/udpa/go v0.0.0-20220112060539-c52dc94e7fbe // indirect
	github.com/cncf/xds/go v0.0.0-20230105202645-06c439db220b // indirect
	github.com/cockroachdb/apd/v2 v2.0.1 // indirect
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/containerd/console v1.0.3 // indirect
	github.com/containerd/continuity v0.3.0 // indirect
	github.com/containerd/fifo v1.0.0 // indirect
	github.com/containerd/ttrpc v1.1.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/coreos/prometheus-operator v0.41.1 // indirect
	github.com/creack/pty v1.1.11 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/ebitengine/purego v0.8.0 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v0.9.1 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-gorp/gorp/v3 v3.0.2 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.20.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/grafana/pyroscope-go/godeltaprof v0.1.8 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jmoiron/sqlx v1.3.4 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v0.0.0-20191119172530-79f836b90111 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/lib/pq v1.10.4 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/moby/term v0.0.0-20220808134915-39b0c02b01ae // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/mozillazg/go-pinyin v0.18.0 // indirect
	github.com/mpvl/unique v0.0.0-20150818121801-cbe035fff7de // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/ncabatoff/go-seq v0.0.0-20180805175032-b08ef85ed833 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/opencontainers/runc v1.1.4 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/opencontainers/selinux v1.10.1 // indirect
	github.com/openshift/api v0.0.0-20230503133300-8bbcb7ca7183 // indirect
	github.com/openshift/client-go v0.0.0-20210112165513-ebc401615f47 // indirect
	github.com/openshift/custom-resource-status v1.1.2 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pelletier/go-toml/v2 v2.0.9 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/rubenv/sql-migrate v1.1.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/src-d/gcfg v1.4.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.starlark.net v0.0.0-20200306205701-8dd3e2ee1dd5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.25.0 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/mod v0.12.0 // indirect
	golang.org/x/term v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230815205213-6bfd019c3878 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230815205213-6bfd019c3878 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/src-d/go-billy.v4 v4.3.2 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/gengo v0.0.0-20230829151522-9cce18d56c01 // indirect
	k8s.io/helm v2.17.0+incompatible // indirect
	k8s.io/kube-openapi v0.0.0-20230717233707-2695361300d9 // indirect
	k8s.io/utils v0.0.0-20230505201702-9f6742963106 // indirect
	kubevirt.io/containerized-data-importer-api v1.57.0-alpha1 // indirect
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.0.0-20220329064328-f3cc58c6ed90 // indirect
	oras.land/oras-go v1.1.1 // indirect
	sigs.k8s.io/kustomize/api v0.12.1 // indirect
	sigs.k8s.io/kustomize/kyaml v0.13.9 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.3.0 // indirect
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20201106050909-4977a11b4351 // indirect
	github.com/klauspost/compress v1.17.8 // indirect
	github.com/mattn/go-sqlite3 v1.14.17 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.1
	github.com/spf13/cobra v1.7.0 // indirect
	github.com/xanzy/ssh-agent v0.3.0 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	golang.org/x/oauth2 v0.8.0 // indirect
	golang.org/x/tools v0.12.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/genproto v0.0.0-20230815205213-6bfd019c3878 // indirect
	k8s.io/kubectl v0.24.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
)

// Pinned to kubernetes-1.23.12
replace (
	github.com/atcdot/gorm-bulk-upsert => github.com/goodrain/gorm-bulk-upsert v1.0.1-0.20210608013724-7e7870d16357
	github.com/containerd/containerd => github.com/containerd/containerd v1.5.13
	github.com/crossplane/crossplane-runtime => github.com/crossplane/crossplane-runtime v0.11.0
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/docker/docker v20.10.2+incompatible
	github.com/envoyproxy/go-control-plane => github.com/envoyproxy/go-control-plane v0.9.5
	github.com/godbus/dbus => github.com/godbus/dbus/v5 v5.0.4
	github.com/goodrain/rainbond-oam => github.com/goodrain/rainbond-oam v0.0.0-20241120151145-067f8a6d718a
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.12.1
	github.com/prometheus/common => github.com/prometheus/common v0.15.0
	github.com/prometheus/procfs => github.com/prometheus/procfs v0.7.3
	google.golang.org/grpc => google.golang.org/grpc v1.27.1
	helm.sh/helm/v3 => helm.sh/helm/v3 v3.9.0
	k8s.io/api => k8s.io/api v0.26.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.26.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.26.4
	k8s.io/apiserver => k8s.io/apiserver v0.26.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.26.4
	k8s.io/client-go => k8s.io/client-go v0.26.4
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.26.4
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.26.4
	k8s.io/code-generator => k8s.io/code-generator v0.26.4
	k8s.io/component-base => k8s.io/component-base v0.26.4
	k8s.io/component-helpers => k8s.io/component-helpers v0.26.4
	k8s.io/controller-manager => k8s.io/controller-manager v0.26.4
	k8s.io/cri-api => k8s.io/cri-api v0.26.4
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.26.4
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.26.4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.26.4
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20221012153701-172d655c2280
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.26.4
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.26.4
	k8s.io/kubectl => k8s.io/kubectl v0.26.4
	k8s.io/kubelet => k8s.io/kubelet v0.26.4
	k8s.io/kubernetes => k8s.io/kubernetes v1.24.1
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.26.4
	k8s.io/metrics => k8s.io/metrics v0.26.4
	k8s.io/mount-utils => k8s.io/mount-utils v0.26.4
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.26.4
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.26.4
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client => sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.0.24
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.14.7
)
