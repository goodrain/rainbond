package cnb

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	corev1 "k8s.io/api/core/v1"
)

type packageWorkspaceTestManager struct {
	db.Manager
}

func (m packageWorkspaceTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return packageWorkspaceTestServices{}
}

type packageWorkspaceTestServices struct {
	dbdao.TenantServiceDao
}

func (packageWorkspaceTestServices) GetServiceByID(string) (*dbmodel.TenantServices, error) {
	return nil, errors.New("fixture has no database component")
}

func usePackageWorkspaceTestDB(t *testing.T) {
	t.Helper()
	previous := db.GetManager()
	db.SetTestManager(packageWorkspaceTestManager{})
	t.Cleanup(func() { db.SetTestManager(previous) })
}

func writeCNBSourceArchive(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	name := "source.zip"
	file, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0644)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return name
}

// capability_id: rainbond.cnb.package-workspace-isolation
func TestCNBPackageBuildUsesPristineWorkspace(t *testing.T) {
	usePackageWorkspaceTestDB(t)
	for _, tc := range []struct {
		name     string
		locked   bool
		vendored bool
		fail     bool
	}{
		{name: "unlocked repeated build"},
		{name: "archive lock is preserved", locked: true},
		{name: "archive modules are preserved", vendored: true},
		{name: "failed builds also release workspace", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := newNodeDir(t)
			files := map[string]string{
				"package.json": `{"name":"package-demo","version":"1.0.0"}`,
				"src/main.js":  "console.log('from archive');",
			}
			if tc.locked {
				files["package-lock.json"] = `{"lockfileVersion":3,"name":"package-demo"}`
			}
			if tc.vendored {
				files["node_modules/from-archive.txt"] = "user supplied dependency"
			}
			name := writeCNBSourceArchive(t, source, files)
			archiveBefore, err := os.ReadFile(filepath.Join(source, name))
			if err != nil {
				t.Fatal(err)
			}
			// Reproduce the retained directory from the previous CNB job.
			if err := os.WriteFile(filepath.Join(source, "package-lock.json"), []byte("generated lock"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(t.TempDir(), "missing-layer", "node_modules"), filepath.Join(source, "node_modules")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(source, "dist"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "dist", "stale.txt"), []byte("old output"), 0644); err != nil {
				t.Fatal(err)
			}

			var workspaces []string
			ctrl := &mockJobCtrl{execJobFn: func(ctx context.Context, pod *corev1.Pod, output io.Writer, result *channels.RingChannel) error {
				workspace := ""
				for _, volume := range pod.Spec.Volumes {
					if volume.Name == "workspace" && volume.HostPath != nil {
						workspace = strings.TrimPrefix(volume.HostPath.Path, "/opt/rainbond")
					}
				}
				if workspace == "" {
					return fmt.Errorf("workspace mount missing")
				}
				workspaces = append(workspaces, workspace)
				if workspace == source {
					t.Error("CNB job reused the retained package directory")
				}
				for name, want := range files {
					got, err := os.ReadFile(filepath.Join(workspace, name))
					if err != nil || string(got) != want {
						t.Errorf("archive file %s = %q, %v; want %q", name, got, err, want)
					}
				}
				for _, name := range []string{"dist/stale.txt", "node_modules/generated.txt"} {
					if _, err := os.Lstat(filepath.Join(workspace, name)); !os.IsNotExist(err) {
						t.Errorf("previous build output %s leaked into new job: %v", name, err)
					}
				}
				if !tc.locked {
					if _, err := os.Lstat(filepath.Join(workspace, "package-lock.json")); !os.IsNotExist(err) {
						t.Errorf("generated lock leaked into unlocked build: %v", err)
					}
					for _, arg := range pod.Spec.Containers[0].Args {
						if strings.HasPrefix(arg, "-cache-image=") && !tc.vendored {
							t.Errorf("generated files changed the cache policy: %s", arg)
						}
					}
				}
				modules := filepath.Join(workspace, "node_modules")
				// This is the filesystem operation that failed with "file exists"
				// when npm ci encountered the previous job's dangling symlink.
				if err := os.MkdirAll(modules, 0755); err != nil {
					return fmt.Errorf("create node_modules: %w", err)
				}
				if err := os.WriteFile(filepath.Join(modules, "generated.txt"), []byte("build output"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(workspace, "package-lock.json"), []byte("new generated lock"), 0644); err != nil {
					return err
				}
				if tc.fail {
					return fmt.Errorf("synthetic build failure")
				}
				go func() {
					result.In() <- "complete"
					result.In() <- "logcomplete"
				}()
				return nil
			}}
			b := newTestBuilder(ctrl)
			re := &build.Request{
				ServiceID: "package-demo", RbdNamespace: "ns", Arch: "amd64",
				SourceDir: source, ServerType: "pkg", Lang: code.Nodejs,
				BuildStrategy: "cnb",
				Commit:        build.Commit{Message: name}, BuildEnvs: map[string]string{},
				Logger: event.GetTestLogger(),
			}
			for attempt := 0; attempt < 2; attempt++ {
				re.DeployVersion = fmt.Sprintf("build-%d", attempt)
				_, err := b.Build(re)
				if (err != nil) != tc.fail {
					t.Errorf("build %d error = %v, expected failure = %v", attempt, err, tc.fail)
				}
				if re.SourceDir != source {
					t.Errorf("build changed the caller's source reference: %s", re.SourceDir)
				}
			}
			if len(workspaces) != 2 {
				t.Fatalf("expected two build jobs, got %d", len(workspaces))
			}
			if workspaces[0] == workspaces[1] {
				t.Error("repeated builds shared one workspace")
			}
			for _, workspace := range workspaces {
				if workspace != source {
					if _, err := os.Stat(workspace); !os.IsNotExist(err) {
						t.Errorf("temporary build workspace not released: %s, %v", workspace, err)
					}
				}
			}
			archiveAfter, err := os.ReadFile(filepath.Join(source, name))
			if err != nil || !bytes.Equal(archiveBefore, archiveAfter) {
				t.Fatalf("original uploaded archive changed: %v", err)
			}
			lock, err := os.ReadFile(filepath.Join(source, "package-lock.json"))
			if err != nil || string(lock) != "generated lock" {
				t.Errorf("retained input directory was modified: %q, %v", lock, err)
			}
		})
	}
	for _, failure := range []string{"corrupt archive", "empty lock", "version policy"} {
		t.Run(failure+" releases workspace before job", func(t *testing.T) {
			source := newNodeDir(t)
			contents := map[string]string{"package.json": `{"name":"package-demo"}`}
			if failure == "empty lock" {
				contents["package-lock.json"] = ""
			}
			name := writeCNBSourceArchive(t, source, contents)
			if failure == "corrupt archive" {
				if err := os.WriteFile(filepath.Join(source, name), []byte("invalid zip"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			ctrl := &mockJobCtrl{execJobFn: func(context.Context, *corev1.Pod, io.Writer, *channels.RingChannel) error {
				t.Error("invalid input reached the build job")
				return errors.New("unexpected build job")
			}}
			re := &build.Request{SourceDir: source, ServerType: "pkg", Lang: code.Nodejs,
				Commit: build.Commit{Message: name}, Logger: event.GetTestLogger()}
			if failure == "version policy" {
				re.BuildStrategy = "cnb"
				re.BuildEnvs = map[string]string{"CNB_NODE_VERSION": "20.20.0"}
				re.CNBVersionPolicy = &build.CNBVersionPolicy{Languages: map[string]build.CNBLanguagePolicy{
					"nodejs": {AllowedVersions: []string{"24.13.0"}},
				}}
			}
			if _, err := newTestBuilder(ctrl).Build(re); err == nil {
				t.Fatal("invalid package/configuration was accepted")
			}
			entries, err := os.ReadDir(filepath.Dir(source))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".cnb-package-") {
					t.Errorf("early failure leaked a workspace: %s", entry.Name())
				}
			}
			if re.SourceDir != source {
				t.Errorf("early failure changed caller source: %s", re.SourceDir)
			}
		})
	}
}

// capability_id: rainbond.cnb.package-workspace-isolation
func TestCNBNonPackageBuildKeepsSourceDirectory(t *testing.T) {
	usePackageWorkspaceTestDB(t)
	source := newNodeDir(t)
	var mounted string
	ctrl := &mockJobCtrl{execJobFn: func(ctx context.Context, pod *corev1.Pod, output io.Writer, result *channels.RingChannel) error {
		for _, volume := range pod.Spec.Volumes {
			if volume.Name == "workspace" {
				mounted = volume.HostPath.Path
			}
		}
		go func() {
			result.In() <- "complete"
			result.In() <- "logcomplete"
		}()
		return nil
	}}
	_, err := newTestBuilder(ctrl).Build(&build.Request{
		ServiceID: "git-demo", DeployVersion: "v1", SourceDir: source,
		ServerType: "git", Lang: code.Nodejs, Logger: event.GetTestLogger(),
		Commit: build.Commit{Message: "ordinary git commit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mounted != filepath.Join("/opt/rainbond", source) {
		t.Errorf("non-package source mount changed: %q", mounted)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("non-package source was removed: %v", err)
	}
}
