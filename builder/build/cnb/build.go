package cnb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goodrain/rainbond/builder/build"
	jobc "github.com/goodrain/rainbond/builder/job"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func init() {
	build.RegisterCNBBuilder(NewBuilder)
}

// Builder implements the build.Build interface for Cloud Native Buildpacks
type Builder struct {
	jobCtrl          jobc.Controller
	createAuthSecret func(*build.Request) (corev1.Secret, error)
	deleteAuthSecret func(*build.Request, string)
	prepareBuildKit  func(ctx context.Context, kubeClient kubernetes.Interface, namespace, cmName, imageDomain string) error
}

// NewBuilder creates a new CNB builder
func NewBuilder() (build.Build, error) {
	return &Builder{
		jobCtrl:          jobc.GetJobController(),
		createAuthSecret: build.CreateAuthSecret,
		deleteAuthSecret: build.DeleteAuthSecret,
		prepareBuildKit:  sources.PrepareBuildKitTomlCM,
	}, nil
}

// Build executes the CNB build process
func (b *Builder) Build(re *build.Request) (*build.Response, error) {
	re.Logger.Info("Starting CNB build", map[string]string{"step": "builder-exector"})

	b.stopPreBuildJob(re)

	if err := b.validateProjectFiles(re); err != nil {
		return nil, err
	}

	lang := getLanguageConfig(re)
	if err := lang.InjectMirrorConfig(re); err != nil {
		logrus.Warnf("inject mirror config failed: %v", err)
	}

	if err := b.setSourceDirPermissions(re); err != nil {
		logrus.Warnf("set source dir permissions failed: %v", err)
	}

	buildImageName := build.CreateImageName(re.ServiceID, re.DeployVersion)

	if err := b.runCNBBuildJob(re, buildImageName); err != nil {
		re.Logger.Error(util.Translation("CNB build failed"), map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("CNB build job error:", err.Error())
		return nil, err
	}

	re.Logger.Info("CNB build success", map[string]string{"step": "build-exector"})
	return &build.Response{
		MediumPath: buildImageName,
		MediumType: build.ImageMediumType,
	}, nil
}

// stopPreBuildJob stops any previous build jobs for the service (best-effort).
func (b *Builder) stopPreBuildJob(re *build.Request) {
	jobList, err := b.jobCtrl.GetServiceJobs(re.ServiceID)
	if err != nil {
		logrus.Errorf("get pre build job for service %s failure: %s", re.ServiceID, err.Error())
		return
	}
	for _, job := range jobList {
		b.jobCtrl.DeleteJob(job.Name)
	}
}

// validateProjectFiles checks for common project configuration issues
func (b *Builder) validateProjectFiles(re *build.Request) error {
	lockFiles := []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml"}
	for _, lockFile := range lockFiles {
		lockfilePath := filepath.Join(re.SourceDir, lockFile)
		if info, err := os.Stat(lockfilePath); err == nil {
			if info.Size() == 0 {
				msg := fmt.Sprintf("ERROR: %s is empty. Please run the appropriate install command (npm install / yarn install / pnpm install) to generate a valid lock file, then commit and push.", lockFile)
				re.Logger.Error(msg, map[string]string{"step": "build-code"})
				return fmt.Errorf("empty lock file: %s", lockFile)
			}
		}
	}
	return nil
}

// setSourceDirPermissions sets the source directory permissions recursively for CNB user
func (b *Builder) setSourceDirPermissions(re *build.Request) error {
	gitDir := filepath.Join(re.SourceDir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		if err := os.RemoveAll(gitDir); err != nil {
			logrus.Warnf("failed to remove .git directory: %v", err)
		}
	}
	return filepath.WalkDir(re.SourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chmod(path, 0777)
	})
}
