package cloudos

import (
	"fmt"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type aliOSS struct {
	*oss.Client
	*Config
}

func newAliOSS(cfg *Config) (CloudOSer, error) {
	client, err := oss.New(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, err
	}
	return &aliOSS{Client: client, Config: cfg}, nil
}

func (a *aliOSS) PutObject(okey, filepath string) error {
	// verify if bucket exists and you have permission to access it.
	_, err := a.GetBucketStat(a.BucketName)
	if err != nil {
		svcErr, ok := err.(oss.ServiceError)
		if !ok {
			return err
		}
		return svcErrToS3SDKError(svcErr)
	}

	bucket, err := a.Bucket(a.BucketName)
	if err != nil {
		return fmt.Errorf("failed to gets the bucket instance: %v", err)
	}

	err = bucket.PutObjectFromFile(okey, filepath)
	if err != nil {
		return fmt.Errorf("failed to put object: %v", err)
	}

	return err
}

func (a *aliOSS) GetObject(objectKey, filePath string) error {
	bucket, err := a.Bucket(a.BucketName)
	if err != nil {
		return fmt.Errorf("failed to gets the bucket instance: %v", err)
	}

	err = bucket.GetObjectToFile(objectKey, filePath)
	if err != nil {
		svcErr, ok := err.(oss.ServiceError)
		if !ok {
			return err
		}
		return svcErrToS3SDKError(svcErr)
	}
	return nil
}

func (a *aliOSS) DeleteObject(objkey string) error {
	bucket, err := a.Bucket(a.BucketName)
	if err != nil {
		return fmt.Errorf("failed to gets the bucket instance: %v", err)
	}

	return bucket.DeleteObject(objkey)
}

func svcErrToS3SDKError(svcErr oss.ServiceError) S3SDKError {
	return S3SDKError{
		Code:       svcErr.Code,
		Message:    svcErr.Message,
		RawMessage: svcErr.RawMessage,
		StatusCode: svcErr.StatusCode,
	}
}
