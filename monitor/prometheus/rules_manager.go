package prometheus

import (
	"io/ioutil"
	"os"

	"github.com/goodrain/rainbond/cmd/monitor/option"
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

//AlertingRulesConfig alerting rule config
type AlertingRulesConfig struct {
	Groups []*AlertingNameConfig `yaml:"groups" json:"groups"`
}

//AlertingNameConfig alerting config
type AlertingNameConfig struct {
	Name  string         `yaml:"name" json:"name"`
	Rules []*RulesConfig `yaml:"rules" json:"rules"`
}

//RulesConfig rule config
type RulesConfig struct {
	Alert       string            `yaml:"alert" json:"alert"`
	Expr        string            `yaml:"expr" json:"expr"`
	For         string            `yaml:"for" json:"for"`
	Labels      map[string]string `yaml:"labels" json:"labels"`
	Annotations map[string]string `yaml:"annotations" json:"annotations"`
}

//AlertingRulesManager alerting rule manage
type AlertingRulesManager struct {
	RulesConfig *AlertingRulesConfig
	config      *option.Config
}

//NewRulesManager new rule manager
func NewRulesManager(config *option.Config) *AlertingRulesManager {
	region := os.Getenv("REGION_NAME")
	if region == "" {
		region = "default"
	}
	getCommonLabels := func(labels ...map[string]string) map[string]string {
		var resultLabel = make(map[string]string)
		for _, l := range labels {
			for k, v := range l {
				resultLabel[k] = v
			}
		}
		resultLabel["Alert"] = "Rainbond"
		resultLabel["Region"] = region
		return resultLabel
	}
	getseverityLabels := func(severity string) map[string]string {
		return map[string]string{
			"Alert":    "Rainbond",
			"severity": severity,
			"Region":   region,
		}
	}
	a := &AlertingRulesManager{
		RulesConfig: &AlertingRulesConfig{
			Groups: []*AlertingNameConfig{
				{
					Name: "GatewayHealth",
					Rules: []*RulesConfig{
						{
							Alert:  "GatewayDown",
							Expr:   "absent(up{job=\"gateway\"}) or up{job=\"gateway\"}==0",
							For:    "20s",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "网关组件: {{ $labels.instance }} 出现故障",
								"summary":     "网关组件故障",
							},
						},
						{
							Alert:  "RequestSizeTooMuch",
							Expr:   "sum by (instance, host) (rate(gateway_request_size_sum[5m])) > 1024*1024*10",
							For:    "20s",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "5分钟内, 请求http域名: {{ $labels.host }} 的请求大小高于10M,为 {{ humanize $value }} M",
								"summary":     "请求流量过大",
							},
						},
						{
							Alert:  "ResponseSizeTooMuch",
							Expr:   "sum by (instance, host) (rate(gateway_response_size_sum[5m])) > 1024*1024*10",
							For:    "20s",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "5分钟内, http域名: {{ $labels.host }} 的响应大小高于10M,为 {{ humanize $value }} M",
								"summary":     "响应流量过大",
							},
						},
						{
							Alert:       "RequestMany",
							Expr:        "rate(gateway_requests[5m]) > 200",
							For:         "10s",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"description": "5分钟内, http域名: {{ $labels.host }} 的请求数高于200,为 {{ humanize $value }}"},
						},
						{
							Alert:       "FailureRequestMany",
							Expr:        "rate(gateway_requests{status=~\"5..\"}[5m]) > 5",
							For:         "10s",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"description": "5分钟内, http域名: {{ $labels.host }} 的失败请求数高于5个,为 {{ humanize $value }} 个,状态码为[5..]"},
						},
					},
				},
				{
					Name: "BuilderHealth",
					Rules: []*RulesConfig{
						{
							Alert:  "BuilderDown",
							Expr:   "absent(up{component=\"builder\"}) or up{component=\"builder\"}==0",
							For:    "1m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "构建组件(rbd-chaos) {{ $labels.instance }} 出现故障",
								"summary":     "构建组件(rbd-chaos)故障",
							},
						},
						{
							Alert:       "BuilderUnhealthy",
							Expr:        "builder_exporter_health_status == 0",
							For:         "3m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"description": "构建组件(rbd-chaos) {{ $labels.instance }} 不健康"},
						},
						{
							Alert:       "BuilderTaskError",
							Expr:        "builder_exporter_builder_current_concurrent_task == builder_exporter_builder_max_concurrent_task",
							For:         "20s",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"summary": "构建组件(rbd-chaos)并发执行任务数量达到最大,负载过高"},
						},
					},
				},
				{
					Name: "WorkerHealth",
					Rules: []*RulesConfig{
						{
							Alert:  "WorkerDown",
							Expr:   "absent(worker_exporter_health_status) or worker_exporter_health_status==0",
							For:    "5m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "rbd-worker组件 {{ $labels.instance }} 出现故障",
								"summary":     "rbd-worker组件故障",
							},
						},
						{
							Alert:  "WorkerUnhealthy",
							Expr:   "app_resource_exporter_health_status == 0",
							For:    "5m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"summary":     "rbd-worker组件不健康",
								"description": "rbd-worker组件 {{ $labels.instance }} 不健康",
							},
						},
						{
							Alert:  "WorkerTaskError",
							Expr:   "app_resource_exporter_worker_task_error > 50",
							For:    "5m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "rbd-worker组件 {{ $labels.instance }} 执行任务错误数大于50",
							},
						},
					},
				},
				{
					Name: "MqHealth",
					Rules: []*RulesConfig{
						{
							Alert:  "MqDown",
							Expr:   "absent(up{component=\"mq\"}) or up{component=\"mq\"}==0",
							For:    "2m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "消息队列组件(rbd-mq) {{ $labels.instance }} 出现故障",
								"summary":     "消息队列组件(rbd-mq)出现故障",
							},
						},
						{
							Alert:       "MqUnhealthy",
							Expr:        "acp_mq_exporter_health_status == 0",
							For:         "3m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"summary": "消息队列组件(rbd-mq)不健康"},
						},
						{
							Alert:  "MqMessageQueueBlock",
							Expr:   "acp_mq_queue_message_number > 0",
							For:    "1m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"summary":     "消息队列阻塞",
								"description": "消息 {{ $labels.topic }} 阻塞, 消息大小为 {{ humanize $value }}",
							},
						},
					},
				},
				{
					Name: "EventlogHealth",
					Rules: []*RulesConfig{
						{
							Alert:       "EventLogUnhealthy",
							Expr:        "event_log_exporter_health_status == 0",
							For:         "3m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"summary": "rbd-eventlog组件 {{ $labels.instance }} 不健康"},
						},
						{
							Alert:  "EventLogDown",
							Expr:   "absent(up{component=\"eventlog\"}) or up{component=\"eventlog\"}==0",
							For:    "3m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "rbd-eventlog组件 {{ $labels.instance }} 出现故障",
								"summary":     "rbd-eventlog组件出现故障",
							},
						},
					},
				},
				{
					Name: "WebcliHealth",
					Rules: []*RulesConfig{
						{
							Alert:  "WebcliDown",
							Expr:   "absent(up{component=\"webcli\"}) or up{component=\"webcli\"}==0",
							For:    "20s",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "rbd-webcli组件 {{ $labels.instance }} 出现故障",
								"summary":     "rbd-webcli组件故障",
							},
						},
						{
							Alert:       "WebcliUnhealthy",
							Expr:        "webcli_exporter_health_status == 0",
							For:         "3m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"summary": "rbd-webcli 组件 {{ $labels.instance }} 不健康"},
						},
						{
							Alert:       "WebcliUnhealthy",
							Expr:        "rate(webcli_exporter_execute_command_failed[5m]) > 5",
							For:         "3m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"summary": "5分钟内, rbd-webcli组件执行命令错误数大于5个"},
						},
					},
				},
				{
					Name: "NodeHealth",
					Rules: []*RulesConfig{
						{
							Alert:  "NodeDown",
							Expr:   "absent(up{component=\"rbd_node\"}) or up{component=\"rbd_node\"} == 0",
							For:    "30s",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "rbd_node组件 {{ $labels.instance }} 出现故障",
								"summary":     "rbd_node组件故障",
							},
						},
						{
							Alert:       "HighCpuUsageOnNode",
							Expr:        "sum by(instance) (rate(process_cpu_seconds_total[5m])) * 100 > 85",
							For:         "5m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"description": "5分钟内, 节点 {{ $labels.instance }} 使用的CPU资源高于85%. CPU使用量为 {{ humanize $value }}%", "summary": "CPU占用率过高警告"},
						},
						{
							Alert:       "HighLoadOnNode",
							Expr:        "sum(node_load5) by(instance) > count by(instance) (count by(job, instance, cpu) (node_cpu)) * 0.7",
							For:         "5m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"description": "节点 {{ $labels.instance }} 正处于高负载状态. 5分钟负载量为 {{ humanize $value}}", "summary": "节点高负载警告"},
						},
						{
							Alert:       "InodeFreerateLow",
							Expr:        "node_filesystem_files_free{fstype=~\"ext4|xfs\"} / node_filesystem_files{fstype=~\"ext4|xfs\"} < 0.3",
							For:         "5m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"description": "节点 {{ $labels.instance }} 上 inode 剩余可用率过低, 当前可用率为 {{ humanize $value }}%"},
						},
						{
							Alert:       "HighRootdiskUsageOnNode",
							Expr:        "(node_filesystem_size{mountpoint='/'} - node_filesystem_free{mountpoint='/'}) * 100 / node_filesystem_size{mountpoint='/'} > 80",
							For:         "5m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"description": "磁盘使用率高于 80%, 当前使用率为 {{ humanize $value }}%. 被使用磁盘的挂载点为 {{ $labels.mountpoint }}", "summary": "根分区磁盘使用率过高警告"},
						},
						{
							Alert:       "HighDockerdiskUsageOnNode",
							Expr:        "(node_filesystem_size{mountpoint='/var/lib/docker'} - node_filesystem_free{mountpoint='/var/lib/docker'}) * 100 / node_filesystem_size{mountpoint='/var/lib/docker'} > 80",
							For:         "5m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"description": "磁盘使用率高于 80%, 当前使用率为 {{ humanize $value }}%. 被使用磁盘的挂载点为 {{ $labels.mountpoint }}", "summary": "Docker分区磁盘使用率过高警告"},
						},
						{
							Alert:       "HighMemoryUsageOnNode",
							Expr:        "((node_memory_MemTotal - node_memory_MemAvailable) / node_memory_MemTotal) * 100 > 80",
							For:         "5m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"description": "节点 {{ $labels.instance }} 使用内存过高. 内存使用率大概为 {{ humanize $value}}%", "summary": "内存使用率过高警告"},
						},
						{
							Alert:       "StorageFull",
							Expr:        "(node_filesystem_size{mountpoint=\"/grdata\"} - node_filesystem_free{mountpoint=\"/grdata\"}) * 100 / node_filesystem_size{mountpoint=\"/grdata\"} > 80",
							For:         "1m",
							Labels:      getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{"description": "节点 {{ $labels.instance }} 上的共享存储空间已经使用80%", "summary": "共享存储使用率过高警告"},
						},
					},
				},
				{
					Name: "ClusterHealth",
					Rules: []*RulesConfig{
						{
							Alert:  "InsufficientClusteMemoryResources",
							Expr:   "max(rbd_api_exporter_cluster_memory_total) - max(sum(namespace_resource_memory_request) by (instance)) < 2048",
							For:    "2m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "集群剩余调度内存为 {{ humanize $value }} MB, 不足2048MB",
								"summary":     "集群内存资源不足",
							},
						},
						{
							Alert:  "InsufficientClusteCPUResources",
							Expr:   "max(rbd_api_exporter_cluster_cpu_total) - max(sum(namespace_resource_cpu_request) by (instance)) < 500",
							For:    "2m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "集群剩余调度cpu资源为 {{ humanize $value }}, 不足500m",
								"summary":     "集群cpu资源不足",
							},
						},
						{
							Alert:  "InsufficientTenantResources",
							Expr:   "sum(rbd_api_exporter_tenant_memory_limit) by(namespace) - sum(namespace_resource_memory_request)by (namespace) < sum(rbd_api_exporter_tenant_memory_limit) by(namespace) *0.2 and sum(rbd_api_exporter_tenant_memory_limit) by(namespace) > 0",
							For:    "2m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "租户剩余可用内存容量为 {{ humanize $value }} MB, 不足限制的20%",
								"summary":     "租户内存资源不足",
							},
						},
					},
				},
				{
					Name: "EtcdHealth",
					Rules: []*RulesConfig{
						{
							Alert:  "EtcdDown",
							Expr:   "absent(up{component=\"etcd\"}) or up{component=\"etcd\"}==0",
							For:    "1m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "etcd组件 {{ $labels.instance }} 出现故障",
								"summary":     "etcd组件故障",
							},
						},
						{
							Alert:  "EtcdLoseLeader",
							Expr:   "etcd_server_has_leader == 0",
							For:    "1m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "etcd组件 {{ $labels.instance }} 丢失leader",
								"summary":     "etcd组件丢失leader",
							},
						},
						{
							Alert:  "InsufficientMembers",
							Expr:   "count(up{job=\"etcd\"} == 0) > (count(up{job=\"etcd\"}) / 2 - 1)",
							For:    "1m",
							Labels: getseverityLabels("critical"),
							Annotations: map[string]string{
								"description": "警告: 如果再有一个etcd节点故障，集群将不可用",
								"summary":     "etcd集群可用节点不足警告",
							},
						},
						{
							Alert:  "HighNumberOfLeaderChanges",
							Expr:   "increase(etcd_server_leader_changes_seen_total{job=\"etcd\"}[1h]) > 3",
							For:    "1m",
							Labels: getseverityLabels("warning"),
							Annotations: map[string]string{
								"description": "etcd实例 {{ $labels.instance }} leader最近一小时发生的变更次数:{{ $value }}",
								"summary":     "etcd集群中出现大量的leader变更",
							},
						},
						{
							Alert:  "HighNumberOfFailedGRPCRequests",
							Expr:   "sum(rate(etcd_grpc_requests_failed_total{job=\"etcd\"}[5m])) BY (grpc_method) / sum(rate(etcd_grpc_total{job=\"etcd\"}[5m])) BY (grpc_method) > 0.05",
							For:    "5m",
							Labels: getseverityLabels("critical"),
							Annotations: map[string]string{
								"description": "通过grpc方式 {{ $labels.grpc_method }}, 请求etcd节点: {{ $labels.instance}}, 请求失败数大于0.05,失败数为: {{ $value }}",
								"summary":     "ETCD grpc失败请求大于0.05",
							},
						},
						{
							Alert:  "HighNumberOfFailedHTTPRequests",
							Expr:   "sum(rate(etcd_http_failed_total{job=\"etcd\"}[5m])) BY (method) / sum(rate(etcd_http_received_total{job=\"etcd\"}[5m]))BY (method) > 0.05",
							For:    "1m",
							Labels: getseverityLabels("critical"),
							Annotations: map[string]string{
								"description": "etcd节点 {{ $labels.instance }}，http请求方法 {{ $labels.method }}, 失败次数大于0.05,为 {{ $value }}",
								"summary":     "etcd 1分钟内 HTTP 请求失败数大于0.05",
							},
						},
						{
							Alert:  "GRPCRequestsSlow",
							Expr:   "histogram_quantile(0.99, rate(etcd_grpc_unary_requests_duration_seconds_bucket[5m])) > 0.15",
							For:    "1m",
							Labels: getseverityLabels("critical"),
							Annotations: map[string]string{
								"description": "etcd节点 { $labels.instance }}, grpc查询方法 {{ $labels.grpc_method}} 太慢, 大于0.15",
								"summary":     "grpc慢查询",
							},
						},
						{
							Alert:  "DatabaseSpaceExceeded",
							Expr:   "etcd_mvcc_db_total_size_in_bytes/etcd_server_quota_backend_bytes > 0.80",
							For:    "1m",
							Labels: getseverityLabels("critical"),
							Annotations: map[string]string{
								"description": "etcd节点 {{ $labels.instance }}, job为 {{ $labels.job }}, 数据库空间使用率高于80%。",
								"summary":     "etcd数据库空间过度使用",
								"runbook":     "Please consider manual compaction and defrag. https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/maintenance.md",
							},
						},
					},
				},
				{
					Name: "APIHealth",
					Rules: []*RulesConfig{
						{
							Alert:  "APIDown",
							Expr:   "absent(up{job=\"rbdapi\"}) or up{job=\"rbdapi\"}==0",
							For:    "1m",
							Labels: getCommonLabels(map[string]string{"PageAlarm": "true"}),
							Annotations: map[string]string{
								"description": "rbd-api组件 {{ $labels.instance }} 出现故障",
								"summary":     "rbd-api组件故障",
							},
						},
					},
				},
			},
		},
		config: config,
	}
	return a
}

//LoadAlertingRulesConfig load alerting rule config
func (a *AlertingRulesManager) LoadAlertingRulesConfig() error {
	logrus.Info("Load AlertingRules config file.")
	content, err := ioutil.ReadFile(a.config.AlertingRulesFile)
	if err != nil {
		logrus.Error("Failed to read AlertingRules config file: ", err)
		logrus.Info("Init config file by default values.")
		return nil
	}
	if err := yaml.Unmarshal(content, a.RulesConfig); err != nil {
		logrus.Error("Unmarshal AlertingRulesConfig config string to object error.", err.Error())
		return err
	}
	logrus.Debugf("Loaded config file to memory: %+v", a)

	return nil
}

//SaveAlertingRulesConfig save alerting rule config
func (a *AlertingRulesManager) SaveAlertingRulesConfig() error {
	logrus.Info("Save alerting rules config file.")

	data, err := yaml.Marshal(a.RulesConfig)
	if err != nil {
		logrus.Error("Marshal alerting rules config to yaml error.", err.Error())
		return err
	}

	err = ioutil.WriteFile(a.config.AlertingRulesFile, data, 0644)
	if err != nil {
		logrus.Error("Write alerting rules config file error.", err.Error())
		return err
	}

	return nil
}

//AddRules add rule
func (a *AlertingRulesManager) AddRules(val AlertingNameConfig) error {
	group := a.RulesConfig.Groups
	group = append(group, &val)
	return nil
}

//InitRulesConfig init rule config
func (a *AlertingRulesManager) InitRulesConfig() {
	_, err := os.Stat(a.config.AlertingRulesFile) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return
		}
		a.SaveAlertingRulesConfig()
		return
	}
	return
}
