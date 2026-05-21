package exector

import "testing"

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
