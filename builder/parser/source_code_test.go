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

package parser

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
)

func TestParseDockerfileInfo(t *testing.T) {
	parse := &SourceCodeParse{
		source:  "source",
		ports:   make(map[int]*Port),
		volumes: make(map[string]*Volume),
		envs:    make(map[string]*Env),
		logger:  nil,
		image:   parseImageName(builder.RUNNERIMAGENAME),
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
		RepositoryURL: "https://github.com/barnettZQG/fserver.git",
		Branch:        "master",
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
