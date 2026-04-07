package helm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	helmpkgrepo "helm.sh/helm/v3/pkg/repo"
)

// capability_id: rainbond.helm-repo.add
func TestRepoAdd(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	assert.NoError(t, os.MkdirAll(repoDir, 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(repoDir, "index.yaml"), []byte("apiVersion: v1\nentries: {}\n"), 0644))
	repoFile := filepath.Join(root, "repositories.yaml")
	repo := NewRepo(repoFile, filepath.Join(root, "cache"))
	repoURL := "file://" + repoDir
	err := repo.Add("demo", repoURL, "", "")
	assert.NoError(t, err)

	content, readErr := os.ReadFile(repoFile)
	assert.NoError(t, readErr)
	assert.Contains(t, string(content), repoURL)
}

// capability_id: rainbond.helm-repo.reject-deprecated
func TestRepoAddRejectsDeprecatedRepo(t *testing.T) {
	repo := NewRepo(
		filepath.Join(t.TempDir(), "repositories.yaml"),
		filepath.Join(t.TempDir(), "cache"))

	err := repo.Add("stable", "https://kubernetes-charts.storage.googleapis.com/stable", "", "")

	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no longer available")
	}
}

// capability_id: rainbond.helm-repo.add-idempotent
func TestRepoAddSkipsExistingConfig(t *testing.T) {
	root := t.TempDir()
	repoFile := filepath.Join(root, "repositories.yaml")
	entries := helmpkgrepo.NewFile()
	entries.Update(&helmpkgrepo.Entry{
		Name: "demo",
		URL:  "https://charts.example.com/stable",
	})
	assert.NoError(t, entries.WriteFile(repoFile, 0644))

	repo := NewRepo(repoFile, filepath.Join(root, "cache"))
	err := repo.Add("demo", "https://charts.example.com/stable", "", "")

	assert.NoError(t, err)
	content, readErr := os.ReadFile(repoFile)
	assert.NoError(t, readErr)
	assert.Contains(t, string(content), "https://charts.example.com/stable")
}
