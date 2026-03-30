package cloudos

import "testing"

// capability_id: rainbond.cloud-storage.provider-parse
func TestStr2S3Provider(t *testing.T) {
	provider, err := Str2S3Provider("storage")
	if err != nil {
		t.Fatal(err)
	}
	if provider != S3ProviderS3 {
		t.Fatalf("expected storage provider, got %q", provider)
	}

	provider, err = Str2S3Provider("alioss")
	if err != nil {
		t.Fatal(err)
	}
	if provider != S3ProviderAliOSS {
		t.Fatalf("expected alioss provider, got %q", provider)
	}

	if _, err := Str2S3Provider("minio"); err == nil {
		t.Fatal("expected unsupported provider error")
	}
}

// capability_id: rainbond.cloud-storage.driver-factory
func TestNewDispatchesProviderDrivers(t *testing.T) {
	aliossDriver, err := New(&Config{
		ProviderType: S3ProviderAliOSS,
		Endpoint:     "dummy",
		AccessKey:    "dummy",
		SecretKey:    "dummy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := aliossDriver.(*aliOSS); !ok {
		t.Fatalf("expected aliOSS driver, got %T", aliossDriver)
	}

	storageDriver, err := New(&Config{
		ProviderType: S3ProviderS3,
		Endpoint:     "dummy",
		AccessKey:    "dummy",
		SecretKey:    "dummy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := storageDriver.(*s3Driver); !ok {
		t.Fatalf("expected s3Driver, got %T", storageDriver)
	}

	if _, err := New(&Config{ProviderType: "unsupported"}); err == nil {
		t.Fatal("expected unsupported provider error")
	}
}
