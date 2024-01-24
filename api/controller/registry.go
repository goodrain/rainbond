package controller

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"strings"

	"crypto/tls"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/builder/sources/registry"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/url"
)

// Registry -
type Registry struct {
}

// RepositoryTags -
type RepositoryTags struct {
	Registry   string   `json:"registry"`
	Repository string   `json:"repository"`
	Tags       []string `json:"tags"`
	Total      int      `json:"total"`
}

// CheckRegistry 根据镜像仓库账号密码 检查镜像仓库是否可用
func (r2 *Registry) CheckRegistry(w http.ResponseWriter, r *http.Request) {
	var req api_model.SearchByDomainRequest
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	parse, err := url.Parse(req.Domain)
	if err != nil {
		logrus.Errorf("parse url error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest(err.Error()))
		return
	}

	options := make([]name.Option, 0)
	if parse.Scheme == "http" {
		options = append(options, name.Insecure)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	fmt.Println(parse.Host)
	registryCfg, err := name.NewRegistry(req.Domain, options...)
	if err != nil {
		logrus.Errorf("parse registry error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest(err.Error()))
		return
	}

	ctx := context.TODO()
	_, err = transport.NewWithContext(ctx, registryCfg, &authn.Basic{
		Username: req.UserName,
		Password: req.Password,
	}, tr, []string{})
	if err != nil {
		logrus.Errorf("check registry error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest(err.Error()))
		return
	}

	httputil.ReturnSuccess(r, w, true)
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
	var regUrl string
	if strings.HasSuffix(req.Domain, "/") {
		regUrl = req.Domain + r.URL.Query().Get("repo")
	} else {
		regUrl = req.Domain + "/" + r.URL.Query().Get("repo")
	}
	repo, err := name.NewRepository(regUrl)
	if err != nil {
		logrus.Errorf("parse registry error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest(err.Error()))
		return
	}

	authenticator := authn.FromConfig(authn.AuthConfig{
		Username: req.UserName,
		Password: req.Password,
	})

	tags, err := remote.List(repo, remote.WithAuth(authenticator))
	if err != nil {
		logrus.Errorf("get tags error %s", err.Error())
		httputil.ReturnBcodeError(r, w, bcode.NewBadRequest(err.Error()))
		return
	}
	httputil.ReturnSuccess(r, w, &RepositoryTags{
		Registry:   repo.RegistryStr(),
		Repository: repo.RepositoryStr(),
		Tags:       tags,
		Total:      len(tags),
	})
}
