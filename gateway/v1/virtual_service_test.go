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

package v1

import "testing"

func TestVirtualService_Equals(t *testing.T) {
	v := newFakeVirtualService()
	vlocA:= newFakeLocation()
	vlocB:= newFakeLocation()
	v.Locations = append(v.Locations, vlocA)
	v.Locations = append(v.Locations, vlocB)
	v.SSLCert = newFakeSSLCert()

	c := newFakeVirtualService()
	clocA:= newFakeLocation()
	clocB:= newFakeLocation()
	c.Locations = append(c.Locations, clocA)
	c.Locations = append(c.Locations, clocB)
	c.SSLCert = newFakeSSLCert()


	if !v.Equals(c) {
		t.Errorf("v should equal c")
	}
}

func newFakeVirtualService() *VirtualService {
	return &VirtualService{
		Meta:                   newFakeMeta(),
		Enabled:                true,
		Protocol:               "Http",
		BackendProtocol:        "Http",
		Port:                   80,
		Listening:              []string{"a", "b", "c"},
		Note:                   "foo-node",
		DefaultPoolName:        "default-pool-name",
		RuleNames:              []string{"a", "b", "c"},
		SSLdecrypt:             true,
		DefaultCertificateName: "default-certificate-name",
		RequestLogEnable:       true,
		RequestLogFileName: "/var/log/gateway/request.log",
		RequestLogFormat: "request-log-format",
		ConnectTimeout: 70,
		Timeout: 70,
		ServerName:"foo-server_name",
		PoolName: "foo-pool-name",
		ForceSSLRedirect: true,
	}
}
