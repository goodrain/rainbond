package storage

import (
	"context"
	"mime/multipart"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/event"
	"github.com/sirupsen/logrus"
)

// Component configures the platform storage client.
type Component struct {
	StorageCli    InterfaceStorage
	storageConfig *configs.StorageConfig
}

var defaultComponent *Component

// New -
func New() *Component {
	storageConfig := configs.Default().StorageConfig

	defaultComponent = &Component{
		storageConfig: storageConfig,
	}
	return defaultComponent
}

// Start -
func (s *Component) Start(ctx context.Context) error {
	var storageCli InterfaceStorage
	logrus.Infof("initialize storage client type=%s", s.storageConfig.StorageType)
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
		s3Storage := &S3Storage{s3Client: s3Client}

		// API 启动时主动初始化 bucket 生命周期策略
		logrus.Info("Initializing S3 bucket lifecycle policies on startup...")
		if err := s3Storage.InitBucketLifecycle(); err != nil {
			logrus.Warnf("Failed to initialize bucket lifecycle policies: %v (non-fatal, continuing startup)", err)
			// 不返回错误，允许 API 继续启动，生命周期策略会在后续操作中自动创建
		} else {
			logrus.Info("Successfully initialized S3 bucket lifecycle policies")
		}

		storageCli = s3Storage
	} else {
		storageCli = &LocalStorage{}
	}
	s.StorageCli = storageCli
	return nil
}

// CloseHandle -
func (s *Component) CloseHandle() {
}

// Default -
func Default() *Component {
	return defaultComponent
}

// InterfaceStorage defines the platform file and chunk operations.
type InterfaceStorage interface {
	MkdirAll(path string) error
	Unzip(archive, target string, currentDirectory bool) error
	ReadDir(dirName string) ([]string, error)
	ServeFile(w http.ResponseWriter, r *http.Request, filePath string)
	SaveFile(fileName string, reader multipart.File) error
	UploadFileToFile(src string, dst string, logger event.Logger) error
	DownloadDirToDir(srcDir, dstDir string) error
	DownloadFileToDir(srcFile, dstDir string) error
	// ReadFile reads a file directly from storage and returns a reader
	ReadFile(filePath string) (ReadCloser, error)

	// 分片上传相关方法
	SaveChunk(sessionID string, chunkIndex int, reader multipart.File) (string, error)
	MergeChunks(sessionID string, outputPath string, totalChunks int) error
	ChunkExists(sessionID string, chunkIndex int) bool
	CleanupChunks(sessionID string) error
	GetChunkDir(sessionID string) string
}

// ReadCloser reads a stored object and releases its resources.
type ReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}

// SrcFile is a readable copy source.
type SrcFile interface {
	Read([]byte) (int, error)
}

// DstFile 目标文件接口
type DstFile interface {
	Write([]byte) (int, error)
}
