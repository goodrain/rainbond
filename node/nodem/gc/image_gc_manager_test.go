package gc

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/client"
)

var dockerTimeout = 10 * time.Second

func defaultContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), dockerTimeout)
	return ctx
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

func TestDockerRootDir(t *testing.T) {
	dockerCli, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}

	dockerInfo, err := dockerCli.Info(defaultContext())
	if err != nil {
		t.Errorf("docker info: %v", err)
	}

	t.Logf("docker root dir: %s", dockerInfo.DockerRootDir)
}

func TestFoobar(t *testing.T) {
	f := 62.123
	t.Errorf("%0.f%%", f)
}
