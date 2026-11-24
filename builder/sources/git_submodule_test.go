package sources

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/goodrain/rainbond/event"
)

// capability_id: rainbond.source-repo.shallow-submodules
func TestGitCloneOrPullSubmoduleDepth(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for the local upload-pack fixture")
	}
	for _, testCase := range []struct {
		name       string
		allowExact bool
		wantFull   bool
		legacy     bool
	}{
		{name: "shallow clone can fetch pinned commit", allowExact: true},
		{name: "full clone fallback when pinned commit is unavailable", wantFull: true},
		{name: "standalone clone retains full submodule history", allowExact: true, wantFull: true, legacy: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			moduleDir := filepath.Join(root, "module.git")
			sourceDir := filepath.Join(root, "source.git")
			runSubmoduleFixtureGit(t, root, "init", "-b", "main", moduleDir)
			runSubmoduleFixtureGit(t, moduleDir, "commit", "--allow-empty", "-m", "older history")
			older := plumbing.NewHash(runSubmoduleFixtureGit(t, moduleDir, "rev-parse", "HEAD"))
			if err := os.WriteFile(filepath.Join(moduleDir, "file.txt"), []byte("pinned\n"), 0600); err != nil {
				t.Fatal(err)
			}
			runSubmoduleFixtureGit(t, moduleDir, "add", "file.txt")
			runSubmoduleFixtureGit(t, moduleDir, "commit", "-m", "pinned version")
			pinned := plumbing.NewHash(runSubmoduleFixtureGit(t, moduleDir, "rev-parse", "HEAD"))

			runSubmoduleFixtureGit(t, root, "init", "-b", "main", sourceDir)
			runSubmoduleFixtureGit(t, sourceDir, "submodule", "add", moduleDir, "linux")
			runSubmoduleFixtureGit(t, sourceDir, "commit", "-m", "pin submodule")
			runSubmoduleFixtureGit(t, moduleDir, "commit", "--allow-empty", "-m", "newer version")
			allowExact := "false"
			if testCase.allowExact {
				allowExact = "true"
			}
			runSubmoduleFixtureGit(t, moduleDir, "config", "uploadpack.allowReachableSHA1InWant", allowExact)
			runSubmoduleFixtureGit(t, moduleDir, "config", "uploadpack.allowTipSHA1InWant", "false")

			clone := GitCloneOrPull
			if testCase.legacy {
				clone = GitClone
			}
			destination := filepath.Join(root, "checkout")
			repo, _, err := clone(CodeSourceInfo{RepositoryURL: sourceDir, Branch: "main"}, destination, event.GetTestLogger(), 1)
			if err != nil {
				t.Fatal(err)
			}
			worktree, err := repo.Worktree()
			if err != nil {
				t.Fatal(err)
			}
			module, err := worktree.Submodule("linux")
			if err != nil {
				t.Fatal(err)
			}
			moduleRepo, err := module.Repository()
			if err != nil {
				t.Fatal(err)
			}
			head, err := moduleRepo.Head()
			if err != nil {
				t.Fatal(err)
			}
			if head.Hash() != pinned {
				t.Fatalf("submodule HEAD = %s, want %s", head.Hash(), pinned)
			}
			if _, err := moduleRepo.CommitObject(older); testCase.wantFull && err != nil || !testCase.wantFull && !errors.Is(err, plumbing.ErrObjectNotFound) {
				t.Fatalf("lookup older commit: %v (want full history: %t)", err, testCase.wantFull)
			}
			data, err := os.ReadFile(filepath.Join(destination, "linux", "file.txt"))
			if err != nil || string(data) != "pinned\n" {
				t.Fatalf("submodule worktree file = %q, err = %v", data, err)
			}
		})
	}
}

func runSubmoduleFixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-c", "protocol.file.allow=always", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}
