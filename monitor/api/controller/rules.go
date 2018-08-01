package controller

import (
	"net/http"
	"io/ioutil"
	httputil "github.com/goodrain/rainbond/util/http"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/go-chi/chi"
	"encoding/json"
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
	logrus.Info("add rules")
	in, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	println(string(in))
	var RulesConfig prometheus.AlertingNameConfig

	unmarshalErr := json.Unmarshal(in, &RulesConfig)
	if unmarshalErr != nil {
		logrus.Info(unmarshalErr)
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}

	c.Rules.LoadAlertingRulesConfig()

	group := c.Rules.RulesConfig.Groups
	for _, v := range group {
		if v.Name == RulesConfig.Name {
			httputil.ReturnError(r, w, 400, "Rule already exists")
			return
		}
	}
	group = append(group, &RulesConfig)
	c.Rules.RulesConfig.Groups = group
	c.Rules.SaveAlertingRulesConfig()
	c.Manager.RestartDaemon()
	httputil.ReturnSuccess(r, w, "Add rule successfully")

}

func (c *ControllerManager) GetRules(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("get rule")
	rulesName := chi.URLParam(r, "rules_name")
	c.Rules.LoadAlertingRulesConfig()

	for _, v := range c.Rules.RulesConfig.Groups {
		if v.Name == rulesName {
			httputil.ReturnSuccess(r, w, v)
			return
		}
	}

	httputil.ReturnError(r, w, 404, "Rule does not exist")
}

func (c *ControllerManager) DelRules(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("delete rule")
	rulesName := chi.URLParam(r, "rules_name")
	c.Rules.LoadAlertingRulesConfig()
	groupsList := c.Rules.RulesConfig.Groups
	for i, v := range groupsList {
		if v.Name == rulesName {
			groupsList = append(groupsList[:i], groupsList[i+1:]...)
			c.Rules.RulesConfig.Groups = groupsList
			c.Rules.SaveAlertingRulesConfig()
			c.Manager.RestartDaemon()
			httputil.ReturnSuccess(r, w, "successfully deleted")
			return
		}
	}
	httputil.ReturnSuccess(r, w, "")
}

func (c *ControllerManager) RegRules(w http.ResponseWriter, r *http.Request) {
	rulesName := chi.URLParam(r, "rules_name")
	in, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	println(string(in))

	var RulesConfig prometheus.AlertingNameConfig

	unmarshalErr := json.Unmarshal(in, &RulesConfig)
	if unmarshalErr != nil {
		logrus.Info(unmarshalErr)
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}

	c.Rules.LoadAlertingRulesConfig()

	group := c.Rules.RulesConfig.Groups
	for i, v := range group {
		if v.Name == rulesName {
			group[i] = &RulesConfig
			c.Manager.RestartDaemon()
			httputil.ReturnSuccess(r, w, "Update rule succeeded")
			c.Rules.SaveAlertingRulesConfig()
			return
		}
	}
	httputil.ReturnError(r, w, 404, "The rule to be updated does not exist")
}

func (c *ControllerManager) GetAllRules(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("get all rule")
	c.Rules.LoadAlertingRulesConfig()
	val := c.Rules.RulesConfig
	httputil.ReturnSuccess(r, w, val)
}
