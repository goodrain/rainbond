package storage

import (
	"fmt"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/zip"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type LocalStorage struct {
}

func (l *LocalStorage) Glob(dirPath string) ([]string, error) {
	return filepath.Glob(path.Join(dirPath, "*"))
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

func (l *LocalStorage) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (l *LocalStorage) ServeFile(w http.ResponseWriter, r *http.Request, filePath string) {
	http.ServeFile(w, r, filePath)
}

func (l *LocalStorage) OpenFile(fileName string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(fileName, flag, perm)
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
func (l *LocalStorage) CopyFileWithProgress(src, dst string, logger event.Logger) error {
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
	progressID := uuid.NewV4().String()[0:7]
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
