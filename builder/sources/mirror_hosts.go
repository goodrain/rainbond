// RAINBOND, Application Management Platform
// Copyright (C) 2014-2026 Goodrain Co., Ltd.

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

package sources

import (
	"crypto/tls"
	"net/http"
	"strings"

	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/containerd/remotes/docker"
)

// mirrorRegistryHosts wraps a containerd RegistryHosts resolver so docker.io
// pulls try the configured mirrors first and fall back to the upstream
// registry. The containerd resolver walks the returned hosts in order, so a
// dead mirror only costs one failed attempt instead of failing the pull.
// Non docker.io registries always pass through untouched.
func mirrorRegistryHosts(mirrors []string, fallback docker.RegistryHosts) docker.RegistryHosts {
	return func(host string) ([]docker.RegistryHost, error) {
		base, err := fallback(host)
		if err != nil {
			return nil, err
		}
		if host != "docker.io" || len(mirrors) == 0 {
			return base, nil
		}
		hosts := make([]docker.RegistryHost, 0, len(mirrors)+len(base))
		for _, m := range mirrors {
			hosts = append(hosts, mirrorRegistryHost(m))
		}
		return append(hosts, base...), nil
	}
}

func mirrorRegistryHost(mirrorURL string) docker.RegistryHost {
	scheme := "https"
	host := strings.TrimSpace(mirrorURL)
	if strings.HasPrefix(host, "http://") {
		scheme = "http"
		host = strings.TrimPrefix(host, "http://")
	} else {
		host = strings.TrimPrefix(host, "https://")
	}
	host = strings.TrimSuffix(host, "/")
	return docker.RegistryHost{
		Client: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
		Authorizer:   docker.NewDockerAuthorizer(),
		Host:         host,
		Scheme:       scheme,
		Path:         "/v2",
		Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
	}
}

// mirrorPullRefs returns the references a docker-daemon pull should try in
// order. For docker.io images each mirror host yields a rewritten reference
// (with the library/ namespace completion ParseDockerRef performs), and the
// normalized upstream reference closes the list as the final fallback. Images
// from any other registry return only the original reference.
func mirrorPullRefs(image string, mirrors []string) []string {
	if len(mirrors) == 0 {
		return []string{image}
	}
	named, err := refdocker.ParseDockerRef(image)
	if err != nil || refdocker.Domain(named) != "docker.io" {
		return []string{image}
	}
	canonical := named.String()
	remainder := strings.TrimPrefix(canonical, "docker.io/")
	refs := make([]string, 0, len(mirrors)+1)
	for _, m := range mirrors {
		host := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(m), "https://"), "http://")
		host = strings.TrimSuffix(host, "/")
		if host == "" {
			continue
		}
		refs = append(refs, host+"/"+remainder)
	}
	return append(refs, canonical)
}
