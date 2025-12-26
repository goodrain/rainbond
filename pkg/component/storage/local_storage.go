package storage

import (
	"fmt"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/zip"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type LocalStorage struct {
}

func (l *LocalStorage) MkdirAll(path string) error {
	if !util.DirIsEmpty(path) {
		os.RemoveAll(path)
	}
	if err := util.CheckAndCreateDir(path); err != nil {
		return err
	}
	return nil
}

func (l *LocalStorage) ServeFile(w http.ResponseWriter, r *http.Request, filePath string) {
	http.ServeFile(w, r, filePath)
}

// Unzip archive file to target dir
func (l *LocalStorage) Unzip(archive, target string, currentDirectory bool) error {
	reader, err := zip.OpenDirectReader(archive)
	if err != nil {
		return fmt.Errorf("error opening archive: %v", err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	for _, file := range reader.File {
		run := func() error {
			path := filepath.Join(target, file.Name)
			if currentDirectory {
				p := strings.Split(file.Name, "/")[1:]
				path = filepath.Join(target, strings.Join(p, "/"))
			}
			if file.FileInfo().IsDir() {
				os.MkdirAll(path, file.Mode())
				if file.Comment != "" && strings.Contains(file.Comment, "/") {
					guid := strings.Split(file.Comment, "/")
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

			fileReader, err := file.Open()
			if err != nil {
				return fmt.Errorf("fileReader; error opening file: %v", err)
			}
			defer fileReader.Close()
			targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return fmt.Errorf("targetFile; error opening file: %v", err)
			}
			defer targetFile.Close()

			if _, err := io.Copy(targetFile, fileReader); err != nil {
				return fmt.Errorf("error copy file: %v", err)
			}
			if file.Comment != "" && strings.Contains(file.Comment, "/") {
				guid := strings.Split(file.Comment, "/")
				if len(guid) == 2 {
					uid, _ := strconv.Atoi(guid[0])
					gid, _ := strconv.Atoi(guid[1])
					if err := os.Chown(path, uid, gid); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if err := run(); err != nil {
			return err
		}
	}

	return nil
}

func (l *LocalStorage) SaveFile(fileName string, reader multipart.File) error {
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		logrus.Errorf("Failed to open file: %s", err.Error())
		return err
	}
	defer file.Close()
	if _, err := io.Copy(file, reader); err != nil {
		logrus.Errorf("Failed to write file：%s", err.Error())
		return err
	}
	return nil
}

// CopyFileWithProgress 复制文件，带进度
func (l *LocalStorage) UploadFileToFile(src, dst string, logger event.Logger) error {
	srcFile, err := os.OpenFile(src, os.O_RDONLY, 0644)
	if err != nil {
		if logger != nil {
			logger.Error("打开源文件失败", map[string]string{"step": "share"})
		}
		logrus.Errorf("open file %s error", src)
		return err
	}
	defer srcFile.Close()
	srcStat, err := srcFile.Stat()
	if err != nil {
		if logger != nil {
			logger.Error("打开源文件失败", map[string]string{"step": "share"})
		}
		return err
	}
	// 验证并创建目标目录
	dir := filepath.Dir(dst)
	if err := util.CheckAndCreateDir(dir); err != nil {
		if logger != nil {
			logger.Error("检测并创建目标文件目录失败", map[string]string{"step": "share"})
		}
		return err
	}
	// 先删除文件如果存在
	os.RemoveAll(dst)
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if logger != nil {
			logger.Error("打开目标文件失败", map[string]string{"step": "share"})
		}
		return err
	}
	defer dstFile.Close()
	allSize := srcStat.Size()
	return CopyWithProgress(srcFile, dstFile, allSize, logger)
}

func (l *LocalStorage) ReadDir(dirName string) ([]string, error) {
	packages, err := ioutil.ReadDir(dirName)
	if err != nil {
		return nil, err
	}
	packageArr := make([]string, 0, 10)
	for _, dir := range packages {
		if dir.IsDir() {
			continue
		}
		packageArr = append(packageArr, dir.Name())
	}
	return packageArr, nil
}

// CopyWithProgress copy file
func CopyWithProgress(srcFile SrcFile, dstFile DstFile, allSize int64, logger event.Logger) (err error) {
	var written int64
	buf := make([]byte, 1024*1024)
	progressID := uuid.New().String()[0:7]
	for {
		nr, er := srcFile.Read(buf)
		if nr > 0 {
			nw, ew := dstFile.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
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
	if err != nil {
		return err
	}
	if written != allSize {
		return io.ErrShortWrite
	}
	return nil
}

func (l *LocalStorage) DownloadDirToDir(srcDir, dstDir string) error {
	return nil
}

func (l *LocalStorage) DownloadFileToDir(srcFile, dstDir string) error {
	return nil
}

// GetChunkDir 获取分片存储目录
func (l *LocalStorage) GetChunkDir(sessionID string) string {
	return fmt.Sprintf("/grdata/package_build/temp/chunks/%s", sessionID)
}

// SaveChunk 保存分片文件
func (l *LocalStorage) SaveChunk(sessionID string, chunkIndex int, reader multipart.File) (string, error) {
	chunkDir := l.GetChunkDir(sessionID)
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create chunk directory: %v", err)
	}

	chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%d", chunkIndex))
	file, err := os.OpenFile(chunkPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		logrus.Errorf("Failed to create chunk file: %s", err.Error())
		return "", err
	}
	defer file.Close()

	written, err := io.Copy(file, reader)
	if err != nil {
		logrus.Errorf("Failed to write chunk file: %s", err.Error())
		return "", err
	}

	logrus.Debugf("Saved chunk %d, size: %d bytes, path: %s", chunkIndex, written, chunkPath)
	return chunkPath, nil
}

// ChunkExists 检查分片是否存在
func (l *LocalStorage) ChunkExists(sessionID string, chunkIndex int) bool {
	chunkPath := filepath.Join(l.GetChunkDir(sessionID), fmt.Sprintf("chunk_%d", chunkIndex))
	_, err := os.Stat(chunkPath)
	return err == nil
}

// MergeChunks 合并所有分片到目标文件
func (l *LocalStorage) MergeChunks(sessionID string, outputPath string, totalChunks int) error {
	chunkDir := l.GetChunkDir(sessionID)

	// 验证所有分片是否存在
	for i := 0; i < totalChunks; i++ {
		if !l.ChunkExists(sessionID, i) {
			return fmt.Errorf("chunk %d is missing", i)
		}
	}

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

	// 按顺序合并所有分片
	var totalWritten int64
	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%d", i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return fmt.Errorf("failed to open chunk %d: %v", i, err)
		}

		written, err := io.Copy(outputFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			return fmt.Errorf("failed to merge chunk %d: %v", i, err)
		}

		totalWritten += written
		logrus.Debugf("Merged chunk %d, size: %d bytes", i, written)
	}

	logrus.Infof("Successfully merged %d chunks to %s, total size: %d bytes", totalChunks, outputPath, totalWritten)
	return nil
}

// CleanupChunks 清理分片文件
func (l *LocalStorage) CleanupChunks(sessionID string) error {
	chunkDir := l.GetChunkDir(sessionID)
	if err := os.RemoveAll(chunkDir); err != nil {
		logrus.Errorf("Failed to cleanup chunks: %v", err)
		return err
	}
	logrus.Debugf("Cleaned up chunks for session: %s", sessionID)
	return nil
}

// ReadFile reads a file directly from local storage and returns a reader
func (l *LocalStorage) ReadFile(filePath string) (ReadCloser, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return file, nil
}
