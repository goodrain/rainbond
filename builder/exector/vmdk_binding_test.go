package exector

import (
	"errors"
	"testing"
)

// capability_id: rainbond.vm-build.vmdk-runtime-binding
func TestVMDKRuntimeConsumerMustUseThisConvertedRegistryImage(t *testing.T) {
	for _, raw := range []string{
		`{"disk":{"source_type":"registry","format":"qcow2","image_url":"docker://goodrain.me/zqh/windows:20261123"}}`,
		`[{"volume_name":"disk","source_type":"registry","format":"qcow2","image_url":"zqh/windows:20261123"}]`,
	} {
		if err := validateVMDKRuntimeConsumer(raw, "zqh/windows:20261123", "goodrain.me/zqh/windows:20261123"); err != nil {
			t.Fatal("converted task registry binding rejected", err)
		}
	}
	for _, raw := range []string{
		`{"disk":{"source_type":"http","format":"qcow2","image_url":"https://example.invalid/renamed.img"}}`,
		`{"disk":{"source_type":"registry","format":"vmdk","image_url":"goodrain.me/zqh/windows:20261123"}}`,
		`{"disk":{"source_type":"registry","format":"qcow2","image_url":"goodrain.me/other:20261123"}}`,
		`{"disk":{"source_type":"disk","format":"tar","image_url":"https://example.invalid/renamed.tar"}}`,
		`{}`, `null`, `{"data":{"source_type":"registry","format":"qcow2","image_url":"goodrain.me/zqh/windows:20261123"}}`,
	} {
		if err := validateVMDKRuntimeConsumer(raw, "zqh/windows:20261123", "goodrain.me/zqh/windows:20261123"); err == nil {
			t.Fatal("unconverted/unrelated root consumer accepted", raw)
		}
	}
	good := `{"disk":{"source_type":"registry","format":"qcow2","image_url":"zqh/windows:20261123"}}`
	item := &VMBuildItem{Image: "zqh/windows:20261123", readVMRootImports: func() (string, error) { return good, nil }}
	if err := item.validateVMDKRuntimeBinding(); err != nil {
		t.Fatal(err)
	}
	item.readVMRootImports = func() (string, error) { return "", errors.New("read unavailable") }
	if err := item.validateVMDKRuntimeBinding(); err == nil {
		t.Fatal("binding read error was hidden")
	}
	item.SkipVersionStore = true
	item.readVMRootImports = func() (string, error) {
		t.Fatal("delivery-only artifact must not claim a runtime consumer")
		return "", nil
	}
	if err := item.validateVMDKRuntimeBinding(); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-build.vmdk-runtime-binding
func TestVMDKBareInternalImageNeverMatchesExternalDockerHub(t *testing.T) {
	for _, actual := range []string{"zqh:vm-id", "docker://zqh:vm-id", "goodrain.me/zqh:vm-id", "docker://goodrain.me/zqh:vm-id"} {
		raw := `{"disk":{"source_type":"registry","format":"qcow2","image_url":"` + actual + `"}}`
		if err := validateVMDKRuntimeConsumer(raw, "zqh:vm-id", "goodrain.me/zqh:vm-id"); err != nil {
			t.Fatal("internal task image rejected", actual, err)
		}
	}
	for _, actual := range []string{"docker.io/library/zqh:vm-id", "docker://docker.io/library/zqh:vm-id", "index.docker.io/library/zqh:vm-id", "docker.io/zqh:vm-id", "other.example/zqh:vm-id"} {
		raw := `{"disk":{"source_type":"registry","format":"qcow2","image_url":"` + actual + `"}}`
		if err := validateVMDKRuntimeConsumer(raw, "zqh:vm-id", "goodrain.me/zqh:vm-id"); err == nil {
			t.Fatal("external registry falsely matched bare internal task image", actual)
		}
	}
}
