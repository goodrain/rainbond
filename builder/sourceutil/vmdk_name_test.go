package sourceutil

import "testing"

// capability_id: rainbond.vm-build.vmdk-self-contained
func TestVMDKInputNamesAllowDefaultVMwareNamesWithoutRelaxingDockerNames(t *testing.T) {
	for _, name := range []string{"Windows 10 x64.vmdk", "迁移 磁盘.VMDK", "Windows10.vmdk"} {
		if !IsSafeVMVMDKInputFileName(name) {
			t.Fatal("legitimate VMDK input name rejected", name)
		}
	}
	if IsSafeVMImageFileName("Windows 10 x64.vmdk") {
		t.Fatal("input policy must not relax final Dockerfile interpolation")
	}
	for _, name := range []string{"../disk.vmdk", "/disk.vmdk", "--disk.vmdk", "disk$HOME.vmdk", "disk`id`.vmdk", "disk;id.vmdk", "disk\nRUN id.vmdk", "disk\u202e.vmdk", "disk\u200b.vmdk", "dir\\disk.vmdk", "disk*.vmdk", "bad\tname.vmdk", "space name.qcow2"} {
		if IsSafeVMVMDKInputFileName(name) {
			t.Fatal("unsafe input name accepted", name)
		}
	}
	for _, source := range []string{"https://example.invalid/Windows%2010%20x64.vmdk", "https://example.invalid/迁移%20磁盘.vmdk"} {
		name, err := VMImageFileNameFromURL(source)
		if err != nil || !IsSafeVMVMDKInputFileName(name) {
			t.Fatal(name, err)
		}
	}
	for _, source := range []string{"https://example.invalid/%2e%2e%2fdisk.vmdk", "https://example.invalid/disk%0aRUN.vmdk", "https://example.invalid/disk%24HOME.vmdk", "https://example.invalid/space%20name.qcow2"} {
		if _, err := VMImageFileNameFromURL(source); err == nil {
			t.Fatal("unsafe/legacy-space URL accepted", source)
		}
	}
}
