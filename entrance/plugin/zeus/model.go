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

package zeus

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/goodrain/rainbond/entrance/core/object"
)

//Source zeus资源模型
type Source struct {
	Properties Properties `json:"properties"`
}

//GetJSON 序列化json
func (z Source) GetJSON() ([]byte, error) {
	re, err := json.Marshal(z)
	if err != nil {
		return nil, err
	}
	return re, nil
}

//GetJSON 获取json
func GetJSON(obj interface{}) []byte {
	re, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	return re
}

type Properties interface {
}

type PoolProperties struct {
	Basic      PoolBasic      `json:"basic"`
	Connection PoolConnection `json:"connection"`
}
type VSProperties struct {
	Basic VSBasic `json:"basic"`
	SSL   VSssl   `json:"ssl"`
	Log   VsLog   `json:"log"`
}

type VsLog struct {
	Enabled                  bool   `json:"enabled"`
	Filename                 string `json:"filename"`
	Format                   string `json:"format"`
	SSLFailures              bool   `json:"ssl_failures"`
	ClientConnectionFailures bool   `json:"client_connection_failures"`
}

//VSssl 虚拟服务ssl配置
type VSssl struct {
	ServerCertDefault     string        `json:"server_cert_default,omitempty"`
	ServerCertHostMapping []*HostMaping `json:"server_cert_host_mapping"`
}
type HostMaping struct {
	Host            string `json:"host"`
	CertificateName string `json:"certificate"`
}

type PoolBasic struct {
	BandwidthClass string      `json:"bandwidth_class"`
	Monitors       []string    `json:"monitors"`
	NodesTable     []*ZeusNode `json:"nodes_table"`
	FailurePool    string      `json:"failure_pool"`
	Note           string      `json:"note"`
}

type PoolConnection struct {
	MaxReplyTime int `json:"max_reply_time"`
}

type VSBasic struct {
	AddXForwardedFor bool     `json:"add_x_forwarded_for"`
	ListenONAny      bool     `json:"listen_on_any"`
	ListenONHosts    []string `json:"listen_on_hosts,omitempty"`
	Note             string   `json:"note"`
	DefaultPoolName  string   `json:"pool"`
	Port             int32    `json:"port"`
	Enabled          bool     `json:"enabled"`
	Protocol         string   `json:"protocol"` //stream http https 等等
	RequestRules     []string `json:"request_rules,omitempty"`
	ResponseRules    []string `json:"response_rules,omitempty"`
	SSLDecrypt       bool     `json:"ssl_decrypt"`
	ConnectTimeout   int      `json:"connect_timeout"`
}

type ZeusNode struct {
	Node   string `json:"node"`
	State  string `json:"state"`
	Weight int    `json:"weight"`
}

type ZeusRules struct {
	rules []*object.RuleObject
	name  string
}

type SSLProperties struct {
	Basic SSL `json:"basic"`
}

type SSL struct {
	Note    string `json:"note"`
	Private string `json:"private"`
	Public  string `json:"public"`
	Request string `json:"request"`
}

//CreateHTTPRule http rule list
func CreateHTTPRule(rules []*object.RuleObject) ZeusRules {
	return ZeusRules{
		rules: rules,
		name:  "httpproxy",
	}
}

//CreateHTTPSRule https rule list
func CreateHTTPSRule(rules []*object.RuleObject) ZeusRules {
	return ZeusRules{
		rules: rules,
		name:  "httpsproxy",
	}
}

func (z ZeusRules) String() string {
	if z.rules != nil && len(z.rules) > 0 {
		result := bytes.NewBuffer(nil)
		result.WriteString(`$HttpHost = http.getHostHeader() ;\n `)
		for i, r := range z.rules {
			if i == 0 {
				if r.TransferHTTP {
					result.WriteString(fmt.Sprintf("if ($HttpHost == \"%s\") { http.changeSite(\"https://%s\");} \n ", r.DomainName, r.DomainName))
				} else {
					result.WriteString(fmt.Sprintf("if ($HttpHost == \"%s\") { pool.use(\"%s\");} \n ", r.DomainName, r.PoolName))
				}
			} else {
				if r.TransferHTTP {
					result.WriteString(fmt.Sprintf("else if ($HttpHost == \"%s\") { http.changeSite(\"https://%s\");} \n ", r.DomainName, r.DomainName))
				} else {
					result.WriteString(fmt.Sprintf("else if ($HttpHost == \"%s\") { pool.use(\"%s\");} \n ", r.DomainName, r.PoolName))
				}
			}
		}
		result.WriteString(`else {pool.use("discard");}`)
		return result.String()
	}
	return ""
}

//Bytes return rule script
func (z ZeusRules) Bytes() []byte {
	if z.rules != nil && len(z.rules) > 0 {
		result := bytes.NewBuffer(nil)
		result.WriteString("$HttpHost = http.getHostHeader() ;\n ")
		for i, r := range z.rules {
			if i == 0 {
				if r.TransferHTTP {
					result.WriteString(fmt.Sprintf("if ($HttpHost == \"%s\") { http.changeSite(\"https://%s\");} \n ", r.DomainName, r.DomainName))
				} else {
					result.WriteString(fmt.Sprintf("if ($HttpHost == \"%s\") { pool.use(\"%s\");} \n ", r.DomainName, r.PoolName))
				}
			} else {
				if r.TransferHTTP {
					result.WriteString(fmt.Sprintf("else if ($HttpHost == \"%s\") { http.changeSite(\"https://%s\");} \n ", r.DomainName, r.DomainName))
				} else {
					result.WriteString(fmt.Sprintf("else if ($HttpHost == \"%s\") { pool.use(\"%s\");} \n ", r.DomainName, r.PoolName))
				}
			}
		}
		result.WriteString("else {pool.use(\"discard\");}")
		return result.Bytes()
	}
	return nil
}

//Name return rule list name
func (z ZeusRules) Name() string {
	return z.name
}
