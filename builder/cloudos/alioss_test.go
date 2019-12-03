package cloudos

import (
	"testing"
)

func TestAliOssDeleteObject(t *testing.T) {
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
