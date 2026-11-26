package exector

import (
	"errors"
	"strings"
	"testing"
)

// capability_id: rainbond.vm-build.vmdk-runtime-binding
func TestVMDKRuntimeConsumerMustUseThisValidatedRegistryImage(t *testing.T) {
	for _, raw := range []string{
		`{"disk":{"source_type":"registry","format":"vmdk","image_url":"docker://goodrain.me/zqh/windows:20261123"}}`,
		`[{"volume_name":"disk","source_type":"registry","format":"vmdk","image_url":"zqh/windows:20261123"}]`,
		`[{"disk_key":"disk","source_type":" registry ","format":" VMDK ","image_url":"docker://zqh/windows:20261123"}]`,
		`{"disk":{"source_type":"registry","format":"qcow2","image_url":"docker://goodrain.me/zqh/windows:20261123"}}`,
		`[{"volume_name":"disk","source_type":"registry","format":"qcow2","image_url":"zqh/windows:20261123"}]`,
	} {
		if err := validateVMDiskRuntimeConsumer(raw, "zqh/windows:20261123", "goodrain.me/zqh/windows:20261123"); err != nil {
			t.Fatal("validated task registry binding rejected", err)
		}
	}
	for _, raw := range []string{
		`{"disk":{"source_type":"http","format":"qcow2","image_url":"https://example.invalid/renamed.img"}}`,
		`{"disk":{"source_type":"http","format":"vmdk","image_url":"https://example.invalid/source.vmdk"}}`,
		`{"disk":{"source_type":"registry","format":"iso","image_url":"goodrain.me/zqh/windows:20261123"}}`,
		`{"disk":{"source_type":"registry","format":"","image_url":"goodrain.me/zqh/windows:20261123"}}`,
		`{"disk":{"source_type":"registry","format":"vmdk","image_url":"goodrain.me/zqh/windows:another-task"}}`,
		`{"disk":{"source_type":"registry","format":"qcow2","image_url":"goodrain.me/other:20261123"}}`,
		`{"disk":{"source_type":"disk","format":"tar","image_url":"https://example.invalid/renamed.tar"}}`,
		`{}`, `null`, `{"data":{"source_type":"registry","format":"qcow2","image_url":"goodrain.me/zqh/windows:20261123"}}`,
	} {
		if err := validateVMDiskRuntimeConsumer(raw, "zqh/windows:20261123", "goodrain.me/zqh/windows:20261123"); err == nil {
			t.Fatal("unvalidated/unrelated root consumer accepted", raw)
		}
	}
	good := `{"disk":{"source_type":"registry","format":"vmdk","image_url":"zqh/windows:20261123"}}`
	item := &VMBuildItem{Image: "zqh/windows:20261123", readVMRootImports: func() (string, error) { return good, nil }}
	if err := item.validateVMDiskRuntimeBinding(); err != nil {
		t.Fatal(err)
	}
	item.readVMRootImports = func() (string, error) { return "", errors.New("read unavailable") }
	if err := item.validateVMDiskRuntimeBinding(); err == nil {
		t.Fatal("binding read error was hidden")
	}
	item.SkipVersionStore = true
	item.readVMRootImports = func() (string, error) {
		t.Fatal("delivery-only artifact must not claim a runtime consumer")
		return "", nil
	}
	if err := item.validateVMDiskRuntimeBinding(); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-build.vmdk-runtime-binding
func TestVMDKBareInternalImageNeverMatchesExternalDockerHub(t *testing.T) {
	for _, format := range []string{"vmdk", "qcow2"} {
		for _, actual := range []string{"zqh:vm-id", "docker://zqh:vm-id", "goodrain.me/zqh:vm-id", "docker://goodrain.me/zqh:vm-id"} {
			raw := `{"disk":{"source_type":"registry","format":"` + format + `","image_url":"` + actual + `"}}`
			if err := validateVMDiskRuntimeConsumer(raw, "zqh:vm-id", "goodrain.me/zqh:vm-id"); err != nil {
				t.Fatal("internal task image rejected", format, actual, err)
			}
		}
		for _, actual := range []string{"docker.io/library/zqh:vm-id", "docker://docker.io/library/zqh:vm-id", "index.docker.io/library/zqh:vm-id", "docker.io/zqh:vm-id", "other.example/zqh:vm-id", "goodrain.me/zqh:another-task"} {
			raw := `{"disk":{"source_type":"registry","format":"` + format + `","image_url":"` + actual + `"}}`
			if err := validateVMDiskRuntimeConsumer(raw, "zqh:vm-id", "goodrain.me/zqh:vm-id"); err == nil {
				t.Fatal("unrelated registry image falsely matched internal task image", format, actual)
			}
		}
	}
}

// capability_id: rainbond.vm-build.vmdk-runtime-binding
func TestVMDKValidatedRuntimeConsumerRequiresOneRootDisk(t *testing.T) {
	for _, raw := range []string{
		`{"disk":{"source_type":"registry","format":"vmdk","image_url":"goodrain.me/zqh:vm-id"},"second":{"volume_name":"disk","source_type":"registry","format":"qcow2","image_url":"goodrain.me/zqh:vm-id"}}`,
		`[{"disk_key":"disk","source_type":"registry","format":"vmdk","image_url":"goodrain.me/zqh:vm-id"},{"volume_name":"disk","source_type":"registry","format":"qcow2","image_url":"goodrain.me/zqh:vm-id"}]`,
		`{"data":{"source_type":"registry","format":"vmdk","image_url":"goodrain.me/zqh:vm-id"}}`,
	} {
		if err := validateVMDiskRuntimeConsumer(raw, "zqh:vm-id", "goodrain.me/zqh:vm-id"); err == nil {
			t.Fatal("missing or duplicate root disk accepted", raw)
		}
	}
	message := invalidVMDiskConsumer().Error()
	if !strings.Contains(message, "本次校验发布的磁盘镜像") || strings.Contains(message, "QCOW2") || strings.Contains(message, "转换") {
		t.Fatal("binding rejection incorrectly requires a converted QCOW2 image", message)
	}
}
