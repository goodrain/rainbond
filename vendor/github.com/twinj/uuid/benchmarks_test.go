package uuid_test

import (
	. "github.com/twinj/uuid"
	"testing"
)

var name UniqueName = Name("www.example.com")

func BenchmarkNewV1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewV1() // Sets up initial store on first run
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewV2(DomainGroup)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewV3(NameSpaceDNS, name)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewV4()
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV5(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewV5(NameSpaceDNS, name)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkCompare(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Compare(NameSpaceDNS, NameSpaceURL)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkEqual(b *testing.B) {
	s := "f3593cff-ee92-40df-4086-87825b523f13"
	id, _ := Parse(s)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Equal(id, id)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNew(b *testing.B) {
	id := NewV2(DomainGroup).Bytes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		New(id)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewHex(b *testing.B) {
	s := "6ba7b8149dad11d180b400c04fd430c8"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewHex(s)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkParse(b *testing.B) {
	s := "f3593cff-ee92-40df-4086-87825b523f13"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Parse(s)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Now()
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkFormatter(b *testing.B) {
	id := NewV2(DomainGroup)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Formatter(id, "{%X-%X-%X-%x-%X}")
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkUuid_Bytes(b *testing.B) {
	id := make(Uuid, 16)
	copy(id, NameSpaceDNS.Bytes())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id.Bytes()
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkUuid_String_Canonical(b *testing.B) {
	id := NewV2(DomainGroup)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkUuid_String_NonCanonical(b *testing.B) {
	SwitchFormat(FormatUrn)
	id := NewV2(DomainGroup)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
	b.StopTimer()
	b.ReportAllocs()
	SwitchFormat(FormatCanonical)
}

func BenchmarkUuid_Variant(b *testing.B) {
	id := NewV2(DomainGroup)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.Variant()
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkUuid_Version(b *testing.B) {
	id := NewV2(DomainGroup)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.Version()
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNameSpace_Bytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NameSpaceDNS.Bytes()
	}
	b.StopTimer()
	b.ReportAllocs()
}
