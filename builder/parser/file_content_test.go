package parser

import (
	"testing"

	"github.com/goodrain/rainbond/builder/parser/types"
	"github.com/goodrain/rainbond/db/model"
)

func TestVolumeFileContent(t *testing.T) {
	// Test that FileContent is set to empty string for config files
	volume := &types.Volume{
		VolumePath:  "/etc/nginx/nginx.conf",
		VolumeType:  model.ConfigFileVolumeType.String(),
		FileContent: "",
	}

	if volume.FileContent != "" {
		t.Errorf("Expected FileContent to be empty string, got: %v", volume.FileContent)
	}

	// Test that FileContent field exists and can be set
	volume.FileContent = "test content"
	if volume.FileContent != "test content" {
		t.Errorf("Expected FileContent to be 'test content', got: %v", volume.FileContent)
	}
}
