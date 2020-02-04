package controller

import (
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/handler"
	api_model "github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
)

// CleanEtcd clean etcd without response, ignore clean success or not
func CleanEtcd(w http.ResponseWriter, r *http.Request) {
	var req api_model.EtcdCleanReq
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		logrus.Error("parse etcdclean req failed")
		return
	}

	if len(req.Keys) == 0 {
		return
	}

	h := handler.GetEtcdHandler()
	h.CleanEtcd(req.Keys)
}
