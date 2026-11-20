//go:build integration

package exector

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// capability_id: rainbond.vm-build.img-content-detection
// capability_id: rainbond.vm-run.raw-image-format
func TestVMDiskConversionPreservesGuestContent(t *testing.T) {
	qemuImg := requireVMQEMUImg(t)
	for _, tt := range []struct {
		format   string
		fileName string
	}{
		{format: "qcow2", fileName: "source.img"},
		{format: "raw", fileName: "source.img"},
		{format: "qcow2", fileName: "乌班图.img"},
		{format: "raw", fileName: "乌班图.img"},
		{format: "raw", fileName: "source.raw"},
		{format: "raw", fileName: "磁盘.RAW"},
		{format: "qcow2", fileName: "source.raw"},
	} {
		t.Run(tt.format+"/"+tt.fileName, func(t *testing.T) {
			dir := t.TempDir()
			raw := make([]byte, 1024*1024)
			copy(raw[512:], []byte("Rainbond guest disk content regression"))
			raw[510], raw[511] = 0x55, 0xaa
			reference := filepath.Join(dir, "reference.raw")
			if err := os.WriteFile(reference, raw, 0600); err != nil {
				t.Fatal(err)
			}
			original := filepath.Join(dir, "original."+tt.format)
			runVMQEMUImg(t, qemuImg, "convert", "-f", "raw", "-O", tt.format, reference, original)
			if err := os.Rename(original, filepath.Join(dir, tt.fileName)); err != nil {
				t.Fatal(err)
			}
			if err := runVMConversionDockerfile(t, qemuImg, dir, tt.fileName); err != nil {
				t.Fatalf("convert %s content renamed to %s: %v", tt.format, tt.fileName, err)
			}
			converted := filepath.Join(dir, "rootdisk.qcow2")
			var info struct {
				Format string `json:"format"`
			}
			if err := json.Unmarshal(runVMQEMUImg(t, qemuImg, "info", "--output=json", converted), &info); err != nil {
				t.Fatal(err)
			}
			if info.Format != "qcow2" {
				t.Fatalf("output format = %q, want qcow2", info.Format)
			}
			// Compare guest-visible sectors, so a nested QCOW2 header cannot pass.
			runVMQEMUImg(t, qemuImg, "compare", "-f", "qcow2", "-F", "raw", converted, reference)
		})
	}
}

func TestVMDiskConversionRejectsInvalidOrDependentImages(t *testing.T) {
	qemuImg := requireVMQEMUImg(t)
	for _, kind := range []string{"unsupported format", "corrupt qcow2", "backing file", "external data file"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source.img")
			switch kind {
			case "unsupported format":
				runVMQEMUImg(t, qemuImg, "create", "-f", "vmdk", source, "1M")
			case "corrupt qcow2":
				runVMQEMUImg(t, qemuImg, "create", "-f", "qcow2", source, "1M")
				content, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				content[7] = 99 // Invalid QCOW2 version, retaining the probeable header.
				if err := os.WriteFile(source, content, 0600); err != nil {
					t.Fatal(err)
				}
			case "backing file":
				backing := filepath.Join(dir, "backing.raw")
				runVMQEMUImg(t, qemuImg, "create", "-f", "raw", backing, "1M")
				runVMQEMUImg(t, qemuImg, "create", "-f", "qcow2", "-F", "raw", "-b", backing, source)
			case "external data file":
				runVMQEMUImg(t, qemuImg, "create", "-f", "qcow2", "-o", "data_file="+filepath.Join(dir, "data.raw"), source, "1M")
			}
			if err := runVMConversionDockerfile(t, qemuImg, dir, "source.img"); err == nil {
				t.Fatal("expected image conversion to fail")
			}
			if _, err := os.Stat(filepath.Join(dir, "rootdisk.qcow2")); !os.IsNotExist(err) {
				t.Fatalf("rejected image must not produce an output disk: %v", err)
			}
		})
	}
}

func TestVMNativeRawConversionRejectsExternalReferences(t *testing.T) {
	qemuImg := requireVMQEMUImg(t)
	for _, external := range []string{"backing", "data"} {
		t.Run(external, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "disk.raw")
			if external == "backing" {
				backing := filepath.Join(dir, "backing.raw")
				runVMQEMUImg(t, qemuImg, "create", "-f", "raw", backing, "1M")
				runVMQEMUImg(t, qemuImg, "create", "-f", "qcow2", "-F", "raw", "-b", backing, source)
			} else {
				runVMQEMUImg(t, qemuImg, "create", "-f", "qcow2", "-o", "data_file="+filepath.Join(dir, "data.raw"), source, "1M")
			}
			if err := runVMConversionDockerfile(t, qemuImg, dir, "disk.raw"); err == nil {
				t.Fatal("raw extension must not bypass external reference rejection")
			}
			if _, err := os.Stat(filepath.Join(dir, "rootdisk.qcow2")); !os.IsNotExist(err) {
				t.Fatal("rejected image must not produce an output disk")
			}
		})
	}
}

func requireVMQEMUImg(t *testing.T) string {
	t.Helper()
	name := os.Getenv("QEMU_IMG")
	if name == "" {
		name = "qemu-img"
	}
	qemuImg, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("integration tests require qemu-img (or QEMU_IMG): %v", err)
	}
	qemuImg, err = filepath.Abs(qemuImg)
	if err != nil {
		t.Fatal(err)
	}
	return qemuImg
}

func runVMQEMUImg(t *testing.T, qemuImg string, args ...string) []byte {
	t.Helper()
	out, err := exec.Command(qemuImg, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("qemu-img %s: %v\n%s", args[0], err, out)
	}
	return out
}

func runVMConversionDockerfile(t *testing.T, qemuImg, dir, fileName string) error {
	t.Helper()
	dockerfile, err := renderVMDockerfile(fileName)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dockerfile, "\nCOPY "+fileName+" /work/source.img\n") {
		t.Fatalf("expected converter COPY to preserve source filename %q", fileName)
	}
	if fileName != "source.img" {
		// Stage the named upload at the fixed path used by the converter COPY.
		content, err := os.ReadFile(filepath.Join(dir, fileName))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "source.img"), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var commands []string
	for _, line := range strings.Split(dockerfile, "\n") {
		if strings.HasPrefix(line, "RUN ") {
			commands = append(commands, strings.TrimPrefix(line, "RUN "))
		}
	}
	if len(commands) != 1 {
		t.Fatalf("expected one conversion RUN instruction, got %d", len(commands))
	}
	// Execute the generated conversion instruction with only its container paths
	// relocated. The qemu-img binary and input content are real, without Docker.
	command := strings.ReplaceAll(commands[0], "/usr/bin/qemu-img", `"$QEMU_IMG"`)
	command = strings.ReplaceAll(command, "/work/", "./")
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "QEMU_IMG="+qemuImg)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		t.Logf("conversion output: %s", output.String())
		return err
	}
	return nil
}
