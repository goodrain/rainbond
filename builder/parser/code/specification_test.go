package code

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckCodeSpecificationJavaJarCNBSkipsProcfileRequirement(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "demo.jar"), []byte("jar"), 0644); err != nil {
		t.Fatalf("write demo.jar: %v", err)
	}

	spec := CheckCodeSpecification(dir, JavaJar, "git", "cnb")
	if !spec.Conform {
		t.Fatalf("expected java-jar cnb build to skip Procfile requirement, got %#v", spec.Noconform)
	}
}

func TestCheckCodeSpecificationJavaJarSlugStillRequiresProcfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "demo.jar"), []byte("jar"), 0644); err != nil {
		t.Fatalf("write demo.jar: %v", err)
	}

	spec := CheckCodeSpecification(dir, JavaJar, "git", "slug")
	if spec.Conform {
		t.Fatal("expected java-jar slug build to keep Procfile requirement")
	}
	if _, ok := spec.Noconform["识别为JavaJar语言,Procfile文件未定义"]; !ok {
		t.Fatalf("expected Procfile missing error, got %#v", spec.Noconform)
	}
}

func TestCheckCodeSpecificationJavaWarHasNoProcfileRequirement(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "demo.war"), []byte("war"), 0644); err != nil {
		t.Fatalf("write demo.war: %v", err)
	}

	spec := CheckCodeSpecification(dir, JaveWar, "git", "cnb")
	if !spec.Conform {
		t.Fatalf("expected java-war cnb build to pass existing specification, got %#v", spec.Noconform)
	}
}
