package code

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRuntimeByStrategyDotnetDetectsTargetFramework(t *testing.T) {
	dir := t.TempDir()
	project := `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>`
	if err := os.WriteFile(filepath.Join(dir, "demo.csproj"), []byte(project), 0644); err != nil {
		t.Fatalf("write csproj: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, NetCore, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["RUNTIMES"]; got != "8.0" {
		t.Fatalf("expected RUNTIMES=8.0, got %q", got)
	}
}

func TestCheckRuntimeByStrategyDotnetDetectsTargetFrameworks(t *testing.T) {
	dir := t.TempDir()
	project := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFrameworks>net9.0;net8.0</TargetFrameworks>
  </PropertyGroup>
</Project>`
	if err := os.WriteFile(filepath.Join(dir, "demo.csproj"), []byte(project), 0644); err != nil {
		t.Fatalf("write csproj: %v", err)
	}

	runtimeInfo, err := CheckRuntimeByStrategy(dir, NetCore, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy returned error: %v", err)
	}
	if got := runtimeInfo["RUNTIMES"]; got != "9.0" {
		t.Fatalf("expected RUNTIMES=9.0, got %q", got)
	}
}
