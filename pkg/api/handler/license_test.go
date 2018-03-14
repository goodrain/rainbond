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
	"testing"
)

func TestLicense(t *testing.T) {
	licenseStr := "0QFGbwit-"
	text, err := decrypt(key, licenseStr)
	if err != nil {
		fmt.Println("decrypt err")
	}
	fmt.Printf("license text: %s", text)
}

func TestToken(t *testing.T) {
	licenseStr := "0QFGbwit-"
	token, err := BasePack([]byte(licenseStr))
	if err != nil {
		fmt.Printf("error")
	}
	fmt.Printf("token is: %v\n", token)
	fmt.Printf("len is %v\n", len(token))
}
