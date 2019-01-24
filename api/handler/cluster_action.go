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

package handler

import (
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/proxy"
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/node/nodem/client"
	"net/http"
	"strconv"
	"strings"
)

type ClusterAction struct {
	OptCfg *option.Config
}

//CreateManager create Manger
func CreateClusterManager(conf *option.Config) ClusterHandler {
	return ClusterAction{
		OptCfg: conf,
	}
}

func (c ClusterAction) GetAllocatableResources(proxy proxy.Proxy) (int64, int64, error) {
	var allCPU int64 // allocatable CPU
	var allMem int64 // allocatable memory
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/v2/nodes/rule/compute",
		c.OptCfg.NodeAPI), nil)
	if err != nil {
		return 0, 0, fmt.Errorf("error creating http request: %v", err)
	}
	resp, err := proxy.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("error getting cluster resources: %v", err)
	}
	if resp.Body != nil {
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return 0, 0, fmt.Errorf("error getting cluster resources: status code: %d; "+
				"response: %v", resp.StatusCode, resp)
		}
		type foo struct {
			List []*client.HostNode `json:"list"`
		}
		var f foo
		err = json.NewDecoder(resp.Body).Decode(&f)
		if err != nil {
			return 0, 0, fmt.Errorf("error decoding response body: %v", err)
		}

		for _, n := range f.List {
			if n.Status != "running" {
				logrus.Warningf("node %s isn't running, ignore it", n.ID)
				continue
			}
			if k := n.NodeStatus.KubeNode; k != nil {
				s := strings.Replace(k.Status.Allocatable.Cpu().String(), "m", "", -1)
				i, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return 0, 0, fmt.Errorf("error converting string to int64: %v", err)
				}
				allCPU = allCPU + i
				allMem = allMem + k.Status.Allocatable.Memory().Value()/(1024*1024)
			}
		}
	}

	return allCPU, allMem, nil
}
