package share

import (
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestArtifactShareUsesPublishedOutputWithoutSourceComponent(t *testing.T) {
	request := apimodel.ArtifactShareRequest{ServiceKey: "template-web", Version: "1.0.0", SourceImage: "source.local/team/web:fixed"}
	request.ImageInfo.HubURL = "market.local"
	request.ImageInfo.Namespace = "catalog"
	task, result, err := prepareArtifactShare(request)
	require.NoError(t, err)
	require.Equal(t, "share-image", task.TaskType)
	require.Equal(t, "market.local/catalog/template-web:1.0.0", result.ImageName)
	require.Equal(t, request.SourceImage, task.TaskBody.(map[string]interface{})["local_image_name"])
}

func TestArtifactShareRejectsAmbiguousOrMissingArtifact(t *testing.T) {
	for _, request := range []apimodel.ArtifactShareRequest{
		{}, {ServiceKey: "web", Version: "1", SourceImage: "registry/web:fixed", SourceSlug: "/grdata/build/tenant/file.tgz"},
	} {
		_, _, err := prepareArtifactShare(request)
		require.Error(t, err)
	}
}
