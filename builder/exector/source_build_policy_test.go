package exector

import (
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/event"
)

type testEventManager struct{}

func (m *testEventManager) GetLogger(eventID string) event.Logger {
	return &testLogger{eventID: eventID}
}
func (m *testEventManager) Start() error               { return nil }
func (m *testEventManager) Close() error               { return nil }
func (m *testEventManager) ReleaseLogger(event.Logger) {}

type testLogger struct {
	eventID string
}

func (l *testLogger) Info(string, map[string]string)                  {}
func (l *testLogger) Error(string, map[string]string)                 {}
func (l *testLogger) Debug(string, map[string]string)                 {}
func (l *testLogger) Event() string                                   { return l.eventID }
func (l *testLogger) CreateTime() time.Time                           { return time.Now() }
func (l *testLogger) GetChan() chan []byte                            { return nil }
func (l *testLogger) SetChan(chan []byte)                             {}
func (l *testLogger) GetWriter(step, level string) event.LoggerWriter { return testLoggerWriter{} }

type testLoggerWriter struct{}

func (w testLoggerWriter) Write(p []byte) (int, error)      { return len(p), nil }
func (w testLoggerWriter) SetFormat(map[string]interface{}) {}

func TestNewSouceCodeBuildItemParsesCNBVersionPolicy(t *testing.T) {
	previousManager := event.GetManager()
	event.NewTestManager(&testEventManager{})
	defer event.NewTestManager(previousManager)

	item := NewSouceCodeBuildItem([]byte(`{
		"event_id":"evt-1",
		"tenant_id":"tenant-1",
		"service_id":"service-1",
		"tenant_name":"team-a",
		"service_alias":"demo",
		"repo_url":"https://example.com/demo.git",
		"server_type":"git",
		"branch":"main",
		"lang":"python",
		"build_type":"cnb",
		"build_strategy":"cnb",
		"envs":"{}",
		"configs":{},
		"cnb_version_policy":{
			"version":1,
			"languages":{
				"python":{
					"lang_key":"python",
					"allowed_versions":["3.11"],
					"default_version":"3.11"
				}
			}
		}
	}`))

	if item.BuildStrategy != "cnb" {
		t.Fatalf("expected build strategy cnb, got %q", item.BuildStrategy)
	}
	if item.CNBVersionPolicy == nil {
		t.Fatal("expected cnb version policy to be parsed")
	}
	if item.CNBVersionPolicy.Version != 1 {
		t.Fatalf("expected policy version 1, got %d", item.CNBVersionPolicy.Version)
	}
	policy, ok := item.CNBVersionPolicy.Languages["python"]
	if !ok {
		t.Fatal("expected python policy to be present")
	}
	if policy.LangKey != "python" {
		t.Fatalf("expected lang_key python, got %q", policy.LangKey)
	}
	if len(policy.AllowedVersions) != 1 || policy.AllowedVersions[0] != "3.11" {
		t.Fatalf("expected allowed_versions [3.11], got %#v", policy.AllowedVersions)
	}
}

func TestSourceCodeBuildItemValidateCNBVersionPolicy(t *testing.T) {
	previousChecker := hasEnterpriseCNBPlugin
	defer func() { hasEnterpriseCNBPlugin = previousChecker }()

	t.Run("enterprise cnb build requires snapshot", func(t *testing.T) {
		hasEnterpriseCNBPlugin = func() bool { return true }
		item := &SourceCodeBuildItem{
			BuildType:     "cnb",
			BuildStrategy: "cnb",
		}

		err := item.validateCNBVersionPolicy()
		if err == nil {
			t.Fatal("expected missing cnb_version_policy to fail for enterprise cnb build")
		}
		if !strings.Contains(err.Error(), "cnb_version_policy") {
			t.Fatalf("expected cnb_version_policy error, got %v", err)
		}
	})

	t.Run("oss cnb build keeps old payloads valid", func(t *testing.T) {
		hasEnterpriseCNBPlugin = func() bool { return false }
		item := &SourceCodeBuildItem{
			BuildType:     "cnb",
			BuildStrategy: "cnb",
		}

		if err := item.validateCNBVersionPolicy(); err != nil {
			t.Fatalf("expected oss cnb build without snapshot to stay valid, got %v", err)
		}
	})

	t.Run("enterprise cnb build accepts snapshot", func(t *testing.T) {
		hasEnterpriseCNBPlugin = func() bool { return true }
		item := &SourceCodeBuildItem{
			BuildType:        "cnb",
			BuildStrategy:    "cnb",
			CNBVersionPolicy: &build.CNBVersionPolicy{Version: 1},
		}

		if err := item.validateCNBVersionPolicy(); err != nil {
			t.Fatalf("expected enterprise cnb build with snapshot to pass, got %v", err)
		}
	})
}
