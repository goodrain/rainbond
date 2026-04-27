package build

import (
	"net"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// NormalizeHostAliases drops invalid entries and merges hostnames that share the same IP.
func NormalizeHostAliases(hostAliases []HostAlias) []corev1.HostAlias {
	return normalizeHostAliases(hostAliases)
}

func normalizeHostAliases(hostAliases []HostAlias) []corev1.HostAlias {
	merged := make(map[string][]string, len(hostAliases))
	seen := make(map[string]map[string]struct{}, len(hostAliases))
	order := make([]string, 0, len(hostAliases))

	for _, alias := range hostAliases {
		ip := normalizeHostAliasIP(alias.IP)
		if ip == "" {
			continue
		}
		hostnames := normalizeHostnames(alias.Hostnames)
		if len(hostnames) == 0 {
			continue
		}
		if _, ok := merged[ip]; !ok {
			merged[ip] = nil
			seen[ip] = make(map[string]struct{}, len(hostnames))
			order = append(order, ip)
		}
		for _, hostname := range hostnames {
			if _, ok := seen[ip][hostname]; ok {
				continue
			}
			seen[ip][hostname] = struct{}{}
			merged[ip] = append(merged[ip], hostname)
		}
	}

	result := make([]corev1.HostAlias, 0, len(order))
	for _, ip := range order {
		result = append(result, corev1.HostAlias{IP: ip, Hostnames: merged[ip]})
	}
	return result
}

func normalizeHostAliasIP(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}
	return ip.String()
}

func normalizeHostnames(hostnames []string) []string {
	normalized := make([]string, 0, len(hostnames))
	seen := make(map[string]struct{}, len(hostnames))
	for _, hostname := range hostnames {
		hostname = strings.TrimSpace(hostname)
		if hostname == "" {
			continue
		}
		if _, ok := seen[hostname]; ok {
			continue
		}
		seen[hostname] = struct{}{}
		normalized = append(normalized, hostname)
	}
	return normalized
}
