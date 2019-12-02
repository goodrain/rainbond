// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"
)

type Packet interface {
	Serialize() []byte
	IsNull() bool
	IsPing() bool
}

type MessagePacket struct {
	data   string
	isPing bool
}

var errClosed = errors.New("conn is closed")

func (m *MessagePacket) Serialize() []byte {
	return []byte(m.data)
}

func (m *MessagePacket) IsNull() bool {
	return len(m.data) == 0 && !m.isPing
}

func (m *MessagePacket) IsPing() bool {
	return m.isPing
}

type Protocol interface {
	SetConn(conn *net.TCPConn)
	ReadPacket() (Packet, error)
}

type MessageProtocol struct {
	conn      *net.TCPConn
	reader    *bufio.Reader
	cache     *bytes.Buffer
	cacheSize int64
}

func (m *MessageProtocol) SetConn(conn *net.TCPConn) {
	m.conn = conn
	m.reader = bufio.NewReader(conn)
	m.cache = bytes.NewBuffer(nil)
}

//ReadPacket 获取消息流
func (m *MessageProtocol) ReadPacket() (Packet, error) {
	if m.reader != nil {
		message, err := m.Decode()
		if err != nil {
			return nil, err
		}
		if m.isPing(message) {
			return &MessagePacket{isPing: true}, nil
		}
		return &MessagePacket{data: message}, nil
	}
	return nil, errClosed
}
func (m *MessageProtocol) isPing(s string) bool {
	return s == "0x00ping"
}

const maxConsecutiveEmptyReads = 100

//Decode 解码数据流
func (m *MessageProtocol) Decode() (string, error) {
	// 读取消息的长度
	lengthByte, err := m.reader.Peek(4)
	if err != nil {
		return "", err
	}
	lengthBuff := bytes.NewBuffer(lengthByte)
	var length int32
	err = binary.Read(lengthBuff, binary.LittleEndian, &length)
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", errClosed
	}
	if int32(m.reader.Buffered()) < length+4 {
		var retry = 0
		for m.cacheSize < int64(length+4) {
			//read size must <= length+4
			readSize := int64(length+4) - m.cacheSize
			if readSize > int64(m.reader.Buffered()) {
				readSize = int64(m.reader.Buffered())
			}
			buffer := make([]byte, readSize)
			size, err := m.reader.Read(buffer)
			if err != nil {
				return "", err
			}
			//Two consecutive reads 0 bytes, return io.ErrNoProgress
			//Read() will read up to len(p) into p, when possible.
			//After a Read() call, n may be less then len(p).
			//Upon error, Read() may still return n bytes in buffer p. For instance, reading from a TCP socket that is abruptly closed. Depending on your use, you may choose to keep the bytes in p or retry.
			//When a Read() exhausts available data, a reader may return a non-zero n and err=io.EOF. However, depending on implementation, a reader may choose to return a non-zero n and err = nil at the end of stream. In that case, any subsequent reads must return n=0, err=io.EOF.
			//Lastly, a call to Read() that returns n=0 and err=nil does not mean EOF as the next call to Read() may return more data.
			if size <= 0 {
				retry++
				if retry > maxConsecutiveEmptyReads {
					return "", io.ErrNoProgress
				}
				time.Sleep(time.Millisecond * 10)
			} else {
				m.cacheSize += int64(size)
				m.cache.Write(buffer)
			}
		}
		result := m.cache.Bytes()[4:]
		m.cache.Reset()
		m.cacheSize = 0
		return string(result), nil
	}

	// 读取消息真正的内容
	pack := make([]byte, int(4+length))
	size, err := m.reader.Read(pack)
	if err != nil {
		return "", err
	}
	if size == 0 {
		return "", io.ErrNoProgress
	}
	return string(pack[4:]), nil
}
