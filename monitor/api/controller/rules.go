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

	if err := yaml.Unmarshal(in, &RulesConfig); err != nil {
		logrus.Error("Unmarshal prometheus alerting rules config string to object error.", err.Error())
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}

	c.Rules.RulesConfig.LoadAlertingRulesConfig()
	c.Rules.RulesConfig.AddRules(RulesConfig)
	c.Rules.RulesConfig.SaveAlertingRulesConfig()
	c.Manager.RestartDaemon()

}

func (c *ControllerManager) GetRules(w http.ResponseWriter, r *http.Request) {
	rulesName := chi.URLParam(r, "rules_name")
	c.Rules.RulesConfig.LoadAlertingRulesConfig()

	for _, v := range c.Rules.RulesConfig.Groups {
		if v.Name == rulesName {
			res := v.Rules
			httputil.ReturnSuccess(r, w, res)
		}
	}

	httputil.ReturnSuccess(r, w, "Did not find the rule")

}
