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

package util

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"

	"github.com/goodrain/rainbond/pkg/component/k8s"
)

// CloseResponse close response
func CloseResponse(res *http.Response) {
	if res != nil && res.Body != nil {
		res.Body.Close()
	}
}

// CloseRequest close request
func CloseRequest(req *http.Request) {
	if req != nil && req.Body != nil {
		req.Body.Close()
	}
}

func GetCertificateDomains(tlsCert *v1.Secret) ([]v2.HostType, error) {
	// Decode the certificate and private key from base64
	certData, certExists := tlsCert.Data["tls.crt"]
	keyData, keyExists := tlsCert.Data["tls.key"]

	if !certExists || !keyExists {
		return nil, fmt.Errorf("TLS certificate or key not found in the secret")
	}

	certBlock, _ := pem.Decode(certData)
	keyBlock, _ := pem.Decode(keyData)

	if certBlock == nil || keyBlock == nil {
		return nil, fmt.Errorf("failed to decode PEM block from certificate or private key")
	}

	// Parse the certificate to get the domains
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %v", err)
	}

	// Use a map to store unique domains
	uniqueDomains := make(map[v2.HostType]struct{})

	// Add the Common Name (CN) to unique domains
	uniqueDomains[v2.HostType(cert.Subject.CommonName)] = struct{}{}

	// Add Subject Alternative Names (SANs) to unique domains
	for _, dnsName := range cert.DNSNames {
		uniqueDomains[v2.HostType(dnsName)] = struct{}{}
	}

	// Convert the map to a slice
	var domains []v2.HostType
	for domain := range uniqueDomains {
		if domain != "" {
			domains = append(domains, domain)
		}
	}
	return domains, nil
}

// APIVersion -
const APIVersion = "apisix.apache.org/v2"

// ApisixUpstream -
const ApisixUpstream = "ApisixUpstream"

// ApisixRoute -
const ApisixRoute = "ApisixRoute"

// ApisixTLS -
const ApisixTLS = "ApisixTls"

// ResponseRewrite -
const ResponseRewrite = "response-rewrite"

// CheckDomainConflict checks if any of the given domains conflict with existing ApisixTls resources across all namespaces.
// It returns an error with details about the conflict if found, or nil if no conflict exists.
// Parameters:
//   - ctx: context for the operation
//   - domains: list of domains to check
//   - currentNamespace: namespace of the current resource (to allow same-namespace same-name updates)
//   - currentName: name of the current resource (to allow updates to the same resource)
func CheckDomainConflict(ctx context.Context, domains []v2.HostType, currentNamespace, currentName string) error {
	c := k8s.Default().ApiSixClient.ApisixV2()

	// List all namespaces
	namespaces, err := k8s.Default().Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %v", err)
	}

	// Check each namespace for ApisixTls resources
	for _, ns := range namespaces.Items {
		tlsList, err := c.ApisixTlses(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			// Skip namespaces we can't access
			continue
		}

		for _, tls := range tlsList.Items {
			// Skip the current resource being created/updated
			if tls.Namespace == currentNamespace && tls.Name == currentName {
				continue
			}

			// Check for domain conflicts
			for _, newDomain := range domains {
				for _, existingDomain := range tls.Spec.Hosts {
					if domainsConflict(string(newDomain), string(existingDomain)) {
						return fmt.Errorf("domain '%s' conflicts with existing domain '%s' in namespace '%s' (resource: %s)",
							newDomain, existingDomain, tls.Namespace, tls.Name)
					}
				}
			}
		}
	}

	return nil
}

// domainsConflict checks if two domains conflict with each other.
// Domains conflict if:
// - They are exactly the same
// - One is a wildcard that matches the other (e.g., *.example.com matches test.example.com)
// - Both are the same wildcard
func domainsConflict(domain1, domain2 string) bool {
	// Exact match
	if domain1 == domain2 {
		return true
	}

	// Check if domain1 is a wildcard that matches domain2
	if strings.HasPrefix(domain1, "*.") {
		wildcardBase := domain1[2:] // Remove "*."
		if strings.HasSuffix(domain2, wildcardBase) {
			return true
		}
	}

	// Check if domain2 is a wildcard that matches domain1
	if strings.HasPrefix(domain2, "*.") {
		wildcardBase := domain2[2:] // Remove "*."
		if strings.HasSuffix(domain1, wildcardBase) {
			return true
		}
	}

	return false
}
