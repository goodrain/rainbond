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

package model

import (
	"strconv"
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"
)

//AddHTTPRuleStruct is used to add http rule, certificate and rule extensions
type AddHTTPRuleStruct struct {
	HTTPRuleID     string                 `json:"http_rule_id" validate:"http_rule_id|required"`
	ServiceID      string                 `json:"service_id" validate:"service_id|required"`
	ContainerPort  int                    `json:"container_port" validate:"container_port|required"`
	Domain         string                 `json:"domain" validate:"domain|required"`
	Path           string                 `json:"path"`
	Header         string                 `json:"header"`
	Cookie         string                 `json:"cookie"`
	Weight         int                    `json:"weight"`
	IP             string                 `json:"ip"`
	CertificateID  string                 `json:"certificate_id"`
	Certificate    string                 `json:"certificate"`
	PrivateKey     string                 `json:"private_key"`
	RuleExtensions []*RuleExtensionStruct `json:"rule_extensions"`
}

// DbModel return database model
func (h *AddHTTPRuleStruct) DbModel(serviceID string) *dbmodel.HTTPRule {
	return &dbmodel.HTTPRule{
		UUID:          h.HTTPRuleID,
		ServiceID:     serviceID,
		ContainerPort: h.ContainerPort,
		Domain:        h.Domain,
		Path: func() string {
			if !strings.HasPrefix(h.Path, "/") {
				return "/" + h.Path
			}
			return h.Path
		}(),
		Header:        h.Header,
		Cookie:        h.Cookie,
		Weight:        h.Weight,
		IP:            h.IP,
		CertificateID: h.CertificateID,
	}
}

//UpdateHTTPRuleStruct is used to update http rule, certificate and rule extensions
type UpdateHTTPRuleStruct struct {
	HTTPRuleID     string                 `json:"http_rule_id" validate:"http_rule_id|required"`
	ServiceID      string                 `json:"service_id"`
	ContainerPort  int                    `json:"container_port"`
	Domain         string                 `json:"domain"`
	Path           string                 `json:"path"`
	Header         string                 `json:"header"`
	Cookie         string                 `json:"cookie"`
	Weight         int                    `json:"weight"`
	IP             string                 `json:"ip"`
	CertificateID  string                 `json:"certificate_id"`
	Certificate    string                 `json:"certificate"`
	PrivateKey     string                 `json:"private_key"`
	RuleExtensions []*RuleExtensionStruct `json:"rule_extensions"`
}

//DeleteHTTPRuleStruct contains the id of http rule that will be deleted
type DeleteHTTPRuleStruct struct {
	HTTPRuleID string `json:"http_rule_id" validate:"http_rule_id|required"`
}

// AddTCPRuleStruct is used to add tcp rule and rule extensions
type AddTCPRuleStruct struct {
	TCPRuleID      string                 `json:"tcp_rule_id" validate:"tcp_rule_id|required"`
	ServiceID      string                 `json:"service_id" validate:"service_id|required"`
	ContainerPort  int                    `json:"container_port"`
	IP             string                 `json:"ip"`
	Port           int                    `json:"port" validate:"service_id|required"`
	RuleExtensions []*RuleExtensionStruct `json:"rule_extensions"`
}

// DbModel return database model
func (a *AddTCPRuleStruct) DbModel(serviceID string) *dbmodel.TCPRule {
	return &dbmodel.TCPRule{
		UUID:          a.TCPRuleID,
		ServiceID:     serviceID,
		ContainerPort: a.ContainerPort,
		IP:            a.IP,
		Port:          a.Port,
	}
}

// UpdateTCPRuleStruct is used to update tcp rule and rule extensions
type UpdateTCPRuleStruct struct {
	TCPRuleID      string                 `json:"tcp_rule_id" validate:"tcp_rule_id|required"`
	ServiceID      string                 `json:"service_id"`
	ContainerPort  int                    `json:"container_port"`
	IP             string                 `json:"ip"`
	Port           int                    `json:"port"`
	RuleExtensions []*RuleExtensionStruct `json:"rule_extensions"`
}

// DeleteTCPRuleStruct is used to delete tcp rule and rule extensions
type DeleteTCPRuleStruct struct {
	TCPRuleID string `json:"tcp_rule_id" validate:"tcp_rule_id|required"`
}

// RuleExtensionStruct represents rule extensions for http rule or tcp rule
type RuleExtensionStruct struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// AddRuleConfigReq -
type AddRuleConfigReq struct {
	ConfigID string `json:"config_id" validate:"config_id|required"`
	RuleID   string `json:"rule_id" validate:"rule_id|required"`
	Key      string `json:"key" validate:"key|required"`
	Value    string `json:"value" validate:"value|required"`
}

// UpdRuleConfigReq -
type UpdRuleConfigReq struct {
	ConfigID string `json:"config_id" validate:"config_id|required"`
	Key      string `json:"key"`
	Value    string `json:"value"`
}

// DelRuleConfigReq -
type DelRuleConfigReq struct {
	ConfigID string `json:"config_id" validate:"config_id|required"`
}

// AddOrUpdRuleConfigReq -
type AddOrUpdRuleConfigReq struct {
	Configs []*AddRuleConfigReq `json:"configs"`
}

// RuleConfigReq -
type RuleConfigReq struct {
	RuleID    string `json:"rule_id,omitempty" validate:"rule_id|required"`
	ServiceID string
	EventID   string
	Body      Body `json:"body" validate:"body|required"`
}

// Body is a embedded sturct of RuleConfigReq.
type Body struct {
	ProxyConnectTimeout int          `json:"proxy_connect_timeout,omitempty" validate:"proxy_connect_timeout|required"`
	ProxySendTimeout    int          `json:"proxy_send_timeout,omitempty" validate:"proxy_send_timeout|required"`
	ProxyReadTimeout    int          `json:"proxy_read_timeout,omitempty" validate:"proxy_read_timeout|required"`
	ProxyBodySize       int          `json:"proxy_body_size,omitempty" validate:"proxy_body_size|required"`
	SetHeaders          []*SetHeader `json:"set_headers,omitempty" `
	Rewrites            []*Rewrite   `json:"rewrite,omitempty"`
	ProxyBufferSize     int          `json:"proxy_buffer_size,omitempty" validate:"proxy_buffer_size|numeric_between:1,65535"`
	ProxyBufferNumbers  int          `json:"proxy_buffer_numbers,omitempty" validate:"proxy_buffer_size|numeric_between:1,65535"`
	ProxyBuffering      string       `json:"proxy_buffering,omitempty" validate:"proxy_buffering|required"`
}

// HTTPRuleConfig -
type HTTPRuleConfig struct {
	RuleID              string       `json:"rule_id,omitempty" validate:"rule_id|required"`
	ProxyConnectTimeout int          `json:"proxy_connect_timeout,omitempty" validate:"proxy_connect_timeout|required"`
	ProxySendTimeout    int          `json:"proxy_send_timeout,omitempty" validate:"proxy_send_timeout|required"`
	ProxyReadTimeout    int          `json:"proxy_read_timeout,omitempty" validate:"proxy_read_timeout|required"`
	ProxyBodySize       int          `json:"proxy_body_size,omitempty" validate:"proxy_body_size|required"`
	SetHeaders          []*SetHeader `json:"set_headers,omitempty" `
	Rewrites            []*Rewrite   `json:"rewrite,omitempty"`
	ProxyBufferSize     int          `json:"proxy_buffer_size,omitempty" validate:"proxy_buffer_size|numeric_between:1,65535"`
	ProxyBufferNumbers  int          `json:"proxy_buffer_numbers,omitempty" validate:"proxy_buffer_size|numeric_between:1,65535"`
	ProxyBuffering      string       `json:"proxy_buffering,omitempty" validate:"proxy_buffering|required"`
}

// DbModel return database model
func (h *HTTPRuleConfig) DbModel() []*dbmodel.GwRuleConfig {
	var configs []*dbmodel.GwRuleConfig
	configs = append(configs, &dbmodel.GwRuleConfig{
		RuleID: h.RuleID,
		Key:    "proxy-connect-timeout",
		Value:  strconv.Itoa(h.ProxyConnectTimeout),
	})
	configs = append(configs, &dbmodel.GwRuleConfig{
		RuleID: h.RuleID,
		Key:    "proxy-send-timeout",
		Value:  strconv.Itoa(h.ProxySendTimeout),
	})
	configs = append(configs, &dbmodel.GwRuleConfig{
		RuleID: h.RuleID,
		Key:    "proxy-read-timeout",
		Value:  strconv.Itoa(h.ProxyReadTimeout),
	})
	configs = append(configs, &dbmodel.GwRuleConfig{
		RuleID: h.RuleID,
		Key:    "proxy-body-size",
		Value:  strconv.Itoa(h.ProxyBodySize),
	})
	configs = append(configs, &dbmodel.GwRuleConfig{
		RuleID: h.RuleID,
		Key:    "proxy-buffer-size",
		Value:  strconv.Itoa(h.ProxyBufferSize),
	})
	configs = append(configs, &dbmodel.GwRuleConfig{
		RuleID: h.RuleID,
		Key:    "proxy-buffer-numbers",
		Value:  strconv.Itoa(h.ProxyBufferNumbers),
	})
	configs = append(configs, &dbmodel.GwRuleConfig{
		RuleID: h.RuleID,
		Key:    "proxy-buffering",
		Value:  h.ProxyBuffering,
	})
	setheaders := make(map[string]string)
	for _, item := range h.SetHeaders {
		if strings.TrimSpace(item.Key) == "" {
			continue
		}
		if strings.TrimSpace(item.Value) == "" {
			item.Value = "empty"
		}
		// filter same key
		setheaders["set-header-"+item.Key] = item.Value
	}
	for k, v := range setheaders {
		configs = append(configs, &dbmodel.GwRuleConfig{
			RuleID: h.RuleID,
			Key:    k,
			Value:  v,
		})
	}
	return configs
}

//SetHeader set header
type SetHeader struct {
	Key   string `json:"item_key"`
	Value string `json:"item_value"`
}

// Rewrite is a embeded sturct of Body.
type Rewrite struct {
	Regex       string `json:"regex"`
	Replacement string `json:"replacement"`
	Flag        string `json:"flag" validate:"flag|in:last,break,redirect,permanent"`
}

// UpdCertificateReq -
type UpdCertificateReq struct {
	CertificateID   string `json:"certificate_id"`
	CertificateName string `json:"certificate_name"`
	Certificate     string `json:"certificate"`
	PrivateKey      string `json:"private_key"`
}
