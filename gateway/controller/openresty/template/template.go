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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	text_template "text/template"

	"github.com/Sirupsen/logrus"

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

//NewTemplate returns a new Template instance or an
//error if the specified template file contains errors
func NewTemplate(fileName string) (*Template, error) {
	tmplFile, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "unexpected error reading template %v", tmplFile)
	}

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

// NewServerTemplateWithCfgPath creates a configuration file for the nginx server module
func NewServerTemplateWithCfgPath(data []*model.Server, cfgPath string, filename string) error {
	if e := Persist(tmplPath+"/servers.tmpl", data, cfgPath, filename); e != nil {
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

// NewUpstreamTemplateWithCfgPath creates a configuration file for the nginx upstream module
func NewUpstreamTemplateWithCfgPath(data []*model.Upstream, tmpl, cfgPath string, filename string) error {
	if e := Persist(tmplPath+"/"+tmpl, data, cfgPath, filename); e != nil {
		return e
	}
	return nil
}

// NewUpdateUpsTemplate creates a configuration file for the nginx upstream module
func NewUpdateUpsTemplate(data []model.Upstream, tmpl, path string, filename string) error {
	if e := Persist(tmplPath+"/"+tmpl, data, path, filename); e != nil {
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
	tmplBuf := t.bp.Get()
	defer t.bp.Put(tmplBuf)

	outCmdBuf := t.bp.Get()
	defer t.bp.Put(outCmdBuf)

	if err := t.tmpl.Execute(tmplBuf, conf); err != nil {
		return nil, err
	}

	// squeezes multiple adjacent empty lines to be single
	// spaced this is to avoid the use of regular expressions
	//	cmd := exec.Command("/ingress-controller/clean-nginx-conf.sh")
	//	cmd.Stdin = tmplBuf
	//	cmd.Stdout = outCmdBuf
	//	if err := cmd.Run(); err != nil {
	//		logrus.Warningf("unexpected error cleaning template: %v", err)
	//		return tmplBuf.Bytes(), nil
	//	}
	//	return outCmdBuf.Bytes(), nil
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
