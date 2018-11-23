/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"io/ioutil"
	"os"
	"path"
	text_template "text/template"

	"github.com/golang/glog"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/pkg/errors"
)

var (
	defBufferSize = 65535
	//CustomConfigPath custom config file path
	CustomConfigPath = "/run/nginx/conf"
	//tmplPath Tmpl config file path
	tmplPath = "/run/nginxtmp/tmpl"
)

func init() {
	if os.Getenv("NGINX_CONFIG_TMPL") != "" {
		tmplPath = os.Getenv("NGINX_CONFIG_TMPL")
	}
}

// Template ...
type Template struct {
	tmpl *text_template.Template
	//fw   watch.FileWatcher
	bp *BufferPool
}

func NewTemplate(fileName string) (*Template, error) {
	tmplFile, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "unexpected error reading template %v", tmplFile)
	}

	// TODO change the template name
	tmpl, err := text_template.New("gateway").Funcs(funcMap).Parse(string(tmplFile))
	if err != nil {
		return nil, err
	}

	return &Template{
		tmpl: tmpl,
		bp:   NewBufferPool(defBufferSize),
	}, nil
}

// NewNginxTemplate creates a nginx configuration file(nginx.conf)
func NewNginxTemplate(data *model.Nginx, defaultNginxConf string) error {
	if e := Persist(tmplPath+"/nginx.tmpl", data, path.Dir(defaultNginxConf), path.Base(defaultNginxConf)); e != nil {
		return e
	}
	return nil
}

// NewServerTemplate creates a configuration file for the nginx server module
func NewServerTemplate(data []*model.Server, filename string) error {
	if e := Persist(tmplPath+"/servers.tmpl", data, CustomConfigPath, filename); e != nil {
		return e
	}
	return nil
}

// NewUpstreamTemplate creates a configuration file for the nginx upstream module
func NewUpstreamTemplate(data []model.Upstream, tmpl, filename string) error {
	if e := Persist(tmplPath+"/"+tmpl, data, CustomConfigPath, filename); e != nil {
		return e
	}
	return nil
}

// Persist persists the nginx configuration file to disk
func Persist(tmplFilename string, data interface{}, p string, f string) error {
	tpl, err := NewTemplate(tmplFilename)
	if err != nil {
		return err
	}

	rt, err := tpl.Write(data)
	if err != nil {
		return err
	}

	f = fmt.Sprintf("%s/%s", p, f)
	p = path.Dir(f)
	f = path.Base(f)
	if !isExists(p) {
		logrus.Debugf("mkdir %s", p)
		if e := os.MkdirAll(p, 0777); e != nil {
			return e
		}
	}

	if e := ioutil.WriteFile(p+"/"+f, rt, 0666); e != nil {
		return e
	}

	return nil
}

func (t *Template) Write(conf interface{}) ([]byte, error) {
	tmplBuf := t.bp.Get() // TODO 为什么用buffer, 是怎样实现的
	defer t.bp.Put(tmplBuf)

	outCmdBuf := t.bp.Get()
	defer t.bp.Put(outCmdBuf)

	if glog.V(3) { // TODO
		b, err := json.Marshal(conf)
		if err != nil {
			glog.Errorf("unexpected error: %v", err)
		}
		glog.Infof("NGINX configuration: %v", string(b))
	}

	err := t.tmpl.Execute(tmplBuf, conf)
	if err != nil {
		return nil, err
	}

	return tmplBuf.Bytes(), nil
}

func isExists(f string) bool {
	_, err := os.Stat(f)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}
