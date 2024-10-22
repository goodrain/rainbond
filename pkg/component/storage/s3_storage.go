package storage

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util/zip"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type S3Storage struct {
	s3Client *s3.S3
	bucket   string
}

func (s3s *S3Storage) Test() {

}

// Glob 便利目录下所有的文件全路径
func (s3s *S3Storage) Glob(dirPath string) ([]string, error) {
	bucketName, prefix, err := s3s.ParseDirPath(dirPath, false)
	if err != nil {
		return nil, err
	}
	var matchedKeys []string
	// 列出 S3 桶中的对象
	result, err := s3s.s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	})
	for _, item := range result.Contents {
		if *item.Key == prefix {
			continue
		}
		syPath := strings.Split(*item.Key, prefix)[1]
		sPath := strings.Split(syPath, "/")
		nextPath := sPath[0]
		matchedKeys = append(matchedKeys, path.Join(bucketName, prefix, nextPath))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}
	return matchedKeys, nil
}

func (s3s *S3Storage) ReadDir(dirName string) ([]string, error) {
	bucketName, prefix, err := s3s.ParseDirPath(dirName, false)
	if err != nil {
		return nil, err
	}
	var matchedKeys []string
	// 列出 S3 桶中的对象
	result, err := s3s.s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	})
	for _, item := range result.Contents {
		if *item.Key == prefix {
			continue
		}
		syPath := strings.Split(*item.Key, prefix)[1]
		sPath := strings.Split(syPath, "/")
		nextPath := sPath[0]
		matchedKeys = append(matchedKeys, nextPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}
	return matchedKeys, nil
}

// MkdirAll MkdirAll
func (s3s *S3Storage) MkdirAll(dirPath string) error {
	// 解析目录路径
	bucketName, parsePath, err := s3s.ParseDirPath(dirPath, false)
	if err != nil {
		return fmt.Errorf("failed to parse directory path: %w", err)
	}

	// 检查目录是否已存在
	exists, err := s3s.CheckDirExists(bucketName, parsePath)
	if err != nil {
		return fmt.Errorf("failed to check directory existence: %w", err)
	}

	if !exists {
		// 如果目录不存在，创建目录
		_, err = s3s.s3Client.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(parsePath),
			Body:   nil, // 空对象
		})
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	return nil
}

// RemoveAll RemoveAll
func (s3s *S3Storage) RemoveAll(dirPath string) error {
	// 构建删除对象列表
	bucketName, parsePath, err := s3s.ParseDirPath(dirPath, false)
	objectsToDelete := &s3.Delete{
		Objects: []*s3.ObjectIdentifier{},
		Quiet:   aws.Bool(true),
	}

	// 使用分页方式列出并删除所有对象
	err = s3s.s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(parsePath),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			// 将对象添加到待删除列表中
			objectsToDelete.Objects = append(objectsToDelete.Objects, &s3.ObjectIdentifier{
				Key: aws.String(*obj.Key),
			})
		}

		// 删除当前页的对象
		if len(objectsToDelete.Objects) > 0 {
			_, err := s3s.s3Client.DeleteObjects(&s3.DeleteObjectsInput{
				Bucket: aws.String(bucketName),
				Delete: objectsToDelete,
			})
			if err != nil {
				fmt.Printf("Failed to delete objects: %v\n", err)
				return false // 停止分页处理
			}
			// 清空列表，以便下一页对象的处理
			objectsToDelete.Objects = nil
		}

		return !lastPage // 继续处理下一页
	})
	if err != nil {
		return fmt.Errorf("failed to list objects for deletion: %w", err)
	}
	return nil
}

// CheckDirExists 检查目录是否存在
func (s3s *S3Storage) CheckDirExists(bucketName, dirPath string) (bool, error) {
	resp, err := s3s.s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:  aws.String(bucketName),
		Prefix:  aws.String(dirPath),
		MaxKeys: aws.Int64(1), // 只需检查是否有至少一个对象
	})
	if err != nil {
		return false, err
	}
	return len(resp.Contents) > 0, nil
}

// ClearDirectory 清空目录下的内容
func (s3s *S3Storage) ClearDirectory(bucketName, dirPath string) error {
	// 列出所有需要删除的对象
	objectsToDelete := &s3.Delete{
		Objects: []*s3.ObjectIdentifier{},
		Quiet:   aws.Bool(true),
	}

	// 使用分页方式删除所有对象
	err := s3s.s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(dirPath),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			objectsToDelete.Objects = append(objectsToDelete.Objects, &s3.ObjectIdentifier{
				Key: aws.String(*obj.Key),
			})
		}
		return !lastPage
	})
	if err != nil {
		return fmt.Errorf("failed to list objects for deletion: %w", err)
	}

	if len(objectsToDelete.Objects) == 0 {
		return nil // 目录为空，无需清空
	}

	// 批量删除对象
	_, err = s3s.s3Client.DeleteObjects(&s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: objectsToDelete,
	})
	if err != nil {
		return fmt.Errorf("failed to delete objects: %w", err)
	}

	return nil
}

func (s3s *S3Storage) ServeFile(w http.ResponseWriter, r *http.Request, filePath string) {
	// 获取对象
	bucketName, key, err := s3s.ParseDirPath(filePath, true)
	output, err := s3s.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get object from S3: %v", err), http.StatusInternalServerError)
		return
	}
	defer output.Body.Close()

	// 设置响应头
	w.Header().Set("Content-Type", *output.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", key))

	// 将对象内容写入响应
	if _, err := io.Copy(w, output.Body); err != nil {
		http.Error(w, fmt.Sprintf("failed to write response: %v", err), http.StatusInternalServerError)
		return
	}
}

// ensureBucketExists 检查桶是否存在，若不存在则创建桶
func (s3s *S3Storage) ensureBucketExists(bucketName string) error {
	// 检查桶是否存在
	_, err := s3s.s3Client.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		// 如果桶不存在，创建桶
		_, err = s3s.s3Client.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}
	return nil
}

// ParseDirPath 解析 dirPath 并返回桶名和路径
func (s3s *S3Storage) ParseDirPath(dirPath string, isFile bool) (string, string, error) {
	parts := strings.Split(dirPath, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("dirPath is invalid, must include bucket name and path")
	}

	var bucketName string
	var key string
	keyIndex := 1

	for i, p := range parts {
		if p != "" {
			bucketName = p
			keyIndex = i
			break
		}
	}
	// 检查桶名是否为两个单词
	if len(bucketName) == 2 {
		bucketName = "gr" + bucketName
	}

	// 检查桶是否存在
	if err := s3s.ensureBucketExists(bucketName); err != nil {
		return "", "", err
	}
	key = strings.Join(parts[keyIndex+1:], "/")
	if !isFile {
		key += "/"
	}

	return bucketName, key, nil
}

func (s3s *S3Storage) OpenFile(fileName string, flag int, perm os.FileMode) (*os.File, error) {
	bucketName, key, err := s3s.ParseDirPath(fileName, true)
	if err != nil {
		return nil, err
	}
	if flag != os.O_RDONLY {
		return nil, fmt.Errorf("only read access is supported")
	}

	downloader := s3manager.NewDownloaderWithClient(s3s.s3Client)
	file, err := os.CreateTemp("", "s3-")
	if err != nil {
		return nil, err
	}

	_, err = downloader.Download(file, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s3s *S3Storage) Unzip(archive, target string, currentDirectory bool) error {
	bucketName, key, err := s3s.ParseDirPath(archive, true)
	// 下载 S3 中的 ZIP 文件
	zipFile, err := os.CreateTemp("", "archive-*.zip")
	if err != nil {
		return fmt.Errorf("error creating temp file: %v", err)
	}
	defer os.Remove(zipFile.Name()) // 确保临时文件在函数退出时被删除

	obj, err := s3s.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("error downloading file from S3: %v", err)
	}
	defer zipFile.Close()
	if _, err := io.Copy(zipFile, obj.Body); err != nil {
		return fmt.Errorf("error writing to temp file: %v", err)
	}

	// 打开下载的 ZIP 文件
	reader, err := zip.OpenReader(zipFile.Name())
	if err != nil {
		return fmt.Errorf("error opening archive: %v", err)
	}
	defer reader.Close()

	// 创建目标目录
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		if err := extractFile(file, target, currentDirectory); err != nil {
			return err
		}
	}

	return nil
}

func (s3s *S3Storage) SaveFile(fileName string, reader multipart.File) error {
	bucketName, key, err := s3s.ParseDirPath(fileName, true)
	if err != nil {
		logrus.Errorf("Failed to parse file path: %s", err.Error())
		return err
	}
	_, err = s3s.s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   reader,
	})
	if err != nil {
		logrus.Errorf("Failed to upload file: %s", err.Error())
		return err
	}
	return nil
}

func (s3s *S3Storage) CopyFileWithProgress(src, dst string, logger event.Logger) error {
	srcFile, err := os.OpenFile(src, os.O_RDONLY, 0644)
	if err != nil {
		if logger != nil {
			logger.Error("打开源文件失败", map[string]string{"step": "share"})
		}
		logrus.Errorf("open file %s error: %v", src, err)
		return err
	}
	defer srcFile.Close()

	srcStat, err := srcFile.Stat()
	if err != nil {
		if logger != nil {
			logger.Error("获取源文件信息失败", map[string]string{"step": "share"})
		}
		return err
	}
	bucket, key, err := s3s.ParseDirPath(dst, true)
	if err != err {
		logger.Error("解析目标路径失败", map[string]string{"step": "share"})
		return err
	}
	// 开始文件上传
	return s3s.S3CopyWithProgress(srcFile, bucket, key, srcStat.Size(), logger)
}

// S3CopyWithProgress 从源文件复制到 S3，并记录进度
func (s3s *S3Storage) S3CopyWithProgress(srcFile io.Reader, bucket, key string, allSize int64, logger event.Logger) error {
	var written int64
	buf := make([]byte, 1024*1024)
	progressID := uuid.New().String()[0:7]
	// 使用 bytes.Buffer 来保存所有数据
	var buffer bytes.Buffer
	// 创建一个 WaitGroup
	var wg sync.WaitGroup
	wg.Add(1)
	// 启动一个 goroutine 处理源文件的读取
	go func() {
		defer wg.Done()
		for {
			nr, er := srcFile.Read(buf)
			if nr > 0 {
				nw, ew := buffer.Write(buf[:nr])
				if nw > 0 {
					written += int64(nw)
				}
				if ew != nil {
					logrus.Errorf("写入缓冲区失败: %v", ew)
					break
				}
				if nr != nw {
					logrus.Error("写入字节数不匹配")
					break
				}
			}
			if er != nil {
				if er != io.EOF {
					logrus.Errorf("读取源文件失败: %v", er)
				}
				break
			}

			// 记录进度
			if logger != nil {
				progress := "["
				i := int((float64(written) / float64(allSize)) * 50)
				if i == 0 {
					i = 1
				}
				for j := 0; j < i; j++ {
					progress += "="
				}
				progress += ">"
				for len(progress) < 50 {
					progress += " "
				}
				progress += fmt.Sprintf("] %d MB/%d MB", int(written/1024/1024), int(allSize/1024/1024))
				message := fmt.Sprintf(`{"progress":"%s","progressDetail":{"current":%d,"total":%d},"id":"%s"}`, progress, written, allSize, progressID)
				logger.Debug(message, map[string]string{"step": "progress"})
			}
		}
	}()

	wg.Wait() // 等待 goroutine 完成

	// 生成 io.ReadSeeker
	body := bytes.NewReader(buffer.Bytes())

	// 执行 S3 上传
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	}

	_, err := s3s.s3Client.PutObject(input)
	if err != nil {
		if logger != nil {
			logger.Error("上传文件到 S3 失败", map[string]string{"step": "share"})
		}
		return err
	}

	if written != allSize {
		return io.ErrShortWrite
	}
	return nil
}

// extractFile 解压 ZIP 文件中的每个文件
func extractFile(zipFile *zip.File, target string, currentDirectory bool) error {
	run := func() error {
		path := filepath.Join(target, zipFile.Name)
		if currentDirectory {
			p := strings.Split(zipFile.Name, "/")[1:]
			path = filepath.Join(target, strings.Join(p, "/"))
		}
		if zipFile.FileInfo().IsDir() {
			return os.MkdirAll(path, zipFile.Mode())
		}

		// 打开 ZIP 文件中的文件
		fileReader, err := zipFile.Open()
		if err != nil {
			return fmt.Errorf("error opening file in zip: %v", err)
		}
		defer fileReader.Close()

		// 创建目标文件
		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, zipFile.Mode())
		if err != nil {
			return fmt.Errorf("error opening target file: %v", err)
		}
		defer targetFile.Close()

		// 复制内容
		if _, err := io.Copy(targetFile, fileReader); err != nil {
			return fmt.Errorf("error copying file: %v", err)
		}

		// 如果有文件的注释，处理 UID 和 GID
		if zipFile.Comment != "" && strings.Contains(zipFile.Comment, "/") {
			guid := strings.Split(zipFile.Comment, "/")
			if len(guid) == 2 {
				uid, _ := strconv.Atoi(guid[0])
				gid, _ := strconv.Atoi(guid[1])
				if err := os.Chown(path, uid, gid); err != nil {
					return fmt.Errorf("error changing owner: %v", err)
				}
			}
		}
		return nil
	}

	return run()
}
