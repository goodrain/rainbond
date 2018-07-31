package prometheus

import (
	"github.com/Sirupsen/logrus"
	"io/ioutil"
	"gopkg.in/yaml.v2"
)

type AlertingRulesConfig struct {
	Groups []*AlertingNameConfig `yaml:"groups"`
}

type AlertingNameConfig struct {
	Name  string         `yaml:"name"`
	Rules []*RulesConfig `yaml:"rules"`
}

type RulesConfig struct {
	Alert  string            `yaml:"alert"`
	Expr   string            `yaml:"expr"`
	For    string            `yaml:"for"`
	Labels map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

type AlertingRulesManager struct {
	RulesConfig *AlertingRulesConfig

}

func NewRulesManager() *AlertingRulesManager {
	a:= &AlertingRulesManager{
		RulesConfig: &AlertingRulesConfig{
			Groups:[]*AlertingNameConfig{
				&AlertingNameConfig{

					Name: "test",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "MqHealth",
							Expr:        "acp_mq_exporter_health_status{job='mq'} < 1",
							For:         "2m",
							Labels:      map[string]string{"service_name": "mq"},
							Annotations: map[string]string{"summary": "unhealthy"},
						},
					},
				},
				&AlertingNameConfig{

					Name: "test2",
					Rules: []*RulesConfig{
						&RulesConfig{
							Alert:       "builderHealth",
							Expr:        "acp_mq_exporter_health_status{job='mq'} < 1",
							For:         "5m",
							Labels:      map[string]string{"service_name": "builder"},
							Annotations: map[string]string{"summary": "unhealthy"},
						},
					},
				},
			},
		},
	}
	return a
}

func (a *AlertingRulesConfig)LoadAlertingRulesConfig() error {
	logrus.Info("======>Load AlertingRules config file.")
	content, err := ioutil.ReadFile("/etc/prometheus/rules.yml")
	if err != nil {
		logrus.Error("Failed to read AlertingRules config file: ", err)
		logrus.Info("Init config file by default values.")
		return nil
	}
	if err := yaml.Unmarshal(content, a); err != nil {
		logrus.Error("Unmarshal AlertingRulesConfig config string to object error.", err.Error())
		return err
	}
	logrus.Debugf("====>Loaded config file to memory: %+v", a)

	return nil
}


func (a *AlertingRulesConfig)SaveAlertingRulesConfig() error {
	logrus.Debug("===>Save alerting rules config file.")

	data, err := yaml.Marshal(a)
	if err != nil {
		logrus.Error("Marshal alerting rules config to yaml error.", err.Error())
		return err
	}

	err = ioutil.WriteFile("/etc/prometheus/rules.yml", data, 0644)
	if err != nil {
		logrus.Error("Write alerting rules config file error.", err.Error())
		return err
	}

	return nil
}


func (a *AlertingRulesConfig) AddRules(val AlertingNameConfig) error  {
	group := a.Groups
	group = append(group, &val)
	a.Groups = group
	return nil
}