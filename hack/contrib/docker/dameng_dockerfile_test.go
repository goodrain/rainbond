package docker

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// capability_id: rainbond.database.dameng-standard-images
func TestStandardImagesIncludeDamengDriverWithoutUPX(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "api", path: "api/Dockerfile"},
		{name: "worker", path: "worker/Dockerfile"},
		{name: "chaos", path: "chaos/Dockerfile"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("read Dockerfile: %v", err)
			}

			text := string(contents)
			buildStage, finalStage, found := strings.Cut(text, "FROM rainbond/alpine:3")
			if !found {
				t.Fatal("standard Dockerfile must have a final rainbond/alpine stage")
			}
			for _, expected := range []string{
				"COPY --from=dameng go/ /tmp/dameng/",
				"sh scripts/prepare-dameng-go-driver.sh /tmp/dameng",
				"go mod edit -require=github.com/goodrain/dameng-driver@v0.0.0 -replace=github.com/goodrain/dameng-driver=/tmp/dameng/dm-driver",
				"go mod edit -require=github.com/goodrain/dameng-gorm-dialect@v0.0.0 -replace=github.com/goodrain/dameng-gorm-dialect=/tmp/dameng/dm-dialect",
				`-tags "dm sqlite_omit_load_extension netgo"`,
				"unzip",
			} {
				if !strings.Contains(buildStage, expected) {
					t.Fatalf("standard Dockerfile must include Dameng driver setup %q", expected)
				}
			}
			for _, forbidden := range []string{"ENABLE_DM", "upx", " AS compress"} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("standard Dockerfile must not contain %q", forbidden)
				}
			}
			for _, forbidden := range []string{
				"--from=dameng",
				"/tmp/dameng",
				"dmgorm1.zip",
				"dm-go-driver.zip",
				"dm.zip",
				"gorm_v1_dialect.zip",
			} {
				if strings.Contains(finalStage, forbidden) {
					t.Fatalf("final image stage must not contain Dameng build input %q", forbidden)
				}
			}
		})
	}

	driverAdapter, err := os.ReadFile("../../../db/dameng/driver_dm.go")
	if err != nil {
		t.Fatalf("read Dameng driver adapter: %v", err)
	}
	if !strings.Contains(string(driverAdapter), `import _ "github.com/goodrain/dameng-gorm-dialect"`) {
		t.Fatal("Dameng driver adapter must import the GORM dialect as a separate module")
	}
}

// capability_id: rainbond.database.dameng-driver-bundle-preparation
func TestPrepareDamengDriverBundleFromISO(t *testing.T) {
	temporaryDirectory := t.TempDir()
	installerPath := filepath.Join(temporaryDirectory, "DMInstall.bin")
	if err := os.WriteFile(installerPath, fakeDamengInstaller(t), 0o600); err != nil {
		t.Fatalf("write fake installer: %v", err)
	}

	isoPath := filepath.Join(temporaryDirectory, "dm8.iso")
	if err := os.WriteFile(isoPath, []byte("fake ISO"), 0o600); err != nil {
		t.Fatalf("write fake ISO: %v", err)
	}

	toolDirectory := filepath.Join(temporaryDirectory, "tools")
	if err := os.Mkdir(toolDirectory, 0o700); err != nil {
		t.Fatalf("create tool directory: %v", err)
	}
	bsdtarPath := filepath.Join(toolDirectory, "bsdtar")
	const fakeBSDTar = `#!/usr/bin/env bash
set -euo pipefail
case "$1" in
  -tf) printf '%s\n' 'DMInstall.bin' ;;
  -xOf) cat "$DM_INSTALLER" ;;
  *) exit 2 ;;
esac
`
	if err := os.WriteFile(bsdtarPath, []byte(fakeBSDTar), 0o700); err != nil {
		t.Fatalf("write fake bsdtar: %v", err)
	}

	outputPath := filepath.Join(temporaryDirectory, "bundle")
	command := exec.Command("bash", "../../../scripts/prepare-dameng-driver-bundle-from-iso.sh", isoPath, outputPath)
	command.Env = append(os.Environ(), "DM_INSTALLER="+installerPath, "PATH="+toolDirectory+":"+os.Getenv("PATH"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("prepare minimal driver bundle: %v\n%s", err, output)
	}

	for _, expectedPath := range []string{
		"go/dm-go-driver.zip",
		"go/gorm_v1_dialect.zip",
		"python/dpi/libdmdpi.so",
		"python/dpi/dependencies/libcrypto.so",
		"python/dpi/include/DPI.h",
		"python/drivers/python/dmPython/setup.py",
		"python/drivers/python/dmDjango/dmDjango3.0/setup.py",
	} {
		if _, err := os.Stat(filepath.Join(outputPath, expectedPath)); err != nil {
			t.Fatalf("minimal bundle must contain %s: %v", expectedPath, err)
		}
	}
	for _, forbiddenPath := range []string{
		"python/include",
		"source/include/not-required.h",
		"python/drivers/python/dmDjango/dmDjango2.0",
	} {
		if _, err := os.Stat(filepath.Join(outputPath, forbiddenPath)); !os.IsNotExist(err) {
			t.Fatalf("minimal bundle must not contain %s", forbiddenPath)
		}
	}

	bundleDockerfile, err := os.ReadFile("dameng-driver-bundle.Dockerfile")
	if err != nil {
		t.Fatalf("read driver bundle Dockerfile: %v", err)
	}
	for _, expected := range []string{"FROM scratch", "COPY go/ /go/", "COPY python/ /python/"} {
		if !strings.Contains(string(bundleDockerfile), expected) {
			t.Fatalf("driver bundle Dockerfile must contain %q", expected)
		}
	}

	preparationScript, err := os.ReadFile("../../../scripts/prepare-dameng-driver-bundle-from-iso.sh")
	if err != nil {
		t.Fatalf("read ISO preparation script: %v", err)
	}
	if !strings.Contains(string(preparationScript), "LC_ALL=C sed") {
		t.Fatal("ISO preparation must parse the binary installer in the C locale")
	}
	if !strings.Contains(string(preparationScript), "tar_status=${PIPESTATUS[1]}") {
		t.Fatal("ISO preparation must distinguish a successful tar extraction from an upstream broken pipe")
	}
}

// capability_id: rainbond.database.dameng-go-dialect-module
func TestPrepareDamengGoDriverSeparatesDialectModule(t *testing.T) {
	bundleDirectory := t.TempDir()
	writeZipFile(t, filepath.Join(bundleDirectory, "dm-go-driver.zip"), map[string]string{
		"dm/go.mod":    "module dm\n\ngo 1.13\n",
		"dm/driver.go": "package dm\n",
	})
	writeZipFile(t, filepath.Join(bundleDirectory, "gorm_v1_dialect.zip"), map[string]string{
		"dm/dialect_dm.go": "package dm\n\nimport _ \"dm\"\n",
	})

	prepare := exec.Command("sh", "../../../scripts/prepare-dameng-go-driver.sh", bundleDirectory)
	if output, err := prepare.CombinedOutput(); err != nil {
		t.Fatalf("prepare Go driver modules: %v\n%s", err, output)
	}

	for _, expectedPath := range []string{
		"dm-driver/go.mod",
		"dm-driver/driver.go",
		"dm-dialect/go.mod",
		"dm-dialect/dialect_dm.go",
	} {
		if _, err := os.Stat(filepath.Join(bundleDirectory, expectedPath)); err != nil {
			t.Fatalf("prepared Go bundle must contain %s: %v", expectedPath, err)
		}
	}
	if _, err := os.Stat(filepath.Join(bundleDirectory, "dm-driver", "dialect_dm.go")); !os.IsNotExist(err) {
		t.Fatal("GORM dialect must not be copied into the dm driver module")
	}

	dialectModule, err := os.ReadFile(filepath.Join(bundleDirectory, "dm-dialect", "go.mod"))
	if err != nil {
		t.Fatalf("read generated dialect module: %v", err)
	}
	if !strings.Contains(string(dialectModule), "module github.com/goodrain/dameng-gorm-dialect") ||
		!strings.Contains(string(dialectModule), "replace github.com/goodrain/dameng-driver => ../dm-driver") {
		t.Fatal("GORM dialect module must use a distinct module path and the local dm driver")
	}
	driverModule, err := os.ReadFile(filepath.Join(bundleDirectory, "dm-driver", "go.mod"))
	if err != nil {
		t.Fatalf("read generated driver module: %v", err)
	}
	if !strings.Contains(string(driverModule), "module github.com/goodrain/dameng-driver") {
		t.Fatal("prepared driver module must rewrite the official short module path")
	}

	consumerDirectory := t.TempDir()
	consumerModule := "module dm-consumer\n\ngo 1.25.0\n\nrequire (\n\tgithub.com/goodrain/dameng-driver v0.0.0\n\tgithub.com/goodrain/dameng-gorm-dialect v0.0.0\n)\n\nreplace github.com/goodrain/dameng-driver => " + filepath.Join(bundleDirectory, "dm-driver") + "\nreplace github.com/goodrain/dameng-gorm-dialect => " + filepath.Join(bundleDirectory, "dm-dialect") + "\n"
	if err := os.WriteFile(filepath.Join(consumerDirectory, "go.mod"), []byte(consumerModule), 0o600); err != nil {
		t.Fatalf("write consumer go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(consumerDirectory, "main.go"), []byte("package main\n\nimport _ \"github.com/goodrain/dameng-gorm-dialect\"\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatalf("write consumer main: %v", err)
	}
	consumer := exec.Command("go", "run", "-mod=mod", ".")
	consumer.Dir = consumerDirectory
	if output, err := consumer.CombinedOutput(); err != nil {
		t.Fatalf("compile separated Go driver and dialect modules: %v\n%s", err, output)
	}
}

func writeZipFile(t *testing.T, path string, files map[string]string) {
	t.Helper()

	destination, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip %s: %v", path, err)
	}
	zipWriter := zip.NewWriter(destination)
	for name, contents := range files {
		entry, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip %s: %v", path, err)
	}
	if err := destination.Close(); err != nil {
		t.Fatalf("close zip file %s: %v", path, err)
	}
}

func fakeDamengInstaller(t *testing.T) []byte {
	t.Helper()

	var compressedPayload bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedPayload)
	tarWriter := tar.NewWriter(gzipWriter)
	directories := []string{
		"source/",
		"source/include/",
		"source/drivers/",
		"source/drivers/go/",
		"source/drivers/dpi/",
		"source/drivers/dpi/dependencies/",
		"source/drivers/dpi/include/",
		"source/drivers/python/",
		"source/drivers/python/dmPython/",
		"source/drivers/python/dmDjango/",
		"source/drivers/python/dmDjango/dmDjango2.0/",
		"source/drivers/python/dmDjango/dmDjango3.0/",
	}
	for _, directory := range directories {
		if err := tarWriter.WriteHeader(&tar.Header{Name: directory, Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
			t.Fatalf("write fake driver directory %s: %v", directory, err)
		}
	}
	files := map[string]string{
		"source/include/not-required.h":                              "unrelated full DM include tree",
		"source/drivers/go/dm-go-driver.zip":                         "go driver",
		"source/drivers/go/gorm_v1_dialect.zip":                      "gorm dialect",
		"source/drivers/dpi/libdmdpi.so":                             "dpi runtime",
		"source/drivers/dpi/dependencies/libcrypto.so":               "dpi dependency",
		"source/drivers/dpi/include/DPI.h":                           "dpi header",
		"source/drivers/python/dmPython/setup.py":                    "dmPython source",
		"source/drivers/python/dmDjango/dmDjango2.0/setup.py":        "old adapter",
		"source/drivers/python/dmDjango/dmDjango3.0/setup.py":        "supported adapter",
	}
	for name, contents := range files {
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(contents))}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write fake driver header %s: %v", name, err)
		}
		if _, err := tarWriter.Write([]byte(contents)); err != nil {
			t.Fatalf("write fake driver contents %s: %v", name, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close fake driver tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close fake driver gzip: %v", err)
	}

	return append([]byte("#!/usr/bin/env bash\nskip=3\n"), compressedPayload.Bytes()...)
}

// capability_id: rainbond.database.dameng-standard-image-actions
func TestStandardDamengBuildWorkflows(t *testing.T) {
	workflows := []struct {
		name string
		path string
	}{
		{name: "development", path: "../../../.github/workflows/dev-build.yml"},
		{name: "release", path: "../../../.github/workflows/release-v6.yml"},
	}

	for _, workflow := range workflows {
		t.Run(workflow.name, func(t *testing.T) {
			contents, err := os.ReadFile(workflow.path)
			if err != nil {
				t.Fatalf("read workflow: %v", err)
			}

			text := string(contents)
			if !strings.Contains(text, "DAMENG_DRIVER_BUNDLE_IMAGE: registry.cn-hangzhou.aliyuncs.com/zhangqihang/rainbond-dameng-driver-bundle@sha256:") {
				t.Fatal("workflow must declare a fixed, digest-pinned Dameng driver bundle image")
			}
			if strings.Contains(text, "vars.DAMENG_DRIVER_BUNDLE_IMAGE") {
				t.Fatal("workflow must not require an Action user to configure a Dameng driver bundle variable")
			}
			if strings.Contains(text, "test -n \"$DAMENG_DRIVER_BUNDLE_IMAGE\"") {
				t.Fatal("workflow must validate the fixed Dameng image instead of requiring a user-provided variable")
			}
			if strings.Contains(text, "echo \"$DAMENG_DRIVER_BUNDLE_IMAGE\"") || strings.Contains(text, "echo $DAMENG_DRIVER_BUNDLE_IMAGE") {
				t.Fatal("workflow must not print DAMENG_DRIVER_BUNDLE_IMAGE")
			}

			regionJob := workflowJob(t, text, "rainbond-region:")
			assertRegionDamengBuildScope(t, regionJob)

			allInOneJob := workflowJob(t, text, "rainbond-allinone:")
			assertAlwaysDamengBuildScope(t, allInOneJob)
			if workflow.name == "development" {
				assertConsoleBranchIsRequired(t, allInOneJob)
			}
		})
	}
}

func TestBuildPushActionsDisableRegistryAttestations(t *testing.T) {
	workflows := []struct {
		name string
		path string
	}{
		{name: "development", path: "../../../.github/workflows/dev-build.yml"},
		{name: "release", path: "../../../.github/workflows/release-v6.yml"},
	}

	for _, workflow := range workflows {
		t.Run(workflow.name, func(t *testing.T) {
			contents, err := os.ReadFile(workflow.path)
			if err != nil {
				t.Fatalf("read workflow: %v", err)
			}

			steps := buildPushActionSteps(string(contents))
			if len(steps) == 0 {
				t.Fatal("workflow must contain docker/build-push-action@v6 steps")
			}
			for index, step := range steps {
				for _, expected := range []string{"provenance: false", "sbom: false"} {
					if !strings.Contains(step, expected) {
						t.Fatalf("build-push step %d must set %s", index+1, expected)
					}
				}
			}
		})
	}
}

func buildPushActionSteps(workflow string) []string {
	lines := strings.Split(workflow, "\n")
	var steps []string
	for index, line := range lines {
		if strings.TrimSpace(line) != "uses: docker/build-push-action@v6" {
			continue
		}

		start := index
		for start > 0 && !strings.HasPrefix(lines[start], "      - ") {
			start--
		}
		if !strings.HasPrefix(lines[start], "      - ") {
			continue
		}

		end := index + 1
		for end < len(lines) && !strings.HasPrefix(lines[end], "      - ") {
			end++
		}
		steps = append(steps, strings.Join(lines[start:end], "\n"))
	}
	return steps
}

func assertRegionDamengBuildScope(t *testing.T, job string) {
	t.Helper()
	for _, component := range []string{"api", "chaos", "worker"} {
		if !strings.Contains(job, "- name: "+component+"\n            requires_dameng: true") {
			t.Fatalf("region matrix must mark %s as requiring the Dameng driver bundle", component)
		}
	}
	for _, component := range []string{"init-probe", "mq"} {
		if !strings.Contains(job, "- name: "+component+"\n            requires_dameng: false") {
			t.Fatalf("region matrix must mark %s as not requiring the Dameng driver bundle", component)
		}
	}

	validationIndex := strings.Index(job, "docker buildx imagetools inspect \"$DAMENG_DRIVER_BUNDLE_IMAGE\"")
	if validationIndex < 0 {
		t.Fatal("region build must validate the fixed Dameng driver bundle image")
	}
	ifIndex := strings.LastIndex(job[:validationIndex], "if: ${{ matrix.component.requires_dameng }}")
	if ifIndex < 0 {
		t.Fatal("region driver bundle validation must only run for Dameng components")
	}
	buildIndex := strings.Index(job, "uses: docker/build-push-action@v6")
	if validationIndex > buildIndex {
		t.Fatal("region build must validate the Dameng driver bundle image before build")
	}
	registryLoginIndex := strings.Index(job, "- name: Login to Aliyun Container Registry")
	if registryLoginIndex < 0 || validationIndex < registryLoginIndex {
		t.Fatal("region build must validate the Dameng driver bundle after registry login")
	}

	const conditionalContext = "build-contexts: ${{ matrix.component.requires_dameng && format('dameng=docker-image://{0}', env.DAMENG_DRIVER_BUNDLE_IMAGE) || '' }}"
	if !strings.Contains(job, conditionalContext) {
		t.Fatal("region build must only resolve the Dameng named build context for Dameng components")
	}
}

func assertAlwaysDamengBuildScope(t *testing.T, job string) {
	t.Helper()
	validationIndex := strings.Index(job, "docker buildx imagetools inspect \"$DAMENG_DRIVER_BUNDLE_IMAGE\"")
	if validationIndex < 0 {
		t.Fatal("all-in-one build must validate the fixed Dameng driver bundle image")
	}
	buildIndex := strings.Index(job, "uses: docker/build-push-action@v6")
	if validationIndex > buildIndex {
		t.Fatal("all-in-one build must validate the Dameng driver bundle image before build")
	}
	registryLoginIndex := strings.Index(job, "- name: Login to Aliyun Container Registry")
	if registryLoginIndex < 0 || validationIndex < registryLoginIndex {
		t.Fatal("all-in-one build must validate the Dameng driver bundle after registry login")
	}
	if !strings.Contains(job, "dameng=docker-image://${{ env.DAMENG_DRIVER_BUNDLE_IMAGE }}") {
		t.Fatal("all-in-one build must provide the Dameng named build context")
	}
	if strings.Contains(job, "matrix.component.requires_dameng") {
		t.Fatal("all-in-one build must always provide the Dameng named build context")
	}
}

func assertConsoleBranchIsRequired(t *testing.T, job string) {
	t.Helper()
	resolveRef := workflowStep(t, job, "Resolve console ref")
	if !strings.Contains(resolveRef, "git ls-remote --exit-code --heads https://github.com/goodrain/rainbond-console.git") {
		t.Fatal("console ref resolution must verify the requested remote branch")
	}
	if strings.Contains(resolveRef, "ref=main") {
		t.Fatal("console ref resolution must not fall back to main when the requested branch is absent")
	}
	if !strings.Contains(resolveRef, "::error::") || !strings.Contains(resolveRef, "exit 1") {
		t.Fatal("console ref resolution must emit an error and exit when the requested branch is absent")
	}
}

func workflowJob(t *testing.T, workflow, jobName string) string {
	t.Helper()
	lines := strings.Split(workflow, "\n")
	for index, line := range lines {
		if line != "  "+jobName {
			continue
		}
		end := len(lines)
		for next := index + 1; next < len(lines); next++ {
			if strings.HasPrefix(lines[next], "  ") && (len(lines[next]) == 2 || lines[next][2] != ' ') {
				end = next
				break
			}
		}
		return strings.Join(lines[index:end], "\n")
	}
	t.Fatalf("workflow must define %s", jobName)
	return ""
}

func workflowStep(t *testing.T, job, stepName string) string {
	t.Helper()
	needle := "      - name: " + stepName
	start := strings.Index(job, needle)
	if start < 0 {
		t.Fatalf("workflow job must define %s", stepName)
	}

	remaining := job[start+len(needle):]
	if end := strings.Index(remaining, "\n      - name: "); end >= 0 {
		return job[start : start+len(needle)+end]
	}
	return job[start:]
}
