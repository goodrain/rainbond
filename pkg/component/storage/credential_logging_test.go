package storage

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/config/configs"
	"github.com/sirupsen/logrus"
)

// capability_id: rainbond.storage.no-credential-startup-logs
func TestStorageStartupDoesNotLogCredentials(t *testing.T) {
	logger := logrus.StandardLogger()
	oldOutput, oldLevel := logger.Out, logger.GetLevel()
	var output bytes.Buffer
	logger.SetOutput(&output)
	logger.SetLevel(logrus.InfoLevel)
	defer func() { logger.SetOutput(oldOutput); logger.SetLevel(oldLevel) }()
	component := &Component{storageConfig: &configs.StorageConfig{StorageType: "local", S3AccessKeyID: "fixture-access-not-real", S3SecretAccessKey: "fixture-secret-not-real"}}
	if err := component.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "fixture-access-not-real") || strings.Contains(output.String(), "fixture-secret-not-real") {
		t.Fatal("storage startup logged credential fields")
	}
}
