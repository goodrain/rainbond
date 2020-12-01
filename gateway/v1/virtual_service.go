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

import corev1 "k8s.io/api/core/v1"

// Protocol defines network protocols supported for things like container ports.
type Protocol string

const (
	// ProtocolTCP is the TCP protocol.
	ProtocolTCP Protocol = "TCP"
	// ProtocolUDP is the UDP protocol.
	ProtocolUDP Protocol = "UDP"
	// ProtocolSCTP is the SCTP protocol.
	ProtocolSCTP Protocol = "SCTP"
)

//VirtualService VirtualService
type VirtualService struct {
	Meta
	Enabled  bool            `json:"enable"`
	Protocol corev1.Protocol `json:"protocol"`
	// BackendProtocol indicates which protocol should be used to communicate with the service
	BackendProtocol        string   `json:"backend-protocol"`
	Port                   int32    `json:"port"`
	Listening              []string `json:"listening"` //if Listening is nil,will listen all
	Note                   string   `json:"note"`
	DefaultPoolName        string   `json:"default_pool_name"`
	RuleNames              []string `json:"rule_names"`
	SSLdecrypt             bool     `json:"ssl_decrypt"`
	DefaultCertificateName string   `json:"default_certificate_name"`
	RequestLogEnable       bool     `json:"request_log_enable"`
	RequestLogFileName     string   `json:"request_log_file_name"`
	RequestLogFormat       string   `json:"request_log_format"`
	//ConnectTimeout The time, in seconds, to wait for data from a new connection. If no data is received within this time, the connection will be closed. A value of 0 (zero) will disable the timeout.
	ConnectTimeout int `json:"connect_timeout"`
	//Timeout A connection should be closed if no additional data has been received for this period of time. A value of 0 (zero) will disable this timeout. Note that the default value may vary depending on the protocol selected.
	Timeout          int                    `json:"timeout"`
	ServerName       string                 `json:"server_name"`
	PoolName         string                 `json:"pool_name"`
	SSlProtocols     string                 `json:"ssl_protocols"`
	SSLCert          *SSLCert               `json:"ssl_cert"`
	Locations        []*Location            `json:"locations"`
	ForceSSLRedirect bool                   `json:"force_ssl_redirect"`
	ExtensionConfig  map[string]interface{} `json:"extension_config"`
}

//Equals equals vs
func (v *VirtualService) Equals(c *VirtualService) bool {
	if v == c {
		return true
	}
	if v == nil || c == nil {
		return false
	}
	if !v.Meta.Equals(&c.Meta) {
		return false
	}
	if v.Enabled != c.Enabled {
		return false
	}
	if v.Protocol != c.Protocol {
		return false
	}
	if v.BackendProtocol != c.BackendProtocol {
		return false
	}
	if v.Port != c.Port {
		return false
	}

	// TODO: this snippet needs improvement
	if len(v.Listening) != len(c.Listening) {
		return false
	}
	for _, vl := range v.Listening {
		flag := false
		for _, cl := range c.Listening {
			if vl == cl {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}

	if v.Note != c.Note {
		return false
	}
	if v.DefaultPoolName != c.DefaultPoolName {
		return false
	}

	// TODO: this snippet needs improvement
	if len(v.RuleNames) != len(c.RuleNames) {
		return false
	}
	for _, vr := range v.RuleNames {
		flag := false
		for _, cr := range c.RuleNames {
			if vr == cr {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}

	if v.SSLdecrypt != c.SSLdecrypt {
		return false
	}
	if v.DefaultCertificateName != c.DefaultCertificateName {
		return false
	}

	if v.RequestLogEnable != c.RequestLogEnable {
		return false
	}
	if v.RequestLogFileName != c.RequestLogFileName {
		return false
	}
	if v.RequestLogFormat != c.RequestLogFormat {
		return false
	}
	if v.ConnectTimeout != c.ConnectTimeout {
		return false
	}
	if v.Timeout != c.Timeout {
		return false
	}
	if v.ServerName != c.ServerName {
		return false
	}
	if v.PoolName != c.PoolName {
		return false
	}

	if len(v.Locations) != len(c.Locations) {
		return false
	}
	for _, vloc := range v.Locations {
		flag := false
		for _, cloc := range c.Locations {
			if vloc.Equals(cloc) {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}

	if !v.SSLCert.Equals(c.SSLCert) {
		return false
	}
	if v.ForceSSLRedirect != c.ForceSSLRedirect {
		return false
	}
	if len(v.ExtensionConfig) != len(c.ExtensionConfig) {
		return false
	}
	for key, ve := range v.ExtensionConfig {
		if c.ExtensionConfig[key] != ve {
			return false
		}
	}

	return true
}
