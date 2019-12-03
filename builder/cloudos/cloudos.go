package cloudos

import (
	"errors"
)

var (
	// ErrUnsupportedS3Provider -
	ErrUnsupportedS3Provider = errors.New("unsupported s3 provider")
)

// S3Provider -
type S3Provider string

var (
	// S3ProviderS3 -
	S3ProviderS3 S3Provider = "s3"
	// S3ProviderAliOSS -
	S3ProviderAliOSS S3Provider = "alioss"
)

func (p S3Provider) String() string {
	return string(p)
}

// Str2S3Provider converts a string to S3Provider.
func Str2S3Provider(value string) (S3Provider, error) {
	switch value {
	case S3ProviderS3.String():
		return S3ProviderS3, nil
	case S3ProviderAliOSS.String():
		return S3ProviderAliOSS, nil
	default:
		return "", ErrUnsupportedS3Provider
	}
}

// CloudOSer is the interface that wraps the required methods to interact with cloud object storage.
type CloudOSer interface {
	PutObject(objkey, filepath string) error
	GetObject(objectKey, filePath string) error
	DeleteObject(objkey string) error
}

// New returns a new CloudOSer.
func New(cfg *Config) (CloudOSer, error) {
	switch cfg.ProviderType {
	case S3ProviderAliOSS:
		return newAliOSS(cfg)
	case S3ProviderS3:
		return newS3(cfg)
	default:
		return nil, ErrUnsupportedS3Provider
	}
}

// Config configuration about cloud object storage.
type Config struct {
	ProviderType S3Provider

	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool

	BucketName string
	Location   string
}
