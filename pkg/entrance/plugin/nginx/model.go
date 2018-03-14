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

package nginx

//AddDomainS AddDomainS
type AddDomainS struct {
	Domain          string
	HTTPS           bool
	TransferHTTP    bool
	CertificateName string
	PoolName        string
	NodeList        []string
}

//SSLCert SSLCert
type SSLCert struct {
	CertName   string
	Key        string
	CA         string
	HTTPMethod HTTPMETHOD
}

//AddPoolNodeS AddPoolNodeS
type AddPoolNodeS struct {
	PoolName   string
	NodeList   []string
	DomainList []string
}

type StreamNodeS struct {
	PoolName string
	NodeList []string
}

type HttpNodeS struct {
	PoolName string
	NodeList []string
}

type AddUserDomainS struct {
	OldDomain string
	NewDomain string
	PoolName  string
	NodeList  []string
}

type AddVirtualServerS struct {
	virtual_server_name string
	Port                string
	PoolName            string
	NodeList            []string
}

type DeleteDomainS struct {
	Domain     string
	PoolName   string
	DomainList []string
}

type DeleteVirtualServerS struct {
	VirtualServerName string
	Port              string
	PoolName          string
}

type DrainNodeS struct {
	PoolName   string
	NodeList   []string
	TlnType    string
	TlnPort    string
	DomainList []string
}

type DrainPoolS struct {
	PoolName   string
	TlnType    string
	TlnPort    string
	DomainList []string
}

type PoolName struct {
	Tenantname  string
	Servicename string
	Port        string
}

type HTTPMETHOD string

const (
	PoolExpString string     = "(.*)@(.*)_([0-9]*)\\.Pool"
	MethodPOST    HTTPMETHOD = "POST"
	MethodPUT     HTTPMETHOD = "PUT"
	MethodDELETE  HTTPMETHOD = "DELETE"
)

type MethodHTTPArgs struct {
	PoolName     *PoolName
	UpStream     []byte
	Method       HTTPMETHOD
	UpStreamName string //poolname
	Url          string
	Domain       string
}

type MethodHTTPSArgs struct {
	PoolName *PoolName
	UpStream []byte
	Method   HTTPMETHOD
	URL      string
	Domain   string
}
