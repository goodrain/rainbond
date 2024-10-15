package configs

import "github.com/spf13/pflag"

type StorageConfig struct {
	StorageType       string `json:"storage_type"`
	S3Endpoint        string `json:"s3_endpoint"`
	S3AccessKeyID     string `json:"s3_access_key_id"`
	S3SecretAccessKey string `json:"s3_secret_access_key"`
}

func AddStorageFlags(fs *pflag.FlagSet, sc *StorageConfig) {
	fs.StringVar(&sc.StorageType, "storage-type", "s3", "s3 or local")
	fs.StringVar(&sc.S3Endpoint, "s3-endpoint", "http://minio-service:9000", "s3 endpoint")
	fs.StringVar(&sc.S3AccessKeyID, "s3-access-key-id", "admin1234", "s3 accessKeyID")
	fs.StringVar(&sc.S3SecretAccessKey, "s3-secret-access-key", "admin1234", "s3 secretAccessKey")
}
