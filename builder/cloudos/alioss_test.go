package cloudos

import (
	"testing"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// capability_id: rainbond.cloud-storage.alioss-error-map
func TestSvcErrToS3SDKError(t *testing.T) {
	got := svcErrToS3SDKError(oss.ServiceError{
		Code:       "NoSuchBucket",
		Message:    "bucket not found",
		RawMessage: "raw-body",
		StatusCode: 404,
	})

	if got.Code != "NoSuchBucket" || got.Message != "bucket not found" || got.RawMessage != "raw-body" || got.StatusCode != 404 {
		t.Fatalf("unexpected mapped error: %+v", got)
	}
}

func TestAliOssDeleteObject(t *testing.T) {
	t.Skip("integration test requires live AliOSS backend")
	cfg := &Config{
		ProviderType: S3ProviderAliOSS,
		Endpoint:     "dummy",
		AccessKey:    "dummy",
		SecretKey:    "dummy",
		BucketName:   "hrhtest",
	}
	cr, err := newAliOSS(cfg)
	if err != nil {
		t.Fatalf("create alioss: %v", err)
	}

	if err := cr.DeleteObject("ca932c3215ec4d3891c30799e9aaacba_20191024205031.zip"); err != nil {
		t.Error(err)
	}
}