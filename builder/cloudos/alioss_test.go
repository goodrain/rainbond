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
