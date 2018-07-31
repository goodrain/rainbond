package controller

import (
	"net/http"
	"io/ioutil"
	httputil "github.com/goodrain/rainbond/util/http"

	"github.com/Sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"github.com/goodrain/rainbond/monitor/prometheus"
)

func AddRules(w http.ResponseWriter, r *http.Request) {
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

}

