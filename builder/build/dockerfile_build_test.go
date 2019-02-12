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

package build

import (
	"testing"
)

func TestGetARGs(t *testing.T) {
	buildEnvs := make(map[string]string)
	buildEnvs["ARG_TEST"] = "abcdefg"
	buildEnvs["PROC_ENV"] = "{\"procfile\": \"\", \"dependencies\": {}, \"language\": \"dockerfile\", \"runtimes\": \"\"}"

	args := GetARGs(buildEnvs)
	if v := buildEnvs["ARG_TEST"]; *args["TEST"] != v {
		t.Errorf("Expected %s for arg[\"%s\"], but returned %s", buildEnvs["ARG_TEST"], "ARG_TEST", *args["TEST"])
	}
	if PROC_ENV := args["PROC_ENV"]; PROC_ENV != nil {
		t.Errorf("Expected nil for  args[\"PROC_ENV\"], but returned %v", PROC_ENV)
	}
}
