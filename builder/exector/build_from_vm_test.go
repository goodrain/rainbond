package exector

import (
	"testing"
)

// capability_id: rainbond.vm-run.build-media-paths
func TestResolveVMBuildMediaDistinguishesISOAndDiskImages(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     vmBuildMedia
	}{
		{name: "plain iso", fileName: "ubuntu.iso", want: vmBuildMediaISO},
		{name: "plain qcow2", fileName: "ubuntu.qcow2", want: vmBuildMediaDisk},
		{name: "plain img", fileName: "ubuntu.img", want: vmBuildMediaDisk},
		{name: "gzip disk export", fileName: "disk.img.gz", want: vmBuildMediaDisk},
		{name: "xz disk export", fileName: "disk.qcow2.xz", want: vmBuildMediaDisk},
		{name: "tar disk export", fileName: "disk.tar", want: vmBuildMediaDisk},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveVMBuildMedia(tt.fileName)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestResolveVMBuildMediaRejectsUnknownFormats(t *testing.T) {
	if _, err := resolveVMBuildMedia("ubuntu.vmdk"); err == nil {
		t.Fatal("expected unknown vm media format to fail")
	}
}

func TestRenderVMDockerfileUsesDedicatedTemplatesPerMedia(t *testing.T) {
	isoDockerfile, err := renderVMDockerfile("installer.iso")
	if err != nil {
		t.Fatalf("render iso dockerfile: %v", err)
	}
	if isoDockerfile != "FROM scratch\nCOPY --chown=107:107 installer.iso /disk/\n" {
		t.Fatalf("unexpected iso dockerfile: %q", isoDockerfile)
	}

	diskDockerfile, err := renderVMDockerfile("rootdisk.qcow2")
	if err != nil {
		t.Fatalf("render qcow2 dockerfile: %v", err)
	}
	if diskDockerfile != "FROM scratch\nADD --chown=107:107 rootdisk.qcow2 /disk/\n" {
		t.Fatalf("unexpected disk dockerfile: %q", diskDockerfile)
	}
}
