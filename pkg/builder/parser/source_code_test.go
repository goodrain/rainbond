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

package parser

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/goodrain/rainbond/pkg/builder/sources"
)

func TestParseDockerfileInfo(t *testing.T) {
	parse := &SourceCodeParse{
		source:  "source",
		ports:   make(map[int]*Port),
		volumes: make(map[string]*Volume),
		envs:    make(map[string]*Env),
		logger:  nil,
		image:   parseImageName("goodrain.me/runner"),
		args:    []string{"start", "web"},
	}
	parse.parseDockerfileInfo("./Dockerfile")
	fmt.Println(parse.GetServiceInfo())
}

//ServiceCheckResult 应用检测结果
type ServiceCheckResult struct {
	//检测状态 Success Failure
	CheckStatus string         `json:"check_status"`
	ErrorInfos  ParseErrorList `json:"error_infos"`
	ServiceInfo []ServiceInfo  `json:"service_info"`
}

func TestSourceCode(t *testing.T) {
	sc := sources.CodeSourceInfo{
		ServerType:    "",
		RepositoryURL: "http://code.goodrain.com/goodrain/goodrain_web.git",
		Branch:        "master",
		User:          "barnett",
		Password:      "5258423Zqg",
	}
	b, _ := json.Marshal(sc)
	p := CreateSourceCodeParse(string(b), nil)
	err := p.Parse()
	if err != nil && err.IsFatalError() {
		t.Fatal(err)
	}
	re := ServiceCheckResult{
		CheckStatus: "Failure",
		ErrorInfos:  err,
		ServiceInfo: p.GetServiceInfo(),
	}
	body, _ := json.Marshal(re)
	fmt.Printf("%s \n", string(body))
}
