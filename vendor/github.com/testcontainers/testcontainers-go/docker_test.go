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

func TestTwoContainersExposingTheSamePort(t *testing.T) {
	ctx := context.Background()
	nginxA, err := GenericContainer(ctx, GenericContainerRequest{
		ContainerRequest: ContainerRequest{
			Image: "nginx",
			ExposedPorts: []string{
				"80/tcp",
			},
		},
		Started: true,
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

	nginxB, err := GenericContainer(ctx, GenericContainerRequest{
		ContainerRequest: ContainerRequest{
			Image: "nginx",
			ExposedPorts: []string{
				"80/tcp",
			},
		},
		Started: true,
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

	ipA, err := nginxA.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	portA, err := nginxA.MappedPort(ctx, "80/tcp")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(fmt.Sprintf("http://%s:%s", ipA, portA.Port()))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d. Got %d.", http.StatusOK, resp.StatusCode)
	}

	ipB, err := nginxB.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	portB, err := nginxB.MappedPort(ctx, "80")
	if err != nil {
		t.Fatal(err)
	}

	resp, err = http.Get(fmt.Sprintf("http://%s:%s", ipB, portB.Port()))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d. Got %d.", http.StatusOK, resp.StatusCode)
	}
}

func TestContainerCreation(t *testing.T) {
	ctx := context.Background()

	nginxPort := "80/tcp"
	nginxC, err := GenericContainer(ctx, GenericContainerRequest{
		ContainerRequest: ContainerRequest{
			Image: "nginx",
			ExposedPorts: []string{
				nginxPort,
			},
		},
		Started: true,
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
	ip, err := nginxC.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := nginxC.MappedPort(ctx, "80")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(fmt.Sprintf("http://%s:%s", ip, port.Port()))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d. Got %d.", http.StatusOK, resp.StatusCode)
	}
}

func TestContainerCreationAndWaitForListeningPortLongEnough(t *testing.T) {
	t.Skip("Wait needs to be fixed")
	ctx := context.Background()

	nginxPort := "80/tcp"
	// delayed-nginx will wait 2s before opening port
	nginxC, err := GenericContainer(ctx, GenericContainerRequest{
		ContainerRequest: ContainerRequest{
			Image: "menedev/delayed-nginx:1.15.2",
			ExposedPorts: []string{
				nginxPort,
			},
			WaitingFor: wait.ForListeningPort("80"), // default startupTimeout is 60s
		},
		Started: true,
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
	origin, err := nginxC.PortEndpoint(ctx, nat.Port(nginxPort), "http")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(origin)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d. Got %d.", http.StatusOK, resp.StatusCode)
	}
}

func TestContainerCreationTimesOut(t *testing.T) {
	t.Skip("Wait needs to be fixed")
	ctx := context.Background()
	// delayed-nginx will wait 2s before opening port
	nginxC, err := GenericContainer(ctx, GenericContainerRequest{
		ContainerRequest: ContainerRequest{
			Image: "menedev/delayed-nginx:1.15.2",
			ExposedPorts: []string{
				"80/tcp",
			},
			WaitingFor: wait.ForListeningPort("80").WithStartupTimeout(1 * time.Second),
		},
		Started: true,
	})
	if err == nil {
		t.Error("Expected timeout")
		err := nginxC.Terminate(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestContainerRespondsWithHttp200ForIndex(t *testing.T) {
	t.Skip("Wait needs to be fixed")
	ctx := context.Background()

	nginxPort := "80/tcp"
	// delayed-nginx will wait 2s before opening port
	nginxC, err := GenericContainer(ctx, GenericContainerRequest{
		ContainerRequest: ContainerRequest{
			Image: "nginx",
			ExposedPorts: []string{
				nginxPort,
			},
			WaitingFor: wait.ForHTTP("/"),
		},
		Started: true,
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

	origin, err := nginxC.PortEndpoint(ctx, nat.Port(nginxPort), "http")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(origin)
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d. Got %d.", http.StatusOK, resp.StatusCode)
	}
}

func TestContainerRespondsWithHttp404ForNonExistingPage(t *testing.T) {
	t.Skip("Wait needs to be fixed")
	ctx := context.Background()

	nginxPort := "80/tcp"
	// delayed-nginx will wait 2s before opening port
	nginxC, err := GenericContainer(ctx, GenericContainerRequest{
		ContainerRequest: ContainerRequest{
			Image: "nginx",
			ExposedPorts: []string{
				nginxPort,
			},
			WaitingFor: wait.ForHTTP("/nonExistingPage").WithStatusCodeMatcher(func(status int) bool {
				return status == http.StatusNotFound
			}),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	rC, err := RunContainer(ctx, "nginx", RequestContainer{
		ExportedPort: []string{
			nginxPort,
		},
		WaitingFor: wait.ForHTTP("/nonExistingPage").WithStatusCodeMatcher(func(status int) bool {
			return status == http.StatusNotFound
		}),
	})
	if rC != nil {
		t.Fatal(rC)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := nginxC.Terminate(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}()

	origin, err := nginxC.PortEndpoint(ctx, nat.Port(nginxPort), "http")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(origin + "/nonExistingPage")
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status code %d. Got %d.", http.StatusNotFound, resp.StatusCode)
	}
}

func TestContainerCreationTimesOutWithHttp(t *testing.T) {
	t.Skip("Wait needs to be fixed")
	ctx := context.Background()
	// delayed-nginx will wait 2s before opening port
	nginxC, err := GenericContainer(ctx, GenericContainerRequest{
		ContainerRequest: ContainerRequest{
			Image: "menedev/delayed-nginx:1.15.2",
			ExposedPorts: []string{
				"80/tcp",
			},
			WaitingFor: wait.ForHTTP("/").WithStartupTimeout(1 * time.Second),
		},
		Started: true,
	})
	defer func() {
		err := nginxC.Terminate(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}()

	if err == nil {
		t.Error("Expected timeout")
	}
}
