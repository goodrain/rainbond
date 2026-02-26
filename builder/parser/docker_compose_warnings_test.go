package parser

import (
	"io/ioutil"
	"testing"

	"github.com/goodrain/rainbond/event"
)

func TestDockerComposeParseWithWarnings(t *testing.T) {
	// Read test compose file
	content, err := ioutil.ReadFile("compose/testdata/compose-with-warnings.yml")
	if err != nil {
		t.Skipf("Test file not found: %v", err)
		return
	}

	// Create mock logger
	mockLogger := event.NewLogger("test-event", make(chan []byte, 100))

	// Create parser
	parser := &DockerComposeParse{
		source: string(content),
		logger: mockLogger,
		errors: make([]ParseError, 0),
	}

	// Parse
	errors := parser.Parse()

	// Check that we have some errors/warnings
	if len(errors) == 0 {
		t.Error("Expected warnings/errors but got none")
	}

	// Print errors for debugging
	t.Logf("Found %d errors/warnings:", len(errors))
	for i, err := range errors {
		t.Logf("  %d. [%s] %s - %s", i+1, err.ErrorType, err.ErrorInfo, err.SolveAdvice)
	}

	// Check for specific warnings
	hasNetworkWarning := false
	hasSecretWarning := false
	hasConfigWarning := false

	for _, err := range errors {
		if contains(err.ErrorInfo, "networks") {
			hasNetworkWarning = true
		}
		if contains(err.ErrorInfo, "secrets") {
			hasSecretWarning = true
		}
		if contains(err.ErrorInfo, "configs") {
			hasConfigWarning = true
		}
	}

	if !hasNetworkWarning {
		t.Error("Expected network warning but didn't find it")
	}
	if !hasSecretWarning {
		t.Error("Expected secret warning but didn't find it")
	}
	if !hasConfigWarning {
		t.Error("Expected config warning but didn't find it")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDockerComposeParseWithYAMLAnchors(t *testing.T) {
	// Test compose file with YAML anchors and aliases
	// Using version 2 to avoid cache directory requirement (v3+ requires /cache directory)
	composeContent := `version: "2"

x-shared-env: &shared-api-worker-env
  CONSOLE_API_URL: http://api.example.com
  DATABASE_URL: postgres://localhost/db
  REDIS_URL: redis://localhost:6379
  LOG_LEVEL: info
  SHARED_VAR: shared_value

services:
  api:
    image: myapp:latest
    environment:
      <<: *shared-api-worker-env
      MODE: api
      API_PORT: "8080"

  worker:
    image: myapp:latest
    environment:
      <<: *shared-api-worker-env
      MODE: worker
      WORKER_THREADS: "4"
`

	// Create mock logger
	mockLogger := event.NewLogger("test-yaml-anchors", make(chan []byte, 100))

	// Create parser using the factory function to ensure proper initialization
	parser := CreateDockerComposeParse(composeContent, "", "", mockLogger).(*DockerComposeParse)

	// Parse
	errors := parser.Parse()

	// Print any errors for debugging
	if len(errors) > 0 {
		t.Logf("Found %d errors/warnings:", len(errors))
		for i, err := range errors {
			t.Logf("  %d. [%s] %s - %s", i+1, err.ErrorType, err.ErrorInfo, err.SolveAdvice)
		}
	}

	// Check if parsing succeeded (no fatal errors)
	if errors.IsFatalError() {
		t.Fatal("Parsing failed with fatal error")
	}

	// Get the parsed services
	services := parser.GetServiceInfo()
	if len(services) == 0 {
		t.Fatal("No services parsed")
	}

	t.Logf("Parsed %d services", len(services))

	// Check if we have both api and worker services
	var apiService, workerService *ServiceInfo
	for i := range services {
		t.Logf("Service: %s", services[i].Name)
		if services[i].Name == "api" {
			apiService = &services[i]
		}
		if services[i].Name == "worker" {
			workerService = &services[i]
		}
	}

	if apiService == nil {
		t.Fatal("API service not found")
	}
	if workerService == nil {
		t.Fatal("Worker service not found")
	}

	// Check if environment variables are correctly merged
	t.Logf("API service environment variables: %v", apiService.Envs)
	t.Logf("Worker service environment variables: %v", workerService.Envs)

	// Check for shared environment variables in api service
	hasSharedVar := false
	hasConsoleAPI := false
	hasModeAPI := false
	for _, env := range apiService.Envs {
		if env.Name == "SHARED_VAR" && env.Value == "shared_value" {
			hasSharedVar = true
		}
		if env.Name == "CONSOLE_API_URL" && env.Value == "http://api.example.com" {
			hasConsoleAPI = true
		}
		if env.Name == "MODE" && env.Value == "api" {
			hasModeAPI = true
		}
	}

	// Check for shared environment variables in worker service
	hasSharedVarWorker := false
	hasConsoleAPIWorker := false
	hasModeWorker := false
	for _, env := range workerService.Envs {
		if env.Name == "SHARED_VAR" && env.Value == "shared_value" {
			hasSharedVarWorker = true
		}
		if env.Name == "CONSOLE_API_URL" && env.Value == "http://api.example.com" {
			hasConsoleAPIWorker = true
		}
		if env.Name == "MODE" && env.Value == "worker" {
			hasModeWorker = true
		}
	}

	// Report results
	t.Logf("\n=== YAML Anchor/Alias Support Test Results ===")
	t.Logf("API Service:")
	t.Logf("  - Shared variable (SHARED_VAR): %v", hasSharedVar)
	t.Logf("  - Shared variable (CONSOLE_API_URL): %v", hasConsoleAPI)
	t.Logf("  - Service-specific variable (MODE=api): %v", hasModeAPI)
	t.Logf("Worker Service:")
	t.Logf("  - Shared variable (SHARED_VAR): %v", hasSharedVarWorker)
	t.Logf("  - Shared variable (CONSOLE_API_URL): %v", hasConsoleAPIWorker)
	t.Logf("  - Service-specific variable (MODE=worker): %v", hasModeWorker)

	// Verify that YAML anchors/aliases work correctly
	if !hasSharedVar || !hasConsoleAPI || !hasModeAPI {
		t.Error("API service: YAML anchor/alias merge failed - shared environment variables not found")
	}
	if !hasSharedVarWorker || !hasConsoleAPIWorker || !hasModeWorker {
		t.Error("Worker service: YAML anchor/alias merge failed - shared environment variables not found")
	}

	if hasSharedVar && hasConsoleAPI && hasModeAPI && hasSharedVarWorker && hasConsoleAPIWorker && hasModeWorker {
		t.Log("\n✓ YAML anchor/alias syntax is SUPPORTED - all shared environment variables correctly merged")
	} else {
		t.Error("\n✗ YAML anchor/alias syntax is NOT SUPPORTED - environment variables not correctly merged")
	}
}
