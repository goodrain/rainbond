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
				runVMQEMUImg(t, qemuImg, "create", "-f", "vpc", source, "1M")
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
	dockerfile, err := renderVMSourceDockerfile(dir, fileName)
	if err != nil {
		return err
	}
	stagedName := fileName
	if strings.Contains(dockerfile, "\nCOPY source.vmdk /work/source.img\n") {
		stagedName = "source.vmdk"
	}
	if !strings.Contains(dockerfile, "\nCOPY "+stagedName+" /work/source.img\n") {
		t.Fatalf("expected converter COPY to preserve source filename %q", fileName)
	}
	if stagedName != "source.img" {
		// Stage the named upload at the fixed path used by the converter COPY.
		content, err := os.ReadFile(filepath.Join(dir, stagedName))
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
	command = strings.TrimPrefix(command, "--network=none ")
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

// capability_id: rainbond.vm-build.vmdk-self-contained
func TestVMDKConversionPreservesGuestSectorsAndEmbeddedBasename(t *testing.T) {
	qemu := requireVMQEMUImg(t)
	for _, kind := range []string{"monolithicSparse", "streamOptimized"} {
		for _, name := range []string{"windows.vmdk", "renamed.img", "renamed.qcow2", "Windows 10 x64.vmdk", "迁移 磁盘.vmdk"} {
			t.Run(kind+"/"+name, func(t *testing.T) {
				dir := t.TempDir()
				reference := filepath.Join(dir, "reference.raw")
				data := make([]byte, 2<<20)
				copy(data[512:], []byte("original VMware guest data must survive server conversion"))
				data[510] = 0x55
				data[511] = 0xaa
				if err := os.WriteFile(reference, data, 0600); err != nil {
					t.Fatal(err)
				}
				original := filepath.Join(dir, "Windows 10 x64.vmdk")
				runVMQEMUImg(t, qemu, "convert", "-f", "raw", "-O", "vmdk", "-o", "subformat="+kind, reference, original)
				if err := os.Rename(original, filepath.Join(dir, name)); err != nil {
					t.Fatal(err)
				}
				if err := runVMConversionDockerfile(t, qemu, dir, name); err != nil {
					t.Fatal(err)
				}
				out := filepath.Join(dir, "rootdisk.qcow2")
				runVMQEMUImg(t, qemu, "compare", "-f", "qcow2", "-F", "raw", out, reference)
				runVMQEMUImg(t, qemu, "check", "-f", "qcow2", out)
			})
		}
	}
}

// capability_id: rainbond.vm-build.vmdk-self-contained
func TestVMDKConversionRejectsParentsAndExternalFilesBeforeQEMU(t *testing.T) {
	qemu := requireVMQEMUImg(t)
	for _, mode := range []string{"parent CID", "parent hint", "comment parent hint", "external extent", "truncated grain data"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source.vmdk")
			runVMQEMUImg(t, qemu, "create", "-f", "vmdk", source, "1M")
			content, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			descriptor := content[512 : 512+20*512]
			end := bytes.IndexByte(descriptor, 0)
			text := string(descriptor[:end])
			switch mode {
			case "parent CID":
				text = strings.Replace(text, "parentCID=ffffffff", "parentCID=12345678", 1)
			case "parent hint":
				text += "parentFileNameHint=\"http://127.0.0.1:9/must-not-open\"\n"
			case "comment parent hint":
				text += "# parentFileNameHint=\"/outside/must-not-open\"\n"
			case "external extent":
				text = strings.Replace(text, " SPARSE ", " FLAT ", 1)
			case "truncated grain data":
				content = content[:32]
			}
			if len(content) > 512 {
				for i := 512; i < 512+20*512; i++ {
					content[i] = 0
				}
				copy(content[512:], text)
			}
			if err = os.WriteFile(source, content, 0600); err != nil {
				t.Fatal(err)
			}
			if err = runVMConversionDockerfile(t, qemu, dir, "source.vmdk"); err == nil {
				t.Fatal("unsafe VMDK produced a runtime disk", mode)
			}
			if _, err = os.Stat(filepath.Join(dir, "rootdisk.qcow2")); !os.IsNotExist(err) {
				t.Fatal("failed preflight must leave no runtime disk")
			}
		})
	}
}
