// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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
	"fmt"
	"github.com/goodrain/rainbond/api/proxy"
	"github.com/twinj/uuid"
	"strings"
	"testing"

	api_model "github.com/goodrain/rainbond/api/model"

	"github.com/sirupsen/logrus"
	"github.com/pquerna/ffjson/ffjson"
)

func TestABCService(t *testing.T) {
	mm := `{
		"comment":"",
		"container_env":"",
		"domain":"lichao",
		"deploy_version":"",
		"ports_info":[
			{
				"port_alias":"GR45068C5000",
				"protocol":"http",
				"mapping_port":0,
				"container_port":5000,
				"is_outer_service":true,
				"is_inner_service":false
			}
		],
		"dep_sids":null,
		"volumes_info":[
	
		],
		"extend_method":"stateless",
		"operator":"lichao",
		"container_memory":512,
		"service_key":"application",
		"category":"application",
		"service_version":"81701",
		"event_id":"e5bd1926254b447ea97817566b2d71bf",
		"container_cpu":80,
		"namespace":"goodrain",
		"extend_info":{
			"envs":[
	
			],
			"ports":[
	
			]
		},
		"service_type":"application",
		"status":0,
		"node_label":"",
		"replicas":1,
		"image_name":"goodrain.me/runner",
		"service_alias":"gr45068c",
		"service_id":"55c60b74a506261608f5c36f0f45068c",
		"code_from":"gitlab_manual",
		"volume_mount_path":"/data",
		"tenant_id":"3000bf47672b40c19529504651697b29",
		"container_cmd":"start web",
		"host_path":"/grdata/tenant/3000bf47672b40c19529504651697b29/service/55c60b74a506261608f5c36f0f45068c",
		"envs_info":[
	
		],
		"volume_path":"vol55c60b74a5",
		"port_type":"multi_outer"
	}`

	var s api_model.ServiceStruct
	err := ffjson.Unmarshal([]byte(mm), &s)
	if err != nil {
		fmt.Printf("err is %v", err)
	}
	fmt.Printf("json is \n %v", s)
}

func TestUUID(t *testing.T) {
	id := fmt.Sprintf("%s", uuid.NewV4())
	uid := strings.Replace(id, "-", "", -1)
	logrus.Debugf("uuid is %v", uid)
	name := strings.Split(id, "-")[0]
	fmt.Printf("id is %s, uid is %s, name is %v", id, uid, name)
}

func TestGetServicesDisk(t *testing.T) {
	p := proxy.CreateProxy("prometheus", "http", []string{"39.96.189.166:9999"})
	disk := GetServicesDiskDeprecated([]string{"ef75e1d5e3df412a8af06129dae42869"}, p)
	t.Log(disk)
}
