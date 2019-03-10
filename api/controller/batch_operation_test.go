// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

package controller

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/goodrain/rainbond/api/handler"

	"github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
)

func TestBatchOperation(t *testing.T) {
	var build = model.BeatchOperationRequestStruct{}
	buildInfo := []model.BuildInfoRequestStruct{
		model.BuildInfoRequestStruct{
			BuildENVs: map[string]string{
				"MAVEN_SETTING": "java",
			},
			Action: "upgrade",
			Kind:   model.FromCodeBuildKing,
			CodeInfo: model.BuildCodeInfo{
				RepoURL:    "https://github.com/goodrain/java-maven-demo.git",
				Branch:     "master",
				Lang:       "Java-maven",
				ServerType: "git",
			},
			ServiceID: "qwertyuiopasdfghjklzxcvbn",
		},
		model.BuildInfoRequestStruct{
			Action: "upgrade",
			Kind:   model.FromImageBuildKing,
			ImageInfo: model.BuildImageInfo{
				ImageURL: "hub.goodrain.com/xxx/xxx:latest",
				Cmd:      "start web",
			},
			ServiceID: "qwertyuiopasdfghjklzxcvbn",
		},
	}
	startInfo := []model.StartOrStopInfoRequestStruct{
		model.StartOrStopInfoRequestStruct{
			ServiceID: "qwertyuiopasdfghjkzxcvb",
		},
	}
	upgrade := []model.UpgradeInfoRequestStruct{
		model.UpgradeInfoRequestStruct{
			ServiceID:      "qwertyuiopasdfghjkzxcvb",
			UpgradeVersion: "2345678",
		},
	}
	build.Body.BuildInfos = buildInfo
	build.Body.StartInfos = startInfo
	build.Body.StopInfos = startInfo
	build.Body.UpgradeInfos = upgrade

	build.Body.Operation = "stop"
	out, _ := json.MarshalIndent(build.Body, "", "\t")
	fmt.Print(string(out))

	result := handler.BatchOperationResult{
		BatchResult: []handler.OperationResult{
			handler.OperationResult{
				ServiceID:     "qwertyuiopasdfghjkzxcvb",
				Operation:     "build",
				EventID:       "wertyuiodfghjcvbnm",
				Status:        "success",
				ErrMsg:        "",
				DeployVersion: "1234567890",
			},
		},
	}
	rebody := httputil.ResponseBody{
		Bean: result,
	}
	outre, _ := json.MarshalIndent(rebody, "", "\t")
	fmt.Print(string(outre))
}
