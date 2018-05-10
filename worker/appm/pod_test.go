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

package appm

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db"
	dbconfig "github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/event"

	"github.com/Sirupsen/logrus"
)

func init() {
	if err := db.CreateManager(dbconfig.Config{
		MysqlConnectionInfo: "root:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		logrus.Error(err)
	}
	event.NewManager(event.EventConfig{
		EventLogServers: []string{"tcp://127.0.0.1:6366"},
	})
}
func TestCreateEnv(t *testing.T) {
	builder, err := PodTemplateSpecBuilder("2f29882148c19f5f84e3a7cedf6097c7", event.GetManager().GetLogger("system"), "string")
	if err != nil {
		t.Fatal(err)
	}
	envs, err := builder.createEnv()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(envs)
}

func TestBuild(t *testing.T) {
	builder, err := PodTemplateSpecBuilder("2f29882148c19f5f84e3a7cedf6097c7", event.GetManager().GetLogger("system"), "")
	if err != nil {
		t.Fatal(err)
	}
	temp, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(temp)
}

func TestArgsBuild(t *testing.T) {
	cmd := "docker build ${ABC}sadas ${CCD}"
	var reg = regexp.MustCompile(`(?U)\$\{.*\}`)
	resultKey := reg.FindAllString(cmd, -1)
	envs := map[string]string{"ABC": "asdasd", "CCD": "asdasdsssss"}
	for _, rk := range resultKey {
		value := envs[GetConfigKey(rk)]
		cmd = strings.Replace(cmd, rk, value, -1)
	}
	args := strings.Split(cmd, " ")
	fmt.Println(args)
}
