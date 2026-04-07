package validation

import "testing"

// capability_id: rainbond.endpoint.domain-validate
func TestValidateDomain(t *testing.T) {
	if errs := ValidateDomain("example.com"); len(errs) != 0 {
		t.Fatalf("expected valid domain, got %v", errs)
	}
	if errs := ValidateDomain("*.example.com"); len(errs) != 0 {
		t.Fatalf("expected valid wildcard domain, got %v", errs)
	}
	if errs := ValidateDomain("bad domain"); len(errs) == 0 {
		t.Fatal("expected invalid domain errors")
	}
}

// capability_id: rainbond.endpoint.address-split
func TestSplitEndpointAddress(t *testing.T) {
	if got := SplitEndpointAddress("https://1.2.3.4:8443"); got != "1.2.3.4" {
		t.Fatalf("unexpected split result: %q", got)
	}
	if got := SplitEndpointAddress("http://example.com:8080"); got != "example.com" {
		t.Fatalf("unexpected split result: %q", got)
	}
	if got := SplitEndpointAddress("example.com"); got != "example.com" {
		t.Fatalf("unexpected split result: %q", got)
	}
}

// capability_id: rainbond.endpoint.ip-validate
func TestValidateEndpointIP(t *testing.T) {
	if errs := ValidateEndpointIP("1.2.3.4"); len(errs) != 0 {
		t.Fatalf("expected valid ip, got %v", errs)
	}
	if errs := ValidateEndpointIP("0.0.0.0"); len(errs) == 0 {
		t.Fatal("expected unspecified ip error")
	}
	if errs := ValidateEndpointIP("127.0.0.1"); len(errs) == 0 {
		t.Fatal("expected loopback ip error")
	}
	if errs := ValidateEndpointIP("not-an-ip"); len(errs) == 0 {
		t.Fatal("expected invalid ip error")
	}
}

// capability_id: rainbond.endpoint.domain-not-ip
func TestIsDomainNotIP(t *testing.T) {
	if !IsDomainNotIP("example.com") {
		t.Fatal("expected example.com to be treated as domain")
	}
	if IsDomainNotIP("127.0.0.1") {
		t.Fatal("did not expect loopback ip to be treated as domain")
	}
	if !IsDomainNotIP("1.1.1.1") {
		t.Fatal("expected historical special case 1.1.1.1 to be treated as domain")
	}
}
