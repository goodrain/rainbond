package storage

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/event"
	"github.com/sirupsen/logrus"
	"mime/multipart"
	"net/http"
	"os"
)

// StorageComponent -
type StorageComponent struct {
	StorageCli    InterfaceStorage
	storageConfig *configs.StorageConfig
}

var defaultStorageComponent *StorageComponent

// New -
func New() *StorageComponent {
	storageConfig := configs.Default().StorageConfig

	defaultStorageComponent = &StorageComponent{
		storageConfig: storageConfig,
	}
	return defaultStorageComponent
}

// Start -
func (s *StorageComponent) Start(ctx context.Context) error {
	var storageCli InterfaceStorage
	logrus.Infof("create s3 client %v,----%v,----%v", s.storageConfig.StorageType, s.storageConfig.S3AccessKeyID, s.storageConfig.S3SecretAccessKey)
	if s.storageConfig.StorageType == "s3" {
		sess, err := session.NewSession(&aws.Config{
			Endpoint:         aws.String(s.storageConfig.S3Endpoint),
			Region:           aws.String("rainbond"), // 可以根据需要选择区域
			Credentials:      credentials.NewStaticCredentials(s.storageConfig.S3AccessKeyID, s.storageConfig.S3SecretAccessKey, ""),
			S3ForcePathStyle: aws.Bool(true), // 使用路径风格
		})
		if err != nil {
			logrus.Errorf("failed to create session: %v", err)
			return err
		}
		s3Client := s3.New(sess)
		storageCli = &S3Storage{s3Client: s3Client}
	} else {
		storageCli = &LocalStorage{}
	}
	s.StorageCli = storageCli
	return nil
}

// CloseHandle -
func (s *StorageComponent) CloseHandle() {
}

// Default -
func Default() *StorageComponent {
	return defaultStorageComponent
}

type InterfaceStorage interface {
	Glob(dirPath string) ([]string, error)
	MkdirAll(path string) error
	RemoveAll(path string) error
	ServeFile(w http.ResponseWriter, r *http.Request, filePath string)
	OpenFile(fileName string, flag int, perm os.FileMode) (*os.File, error)
	Unzip(archive, target string, currentDirectory bool) error
	SaveFile(fileName string, reader multipart.File) error
	CopyFileWithProgress(src string, dst string, logger event.Logger) error
	ReadDir(dirName string) ([]string, error)
}

type SrcFile interface {
	Read([]byte) (int, error)
}

// DstFile 目标文件接口
type DstFile interface {
	Write([]byte) (int, error)
}
