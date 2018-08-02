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

package main

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"os"
	"testing"
)

func Test_crt(t *testing.T) {
	baseinfo := CertInformation{Country: []string{"CN"}, Organization: []string{"Goodrain"}, IsCA: true,
		OrganizationalUnit: []string{"work-stacks"}, EmailAddress: []string{"zengqg@goodrain.com"},
		Locality: []string{"BeiJing"}, Province: []string{"BeiJing"}, CommonName: "Work-Stacks",
		CrtName: "../../test/ssl/ca.pem", KeyName: "../../test/ssl/ca.key"}

	err := CreateCRT(nil, nil, baseinfo)
	if err != nil {
		t.Log("Create crt error,Error info:", err)
		return
	}
	crtinfo := baseinfo
	crtinfo.IsCA = false
	crtinfo.CrtName = "../../test/ssl/api_server.pem"
	crtinfo.KeyName = "../../test/ssl/api_server.key"
	crtinfo.Names = []pkix.AttributeTypeAndValue{{asn1.ObjectIdentifier{2, 1, 3}, "MAC_ADDR"}}

	crt, pri, err := Parse(baseinfo.CrtName, baseinfo.KeyName)
	if err != nil {
		t.Log("Parse crt error,Error info:", err)
		return
	}
	err = CreateCRT(crt, pri, crtinfo)
	if err != nil {
		t.Log("Create crt error,Error info:", err)
	}
	os.Remove(baseinfo.CrtName)
	os.Remove(baseinfo.KeyName)
	os.Remove(crtinfo.CrtName)
	os.Remove(crtinfo.KeyName)
}
