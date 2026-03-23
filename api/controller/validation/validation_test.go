package validation

import "testing"

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		valid  bool
	}{
		{
			name:   "alphabetic prefix domain from old rules",
			domain: "abc.com",
			valid:  true,
		},
		{
			name:   "numeric prefix domain",
			domain: "12.com",
			valid:  true,
		},
		{
			name:   "alphabetic wildcard domain",
			domain: "*.abc.com",
			valid:  true,
		},
		{
			name:   "numeric wildcard domain",
			domain: "*.12.com",
			valid:  true,
		},
		{
			name:   "uppercase domain rejected",
			domain: "Abc.com",
			valid:  false,
		},
		{
			name:   "scheme in domain rejected",
			domain: "http://12.com",
			valid:  false,
		},
		{
			name:   "double dots rejected",
			domain: "12..com",
			valid:  false,
		},
		{
			name:   "leading hyphen rejected",
			domain: "-12.com",
			valid:  false,
		},
		{
			name:   "trailing hyphen in label rejected",
			domain: "12-.com",
			valid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateDomain(tt.domain)
			if tt.valid && len(errs) > 0 {
				t.Fatalf("ValidateDomain(%q) returned errors %v, want valid", tt.domain, errs)
			}
			if !tt.valid && len(errs) == 0 {
				t.Fatalf("ValidateDomain(%q) returned no errors, want invalid", tt.domain)
			}
		})
	}
}
