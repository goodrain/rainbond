package parser

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
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
