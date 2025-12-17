package storage

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util/zip"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
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

// ensureBucketExists 检查桶是否存在，若不存在则创建桶并配置生命周期策略
func (s3s *S3Storage) ensureBucketExists(bucketName string) error {
	// 检查桶是否存在
	_, err := s3s.s3Client.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})

	bucketExists := true
	if err != nil {
		// 如果桶不存在，创建桶
		_, err = s3s.s3Client.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		bucketExists = false
		logrus.Infof("Created new bucket: %s", bucketName)
	}

	// 检查并确保生命周期策略存在（支持已有桶的升级场景）
	if err := s3s.ensureBucketLifecycle(bucketName, bucketExists); err != nil {
		logrus.Warnf("Failed to configure bucket lifecycle for %s: %v", bucketName, err)
		// 不返回错误，因为桶已创建/存在，生命周期配置失败不应阻止操作
	}

	return nil
}

// ensureBucketLifecycle 检查并配置桶的生命周期策略
func (s3s *S3Storage) ensureBucketLifecycle(bucketName string, bucketExists bool) error {
	// 如果桶已存在，先检查是否已有生命周期策略
	if bucketExists {
		existingConfig, err := s3s.s3Client.GetBucketLifecycleConfiguration(&s3.GetBucketLifecycleConfigurationInput{
			Bucket: aws.String(bucketName),
		})

		if err == nil && existingConfig != nil && len(existingConfig.Rules) > 0 {
			// 检查是否已有我们的规则（通过 Rule ID 判断）
			hasOurRules := false
			for _, rule := range existingConfig.Rules {
				if rule.ID != nil && (*rule.ID == "delete-chunks-1d" ||
					*rule.ID == "delete-restore-1d" ||
					*rule.ID == "delete-temp-events-1d" ||
					*rule.ID == "delete-app-import-1d" ||
					*rule.ID == "delete-app-export-7d" ||
					*rule.ID == "delete-build-tenant-7d" ||
					*rule.ID == "abort-incomplete-multipart-1d") {
					hasOurRules = true
					break
				}
			}

			if hasOurRules {
				logrus.Debugf("Bucket %s already has lifecycle policy configured", bucketName)
				return nil
			}
		}
		// 如果获取失败或没有规则，继续配置
	}

	// 配置生命周期策略
	input := &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
		LifecycleConfiguration: &s3.BucketLifecycleConfiguration{
			Rules: []*s3.LifecycleRule{
				{
					ID:     aws.String("delete-chunks-1d"),
					Status: aws.String("Enabled"),
					Prefix: aws.String("package_build/temp/chunks/"),
					Expiration: &s3.LifecycleExpiration{
						Days: aws.Int64(1), // 分片文件 1 天后自动删除
					},
				},
				{
					ID:     aws.String("delete-restore-1d"),
					Status: aws.String("Enabled"),
					Prefix: aws.String("restore/"),
					Expiration: &s3.LifecycleExpiration{
						Days: aws.Int64(1), // 恢复文件 1 天后自动删除
					},
				},
				{
					ID:     aws.String("delete-temp-events-1d"),
					Status: aws.String("Enabled"),
					Prefix: aws.String("package_build/temp/events/"),
					Expiration: &s3.LifecycleExpiration{
						Days: aws.Int64(1), // 临时事件文件 1 天后自动删除
					},
				},
				// 事件日志永久保存，不自动清理
				// {
				//     ID:     aws.String("delete-event-logs-7d"),
				//     Status: aws.String("Enabled"),
				//     Prefix: aws.String("logs/eventlog/"),
				//     Expiration: &s3.LifecycleExpiration{
				//         Days: aws.Int64(7),
				//     },
				// },
				{
					ID:     aws.String("delete-app-import-1d"),
					Status: aws.String("Enabled"),
					Prefix: aws.String("app/import/"),
					Expiration: &s3.LifecycleExpiration{
						Days: aws.Int64(1), // 应用导入临时文件 1 天后自动删除
					},
				},
				{
					ID:     aws.String("delete-app-export-7d"),
					Status: aws.String("Enabled"),
					Prefix: aws.String("app/"),
					Expiration: &s3.LifecycleExpiration{
						Days: aws.Int64(7), // 应用导出文件 7 天后自动删除
					},
				},
				{
					ID:     aws.String("delete-build-tenant-7d"),
					Status: aws.String("Enabled"),
					Prefix: aws.String("build/tenant/"),
					Expiration: &s3.LifecycleExpiration{
						Days: aws.Int64(7), // 应用发布 Slug 包 7 天后自动删除
					},
				},
				{
					ID:     aws.String("abort-incomplete-multipart-1d"),
					Status: aws.String("Enabled"),
					Prefix: aws.String(""),
					AbortIncompleteMultipartUpload: &s3.AbortIncompleteMultipartUpload{
						DaysAfterInitiation: aws.Int64(1), // 未完成的分段上传 1 天后清理
					},
				},
			},
		},
	}

	_, err := s3s.s3Client.PutBucketLifecycleConfiguration(input)
	if err != nil {
		return fmt.Errorf("failed to configure bucket lifecycle: %w", err)
	}

	logrus.Infof("Successfully configured lifecycle policy for bucket: %s", bucketName)
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
	resp, err := s3s.s3Client.CreateMultipartUpload(&s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
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

	logrus.Infof("[S3下载] 开始下载, srcDir: %s, dstDir: %s, bucket: %s, prefix: %s", srcDir, dstDir, bucketName, prefix)

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
		logrus.Errorf("[S3下载] 列出S3对象失败: %v", err)
		return fmt.Errorf("无法列出 S3 目录 %s: %v", srcDir, err)
	}

	logrus.Infof("[S3下载] S3返回对象数: %d", len(result.Contents))
	for i, item := range result.Contents {
		logrus.Infof("[S3下载] 对象[%d]: Key=%s, Size=%d", i, *item.Key, *item.Size)
	}

	// 下载每个文件
	downloadCount := 0
	for _, item := range result.Contents {
		if *item.Key == prefix {
			logrus.Infof("[S3下载] 跳过目录自身: %s", prefix)
			continue
		}
		key := *item.Key
		dstFilePath := fmt.Sprintf("%s/%s", dstDir, filepath.Base(key))
		logrus.Infof("[S3下载] 正在下载文件: %s -> %s", key, dstFilePath)

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
		written, err := io.Copy(dstFile, result.Body)
		if err != nil {
			dstFile.Close()
			result.Body.Close() // 确保在错误情况下关闭S3连接
			logrus.Errorf("[S3下载] 写入文件失败: %s, error: %v", dstFilePath, err)
			return fmt.Errorf("无法写入文件 %s: %v", dstFilePath, err)
		}

		dstFile.Close()
		result.Body.Close() // 确保在每次循环结束时关闭S3连接
		downloadCount++
		logrus.Infof("[S3下载] 文件下载成功: %s, 大小: %d bytes", dstFilePath, written)
	}

	logrus.Infof("[S3下载] 下载完成, 总共下载了 %d 个文件到 %s", downloadCount, dstDir)
	return nil
}

func (s3s *S3Storage) DownloadFileToDir(srcFile, dstDir string) error {
	// 解析 S3 路径 - 第二个参数应该是 true,因为 srcFile 是文件路径而不是目录
	bucketName, key, err := s3s.ParseDirPath(srcFile, true)
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

// GetChunkDir 获取分片存储目录（S3使用key前缀）
func (s3s *S3Storage) GetChunkDir(sessionID string) string {
	return fmt.Sprintf("package_build/temp/chunks/%s", sessionID)
}

// SaveChunk 保存分片到S3
func (s3s *S3Storage) SaveChunk(sessionID string, chunkIndex int, reader multipart.File) (string, error) {
	// S3 使用统一的bucket（从配置或默认）
	bucketName := "grdata"
	if err := s3s.ensureBucketExists(bucketName); err != nil {
		return "", err
	}

	key := fmt.Sprintf("%s/chunk_%d", s3s.GetChunkDir(sessionID), chunkIndex)

	// 读取分片内容到内存
	content, err := io.ReadAll(reader)
	if err != nil {
		logrus.Errorf("Failed to read chunk data: %v", err)
		return "", err
	}

	// 上传到S3
	_, err = s3s.s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(content),
	})
	if err != nil {
		logrus.Errorf("Failed to upload chunk to S3: %v", err)
		return "", err
	}

	logrus.Debugf("Saved chunk %d to S3, size: %d bytes, key: %s", chunkIndex, len(content), key)
	return key, nil
}

// ChunkExists 检查S3中分片是否存在
func (s3s *S3Storage) ChunkExists(sessionID string, chunkIndex int) bool {
	bucketName := "grdata"
	key := fmt.Sprintf("%s/chunk_%d", s3s.GetChunkDir(sessionID), chunkIndex)

	_, err := s3s.s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	return err == nil
}

// MergeChunks 从S3下载所有分片并合并到本地文件
func (s3s *S3Storage) MergeChunks(sessionID string, outputPath string, totalChunks int) error {
	bucketName := "grdata"
	chunkKeyPrefix := s3s.GetChunkDir(sessionID)

	// 不预先检查所有分片是否存在（避免 N 次 HEAD 请求）
	// 直接在下载时检查，失败会返回明确错误

	// 如果输出路径是S3路径，使用S3的CopyObject合并
	if strings.HasPrefix(outputPath, "/grdata/") || strings.HasPrefix(outputPath, "grdata/") {
		return s3s.mergeChunksToS3(bucketName, chunkKeyPrefix, outputPath, totalChunks)
	}

	// 如果是本地路径，下载并合并到本地
	return s3s.mergeChunksToLocal(bucketName, chunkKeyPrefix, outputPath, totalChunks)
}

// mergeChunksToLocal 下载S3分片并合并到本地文件
func (s3s *S3Storage) mergeChunksToLocal(bucketName, chunkKeyPrefix, outputPath string, totalChunks int) error {
	// 确保输出目录存在
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// 创建输出文件
	outputFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outputFile.Close()

	// 按顺序下载并合并所有分片
	var totalWritten int64
	for i := 0; i < totalChunks; i++ {
		key := fmt.Sprintf("%s/chunk_%d", chunkKeyPrefix, i)

		// 从S3下载分片
		result, err := s3s.s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("failed to download chunk %d from S3: %v", i, err)
		}

		written, err := io.Copy(outputFile, result.Body)
		result.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to merge chunk %d: %v", i, err)
		}

		totalWritten += written
		logrus.Debugf("Merged chunk %d from S3, size: %d bytes", i, written)
	}

	logrus.Infof("Successfully merged %d chunks from S3 to %s, total size: %d bytes", totalChunks, outputPath, totalWritten)
	return nil
}

// mergeChunksToS3 在S3内部合并分片（使用复制）
func (s3s *S3Storage) mergeChunksToS3(bucketName, chunkKeyPrefix, outputPath string, totalChunks int) error {
	// S3不支持直接合并对象，需要先下载到本地临时文件，再上传
	tempFile, err := os.CreateTemp("", "s3-merge-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 下载所有分片到临时文件
	for i := 0; i < totalChunks; i++ {
		key := fmt.Sprintf("%s/chunk_%d", chunkKeyPrefix, i)
		result, err := s3s.s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("failed to download chunk %d: %v", i, err)
		}

		_, err = io.Copy(tempFile, result.Body)
		result.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to write chunk %d to temp file: %v", i, err)
		}
	}

	// 重置文件指针到开始位置
	tempFile.Seek(0, 0)

	// 解析输出路径并上传到S3
	_, outputKey, err := s3s.ParseDirPath(outputPath, true)
	if err != nil {
		return err
	}

	_, err = s3s.s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(outputKey),
		Body:   tempFile,
	})
	if err != nil {
		return fmt.Errorf("failed to upload merged file to S3: %v", err)
	}

	logrus.Infof("Successfully merged %d chunks to S3: %s", totalChunks, outputKey)
	return nil
}

// CleanupChunks 清理S3中的分片文件
func (s3s *S3Storage) CleanupChunks(sessionID string) error {
	bucketName := "grdata"
	prefix := s3s.GetChunkDir(sessionID)

	// 列出所有分片
	result, err := s3s.s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		logrus.Errorf("Failed to list chunks in S3: %v", err)
		return err
	}

	// 批量删除
	if len(result.Contents) == 0 {
		return nil
	}

	objects := make([]*s3.ObjectIdentifier, 0, len(result.Contents))
	for _, obj := range result.Contents {
		objects = append(objects, &s3.ObjectIdentifier{Key: obj.Key})
	}

	_, err = s3s.s3Client.DeleteObjects(&s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &s3.Delete{
			Objects: objects,
			Quiet:   aws.Bool(true),
		},
	})
	if err != nil {
		logrus.Errorf("Failed to delete chunks from S3: %v", err)
		return err
	}

	logrus.Debugf("Cleaned up chunks for session: %s from S3", sessionID)
	return nil
}

// ReadFile reads a file directly from S3 and returns a reader
func (s3s *S3Storage) ReadFile(filePath string) (ReadCloser, error) {
	bucketName, key, err := s3s.ParseDirPath(filePath, true)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file path: %w", err)
	}

	result, err := s3s.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}

	return result.Body, nil
}
