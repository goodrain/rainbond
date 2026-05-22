package exector

import (
	"errors"
	"fmt"
	"testing"

	"github.com/goodrain/rainbond/builder"
)

// capability_id: rainbond.vm-publish.qcow2-image-build
func TestNewImageShareItemCapturesVMImageSource(t *testing.T) {
	body := []byte(`{
		"service_id":"svc-vm",
		"service_alias":"vm-demo",
		"tenant_name":"demo-team",
		"arch":"amd64",
		"image_name":"registry.example.com/team/windows-root:v1",
		"share_id":"share-1",
		"share_info":{
			"service_key":"svc-vm",
			"app_version":"1.0.0",
			"event_id":"evt-1",
			"share_user":"tester",
			"share_scope":"team",
			"image_info":{
				"hub_url":"registry.example.com",
				"namespace":"team",
				"vm_image_source":"https://virt-export/default/disk.img.gz"
			}
		}
	}`)

	item, err := NewImageShareItem(body, nil, nil, "", nil, false)
	if err != nil {
		t.Fatalf("expected image share item to parse: %v", err)
	}

	if item.Arch != "amd64" {
		t.Fatalf("expected arch amd64, got %q", item.Arch)
	}
	if item.ShareInfo.ImageInfo.VMImageSource != "https://virt-export/default/disk.img.gz" {
		t.Fatalf("unexpected vm image source: %q", item.ShareInfo.ImageInfo.VMImageSource)
	}
}

func TestResolveVMLocalBuildImagePrefersDeployVersion(t *testing.T) {
	got := resolveVMLocalBuildImage("svc-vm", "20260521120000", "1.0.0")
	if got != "svc-vm:20260521120000" {
		t.Fatalf("expected deploy version to win, got %q", got)
	}
}

func TestResolveVMLocalBuildImageFallsBackToAppVersion(t *testing.T) {
	got := resolveVMLocalBuildImage("svc-vm", "", "1.0.0")
	if got != "svc-vm:1.0.0" {
		t.Fatalf("expected fallback app version, got %q", got)
	}
}

func TestResolveVMShareLocalImageNameMatchesVMBuildOutput(t *testing.T) {
	got := resolveVMShareLocalImageName("svc-vm", "20260521230400", "1.0.0")
	want := fmt.Sprintf("%s/svc-vm:20260521230400", builder.REGISTRYDOMAIN)
	if got != want {
		t.Fatalf("expected vm share to pull the image produced by vm build, got %q", got)
	}
}

// capability_id: rainbond.image-share.single-attempt
func TestExecuteImageShareOnceDoesNotRetryFailure(t *testing.T) {
	attempts := 0
	status, err := executeImageShareOnce(func() error {
		attempts++
		return errors.New("build code job exec failure")
	})

	if attempts != 1 {
		t.Fatalf("expected image share to run once, got %d attempts", attempts)
	}
	if status != "failure" {
		t.Fatalf("expected failure status, got %q", status)
	}
	if err == nil {
		t.Fatal("expected failure error to be returned")
	}
}

// capability_id: rainbond.image-share.single-attempt
func TestExecuteImageShareOnceReturnsSuccess(t *testing.T) {
	attempts := 0
	status, err := executeImageShareOnce(func() error {
		attempts++
		return nil
	})

	if attempts != 1 {
		t.Fatalf("expected image share to run once, got %d attempts", attempts)
	}
	if status != "success" {
		t.Fatalf("expected success status, got %q", status)
	}
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
