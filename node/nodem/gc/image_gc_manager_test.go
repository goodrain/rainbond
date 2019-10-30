package gc

import (
	"github.com/docker/docker/client"
	"testing"
)

func TestGetFsStats(t *testing.T) {
	fs, err := GetFsStats("/")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("capacity: %v", fs.CapacityBytes)
	t.Logf("available: %v", fs.AvailableBytes)
}

func TestGetImageRef(t *testing.T) {
	dockerCli, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}

	im := realImageGCManager{
		dockerClient: dockerCli,
	}
	if _, err := im.getImageRef("nginx"); err != nil {
		t.Error(err)
	}
}

func TestListImages(t *testing.T) {
	dockerCli, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}

	im := realImageGCManager{
		dockerClient: dockerCli,
	}

	images, err := im.listImages()
	if err != nil {
		t.Fatal(err)
	}

	for _, image := range images {
		t.Logf("%s\n", image.ID)
	}
}

func TestRemoveImage(t *testing.T) {
	dockerCli, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}

	im := realImageGCManager{
		dockerClient: dockerCli,
	}

	if err := im.removeImage("sha256:568c4670fa800978e08e4a51132b995a54f8d5ae83ca133ef5546d092b864acf"); err != nil {
		t.Fatalf("remove image: %v", err)
	}
}
