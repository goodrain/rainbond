package exector

import (
	"errors"
	"fmt"
	"testing"

	"github.com/containerd/containerd"
	dockercli "github.com/docker/docker/client"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/event"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// capability_id: rainbond.vm-publish.http-artifact-image-build
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

// capability_id: rainbond.vm-publish.http-artifact-image-build
func TestShareServiceSkipsRegistryRoundTripWhenTargetMatchesLocalImage(t *testing.T) {
	item := &ImageShareItem{
		ServiceID:      "svc-vm",
		Arch:           "amd64",
		ImageName:      "goodrain.me/svc-vm:20260526121000",
		LocalImageName: "goodrain.me/svc-vm:20260526121000",
		ShareID:        "share-1",
		Logger:         event.NewLogger("evt-skip-share", nil),
		ImageClient:    &countingImageClient{},
	}
	item.ShareInfo.EventID = "evt-skip-share"
	item.ShareInfo.DeployVersion = "20260526121000"
	item.ShareInfo.ShareScope = "enterprise"
	item.ShareInfo.ShareUser = "admin"

	if err := item.ShareService(); err != nil {
		t.Fatalf("expected share service to skip registry round-trip, got %v", err)
	}

	client := item.ImageClient.(*countingImageClient)
	if client.pullCount != 0 || client.tagCount != 0 || client.pushCount != 0 || client.trustedPushCount != 0 {
		t.Fatalf("expected no registry round-trip calls, got pull=%d tag=%d push=%d trustedPush=%d",
			client.pullCount, client.tagCount, client.pushCount, client.trustedPushCount)
	}
}

type countingImageClient struct {
	pullCount        int
	tagCount         int
	pushCount        int
	trustedPushCount int
	metadataCount    int
}

func (c *countingImageClient) GetContainerdClient() *containerd.Client {
	return nil
}

func (c *countingImageClient) GetDockerClient() *dockercli.Client {
	return nil
}

func (c *countingImageClient) ImagePull(imageName, username, password string, logger event.Logger, timeout int) (*ocispec.ImageConfig, error) {
	c.pullCount++
	return &ocispec.ImageConfig{}, nil
}

func (c *countingImageClient) ImagePush(imageName, username, password string, logger event.Logger, timeout int) error {
	c.pushCount++
	return nil
}

func (c *countingImageClient) TrustedImagePush(imageName, username, password string, logger event.Logger, timeout int) error {
	c.trustedPushCount++
	return nil
}

func (c *countingImageClient) ImageTag(source, target string, logger event.Logger, timeout int) error {
	c.tagCount++
	return nil
}

func (c *countingImageClient) ImageRemove(imageName string) error {
	return nil
}

func (c *countingImageClient) ImagesPullAndPush(sourceImage, targetImage, username, password string, logger event.Logger) error {
	return nil
}

func (c *countingImageClient) CheckIfImageExists(imageName string) (string, bool, error) {
	return imageName, true, nil
}

func (c *countingImageClient) ImageSave(image, destination string) error {
	return nil
}

func (c *countingImageClient) ImageLoad(tarFile string, logger event.Logger) ([]string, error) {
	return nil, nil
}

func (c *countingImageClient) GetImageMetadata(image, username, password string, logger event.Logger) (*ocispec.ImageConfig, error) {
	c.metadataCount++
	return &ocispec.ImageConfig{}, nil
}

// capability_id: rainbond.vm-publish.http-artifact-image-build
func TestPrepareVMLocalImageReusesExistingRegistryArtifact(t *testing.T) {
	client := &countingImageClient{}
	item := &ImageShareItem{
		ServiceID:     "svc-vm",
		Arch:          "amd64",
		Logger:        event.NewLogger("evt-reuse-vm-build", nil),
		ImageClient:   client,
		BuildKitImage: "buildkit:test",
		BuildKitArgs:  nil,
		BuildKitCache: false,
	}
	item.ShareInfo.EventID = "evt-reuse-vm-build"
	item.ShareInfo.DeployVersion = "20260526121000"
	item.ShareInfo.AppVersion = "1.0.0"
	item.ShareInfo.ImageInfo.VMImageSource = "https://virt-export/default/disk.img.gz"

	if err := item.prepareVMLocalImage(); err != nil {
		t.Fatalf("expected existing registry artifact to be reused, got %v", err)
	}

	expected := fmt.Sprintf("%s/svc-vm:20260526121000", builder.REGISTRYDOMAIN)
	if item.LocalImageName != expected {
		t.Fatalf("expected local image name %q, got %q", expected, item.LocalImageName)
	}
	if client.metadataCount != 1 {
		t.Fatalf("expected one metadata probe, got %d", client.metadataCount)
	}
	if client.pullCount != 0 || client.tagCount != 0 || client.pushCount != 0 || client.trustedPushCount != 0 {
		t.Fatalf("expected no registry round-trip calls during prepare, got pull=%d tag=%d push=%d trustedPush=%d",
			client.pullCount, client.tagCount, client.pushCount, client.trustedPushCount)
	}
}
