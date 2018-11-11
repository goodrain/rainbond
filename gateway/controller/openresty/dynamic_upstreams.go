// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package openresty

import (
	"bytes"
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/gateway/v1"
	"io/ioutil"
	"net/http"
)

type Upstream struct {
	Name    string
	Servers []*Server
}

type Server struct {
	Host string
	Port int32
}

func UpdateUpstreams(pools []*v1.Pool) {
	logrus.Debug("update http upstreams dynamically.")
	var upstreams []*Upstream
	for _, pool := range pools {
		upstream := &Upstream{}
		upstream.Name = pool.Name
		for _, node := range pool.Nodes {
			server := &Server{
				Host: node.Host,
				Port: node.Port,
			}
			upstream.Servers = append(upstream.Servers, server)
		}
		upstreams = append(upstreams, upstream)
	}
	updateUpstreams(upstreams)
}

func updateUpstreams(upstream []*Upstream) {
	url := "http://localhost:33333/update-upstreams" // TODO
	data, _ := json.Marshal(upstream)
	logrus.Debugf("request contest is %v", string(data))

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		logrus.Errorf("fail to update upstreams: %v", err)
	}
	defer resp.Body.Close()

	logrus.Debugf("dynamically update Upstream, status is %v", resp.Status)
	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("dynamically update Upstream, error is %v", err.Error())
	}

	logrus.Infof("dynamically update Upstream, response is %v", string(res))
}
