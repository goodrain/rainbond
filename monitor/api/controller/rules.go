package controller

import (
	"net/http"
	"io/ioutil"
	httputil "github.com/goodrain/rainbond/util/http"

	"github.com/Sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/go-chi/chi"
)

type ControllerManager struct {
	Rules   *prometheus.AlertingRulesManager
	Manager *prometheus.Manager
}

func NewControllerManager(a *prometheus.AlertingRulesManager, p *prometheus.Manager) *ControllerManager {
	c := &ControllerManager{
		Rules:   a,
		Manager: p,
	}
	return c
}

func (c *ControllerManager) AddRules(w http.ResponseWriter, r *http.Request) {
	in, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}

	var RulesConfig prometheus.AlertingNameConfig

	err = ioutil.WriteFile("/etc/prometheus/cache_rule.yml", in, 0644)
	if err != nil {
		logrus.Error(err.Error())
	}

	content, err := ioutil.ReadFile("/etc/prometheus/cache_rule.yml")
	if err != nil {
		logrus.Error( err)

	}

	if err := yaml.Unmarshal(content, &RulesConfig); err != nil {
		logrus.Error("Unmarshal prometheus alerting rules config string to object error.", err.Error())
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	c.Rules.RulesConfig.LoadAlertingRulesConfig()

	group := c.Rules.RulesConfig.Groups
	for _,v := range group{
		if v.Name == RulesConfig.Name{
			httputil.ReturnError(r, w, 400, "Rule already exists")
			return
		}
	}

	group = append(group, &RulesConfig)
	c.Rules.RulesConfig.SaveAlertingRulesConfig()
	c.Manager.RestartDaemon()
	httputil.ReturnSuccess(r, w, "Add rule successfully")

}

func (c *ControllerManager) GetRules(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("get rule")
	rulesName := chi.URLParam(r, "rules_name")
	c.Rules.RulesConfig.LoadAlertingRulesConfig()

	for _, v := range c.Rules.RulesConfig.Groups {
		if v.Name == rulesName {
			res := v.Rules
			httputil.ReturnSuccess(r, w, res)
			return
		}
	}

	httputil.ReturnError(r, w, 400, "Rule does not exist")
}

func (c *ControllerManager) DelRules(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("delete rule")
	rulesName := chi.URLParam(r, "rules_name")
	c.Rules.RulesConfig.LoadAlertingRulesConfig()
	groupsList := c.Rules.RulesConfig.Groups
	for i, v := range groupsList {
		if v.Name == rulesName {
			groupsList = append(groupsList[:i],groupsList[i+1:]...)
			httputil.ReturnSuccess(r, w, "successfully deleted")
			return
		}
	}
	httputil.ReturnSuccess(r, w, "")
}


func (c *ControllerManager) RegRules(w http.ResponseWriter, r *http.Request) {
	in, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}

	var RulesConfig prometheus.AlertingNameConfig

	err = ioutil.WriteFile("/etc/prometheus/cache_rule.yml", in, 0644)
	if err != nil {
		logrus.Error(err.Error())
	}

	content, err := ioutil.ReadFile("/etc/prometheus/cache_rule.yml")
	if err != nil {
		logrus.Error( err)

	}

	if err := yaml.Unmarshal(content, &RulesConfig); err != nil {
		logrus.Error("Unmarshal prometheus alerting rules config string to object error.", err.Error())
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	c.Rules.RulesConfig.LoadAlertingRulesConfig()

	group := c.Rules.RulesConfig.Groups
	for i,v := range group{
		if v.Name == RulesConfig.Name{
			group[i] = &RulesConfig
			httputil.ReturnSuccess(r, w, "Update rule succeeded")
			return
		}
	}
	httputil.ReturnError(r, w, 400,"The rule to be updated does not exist")
}
