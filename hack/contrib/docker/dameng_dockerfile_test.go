package docker

import (
	"os"
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
				"go mod edit -require=dm@v0.0.0 -replace=dm=/tmp/dameng/dm",
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
			if !strings.Contains(text, "DAMENG_DRIVER_BUNDLE_IMAGE") {
				t.Fatal("workflow must validate DAMENG_DRIVER_BUNDLE_IMAGE before builds")
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

	validationIndex := strings.Index(job, "test -n \"$DAMENG_DRIVER_BUNDLE_IMAGE\"")
	if validationIndex < 0 {
		t.Fatal("region build must validate a nonempty Dameng driver bundle image")
	}
	ifIndex := strings.LastIndex(job[:validationIndex], "if: ${{ matrix.component.requires_dameng }}")
	if ifIndex < 0 {
		t.Fatal("region driver bundle validation must only run for Dameng components")
	}
	buildIndex := strings.Index(job, "uses: docker/build-push-action@v6")
	if validationIndex > buildIndex {
		t.Fatal("region build must validate the Dameng driver bundle image before build")
	}

	const conditionalContext = "build-contexts: ${{ matrix.component.requires_dameng && format('dameng=docker-image://{0}', vars.DAMENG_DRIVER_BUNDLE_IMAGE) || '' }}"
	if !strings.Contains(job, conditionalContext) {
		t.Fatal("region build must only resolve the Dameng named build context for Dameng components")
	}
}

func assertAlwaysDamengBuildScope(t *testing.T, job string) {
	t.Helper()
	validationIndex := strings.Index(job, "test -n \"$DAMENG_DRIVER_BUNDLE_IMAGE\"")
	if validationIndex < 0 {
		t.Fatal("all-in-one build must validate a nonempty Dameng driver bundle image")
	}
	buildIndex := strings.Index(job, "uses: docker/build-push-action@v6")
	if validationIndex > buildIndex {
		t.Fatal("all-in-one build must validate the Dameng driver bundle image before build")
	}
	if !strings.Contains(job, "dameng=docker-image://${{ vars.DAMENG_DRIVER_BUNDLE_IMAGE }}") {
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
