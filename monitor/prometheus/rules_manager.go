package prometheus

import (
	"github.com/Sirupsen/logrus"
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"os"
	"github.com/goodrain/rainbond/cmd/monitor/option"

)

type AlertingRulesConfig struct {
	Groups []*AlertingNameConfig `yaml:"groups" json:"groups"`
}

type AlertingNameConfig struct {
	Name  string         `yaml:"name" json:"name"`
	Rules []*RulesConfig `yaml:"rules" json:"rules"`
}

type RulesConfig struct {
	Alert       string            `yaml:"alert" json:"alert"`
	Expr        string            `yaml:"expr" json:"expr"`
	For         string            `yaml:"for" json:"for"`
	Labels      map[string]string `yaml:"labels" json:"labels"`
	Annotations map[string]string `yaml:"annotations" json:"annotations"`
}

type AlertingRulesManager struct {
	RulesConfig *AlertingRulesConfig
	config           *option.Config
}

func NewRulesManager(config *option.Config) *AlertingRulesManager {
	a := &AlertingRulesManager{
		RulesConfig: &AlertingRulesConfig{
			Groups: []*AlertingNameConfig{
				&AlertingNameConfig{

					Name: "InstanceHealth",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "InstanceDown",
							Expr:        "up == 0",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "builder {{$labels.instance}} down", "description":"{{$labels.instance}} of job {{$labels.job}} has been down for more than 3 minutes"},
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
							Annotations: map[string]string{"summary": "builder unhealthy"},
						},
						&RulesConfig{
							Alert:       "BuilderTaskError",
							Expr:        "builder_exporter_builder_task_error > 30",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "Builder execution task error number is greater than 30"},
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

					Name: "EntranceHealth",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "EntranceUnHealthy",
							Expr:        "acp_entrance_exporter_health_status == 0",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "entrance unhealthy"},
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
							Expr:        "event_log_exporter_instanse_up == 0",
							For:         "3m",
							Labels:      map[string]string{},
							Annotations: map[string]string{"summary": "eventlog service down"},
						},
					},
				},
			},
		},
		config: config,
	}
	return a
}

func (a *AlertingRulesManager) LoadAlertingRulesConfig() error {
	logrus.Info("Load AlertingRules config file.")
	content, err := ioutil.ReadFile(a.config.AlertingRulesFile)
	if err != nil {
		logrus.Error("Failed to read AlertingRules config file: ", err)
		logrus.Info("Init config file by default values.")
		return nil
	}
	if err := yaml.Unmarshal(content, a); err != nil {
		logrus.Error("Unmarshal AlertingRulesConfig config string to object error.", err.Error())
		return err
	}
	logrus.Debugf("Loaded config file to memory: %+v", a)

	return nil
}

func (a *AlertingRulesManager) SaveAlertingRulesConfig() error {
	logrus.Debug("Save alerting rules config file.")

	data, err := yaml.Marshal(a)
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

func (a *AlertingRulesManager) AddRules(val AlertingNameConfig) error {
	group := a.RulesConfig.Groups
	group = append(group, &val)
	return nil
}

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
