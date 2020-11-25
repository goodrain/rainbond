package controller

import (
	"io/ioutil"
	"net/http"

	httputil "github.com/goodrain/rainbond/util/http"

	"encoding/json"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/sirupsen/logrus"
)

//RuleControllerManager controller manager
type RuleControllerManager struct {
	Rules   *prometheus.AlertingRulesManager
	Manager *prometheus.Manager
}

//NewControllerManager new controller manager
func NewControllerManager(a *prometheus.AlertingRulesManager, p *prometheus.Manager) *RuleControllerManager {
	c := &RuleControllerManager{
		Rules:   a,
		Manager: p,
	}
	return c
}

//AddRules add rule
func (c *RuleControllerManager) AddRules(w http.ResponseWriter, r *http.Request) {
	in, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
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
	c.Manager.ReloadConfig()
	httputil.ReturnSuccess(r, w, "Add rule successfully")
}

//GetRules get rules
func (c *RuleControllerManager) GetRules(w http.ResponseWriter, r *http.Request) {
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

//DelRules del rules
func (c *RuleControllerManager) DelRules(w http.ResponseWriter, r *http.Request) {
	rulesName := chi.URLParam(r, "rules_name")
	c.Rules.LoadAlertingRulesConfig()
	groupsList := c.Rules.RulesConfig.Groups
	for i, v := range groupsList {
		if v.Name == rulesName {
			groupsList = append(groupsList[:i], groupsList[i+1:]...)
			c.Rules.RulesConfig.Groups = groupsList
			c.Rules.SaveAlertingRulesConfig()
			c.Manager.ReloadConfig()
			httputil.ReturnSuccess(r, w, "successfully deleted")
			return
		}
	}
	httputil.ReturnSuccess(r, w, "")
}

//RegRules reg rules
func (c *RuleControllerManager) RegRules(w http.ResponseWriter, r *http.Request) {
	rulesName := chi.URLParam(r, "rules_name")
	in, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
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
			c.Rules.SaveAlertingRulesConfig()
			c.Manager.ReloadConfig()
			httputil.ReturnSuccess(r, w, "Update rule succeeded")
			return
		}
	}
	httputil.ReturnError(r, w, 404, "The rule to be updated does not exist")
}

//GetAllRules get all rules
func (c *RuleControllerManager) GetAllRules(w http.ResponseWriter, r *http.Request) {
	c.Rules.LoadAlertingRulesConfig()
	val := c.Rules.RulesConfig
	httputil.ReturnSuccess(r, w, val)
}
