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
	"github.com/golang/glog"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	text_template "text/template"
)

const (
	defBufferSize = 65535
	ConfPath      = "/export/servers/nginx/conf"
	tmplPath      = "/export/servers/nginx/tmpl"
)

func init() {

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
func NewNginxTemplate(data *model.Nginx) error {
	if e := Persist(tmplPath+"/nginx.tmpl", data, ConfPath, "nginx.conf"); e != nil {
		return e
	}
	return nil
}

// NewNginxTemplate creates a configuration file for the nginx http module
func NewHttpTemplate(data *model.Http, filename string) error {
	if e := Persist(tmplPath+"/http.tmpl", data, ConfPath, filename); e != nil {
		return e
	}
	return nil
}

// NewNginxTemplate creates a configuration file for the nginx stream module
func NewStreamTemplate(data *model.Stream, filename string) error {
	if e := Persist(tmplPath+"/stream.tmpl", data, ConfPath, filename); e != nil {
		return e
	}
	return nil
}

// NewNginxTemplate creates a configuration file for the nginx server module
func NewServerTemplate(data []*model.Server, filename string) error {
	if e := Persist(tmplPath+"/servers.tmpl", data, ConfPath, filename); e != nil {
		return e
	}
	return nil
}

// NewNginxTemplate creates a configuration file for the nginx upstream module
func NewUpstreamTemplate(data []model.Upstream, tmpl, filename string) error {
	if e := Persist(tmplPath+"/"+tmpl, data, ConfPath, filename); e != nil {
		return e
	}
	return nil
}

// Persist persists the nginx configuration file to disk
func Persist(tmplFilename string, data interface{}, path string, filename string) error {
	tpl, err := NewTemplate(tmplFilename)
	if err != nil {
		return err
	}

	rt, err := tpl.Write(data)
	if err != nil {
		return err
	}

	if e := os.MkdirAll(path, 0777); e != nil {
		return e
	}

	if e := ioutil.WriteFile(path+"/"+filename, rt, 0666); e != nil {
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
