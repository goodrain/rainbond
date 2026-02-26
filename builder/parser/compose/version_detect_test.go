package compose

import (
	"testing"
)

func TestInferComposeVersion(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected string
	}{
		{
			name: "Compose Spec with profiles",
			yaml: `
services:
  web:
    image: nginx
    profiles:
      - dev
`,
			expected: "spec",
		},
		{
			name: "Compose Spec with extends",
			yaml: `
services:
  web:
    image: nginx
    extends:
      service: base
`,
			expected: "spec",
		},
		{
			name: "Compose Spec with long depends_on",
			yaml: `
services:
  web:
    image: nginx
    depends_on:
      db:
        condition: service_healthy
`,
			expected: "spec",
		},
		{
			name: "v3 with deploy",
			yaml: `
services:
  web:
    image: nginx
    deploy:
      replicas: 3
`,
			expected: "3.8",
		},
		{
			name: "v3 without deploy",
			yaml: `
services:
  web:
    image: nginx
    ports:
      - "80:80"
`,
			expected: "3.0",
		},
		{
			name: "Empty file",
			yaml: ``,
			expected: "spec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferComposeVersion([]byte(tt.yaml))
			if result != tt.expected {
				t.Errorf("inferComposeVersion() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsConfigFile(t *testing.T) {
	tests := []struct {
		name       string
		mountPath  string
		volumeType string
		expected   bool
	}{
		{
			name:       "Config file with .conf extension",
			mountPath:  "/etc/nginx/nginx.conf",
			volumeType: "bind",
			expected:   true,
		},
		{
			name:       "Config file with .yaml extension",
			mountPath:  "/app/config.yaml",
			volumeType: "bind",
			expected:   true,
		},
		{
			name:       "Shell script",
			mountPath:  "/app/start.sh",
			volumeType: "bind",
			expected:   true,
		},
		{
			name:       "Dockerfile",
			mountPath:  "/app/Dockerfile",
			volumeType: "bind",
			expected:   true,
		},
		{
			name:       "Directory mount",
			mountPath:  "/app/data/",
			volumeType: "bind",
			expected:   false,
		},
		{
			name:       "Named volume",
			mountPath:  "/app/data",
			volumeType: "volume",
			expected:   false,
		},
		{
			name:       "Regular file without config extension",
			mountPath:  "/app/data.txt",
			volumeType: "bind",
			expected:   false,
		},
		{
			name:       "Path in /etc/",
			mountPath:  "/etc/app/settings",
			volumeType: "bind",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConfigFile(tt.mountPath, tt.volumeType)
			if result != tt.expected {
				t.Errorf("isConfigFile(%s, %s) = %v, want %v",
					tt.mountPath, tt.volumeType, result, tt.expected)
			}
		})
	}
}
