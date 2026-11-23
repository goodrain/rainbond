package parser

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/event"
)

// capability_id: rainbond.vm-run.raw-image-format
func TestVMServiceParseRemoteNativeRaw(t *testing.T) {
	withDefaultClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(http.NoBody), Request: req}, nil
	}))
	for _, name := range []string{"disk.raw", "disk.RAW", "disk.raw.gz", "disk.RAW.GZ", "disk.raw.xz", "disk.RAW.XZ"} {
		t.Run(name, func(t *testing.T) {
			p := CreateVMServiceParse("https://example.invalid/"+name+"?download=/other.iso#ignored/file.qcow2", event.GetTestLogger())
			if errs := p.Parse(); len(errs) != 0 {
				t.Fatalf("native raw source must pass normal parsing: %#v", errs)
			}
		})
	}
}

func TestVMServiceParseLocalNativeRaw(t *testing.T) {
	for _, name := range []string{"disk.raw", "disk.RAW", "disk.raw.gz", "disk.RAW.GZ", "disk.raw.xz", "disk.RAW.XZ"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, name), []byte("public-fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			info, err := entries[0].Info()
			if err != nil {
				t.Fatal(err)
			}
			p := CreateVMServiceParse("/grdata/package_build/temp/events/owned-test", event.GetTestLogger()).(*VMServiceParse)
			p.readPackageDir = func(source string) ([]os.FileInfo, error) {
				if source != "/grdata/package_build/temp/events/owned-test" {
					t.Fatal("unexpected local source")
				}
				return []os.FileInfo{info}, nil
			}
			if errs := p.Parse(); len(errs) != 0 {
				t.Fatalf("native raw upload rejected: %#v", errs)
			}
		})
	}
}

// capability_id: rainbond.vm-build.vmdk-self-contained
func TestVMServiceParseAcceptsSingleVMDKURLAndLocalUpload(t *testing.T) {
	withDefaultClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(http.NoBody), Request: req}, nil
	}))
	for _, name := range []string{"Windows10.vmdk", "磁盘.VMDK", "Windows 10 x64.vmdk", "迁移 磁盘.vmdk"} {
		t.Run(name, func(t *testing.T) {
			p := CreateVMServiceParse("https://example.invalid/"+strings.ReplaceAll(name, " ", "%20")+"?download=1", event.GetTestLogger())
			if errs := p.Parse(); len(errs) != 0 {
				t.Fatal("VMDK URL rejected", errs)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, name), []byte("header checked later by builder"), 0600); err != nil {
				t.Fatal(err)
			}
			entries, _ := os.ReadDir(dir)
			info, _ := entries[0].Info()
			local := CreateVMServiceParse("/grdata/package_build/temp/events/vmdk", event.GetTestLogger()).(*VMServiceParse)
			local.readPackageDir = func(string) ([]os.FileInfo, error) { return []os.FileInfo{info}, nil }
			if errs := local.Parse(); len(errs) != 0 {
				t.Fatal("VMDK local/upload rejected", errs)
			}
		})
	}
}

// capability_id: rainbond.vm-build.safe-url-filename
func TestVMServiceParseRejectsUnsafeURLBeforeProbe(t *testing.T) {
	withDefaultClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("unsafe name must fail before HTTP probe")
		return nil, nil
	}))
	p := CreateVMServiceParse("https://example.invalid/%2e%2e%2fdisk.raw", event.GetTestLogger())
	if errs := p.Parse(); len(errs) == 0 {
		t.Fatal("expected unsafe filename rejection")
	}
}

// capability_id: rainbond.vm-build.local-source-boundary
func TestVMServiceParseRejectsUnmanagedLocalPathBeforeReading(t *testing.T) {
	for _, source := range []string{"/grdata/other", "/grdata/package_build/temp/events/../victim", "/grdata/package_build/temp/events/id/child"} {
		p := CreateVMServiceParse(source, event.GetTestLogger()).(*VMServiceParse)
		p.readPackageDir = func(string) ([]os.FileInfo, error) {
			t.Fatal("invalid local source was read before validation")
			return nil, nil
		}
		if errs := p.Parse(); len(errs) == 0 {
			t.Fatal("unsafe local source accepted", source)
		}
	}
}
