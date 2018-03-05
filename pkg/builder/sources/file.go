// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package sources

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/goodrain/rainbond/pkg/util"

	"github.com/goodrain/rainbond/pkg/event"
)

//CopyFileWithProgress 复制文件，带进度
func CopyFileWithProgress(src, dst string, logger event.Logger) error {
	srcFile, err := os.OpenFile(src, os.O_RDONLY, 0644)
	if err != nil {
		if logger != nil {
			logger.Error("打开源文件失败", map[string]string{"step": "share"})
		}
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
	var written int64
	buf := make([]byte, 1024*1024)
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
			fmt.Println(i)
			for j := 0; j < i; j++ {
				progress += "="
			}
			progress += ">"
			for len(progress) < 50 {
				progress += " "
			}
			progress += "]"
			message := fmt.Sprintf(`{"progress":"%s","progressDetail":{"current":%d,"total":%d},"id":"%s"}`, progress, written, allSize, srcFile.Name())
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
