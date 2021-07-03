// RAINBOND, Application Management Platform
// Copyright (C) 2021-2021 Goodrain Co., Ltd.

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

package componentdefinition

import (
	"encoding/json"
	"testing"

	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

func TestTemplateContext(t *testing.T) {
	ctx := NewTemplateContext(&v1.AppService{AppServiceBase: v1.AppServiceBase{ServiceID: "1234567890", ServiceAlias: "niasdjaj", TenantID: "098765432345678"}}, cueTemplate, map[string]interface{}{
		"kubernetes": map[string]interface{}{
			"name":      "service-name",
			"namespace": "t-namesapce",
		},
		"port": []map[string]interface{}{},
	})
	manifests, err := ctx.GenerateComponentManifests()
	if err != nil {
		t.Fatal(err)
	}
	show, _ := json.Marshal(manifests)
	t.Log(string(show))
}
