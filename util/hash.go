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

package util

import (
	"crypto/md5"
	"fmt"
	"io"
	"math"
	"os"
)

const filechunk = 8192 // we settle for 8KB
//CreateFileHash compute sourcefile hash and write hashfile
func CreateFileHash(sourceFile, hashfile string) error {
	file, err := os.Open(sourceFile)
	if err != nil {
		return err
	}
	defer file.Close()
	fileinfo, _ := file.Stat()
	if fileinfo.IsDir() {
		return fmt.Errorf("do not support compute folder hash")
	}
	writehashfile, err := os.OpenFile(hashfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0655)
	if err != nil {
		return fmt.Errorf("create hash file error %s", err.Error())
	}
	defer writehashfile.Close()
	if fileinfo.Size() < filechunk {
		return createSmallFileHash(file, writehashfile)
	}
	return createBigFileHash(file, writehashfile)
}

func createBigFileHash(sourceFile, hashfile *os.File) error {
	// calculate the file size
	info, _ := sourceFile.Stat()
	filesize := info.Size()
	blocks := uint64(math.Ceil(float64(filesize) / float64(filechunk)))
	hash := md5.New()

	for i := uint64(0); i < blocks; i++ {
		blocksize := int(math.Min(filechunk, float64(filesize-int64(i*filechunk))))
		buf := make([]byte, blocksize)
		index, err := sourceFile.Read(buf)
		if err != nil {
			return err
		}
		// append into the hash
		_, err = hash.Write(buf[:index])
		if err != nil {
			return err
		}
	}
	_, err := hashfile.Write([]byte(fmt.Sprintf("%x", hash.Sum(nil))))
	if err != nil {
		return err
	}
	return nil
}

func createSmallFileHash(sourceFile, hashfile *os.File) error {
	md5h := md5.New()
	_, err := io.Copy(md5h, sourceFile)
	if err != nil {
		return err
	}
	_, err = hashfile.Write([]byte(fmt.Sprintf("%x", md5h.Sum(nil))))
	if err != nil {
		return err
	}
	return nil
}

//CreateHashString create hash string
func CreateHashString(source string) (hashstr string, err error) {
	md5h := md5.New()
	_, err = md5h.Write([]byte(source))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5h.Sum(nil)), nil
}
