package apigateway

import "testing"

func TestNormalizeLegacyRouteName(t *testing.T) {
	tests := []struct {
		name         string
		routeName    string
		appID        string
		serviceAlias string
		want         string
	}{
		{
			name:         "legacy display name with numeric route prefix",
			routeName:    "312345.comp-ps-s-graf6613",
			appID:        "3",
			serviceAlias: "graf6613",
			want:         "12345.comp-ps-s",
		},
		{
			name:         "legacy display name with alphabetic route prefix from old domain",
			routeName:    "3abc.comp-ps-s-graf6613",
			appID:        "3",
			serviceAlias: "graf6613",
			want:         "abc.comp-ps-s",
		},
		{
			name:         "legacy display name where actual route also starts with app id digits",
			routeName:    "3312.comp-ps-s-graf6613",
			appID:        "3",
			serviceAlias: "graf6613",
			want:         "312.comp-ps-s",
		},
		{
			name:         "actual route name should stay unchanged",
			routeName:    "12345.comp-ps-s",
			appID:        "3",
			serviceAlias: "graf6613",
			want:         "12345.comp-ps-s",
		},
		{
			name:         "actual route name from old alphabetic domain stays unchanged",
			routeName:    "abc.comp-ps-s",
			appID:        "3",
			serviceAlias: "graf6613",
			want:         "abc.comp-ps-s",
		},
		{
			name:         "original name payload should use middle segment",
			routeName:    "123|12345.comp-ps-s|-graf6613",
			appID:        "3",
			serviceAlias: "graf6613",
			want:         "12345.comp-ps-s",
		},
		{
			name:         "empty route name",
			routeName:    "",
			appID:        "3",
			serviceAlias: "graf6613",
			want:         "",
		},
		{
			name:         "multi digit app id in legacy display name",
			routeName:    "1212.comp-ps-s-graf6613",
			appID:        "12",
			serviceAlias: "graf6613",
			want:         "12.comp-ps-s",
		},
		{
			name:         "multiple service aliases uses matching suffix",
			routeName:    "312345.comp-ps-s-graf6613",
			appID:        "3",
			serviceAlias: "nginx,graf6613",
			want:         "12345.comp-ps-s",
		},
		{
			name:         "multiple service aliases with whitespace uses matching suffix",
			routeName:    "312345.comp-ps-s-graf6613",
			appID:        "3",
			serviceAlias: " nginx , graf6613 ",
			want:         "12345.comp-ps-s",
		},
		{
			name:         "old alphabetic domain changed to numeric prefix still parses previous display name safely",
			routeName:    "3abc.comp-ps-s-graf6613",
			appID:        "3",
			serviceAlias: "nginx,graf6613",
			want:         "abc.comp-ps-s",
		},
		{
			name:         "legacy display name without matching service suffix keeps original",
			routeName:    "312345.comp-ps-s-other",
			appID:        "3",
			serviceAlias: "graf6613",
			want:         "312345.comp-ps-s-other",
		},
		{
			name:         "missing app id context keeps original",
			routeName:    "312345.comp-ps-s-graf6613",
			appID:        "",
			serviceAlias: "graf6613",
			want:         "312345.comp-ps-s-graf6613",
		},
		{
			name:         "missing service alias context keeps original",
			routeName:    "312345.comp-ps-s-graf6613",
			appID:        "3",
			serviceAlias: "",
			want:         "312345.comp-ps-s-graf6613",
		},
		{
			name:         "malformed original name falls back to original value",
			routeName:    "123||-graf6613",
			appID:        "3",
			serviceAlias: "graf6613",
			want:         "123||-graf6613",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeLegacyRouteName(tt.routeName, tt.appID, tt.serviceAlias)
			if got != tt.want {
				t.Fatalf("normalizeLegacyRouteName(%q, %q, %q) = %q, want %q", tt.routeName, tt.appID, tt.serviceAlias, got, tt.want)
			}
		})
	}
}

func TestRouteAliasCandidates(t *testing.T) {
	candidates := routeAliasCandidates("12.comp-ps-s", map[string]string{
		"app_id":   "3",
		"graf6613": "service_alias",
	})

	expected := map[string]bool{
		"12.comp-ps-s":             true,
		"312.comp-ps-s":            true,
		"312.comp-ps-s-graf6613":   true,
		"3|12.comp-ps-s|-graf6613": true,
	}

	if len(candidates) != len(expected) {
		t.Fatalf("routeAliasCandidates returned %d candidates, want %d", len(candidates), len(expected))
	}

	for _, candidate := range candidates {
		if !expected[candidate] {
			t.Fatalf("unexpected candidate %q", candidate)
		}
		delete(expected, candidate)
	}

	if len(expected) != 0 {
		t.Fatalf("missing candidates: %v", expected)
	}
}
