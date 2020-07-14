package prometheus

import (
	"io/ioutil"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/monitor/option"
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
	a := &AlertingRulesManager{
		RulesConfig: &AlertingRulesConfig{
			Groups: []*AlertingNameConfig{
				&AlertingNameConfig{
					Name: "GatewayHealth",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "RequestMany",
							Expr:        "rate(gateway_requests[5m]) > 100",
							For:         "10s",
							Labels:      map[string]string{},
							Annotations: map[string]string{"description": "http doamin {{ $labels.host }} per-second requests more than 100"},
						},
						&RulesConfig{
							Alert:       "FailureRequestMany",
							Expr:        "rate(gateway_requests{status=~\"5..\"}[5m]) > 5",
							For:         "10s",
							Labels:      map[string]string{},
							Annotations: map[string]string{"description": "http doamin {{ $labels.host }} per-second failure requests more than 5"},
						},
					},
				},
				&AlertingNameConfig{

					Name: "BuilderHealth",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "BuilderUnhealthy",
							Expr:        "builder_exporter_health_status == 0",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"description": "builder unhealthy"},
						},
						&RulesConfig{
							Alert:       "BuilderTaskError",
							Expr:        "builder_exporter_builder_current_concurrent_task == builder_exporter_builder_max_concurrent_task",
							For:         "20s",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "The build service is performing a maximum number of tasks"},
						},
					},
				},
				&AlertingNameConfig{

					Name: "WorkerHealth",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "WorkerUnhealthy",
							Expr:        "app_resource_exporter_health_status == 0",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "worker unhealthy"},
						},
						&RulesConfig{
							Alert:       "WorkerTaskError",
							Expr:        "app_resource_exporter_worker_task_error > 50",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "worker execution task error number is greater than 50"},
						},
					},
				},
				&AlertingNameConfig{

					Name: "MqHealth",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "MqUnhealthy",
							Expr:        "acp_mq_exporter_health_status == 0",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "mq unhealthy"},
						},
						&RulesConfig{
							Alert:       "TeamTaskMany",
							Expr:        "acp_mq_dequeue_number-acp_mq_enqueue_number > 200",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "The number of tasks in the queue is greater than 200"},
						},
					},
				},
				&AlertingNameConfig{

					Name: "EventlogHealth",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "EventLogUnhealthy",
							Expr:        "event_log_exporter_health_status == 0",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "eventlog unhealthy"},
						},
						&RulesConfig{
							Alert:       "EventLogDown",
							Expr:        "event_log_exporter_instance_up == 0",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "eventlog service down"},
						},
					},
				},
				&AlertingNameConfig{

					Name: "WebcliHealth",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "WebcliUnhealthy",
							Expr:        "webcli_exporter_health_status == 0",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "webcli unhealthy"},
						},
						&RulesConfig{
							Alert:       "WebcliUnhealthy",
							Expr:        "rate(webcli_exporter_execute_command_failed[5m]) > 5",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "The number of errors that occurred while executing the command was greater than 5 per-second."},
						},
					},
				},
				&AlertingNameConfig{

					Name: "NodeHealth",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "high_cpu_usage_on_node",
							Expr:        "sum by(instance) (rate(process_cpu_seconds_total[5m])) * 100 > 70",
							For:         "5m",
							Labels:      map[string]string{"service": "node_cpu"},
							Annotations: map[string]string{"description": "{{ $labels.instance }} is using a LOT of CPU. CPU usage is {{ humanize $value}}%.", "summary": "HIGH CPU USAGE WARNING ON '{{ $labels.instance }}'"},
						},
						&RulesConfig{
							Alert:       "high_la_usage_on_node",
							Expr:        "node_load5 > 5",
							For:         "5m",
							Labels:      map[string]string{"service": "node_load5"},
							Annotations: map[string]string{"description": "{{ $labels.instance }} has a high load average. Load Average 5m is {{ humanize $value}}.", "summary": "HIGH LOAD AVERAGE WARNING ON '{{ $labels.instance }}'"},
						},
						&RulesConfig{
							Alert:       "high_rootdisk_usage_on_node",
							Expr:        "(node_filesystem_size{mountpoint='/'} - node_filesystem_free{mountpoint='/'}) * 100 / node_filesystem_size{mountpoint='/'} > 75",
							For:         "5m",
							Labels:      map[string]string{"service": "high_rootdisk_usage_on_node"},
							Annotations: map[string]string{"description": "More than 75% of disk used. Disk usage {{ humanize $value }} mountpoint {{ $labels.mountpoint }}%.", "summary": "LOW DISK SPACE WARING:NODE '{{ $labels.instance }}"},
						},
						&RulesConfig{
							Alert:       "high_dockerdisk_usage_on_node",
							Expr:        "(node_filesystem_size{mountpoint='/var/lib/docker'} - node_filesystem_free{mountpoint='/var/lib/docker'}) * 100 / node_filesystem_size{mountpoint='/var/lib/docker'} > 75",
							For:         "5m",
							Labels:      map[string]string{"service": "high_dockerdisk_usage_on_node"},
							Annotations: map[string]string{"description": "More than 75% of disk used. Disk usage {{ humanize $value }} mountpoint {{ $labels.mountpoint }}%.", "summary": "LOW DISK SPACE WARING:NODE '{{ $labels.instance }}"},
						},
						&RulesConfig{
							Alert:       "high_memory_usage_on_node",
							Expr:        "((node_memory_MemTotal - node_memory_MemAvailable) / node_memory_MemTotal) * 100 > 80",
							For:         "5m",
							Labels:      map[string]string{"service": "node_memory"},
							Annotations: map[string]string{"description": "{{ $labels.instance }} is using a LOT of MEMORY. MEMORY usage is over {{ humanize $value}}%.", "summary": "HIGH MEMORY USAGE WARNING TASK ON '{{ $labels.instance }}'"},
						},
					},
				},
				&AlertingNameConfig{

					Name: "ClusterHealth",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "cluster_node_unhealth",
							Expr:        "rainbond_cluster_node_health != 0",
							For:         "3m",
							Labels:      map[string]string{"service": "cluster_node_unhealth"},
							Annotations: map[string]string{"description": "cluster node {{ $labels.node_ip }} is unhealth"},
						},
						&RulesConfig{
							Alert:       "cluster_kube_node_unhealth",
							Expr:        "rainbond_cluster_component_health{component=\"KubeNodeReady\"} != 0",
							For:         "3m",
							Labels:      map[string]string{"service": "component_unhealth"},
							Annotations: map[string]string{"description": "kubernetes cluster node {{ $labels.node_ip }} is unhealth"},
						},
						&RulesConfig{
							Alert:       "rainbond_cluster_collector_duration_seconds_timeout",
							Expr:        "rainbond_cluster_collector_duration_seconds > 10",
							For:         "3m",
							Labels:      map[string]string{"service": "cluster_collector"},
							Annotations: map[string]string{"description": "Cluster collector '{{ $labels.instance }}' more than 10s"},
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
	logrus.Debug("Save alerting rules config file.")

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
