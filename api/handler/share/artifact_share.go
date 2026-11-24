package share

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	apimodel "github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/google/uuid"
)

var artifactKey = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

// ShareArtifact reuses the existing asynchronous artifact executors.
func (s *ServiceShareHandle) ShareArtifact(req apimodel.ArtifactShareRequest) (*APIResult, error) {
	task, result, err := prepareArtifactShare(req)
	if err != nil {
		return nil, err
	}
	if err := s.MQClient.SendBuilderTopic(task); err != nil {
		return nil, err
	}
	return result, nil
}

func prepareArtifactShare(req apimodel.ArtifactShareRequest) (client.TaskStruct, *APIResult, error) {
	var task client.TaskStruct
	if !artifactKey.MatchString(req.ServiceKey) || !artifactKey.MatchString(req.Version) ||
		(req.SourceImage == "") == (req.SourceSlug == "") {
		return task, nil, fmt.Errorf("a component key, version and exactly one artifact are required")
	}
	if req.EventID == "" {
		req.EventID = uuid.New().String()
	}
	shareID := uuid.New().String()
	result := &APIResult{EventID: req.EventID, ShareID: shareID}
	info := map[string]interface{}{"service_id": req.ServiceKey, "share_id": shareID,
		"share_info": map[string]interface{}{"service_key": req.ServiceKey, "app_version": req.Version,
			"event_id": req.EventID, "share_user": req.Actor, "share_scope": "delivery",
			"image_info": req.ImageInfo, "slug_info": req.SlugInfo}, "source_auth": req.SourceAuth}
	if req.SourceImage != "" {
		image := dbmodel.ParseImage(req.SourceImage)
		if req.ImageInfo.HubURL != "" {
			image.Host = req.ImageInfo.HubURL
		}
		if req.ImageInfo.Namespace != "" {
			image.Namespace = req.ImageInfo.Namespace
		}
		image.Name = req.ServiceKey + ":" + req.Version
		result.ImageName = image.String()
		info["image_name"], info["local_image_name"] = result.ImageName, req.SourceImage
		task.TaskType = "share-image"
	} else {
		clean := path.Clean(req.SourceSlug)
		if !strings.HasPrefix(clean, "/grdata/build/tenant/") || !strings.HasSuffix(clean, ".tgz") {
			return task, nil, fmt.Errorf("a retained local slug package is required")
		}
		result.SlugPath = path.Join(req.SlugInfo.Namespace, req.ServiceKey, req.Version+".tgz")
		if req.SlugInfo.FTPHost == "" {
			result.SlugPath = path.Join("/grdata/build/tenant", result.SlugPath)
		}
		info["slug_path"], info["local_slug_path"] = result.SlugPath, clean
		task.TaskType = "share-slug"
	}
	task.Topic, task.TaskBody = client.BuilderTopic, info
	return task, result, nil
}
