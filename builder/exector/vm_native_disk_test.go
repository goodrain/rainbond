package exector

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capability_id: rainbond.vm-build.native-disk-artifacts
func TestNativeDiskBuildRejectsMissingRuntimeBindingBeforePublication(t *testing.T) {
	for _, name := range []string{"source.img", "source.raw", "source.qcow2"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, name), make([]byte, 512), 0600); err != nil {
				t.Fatal(err)
			}
			reads := 0
			item := &VMBuildItem{Image: "team/disk:build-one", readVMRootImports: func() (string, error) {
				reads++
				return `{}`, nil
			}}
			if err := item.vmBuild(dir); err == nil || !strings.Contains(err.Error(), "vm_disk_runtime_binding_invalid") {
				t.Fatalf("unbound disk must fail before any image build or publication: %v", err)
			}
			if reads != 1 {
				t.Fatal("every plain disk must verify its server-owned runtime consumer")
			}
			if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); !os.IsNotExist(err) {
				t.Fatal("unbound disk must not reach the image build stage")
			}
		})
	}
}

// capability_id: rainbond.vm-build.native-disk-artifacts
func TestNativeDiskArtifactsPreserveFormatForCDI(t *testing.T) {
	for _, name := range []string{"disk.qcow2", "disk.raw", "disk.img", "disk.vdi", "disk.vhd", "disk.VHDX", "磁盘.qcow2", "迁移.vhdx"} {
		t.Run(name, func(t *testing.T) {
			dockerfile, err := renderVMDockerfile(name)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"RUN --network=none ", "COPY " + name + " /work/source.img", "qemu-img info --output=json", "COPY --from=validate --chown=107:107 /work/source.img /disk/rootdisk.img"} {
				if !strings.Contains(dockerfile, want) {
					t.Fatal("disk must be validated before publishing its original bytes", want)
				}
			}
			if strings.Contains(dockerfile, "qemu-img convert") || strings.Contains(dockerfile, "rootdisk.qcow2") {
				t.Fatal("Builder must leave the single disk conversion to CDI")
			}
		})
	}
}

// capability_id: rainbond.vm-build.native-disk-artifacts
func TestNativeDiskRuntimeBindingRetainsSourceFormat(t *testing.T) {
	for _, format := range []string{"qcow2", "raw", "img", "vmdk", "vdi", "vhd", "vhdx"} {
		t.Run(format, func(t *testing.T) {
			const image = "goodrain.me/team/vm:build-one"
			raw := fmt.Sprintf(`{"disk":{"source_type":"registry","format":%q,"image_url":%q}}`, format, image)
			if err := validateVMDiskRuntimeConsumer(raw, "team/vm:build-one", image); err != nil {
				t.Fatal("published native disk must bind to this build", err)
			}
			if err := validateVMDiskRuntimeConsumer(raw, "team/vm:build-two", "goodrain.me/team/vm:build-two"); err == nil {
				t.Fatal("native disk must never consume another task's artifact")
			}
		})
	}
}
