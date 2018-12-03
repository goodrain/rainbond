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

import (
	"crypto/x509"
	"time"
)

// SSLCert describes a SSL certificate
type SSLCert struct {
	*Meta
	CertificateStr string            `json:"certificate_str"`
	Certificate    *x509.Certificate `json:"certificate,omitempty"`
	PrivateKey     string            `json:"private_key"`
	CertificatePem string            `json:"certificate_pem"`
	// CN contains all the common names defined in the SSL certificate
	CN []string `json:"cn"`
	// ExpiresTime contains the expiration of this SSL certificate in timestamp format
	ExpireTime time.Time `json:"expires"`
}

func (s *SSLCert) Equals(c *SSLCert) bool {
	if s == c {
		return true
	}
	if s == nil || c == nil {
		return false
	}
	if !s.Meta.Equals(c.Meta) {
		return false
	}
	if s.CertificatePem != c.CertificatePem {
		return false
	}
	if s.Certificate != nil && c.Certificate != nil {
		if !s.Certificate.Equal(c.Certificate) {
			return false
		}
	}
	if !(s.Certificate == nil && c.Certificate == nil) {
		return false
	}
	if s.CertificateStr != c.CertificateStr {
		return false
	}
	if s.PrivateKey != c.PrivateKey {
		return false
	}

	if len(s.CN) != len(c.CN) {
		return false
	}
	for _, scn := range s.CN {
		flag := false
		for _, ccn := range c.CN {
			if scn != ccn {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}

	if !s.ExpireTime.Equal(c.ExpireTime) {
		return false
	}
	return true
}
