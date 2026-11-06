package cleanup

import "testing"

func TestSavedEndpointsDoesNotRequireImageReferences(t *testing.T) {
	content := `apiVersion: v1
kind: Endpoints
subsets:
- addresses:
  - ip: 10.0.0.1
  ports:
  - port: 8080
`
	images := 0
	if !inspectSavedWorkload(content, func(string) { images++ }) || images != 0 {
		t.Fatal("standard Endpoints record remained incomplete")
	}
}
