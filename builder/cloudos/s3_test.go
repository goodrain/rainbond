package cloudos

import "testing"

// capability_id: rainbond.cloud-storage.s3-driver-config
func TestNewS3DriverKeepsConfig(t *testing.T) {
	driver, err := newS3(&Config{
		ProviderType: S3ProviderS3,
		Endpoint:     "dummy-endpoint",
		AccessKey:    "ak",
		SecretKey:    "sk",
		BucketName:   "bucket-a",
	})
	if err != nil {
		t.Fatal(err)
	}

	s3obj, ok := driver.(*s3Driver)
	if !ok {
		t.Fatalf("expected *s3Driver, got %T", driver)
	}
	if s3obj.Config.Endpoint != "dummy-endpoint" || s3obj.Config.BucketName != "bucket-a" {
		t.Fatalf("unexpected s3 config: %+v", s3obj.Config)
	}
	if s3obj.s3 == nil {
		t.Fatal("expected aws s3 client to be initialized")
	}
}
