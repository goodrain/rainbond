package upload

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	httputil "github.com/goodrain/rainbond/util/http"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Storage struct {
	output    string
	verbosity int
}

func (s *Storage) StorageDir() string {
	return s.output
}

func NewStorage(rootDir string) *Storage {
	return &Storage{output: rootDir}
}

// UploadHandler is the endpoint for uploading and storing files.
func (s *Storage) UploadHandler(w http.ResponseWriter, r *http.Request) {
	// Performs the processing of writing data into chunk files.
	files, err := process(r, s.StorageDir())

	if err == incomplete {
		httputil.ReturnSuccess(r, w, nil)
		return
	}
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	data := make([]map[string]interface{}, 0)

	for _, file := range files {
		// 验证文件类型
		if err := validateFileType(file.Filepath); err != nil {
			httputil.ReturnError(r, w, 400, err.Error())
			return
		}

		attachment, err := create(s.StorageDir(), file, true)
		if err != nil {
			httputil.ReturnError(r, w, 500, err.Error())
			return
		}
		data = append(data, attachment.ToJson())
	}
	httputil.ReturnSuccess(r, w, data)
}

// validateFileType 验证文件类型为 .tar.gz 或 .zip
func validateFileType(filePath string) error {
	fileExtension := strings.ToLower(filepath.Ext(filePath))
	if !(strings.HasSuffix(filePath, ".tar.gz") || fileExtension == ".zip") {
		return errors.New("file type not allowed. Only .tar.gz and .zip files are permitted.")
	}

	// 打开文件进行格式验证
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if strings.HasSuffix(filePath, ".tar.gz") {
		return validateTarGz(file)
	} else if fileExtension == ".zip" {
		return validateZip(file)
	}

	return errors.New("invalid file type")
}

// validateTarGz 验证文件是否为有效的 tar.gz 格式
func validateTarGz(file io.Reader) error {
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return errors.New("invalid tar.gz format: " + err.Error())
	}
	defer gzr.Close()

	tarReader := tar.NewReader(gzr)
	_, err = tarReader.Next() // 尝试读取第一个文件头，以验证内容
	if err != nil {
		return errors.New("invalid tar.gz content: " + err.Error())
	}
	return nil
}

// validateZip 验证文件是否为有效的 zip 格式
func validateZip(file *os.File) error {
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	// 使用 zip.NewReader 需要提供文件大小
	_, err = zip.NewReader(file, fileInfo.Size())
	if err != nil {
		return errors.New("invalid zip format: " + err.Error())
	}
	return nil
}
