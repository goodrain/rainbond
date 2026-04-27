package build

import "testing"

func TestNormalizeHostAliasesSkipsInvalidEntriesAndMergesDuplicates(t *testing.T) {
	got := normalizeHostAliases([]HostAlias{
		{IP: "", Hostnames: []string{"empty-ip"}},
		{IP: "not-an-ip", Hostnames: []string{"invalid-ip"}},
		{IP: "10.10.10.10", Hostnames: []string{"goodrain.me", ""}},
		{IP: "10.10.10.10", Hostnames: []string{"maven.goodrain.me"}},
		{IP: "2001:db8::10", Hostnames: []string{"lang.goodrain.me"}},
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 normalized host aliases, got %d", len(got))
	}
	if got[0].IP != "10.10.10.10" {
		t.Fatalf("expected first host alias to keep IPv4 address, got %q", got[0].IP)
	}
	if len(got[0].Hostnames) != 2 || got[0].Hostnames[0] != "goodrain.me" || got[0].Hostnames[1] != "maven.goodrain.me" {
		t.Fatalf("unexpected merged hostnames for IPv4 alias: %#v", got[0].Hostnames)
	}
	if got[1].IP != "2001:db8::10" {
		t.Fatalf("expected second host alias to keep valid IPv6 address, got %q", got[1].IP)
	}
	if len(got[1].Hostnames) != 1 || got[1].Hostnames[0] != "lang.goodrain.me" {
		t.Fatalf("unexpected merged hostnames for IPv6 alias: %#v", got[1].Hostnames)
	}
}
