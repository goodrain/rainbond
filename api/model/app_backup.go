package model

// DeleteBackupReq defines a struct to receive the request body
// to delete app backup.
type DeleteBackupReq struct {
	TenantID string `json:"tenant_id"`
	BackupID string `json:"badkup_id"`
	S3Config struct {
		Provider   string `json:"provider"`
		Endpoint   string `json:"endpoint"`
		AccessKey  string `json:"access_key"`
		SecretKey  string `json:"secret_key"`
		BucketName string `json:"bucket_name"`
	} `json:"s3_config"`
}
