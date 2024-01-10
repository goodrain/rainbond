package controller

import (
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/builder/sources/registry"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	"net/http"
)

// Registry -
type Registry struct {
}

// GetAllRepo 根据镜像仓库账号密码 获取所有的镜像仓库
func (r2 *Registry) GetAllRepo(w http.ResponseWriter, r *http.Request) {
	var req api_model.SearchByDomainRequest
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	c, err := registry.NewInsecure(req.Domain, req.UserName, req.Password)
	if err != nil {
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest(err.Error()))
		logrus.Errorf("get repositories error %s", err.Error())
		return
	}
	repositories, err := c.Repositories()
	if err != nil {
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest(err.Error()))
		logrus.Errorf("get repositories error %s", err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, repositories)
}

// GetTagsByRepoName 根据镜像仓库账号密码 获取镜像tags
func (r2 *Registry) GetTagsByRepoName(w http.ResponseWriter, r *http.Request) {
	var req api_model.SearchByDomainRequest
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	c, err := registry.NewInsecure(req.Domain, req.UserName, req.Password)
	if err != nil {
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest(err.Error()))
		logrus.Errorf("get tags error %s", err.Error())
		return
	}

	tags, err := c.Tags(r.URL.Query().Get("repo"))
	if err != nil {
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest(err.Error()))
		logrus.Errorf("get tags error %s", err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, tags)
}
