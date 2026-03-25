// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package build

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/goodrain/rainbond/db"

	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/docker/docker/api/types"
)

func init() {
	buildcreaters = make(map[code.Lang]CreaterBuild)
	buildcreaters[code.Dockerfile] = dockerfileBuilder
	buildcreaters[code.Docker] = dockerfileBuilder
	buildcreaters[code.NetCore] = customDockerBuilder
	buildcreaters[code.JavaJar] = slugBuilder
	buildcreaters[code.JavaMaven] = slugBuilder
	buildcreaters[code.JaveWar] = slugBuilder
	buildcreaters[code.PHP] = slugBuilder
	buildcreaters[code.Python] = slugBuilder
	buildcreaters[code.Nodejs] = slugBuilder
	buildcreaters[code.Golang] = slugBuilder
	buildcreaters[code.OSS] = slugBuilder
	buildcreaters[code.NodeJSDockerfile] = customDockerBuilder
	buildcreaters[code.VMDockerfile] = customDockerBuilder
}

var buildcreaters map[code.Lang]CreaterBuild

// cnbCreater is registered by the cnb subpackage via init()
var cnbCreater CreaterBuild

const (
	// SourceSlugRemovalGateEnv controls whether legacy source slug builds are blocked.
	SourceSlugRemovalGateEnv = "ENABLE_SOURCE_SLUG_REMOVAL_GATE"
)

var (
	// ErrLegacySlugSourceBuildDisabled indicates that source slug build execution is disabled by gate.
	ErrLegacySlugSourceBuildDisabled = errors.New("legacy slug source builds are disabled")
	sourceSlugRemovalGateEnabled     = func() bool {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(SourceSlugRemovalGateEnv))) {
		case "1", "t", "true", "y", "yes", "on":
			return true
		default:
			return false
		}
	}
)

// RegisterCNBBuilder registers the CNB builder factory (called from cnb package init)
func RegisterCNBBuilder(fn CreaterBuild) {
	cnbCreater = fn
}

// IsLegacySlugSourceBuildDisabled reports whether err was caused by the slug removal gate.
func IsLegacySlugSourceBuildDisabled(err error) bool {
	return errors.Is(err, ErrLegacySlugSourceBuildDisabled)
}

func legacySlugSourceBuildDisabledError() error {
	return fmt.Errorf(
		"%w: slug source builds were removed in 6.8.0; migrate this component to build_strategy=cnb via slug-to-cnb migration before rebuilding",
		ErrLegacySlugSourceBuildDisabled,
	)
}

// Build app build pack
type Build interface {
	Build(*Request) (*Response, error)
}

// CreaterBuild CreaterBuild
type CreaterBuild func() (Build, error)

// MediumType Build output medium type
type MediumType string

// ImageMediumType image type
var ImageMediumType MediumType = "image"

// SlugMediumType slug type
var SlugMediumType MediumType = "slug"

// ImageBuildNetworkModeHost use host network mode during docker build
var ImageBuildNetworkModeHost = "host"

// Response build result
type Response struct {
	MediumPath string
	MediumType MediumType
}

// Request build input
type Request struct {
	BuildKitImage    string
	BuildKitArgs     []string
	BuildKitCache    bool
	RbdNamespace     string
	GRDataPVCName    string
	CachePVCName     string
	CacheMode        string
	CachePath        string
	TenantID         string
	SourceDir        string
	CacheDir         string
	TGZDir           string
	RepositoryURL    string
	CodeSouceInfo    sources.CodeSourceInfo
	Branch           string
	ServiceAlias     string
	ServiceID        string
	DeployVersion    string
	Runtime          string
	ServerType       string
	BuildStrategy    string
	CNBVersionPolicy *CNBVersionPolicy
	Commit           Commit
	Lang             code.Lang
	BuildEnvs        map[string]string
	Logger           event.Logger
	ImageClient      sources.ImageClient
	KubeClient       kubernetes.Interface
	ExtraHosts       []string
	HostAlias        []HostAlias
	Ctx              context.Context
	Arch             string
	BRVersion        string
}

// CNBVersionPolicy is the normalized policy snapshot sent by console for CNB builds.
type CNBVersionPolicy struct {
	Version   int                          `json:"version"`
	Languages map[string]CNBLanguagePolicy `json:"languages"`
}

// CNBLanguagePolicy stores the policy for one CNB language.
type CNBLanguagePolicy struct {
	LangKey         string   `json:"lang_key"`
	VisibleVersions []string `json:"visible_versions,omitempty"`
	AllowedVersions []string `json:"allowed_versions,omitempty"`
	DefaultVersion  string   `json:"default_version,omitempty"`
}

// HostAlias holds the mapping between IP and hostnames that will be injected as an entry in the
// pod's hosts file.
type HostAlias struct {
	// IP address of the host file entry.
	IP string `json:"ip,omitempty" protobuf:"bytes,1,opt,name=ip"`
	// Hostnames for the above IP address.
	Hostnames []string `json:"hostnames,omitempty" protobuf:"bytes,2,rep,name=hostnames"`
}

// Commit Commit
type Commit struct {
	User    string
	Message string
	Hash    string
}

// GetBuild GetBuild
func GetBuild(lang code.Lang) (Build, error) {
	if fun, ok := buildcreaters[lang]; ok {
		return fun()
	}
	return slugBuilder()
}

// GetBuildByType returns a builder based on build type
// For Node.js with CNB build type, returns CNB builder
// For other cases, falls back to the default builder for the language
func GetBuildByType(lang code.Lang, buildType string) (Build, error) {
	switch strings.ToLower(strings.TrimSpace(buildType)) {
	case "cnb":
		if cnbCreater == nil {
			return nil, fmt.Errorf("CNB builder not registered")
		}
		// Support Node.js projects (with package.json)
		langStr := string(lang)
		if lang == code.Nodejs || strings.Contains(langStr, string(code.Nodejs)) {
			return cnbCreater()
		}
		// Support pure static projects (no package.json, only HTML)
		if lang == code.Static || strings.Contains(langStr, string(code.Static)) {
			return cnbCreater()
		}
		if lang == code.JavaMaven || lang == code.JaveWar || lang == code.JavaJar || lang == code.Gradle {
			return cnbCreater()
		}
		if lang == code.Python || lang == code.PHP || lang == code.Golang {
			return cnbCreater()
		}
		if lang == code.NetCore {
			return cnbCreater()
		}
		// Other languages fall back to default builder
		return GetBuild(lang)
	case "slug":
		if sourceSlugRemovalGateEnabled() {
			return nil, legacySlugSourceBuildDisabledError()
		}
		return GetBuild(lang)
	default:
		return GetBuild(lang)
	}
}

// CreateImageName create image name
func CreateImageName(serviceID, deployversion string) string {
	imageName := strings.ToLower(fmt.Sprintf("%s/%s:%s", builder.REGISTRYDOMAIN, serviceID, deployversion))
	component, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return imageName
	}
	app, err := db.GetManager().ApplicationDao().GetByServiceID(serviceID)
	if err != nil {
		return imageName
	}
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(component.TenantID)
	if err != nil {
		return imageName
	}
	workloadName := fmt.Sprintf("%s-%s-%s", tenant.Namespace, app.K8sApp, component.K8sComponentName)
	return strings.ToLower(fmt.Sprintf("%s/%s:%s", builder.REGISTRYDOMAIN, workloadName, deployversion))
}

// GetTenantRegistryAuthSecrets GetTenantRegistryAuthSecrets
func GetTenantRegistryAuthSecrets(ctx context.Context, tenantID string, kcli kubernetes.Interface) map[string]types.AuthConfig {
	auths := make(map[string]types.AuthConfig)
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(tenantID)
	if err != nil {
		return auths
	}
	registrySecrets, err := kcli.CoreV1().Secrets(tenant.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "rainbond.io/registry-auth-secret=true",
	})
	if err == nil {
		for _, secret := range registrySecrets.Items {
			d := string(secret.Data["Domain"])
			u := string(secret.Data["Username"])
			p := string(secret.Data["Password"])
			auths[d] = types.AuthConfig{
				Username: u,
				Password: p,
				Auth:     base64.StdEncoding.EncodeToString([]byte(u + ":" + p)),
			}
		}
	}
	return auths
}

// CreateAuthSecret creates the registry authentication secret (used by both dockerfile and CNB builds)
func CreateAuthSecret(re *Request) (corev1.Secret, error) {
	d := &dockerfileBuild{}
	return d.createAuthSecret(re)
}

// DeleteAuthSecret deletes the registry authentication secret
func DeleteAuthSecret(re *Request, secretName string) {
	d := &dockerfileBuild{}
	d.deleteAuthSecret(re, secretName)
}
