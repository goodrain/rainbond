package cleanup

import (
	"reflect"
	"sort"
	"testing"
)

func TestSavedKubeBlocksDefinitionsCollectDirectImages(t *testing.T) {
	content := `apiVersion: apps.kubeblocks.io/v1
kind: ComponentDefinition
spec:
  runtime:
    containers:
    - name: database
      image: goodrain.me/database:v1
  releases:
    images:
      exporter: goodrain.me/exporter:v2
`
	images := []string{}
	if !inspectSavedWorkload(content, func(image string) { images = append(images, image) }) {
		t.Fatal("known KubeBlocks definition remained incomplete")
	}
	sort.Strings(images)
	if !reflect.DeepEqual(images, []string{"goodrain.me/database:v1", "goodrain.me/exporter:v2"}) {
		t.Fatal("KubeBlocks images were not retained", images)
	}
}

func TestSavedKubeBlocksDefinitionWithoutImagesIsComplete(t *testing.T) {
	content := `apiVersion: apps.kubeblocks.io/v1
kind: ShardingDefinition
spec:
  shards: 3
`
	if !inspectSavedWorkload(content, func(string) {}) {
		t.Fatal("image-free KubeBlocks definition remained incomplete")
	}
	unknown := `apiVersion: unknown.example/v1
kind: Unknown
spec: {}
`
	if inspectSavedWorkload(unknown, func(string) {}) {
		t.Fatal("unknown custom resource bypassed reference protection")
	}
}
