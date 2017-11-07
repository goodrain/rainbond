package uuid

import "testing"

func BenchmarkNewV1Resolution_1024(b *testing.B) {
	gen := NewGenerator(GeneratorConfig{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NewV1() // Sets up initial store on first run
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV1Resolution_2048(b *testing.B) {
	gen := NewGenerator(GeneratorConfig{Resolution: 2048})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NewV1() // Sets up initial store on first run
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV1Resolution_3072(b *testing.B) {
	gen := NewGenerator(GeneratorConfig{Resolution: 3072})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NewV1() // Sets up initial store on first run
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV1Resolution_4096(b *testing.B) {
	gen := NewGenerator(GeneratorConfig{Resolution: 4096})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NewV1() // Sets up initial store on first run
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV1Resolution_5120(b *testing.B) {
	gen := NewGenerator(GeneratorConfig{Resolution: 5120})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NewV1() // Sets up initial store on first run
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV1Resolution_6144(b *testing.B) {
	gen := NewGenerator(GeneratorConfig{Resolution: 6144})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NewV1() // Sets up initial store on first run
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV1Resolution_7168(b *testing.B) {
	gen := NewGenerator(GeneratorConfig{Resolution: 7168})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NewV1() // Sets up initial store on first run
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV1Resolution_8192(b *testing.B) {
	gen := NewGenerator(GeneratorConfig{Resolution: 8192})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NewV1() // Sets up initial store on first run
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV1Resolution_9216(b *testing.B) {
	gen := NewGenerator(GeneratorConfig{Resolution: 9216})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NewV1() // Sets up initial store on first run
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkNewV1Resolution_18432(b *testing.B) {
	gen := NewGenerator(GeneratorConfig{Resolution: 18432})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NewV1() // Sets up initial store on first run
	}
	b.StopTimer()
	b.ReportAllocs()
}
