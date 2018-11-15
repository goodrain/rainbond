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

package conversion

import (
	"fmt"
	"os"
	"strings"

	"github.com/goodrain/rainbond/db"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

//createDefaultDomain create default domain
func createDefaultDomain(tenantName, serviceAlias string, servicePort int) string {
	exDomain := os.Getenv("EX_DOMAIN")
	if exDomain == "" {
		return ""
	}
	if strings.Contains(exDomain, ":") {
		exDomain = strings.Split(exDomain, ":")[0]
	}
	if exDomain[0] == '.' {
		exDomain = exDomain[1:]
	}
	exDomain = strings.TrimSpace(exDomain)
	return fmt.Sprintf("%d.%s.%s.%s", servicePort, serviceAlias, tenantName, exDomain)
}

//TenantServiceRegist conv inner and outer service regist
func TenantServiceRegist(as *v1.AppService, dbmanager db.Manager) error {

	return nil
}
