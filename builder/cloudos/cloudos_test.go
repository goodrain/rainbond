package cloudos

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var endpoint = "dummy"
var accessKeyID = "dummy"
var secretAccessKey = "dummy"

func TestFileUpload(t *testing.T) {
	tests := []struct {
		name, bucketName, objkey, filepath string
		providerType                       S3Provider
		expErr                             bool
		statusCode                         int
	}{
		{
			name:         "bucket not found",
			providerType: S3ProviderAliOSS,
			bucketName:   "no-bucket",
			expErr:       true,
			statusCode:   404,
		},
		{
			name:         "ok",
			providerType: S3ProviderAliOSS,
			bucketName:   "hrhtest",
			expErr:       false,
			statusCode:   200,
			objkey:       "goodrain-logo.png",
			filepath:     "goodrain-logo.png",
		},
	}

	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				ProviderType: tc.providerType,
				Endpoint:     endpoint,
				AccessKey:    accessKeyID,
				SecretKey:    secretAccessKey,
				BucketName:   tc.bucketName,
			}
			cloudoser, err := New(cfg)
			if err != nil {
				t.Errorf("error create cloudoser: %v", err)
				return
			}
			dir := "/tmp/groupbackup/0d65c6608729438aad0a94f6317c80d0_20191024180024.zip"
			_, filename := filepath.Split(dir)
			if err := cloudoser.PutObject(filename, dir); err != nil {
				s3err, ok := err.(S3SDKError)
				if !ok {
					t.Errorf("Expected 'S3SDKError' for err, but returned %v: %v", reflect.TypeOf(s3err), err)
					return
				}
				if s3err.StatusCode != tc.statusCode {
					t.Errorf("Expected %d for status code, but returned %d", tc.statusCode, s3err.StatusCode)
				}
			}
		})
	}
}

func TestGetObject(t *testing.T) {
	tests := []struct {
		name, bucketName, objkey, filepath string
		providerType                       S3Provider
		expErr                             bool
		statusCode                         int
	}{
		{
			name:         "ok",
			providerType: S3ProviderAliOSS,
			bucketName:   "hrhtest",
			expErr:       false,
			statusCode:   200,
			objkey:       "goodrain-logo.png",
			filepath:     "/tmp/goodrain-logo.png",
		},
		{
			name:         "object not found",
			providerType: S3ProviderAliOSS,
			bucketName:   "hrhtest",
			expErr:       true,
			statusCode:   404,
			objkey:       "dummy-object-key",
			filepath:     "/tmp/dummy-object-key",
		},
	}

	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				ProviderType: tc.providerType,
				Endpoint:     endpoint,
				AccessKey:    accessKeyID,
				SecretKey:    secretAccessKey,
				BucketName:   tc.bucketName,
			}
			cloudoser, err := New(cfg)
			if err != nil {
				t.Errorf("error create cloudoser: %v", err)
				return
			}
			if err := cloudoser.GetObject(tc.objkey, tc.filepath); err != nil {
				s3err, ok := err.(S3SDKError)
				if !ok {
					t.Errorf("Expected 'S3SDKError' for err, but returned %v", reflect.TypeOf(s3err))
					return
				}
				if s3err.StatusCode != tc.statusCode {
					t.Errorf("Expected %d for status code, but returned %d", tc.statusCode, s3err.StatusCode)
				}
				return
			}

			// clean up
			err = os.Remove(tc.filepath)
			if err != nil {
				t.Errorf("failed to remove file: %v", err)
			}
		})
	}
}

func TestTypeConvert(t *testing.T) {
	foo := S3Provider("Minio1")
	t.Log(foo)
}
