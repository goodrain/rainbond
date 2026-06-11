package resource

import (
	"context"
	"testing"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/testutil"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// capability_id: rainbond.kb-adapter.addon-version-order
func TestGetAddonsReturnsVersionsLatestFirst(t *testing.T) {
	componentVersion := &kbappsv1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
		Spec: kbappsv1.ComponentVersionSpec{
			Releases: []kbappsv1.ComponentVersionRelease{
				{ServiceVersion: "8.0.2"},
				{ServiceVersion: "8.0.30"},
				{ServiceVersion: "8.1.0"},
				{ServiceVersion: "5.7.44"},
			},
		},
	}
	service := NewService(testutil.NewFakeClient(componentVersion))

	addons, err := service.GetAddons(context.Background())
	require.NoError(t, err)
	require.Len(t, addons, 1)
	assert.Equal(t, "mysql", addons[0].Type)
	assert.Equal(t, []string{"8.1.0", "8.0.30", "8.0.2", "5.7.44"}, addons[0].Version)
}
