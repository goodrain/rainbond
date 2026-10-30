package exector

import (
	"encoding/json"
	"testing"

	"github.com/goodrain/rainbond/event"
)

// capability_id: rainbond.builder.source-type-compatibility
func TestNewSourceBuildServerType(t *testing.T) {
	previousManager := event.GetManager()
	event.NewTestManager(&testEventManager{})
	t.Cleanup(func() { event.NewTestManager(previousManager) })

	cases := []struct {
		name    string
		present bool
		value   interface{}
		want    string
	}{
		{name: "missing", want: "git"},
		{name: "null", present: true, value: nil, want: "git"},
		{name: "empty", present: true, value: "", want: "git"},
		{name: "spaces", present: true, value: "   ", want: "git"},
		{name: "whitespace", present: true, value: " \t\n ", want: "git"},
		{name: "git", present: true, value: "git", want: "git"},
		{name: "spaced git", present: true, value: " g it ", want: "git"},
		{name: "svn", present: true, value: "svn", want: "svn"},
		{name: "oss", present: true, value: "oss", want: "oss"},
		{name: "pkg", present: true, value: "pkg", want: "pkg"},
		{name: "other explicit type", present: true, value: "custom", want: "custom"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"event_id": "source-type-test",
				"repo_url": "https://example.com/demo.git",
				"lang":     "NodeJSStatic",
				"envs":     "{}",
			}
			if tc.present {
				payload["server_type"] = tc.value
			}
			input, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal build task: %v", err)
			}

			item := NewSouceCodeBuildItem(input)
			if got := item.CodeSouceInfo.ServerType; got != tc.want {
				t.Fatalf("expected source type %q, got %q", tc.want, got)
			}
		})
	}
}
