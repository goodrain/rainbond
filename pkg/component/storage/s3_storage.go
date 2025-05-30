package storage

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util/zip"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type S3Storage struct {
	s3Client *s3.S3
	bucket   string
}

func (s3s *S3Storage) Test() {

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
	defer output.Body.Close() // 确保S3连接被关闭

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

func (s3s *S3Storage) Unzip(archive, target string, currentDirectory bool) error {
	bucketName, key, err := s3s.ParseDirPath(archive, true)
	// 下载 S3 中的 ZIP 文件
	zipFile, err := os.CreateTemp("", "archive-*.zip")
	if err != nil {
		return fmt.Errorf("error creating temp file: %v", err)
	}
	defer os.Remove(zipFile.Name()) // 确保临时文件在函数退出时被删除
	defer zipFile.Close()

	obj, err := s3s.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("error downloading file from S3: %v", err)
	}
	defer obj.Body.Close() // 确保S3连接被关闭

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

func (s3s *S3Storage) UploadFileToFile(src, dst string, logger event.Logger) error {
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
	if err != nil { // 修复了这里的错误判断条件
		if logger != nil {
			logger.Error("解析目标路径失败", map[string]string{"step": "share"})
		}
		return err
	}
	// 开始文件上传
	return s3s.S3CopyWithProgress(srcFile, bucket, key, srcStat.Size(), logger)
}

// S3CopyWithProgress 从源文件复制到 S3，并记录进度
func (s3s *S3Storage) S3CopyWithProgress(srcFile io.Reader, bucket, key string, allSize int64, logger event.Logger) error {
	// 使用分块上传来处理大文件
	progressID := uuid.New().String()[0:7]
	var written int64

	// 初始化分块上传
	input := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	resp, err := s3s.s3Client.CreateMultipartUpload(input)
	if err != nil {
		if logger != nil {
			logger.Error("初始化分块上传失败", map[string]string{"step": "share"})
		}
		logrus.Errorf("初始化分块上传失败: %v", err)
		return err
	}

	// 分块大小，设置为5MB（AWS S3要求最小5MB）
	chunkSize := int64(5 * 1024 * 1024)
	buffer := make([]byte, chunkSize)
	var partNum int64 = 1
	var completedParts []*s3.CompletedPart

	for {
		n, readErr := io.ReadFull(srcFile, buffer)
		if n <= 0 {
			break
		}

		// 上传分块
		partInput := &s3.UploadPartInput{
			Body:          bytes.NewReader(buffer[:n]),
			Bucket:        aws.String(bucket),
			Key:           aws.String(key),
			PartNumber:    aws.Int64(partNum),
			UploadId:      resp.UploadId,
			ContentLength: aws.Int64(int64(n)),
		}

		partResp, uploadErr := s3s.s3Client.UploadPart(partInput)
		if uploadErr != nil {
			// 上传失败，中止分块上传
			abortInput := &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
				UploadId: resp.UploadId,
			}
			_, _ = s3s.s3Client.AbortMultipartUpload(abortInput)

			if logger != nil {
				logger.Error("上传分块失败", map[string]string{"step": "share"})
			}
			logrus.Errorf("上传分块失败: %v", uploadErr)
			return uploadErr
		}

		// 记录已完成的分块
		completedPart := &s3.CompletedPart{
			ETag:       partResp.ETag,
			PartNumber: aws.Int64(partNum),
		}
		completedParts = append(completedParts, completedPart)

		// 更新已写入的字节数和分块编号
		written += int64(n)
		partNum++

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

		// 如果读取到EOF或者读取的数据不足一个完整的buffer，说明已经读完了
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}

		if readErr != nil {
			// 读取失败，中止分块上传
			abortInput := &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
				UploadId: resp.UploadId,
			}
			_, _ = s3s.s3Client.AbortMultipartUpload(abortInput)

			logrus.Errorf("读取源文件失败: %v", readErr)
			return readErr
		}
	}

	// 完成分块上传
	completeInput := &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: resp.UploadId,
		MultipartUpload: &s3.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}

	_, err = s3s.s3Client.CompleteMultipartUpload(completeInput)
	if err != nil {
		if logger != nil {
			logger.Error("完成分块上传失败", map[string]string{"step": "share"})
		}
		logrus.Errorf("完成分块上传失败: %v", err)
		return err
	}

	// 检查是否上传了所有数据
	if written < allSize {
		logrus.Warnf("文件上传不完整: 已上传 %d 字节，预期 %d 字节", written, allSize)
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

func (s3s *S3Storage) DownloadDirToDir(srcDir, dstDir string) error {
	bucketName, prefix, err := s3s.ParseDirPath(srcDir, false)
	if err != nil {
		return fmt.Errorf("解析源路径失败: %v", err)
	}

	// 检查目标目录是否存在，如果不存在则创建
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("无法创建目录 %s: %v", dstDir, err)
	}

	// 列出 S3 中指定目录下的所有对象
	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	}

	result, err := s3s.s3Client.ListObjectsV2(listInput)
	if err != nil {
		return fmt.Errorf("无法列出 S3 目录 %s: %v", srcDir, err)
	}

	// 下载每个文件
	for _, item := range result.Contents {
		if *item.Key == prefix {
			continue
		}
		key := *item.Key
		dstFilePath := fmt.Sprintf("%s/%s", dstDir, filepath.Base(key))

		// 从 S3 下载文件
		input := &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		}

		result, err := s3s.s3Client.GetObject(input)
		if err != nil {
			return fmt.Errorf("无法从 S3 下载文件 %s: %v", key, err)
		}
		// 注意：这里不能使用defer result.Body.Close()，因为它会在函数结束时才关闭
		// 而不是在每次循环结束时关闭，这会导致内存泄漏

		// 创建目标文件
		dstFile, err := os.OpenFile(dstFilePath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			result.Body.Close() // 确保在错误情况下关闭S3连接
			return fmt.Errorf("无法打开文件 %s: %v", dstFilePath, err)
		}

		// 将 S3 文件内容写入目标文件
		if _, err := io.Copy(dstFile, result.Body); err != nil {
			dstFile.Close()
			result.Body.Close() // 确保在错误情况下关闭S3连接
			return fmt.Errorf("无法写入文件 %s: %v", dstFilePath, err)
		}

		dstFile.Close()
		result.Body.Close() // 确保在每次循环结束时关闭S3连接
	}

	return nil
}

func (s3s *S3Storage) DownloadFileToDir(srcFile, dstDir string) error {
	// 解析 S3 路径
	bucketName, key, err := s3s.ParseDirPath(srcFile, false)
	if err != nil {
		return fmt.Errorf("解析源路径失败: %v", err)
	}
	// 检查目标目录是否存在，如果不存在则创建
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("无法创建目录 %s: %v", dstDir, err)
	}

	// 构造目标文件路径
	dstFilePath := fmt.Sprintf("%s/%s", dstDir, filepath.Base(key))

	// 从 S3 下载文件
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}

	result, err := s3s.s3Client.GetObject(input)
	if err != nil {
		return fmt.Errorf("无法从 S3 下载文件 %s: %v", key, err)
	}
	defer result.Body.Close() // 确保S3连接被关闭

	// 创建目标文件
	dstFile, err := os.OpenFile(dstFilePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开文件 %s: %v", dstFilePath, err)
	}
	defer dstFile.Close()

	// 将 S3 文件内容写入目标文件
	if _, err := io.Copy(dstFile, result.Body); err != nil {
		return fmt.Errorf("无法写入文件 %s: %v", dstFilePath, err)
	}
	return nil
}
