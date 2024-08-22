package util

// 文件: filepath.go
// 说明: 该文件实现了文件路径管理功能的核心组件。文件中定义了用于处理文件路径操作的相关方法，
// 以支持平台内文件和目录的路径管理需求。通过这些方法，Rainbond 平台能够高效地管理和解析文件路径，
// 提供可靠的文件系统操作服务。

import (
	"crypto/sha256"
	"fmt"
	"path"
	"strconv"
)

// DockerLogFilePath returns the directory to save Docker log files
func DockerLogFilePath(homepath, key string) string {
	return path.Join(homepath, getServiceAliasID(key))
}

// DockerLogFileName returns the file name of Docker log file.
func DockerLogFileName(filePath string) string {
	return path.Join(filePath, "stdout.log")
}

// python:
// new_word = str(ord(string[10])) + string + str(ord(string[3])) + 'log' + str(ord(string[2]) / 7)
// new_id = hashlib.sha224(new_word).hexdigest()[0:16]
func getServiceAliasID(ServiceID string) string {
	if len(ServiceID) > 11 {
		newWord := strconv.Itoa(int(ServiceID[10])) + ServiceID + strconv.Itoa(int(ServiceID[3])) + "log" + strconv.Itoa(int(ServiceID[2])/7)
		ha := sha256.New224()
		ha.Write([]byte(newWord))
		return fmt.Sprintf("%x", ha.Sum(nil))[0:16]
	}
	return ServiceID
}

// EventLogFilePath returns the directory to save event log files
func EventLogFilePath(homePath string) string {
	return path.Join(homePath, "eventlog")
}

// EventLogFileName returns the file name of event log file.
func EventLogFileName(filePath, key string) string {
	return path.Join(filePath, key+".log")
}
