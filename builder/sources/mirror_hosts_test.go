package sources

import (
	"testing"

	"github.com/containerd/containerd/remotes/docker"
)

func fakeFallback(host string) ([]docker.RegistryHost, error) {
	return []docker.RegistryHost{{Host: "registry-1.docker.io", Scheme: "https", Path: "/v2"}}, nil
}

// capability_id: rainbond.builder.mirror-containerd-hosts
func TestMirrorRegistryHosts(t *testing.T) {
	t.Run("docker.io gets mirrors first then upstream", func(t *testing.T) {
		hostsFn := mirrorRegistryHosts([]string{"https://docker.1ms.run", "http://insecure.example.com"}, fakeFallback)
		hosts, err := hostsFn("docker.io")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hosts) != 3 {
			t.Fatalf("expected 2 mirrors + upstream, got %d", len(hosts))
		}
		if hosts[0].Host != "docker.1ms.run" || hosts[0].Scheme != "https" {
			t.Fatalf("first host = %+v", hosts[0])
		}
		if hosts[1].Host != "insecure.example.com" || hosts[1].Scheme != "http" {
			t.Fatalf("second host = %+v", hosts[1])
		}
		if hosts[2].Host != "registry-1.docker.io" {
			t.Fatalf("upstream must stay as final fallback, got %+v", hosts[2])
		}
		if hosts[0].Path != "/v2" {
			t.Fatalf("mirror path = %q, want /v2", hosts[0].Path)
		}
	})

	t.Run("other registries untouched", func(t *testing.T) {
		hostsFn := mirrorRegistryHosts([]string{"https://docker.1ms.run"}, fakeFallback)
		hosts, err := hostsFn("myharbor.example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hosts) != 1 || hosts[0].Host != "registry-1.docker.io" {
			t.Fatalf("non docker.io hosts must pass through, got %+v", hosts)
		}
	})

	t.Run("no mirrors passes through", func(t *testing.T) {
		hostsFn := mirrorRegistryHosts(nil, fakeFallback)
		hosts, err := hostsFn("docker.io")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hosts) != 1 {
			t.Fatalf("expected fallback only, got %+v", hosts)
		}
	})
}

// capability_id: rainbond.builder.mirror-docker-ref-rewrite
func TestMirrorPullRefs(t *testing.T) {
	mirrors := []string{"https://docker.1ms.run", "http://insecure.example.com"}
	tests := []struct {
		name  string
		image string
		want  []string
	}{
		{
			name:  "short name gets library completion and mirror candidates",
			image: "nginx",
			want: []string{
				"docker.1ms.run/library/nginx:latest",
				"insecure.example.com/library/nginx:latest",
				"docker.io/library/nginx:latest",
			},
		},
		{
			name:  "namespaced docker.io image",
			image: "bitnami/redis:7.2",
			want: []string{
				"docker.1ms.run/bitnami/redis:7.2",
				"insecure.example.com/bitnami/redis:7.2",
				"docker.io/bitnami/redis:7.2",
			},
		},
		{
			name:  "private registry untouched",
			image: "myharbor.example.com/app:v1",
			want:  []string{"myharbor.example.com/app:v1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mirrorPullRefs(tt.image, mirrors)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}

	t.Run("no mirrors returns original only", func(t *testing.T) {
		got := mirrorPullRefs("nginx", nil)
		if len(got) != 1 || got[0] != "nginx" {
			t.Fatalf("got %v, want [nginx]", got)
		}
	})
}
