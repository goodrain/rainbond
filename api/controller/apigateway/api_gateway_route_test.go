package apigateway

import (
	"testing"

	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
)

func TestParseCertManagerDomains(t *testing.T) {
	domains := parseCertManagerDomains("foo.example.com, bar.example.com ,,baz.example.com")
	if len(domains) != 3 {
		t.Fatalf("expected 3 domains, got %d", len(domains))
	}
	if domains[0] != "foo.example.com" || domains[1] != "bar.example.com" || domains[2] != "baz.example.com" {
		t.Fatalf("unexpected domains: %#v", domains)
	}
}

func TestHasMatchingCertManagerDomain(t *testing.T) {
	tests := []struct {
		name         string
		certDomains  []string
		routeDomains []string
		want         bool
	}{
		{
			name:         "exact match",
			certDomains:  []string{"foo.example.com"},
			routeDomains: []string{"foo.example.com"},
			want:         true,
		},
		{
			name:         "wildcard match",
			certDomains:  []string{"*.example.com"},
			routeDomains: []string{"foo.example.com"},
			want:         true,
		},
		{
			name:         "no overlap",
			certDomains:  []string{"foo.example.com"},
			routeDomains: []string{"bar.example.com"},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasMatchingCertManagerDomain(tt.certDomains, tt.routeDomains)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestRouteMatchesCertManagerDomains(t *testing.T) {
	route := &v2.ApisixRoute{
		Spec: v2.ApisixRouteSpec{
			HTTP: []v2.ApisixRouteHTTP{
				{
					Match: v2.ApisixRouteHTTPMatch{
						Hosts: []string{"foo.example.com", "bar.example.com"},
					},
				},
			},
		},
	}

	if !routeMatchesCertManagerDomains(route, []string{"bar.example.com"}) {
		t.Fatal("expected route to match certificate domains")
	}

	if routeMatchesCertManagerDomains(route, []string{"baz.example.com"}) {
		t.Fatal("expected route not to match unrelated certificate domains")
	}
}
