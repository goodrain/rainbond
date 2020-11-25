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
	"io"
	"os"

	"github.com/goodrain/rainbond/webcli/term"
	"github.com/sirupsen/logrus"
)

//Out out
type Out struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

//CreateOut create out
func CreateOut(tty *os.File) *Out {
	return &Out{
		Stdin:  tty,
		Stdout: tty,
		Stderr: tty,
	}
}

//SetTTY set tty
func (o *Out) SetTTY() term.TTY {
	t := term.TTY{
		Out: o.Stdout,
		In:  o.Stdin,
	}
	if !t.IsTerminalIn() {
		logrus.Errorf("stdin is not tty")
		return t
	}
	// if we get to here, the user wants to attach stdin, wants a TTY, and o.In is a terminal, so we
	// can safely set t.Raw to true
	t.Raw = true
	return t
}
