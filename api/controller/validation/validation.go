package validation

import (
	"fmt"
	"net"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

// ValidateDomain tests that the argument is a valid domain.
func ValidateDomain(domain string) []string {
	if strings.TrimSpace(domain) == "" {
		return nil
	}
	var errs []string
	if strings.Contains(domain, "*") {
		errs = k8svalidation.IsWildcardDNS1123Subdomain(domain)
	} else {
		errs = k8svalidation.IsDNS1123Subdomain(domain)
	}
	return errs
}

// ValidateEndpointAddress tests that the argument is a valid endpoint address.
func ValidateEndpointAddress(address string) []string {
	ip := net.ParseIP(address)
	if ip == nil {
		return ValidateDomain(address)
	}
	return ValidateEndpointIP(address)
}

// ValidateEndpointIP tests that the argument is a valid IP address.
func ValidateEndpointIP(ipAddress string) []string {
	// We disallow some IPs as endpoints or external-ips.  Specifically,
	// unspecified and loopback addresses are nonsensical and link-local
	// addresses tend to be used for node-centric purposes (e.g. metadata
	// service).
	err := []string{}
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		err = append(err, fmt.Sprintf("%s must be a valid IP address", ipAddress))
		return err
	}
	if ip.IsUnspecified() {
		err = append(err, fmt.Sprintf("%s may not be unspecified (0.0.0.0)", ipAddress))
	}
	if ip.IsLoopback() {
		err = append(err, fmt.Sprintf("%s may not be in the loopback range (127.0.0.0/8)", ipAddress))
	}
	if ip.IsLinkLocalUnicast() {
		err = append(err, fmt.Sprintf("%s may not be in the link-local range (169.254.0.0/16)", ipAddress))
	}
	if ip.IsLinkLocalMulticast() {
		err = append(err, fmt.Sprintf("%s may not be in the link-local multicast range (224.0.0.0/24)", ipAddress))
	}
	return err
}
