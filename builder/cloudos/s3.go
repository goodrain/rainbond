package cloudos

import (
	"io/ioutil"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type s3Driver struct {
	s3 *s3.S3
	*Config
}

func newS3(cfg *Config) (CloudOSer, error) {
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, ""),
		Endpoint:         aws.String(cfg.Endpoint),
		Region:           aws.String("us-east-1"),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
	}
	sess := session.New(s3Config)
	s3obj := s3.New(sess)

	s3Driver := s3Driver{
		s3:     s3obj,
		Config: cfg,
	}

	return &s3Driver, nil
}

func (s *s3Driver) PutObject(objkey, filepath string) error {
	fp, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer fp.Close()

	_, err = s.s3.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(objkey),
		Body:   fp,
	})
	return err
}

func (s *s3Driver) GetObject(objkey, filePath string) error {
	resp, err := s.s3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(objkey),
		Range:  aws.String("bytes=" + strconv.FormatInt(0, 10) + "-"),
	})
	if err != nil {
		return err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	ioutil.WriteFile(filePath, b, os.ModePerm)
	return nil
}

func (s *s3Driver) DeleteObject(objkey string) error {
	_, err := s.s3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(objkey),
	})
	return err
}
