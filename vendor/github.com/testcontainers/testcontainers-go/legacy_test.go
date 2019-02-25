package testcontainers

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestLegacyTwoContainersExposingTheSamePort(t *testing.T) {
	ctx := context.Background()
	nginxA, err := RunContainer(ctx, "nginx", RequestContainer{
		ExportedPort: []string{
			"80/tcp",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := nginxA.Terminate(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}()

	nginxB, err := RunContainer(ctx, "nginx", RequestContainer{
		ExportedPort: []string{
			"80/tcp",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := nginxB.Terminate(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}()

	ipA, portA, err := nginxA.GetHostEndpoint(ctx, "80/tcp")
	if err != nil {
		t.Fatal(err)
	}

	ipB, portB, err := nginxB.GetHostEndpoint(ctx, "80/tcp")
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(fmt.Sprintf("http://%s:%s", ipA, portA))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d. Got %d.", http.StatusOK, resp.StatusCode)
	}

	resp, err = http.Get(fmt.Sprintf("http://%s:%s", ipB, portB))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d. Got %d.", http.StatusOK, resp.StatusCode)
	}
}

func TestLegacyContainerCreation(t *testing.T) {
	ctx := context.Background()

	nginxPort := "80/tcp"
	nginxC, err := RunContainer(ctx, "nginx", RequestContainer{
		ExportedPort: []string{
			nginxPort,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := nginxC.Terminate(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}()
	ip, port, err := nginxC.GetHostEndpoint(ctx, nginxPort)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(fmt.Sprintf("http://%s:%s", ip, port))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d. Got %d.", http.StatusOK, resp.StatusCode)
	}
}

func TestLegacyContainerCreationAndWaitForListeningPortLongEnough(t *testing.T) {
	t.Skip("Wait needs to be fixed")
	ctx := context.Background()

	nginxPort := "80/tcp"
	// delayed-nginx will wait 2s before opening port
	nginxC, err := RunContainer(ctx, "menedev/delayed-nginx:1.15.2", RequestContainer{
		ExportedPort: []string{
			nginxPort,
		},
		WaitingFor: wait.ForListeningPort(nat.Port(nginxPort)), // default startupTimeout is 60s
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := nginxC.Terminate(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}()
	ip, port, err := nginxC.GetHostEndpoint(ctx, nginxPort)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(fmt.Sprintf("http://%s:%s", ip, port))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d. Got %d.", http.StatusOK, resp.StatusCode)
	}
}

func TestLegacyContainerCreationTimesOut(t *testing.T) {
	t.Skip("Wait needs to be fixed")
	ctx := context.Background()
	// delayed-nginx will wait 2s before opening port
	nginxC, err := RunContainer(ctx, "menedev/delayed-nginx:1.15.2", RequestContainer{
		ExportedPort: []string{
			"80/tcp",
		},
		WaitingFor: wait.ForListeningPort(nat.Port("80")).WithStartupTimeout(1 * time.Second),
	})
	if err == nil {
		t.Error("Expected timeout")
		err := nginxC.Terminate(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLegacyContainerRespondsWithHttp200ForIndex(t *testing.T) {
	ctx := context.Background()

	nginxPort := "80/tcp"
	// delayed-nginx will wait 2s before opening port
	nginxC, err := RunContainer(ctx, "nginx", RequestContainer{
		ExportedPort: []string{
			nginxPort,
		},
		WaitingFor: wait.ForHTTP("/"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := nginxC.Terminate(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}()

	ip, port, err := nginxC.GetHostEndpoint(ctx, nginxPort)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(fmt.Sprintf("http://%s:%s", ip, port))
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d. Got %d.", http.StatusOK, resp.StatusCode)
	}
}

func TestLegacyContainerRespondsWithHttp404ForNonExistingPage(t *testing.T) {
	ctx := context.Background()

	nginxPort := "80/tcp"
	// delayed-nginx will wait 2s before opening port
	nginxC, err := RunContainer(ctx, "nginx", RequestContainer{
		ExportedPort: []string{
			nginxPort,
		},
		WaitingFor: wait.ForHTTP("/nonExistingPage").WithStatusCodeMatcher(func(status int) bool {
			return status == http.StatusNotFound
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := nginxC.Terminate(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}()

	ip, port, err := nginxC.GetHostEndpoint(ctx, nginxPort)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(fmt.Sprintf("http://%s:%s/nonExistingPage", ip, port))
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status code %d. Got %d.", http.StatusNotFound, resp.StatusCode)
	}
}

func TestLegacyContainerCreationTimesOutWithHttp(t *testing.T) {
	ctx := context.Background()
	// delayed-nginx will wait 2s before opening port
	nginxC, err := RunContainer(ctx, "menedev/delayed-nginx:1.15.2", RequestContainer{
		ExportedPort: []string{
			"80/tcp",
		},
		WaitingFor: wait.ForHTTP("/").WithStartupTimeout(1 * time.Second),
	})

	if err == nil {
		err := nginxC.Terminate(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Error("Expected timeout")
	}
}
