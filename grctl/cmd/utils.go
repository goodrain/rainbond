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

package cmd

import (
	"fmt"
	"os"

	"github.com/goodrain/rainbond/api/util"
)

func handleErr(err *util.APIHandleError) {
	if err != nil {
		if err.Err != nil {
			fmt.Printf(err.String())
			os.Exit(1)
		} else {
			fmt.Printf("API return %d", err.Code)
		}
	}
}
func showError(m string) {
	fmt.Printf("Error: %s\n", m)
	os.Exit(1)
}

func showSuccessMsg(m string) {
	fmt.Printf("Success: %s\n", m)
	os.Exit(0)
}
