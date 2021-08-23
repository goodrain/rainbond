// RAINBOND, Application Management Platform
// Copyright (C) 2014-2020 Goodrain Co., Ltd.

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

package app

import (
	"fmt"
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/kr/pty"
)

func TestTTY(t *testing.T) {
	pty, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		buf := make([]byte, 1024)
		pty.WriteString("hello word")
		var i int
		for {
			i++
			size, err := tty.Read(buf)
			if err != nil {
				log.Printf("Command exited for: %s", err.Error())
				return
			}
			pty.WriteString(string(buf[:size]) + strconv.Itoa(i))
			fmt.Println("tty write:", string(buf[:size]))
			time.Sleep(time.Second * 1)
		}
	}()
	// go func() {
	// 	buf := make([]byte, 1024)
	// 	for {
	// 		size, err := tty.Read(buf)
	// 		if err != nil {
	// 			log.Printf("Command exited for: %s", err.Error())
	// 			return
	// 		}
	// 		pty.Write(buf[:size])
	// 		fmt.Println("pty write:", buf[:size])
	// 	}
	// }()
	time.Sleep(time.Minute * 1)
}
