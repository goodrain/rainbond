package code

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDockerfilesInHiddenDirs(t *testing.T) {
	// 创建临时测试目录
	tmpDir, err := os.MkdirTemp("", "dockerfile-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试目录结构
	testDirs := []string{
		".docker",
		".config/app",
		".github/workflows",
		"services/api",
	}

	for _, dir := range testDirs {
		dirPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatal(err)
		}
		// 在每个目录创建 Dockerfile
		dockerfilePath := filepath.Join(dirPath, "Dockerfile")
		if err := os.WriteFile(dockerfilePath, []byte("FROM alpine"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 测试查找
	dockerfiles := FindDockerfiles(tmpDir, 2, 10)

	// 验证结果
	expectedCount := 4 // .docker, .config/app, .github/workflows, services/api
	if len(dockerfiles) != expectedCount {
		t.Errorf("Expected %d dockerfiles, got %d: %v", expectedCount, len(dockerfiles), dockerfiles)
	}

	// 验证包含隐藏目录中的 Dockerfile
	foundHidden := false
	for _, df := range dockerfiles {
		if filepath.Dir(df) == ".docker" || filepath.Dir(df) == ".config/app" {
			foundHidden = true
			break
		}
	}
	if !foundHidden {
		t.Error("Should find Dockerfiles in hidden directories")
	}
}

func TestFindDockerfilesIgnoreSpecificDirs(t *testing.T) {
	// 创建临时测试目录
	tmpDir, err := os.MkdirTemp("", "dockerfile-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建应该被忽略的目录
	ignoredDirs := []string{
		".git",
		"node_modules",
		"vendor",
	}

	for _, dir := range ignoredDirs {
		dirPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatal(err)
		}
		// 在忽略目录中创建 Dockerfile
		dockerfilePath := filepath.Join(dirPath, "Dockerfile")
		if err := os.WriteFile(dockerfilePath, []byte("FROM alpine"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 测试查找
	dockerfiles := FindDockerfiles(tmpDir, 2, 10)

	// 验证结果：应该为空，因为所有目录都应该被忽略
	if len(dockerfiles) != 0 {
		t.Errorf("Expected 0 dockerfiles in ignored dirs, got %d: %v", len(dockerfiles), dockerfiles)
	}
}
