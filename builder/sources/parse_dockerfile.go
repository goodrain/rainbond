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

package sources

import (
	"io"
	"os"
	"sort"

	"github.com/goodrain/rainbond/util/dockerfile/command"
	"github.com/goodrain/rainbond/util/dockerfile/parser"
)

//Command Represents a single line (layer) in a Dockerfile.
// For example `FROM ubuntu:xenial`
type Command struct {
	Cmd       string   // lowercased command name (ex: `from`)
	SubCmd    string   // for ONBUILD only this holds the sub-command
	Json      bool     // whether the value is written in json form
	Original  string   // The original source line
	StartLine int      // The original source line number
	Flags     []string // Any flags such as `--from=...` for `COPY`.
	Value     []string // The contents of the command (ex: `ubuntu:xenial`)
}

//IOError A failure in opening a file for reading.
type IOError struct {
	Msg string
}

func (e IOError) Error() string {
	return e.Msg
}

//ParseError A failure in parsing the file as a dockerfile.
type ParseError struct {
	Msg string
}

func (e ParseError) Error() string {
	return e.Msg
}

//AllCmds List all legal cmds in a dockerfile
func AllCmds() []string {
	var ret []string
	for k := range command.Commands {
		ret = append(ret, k)
	}
	sort.Strings(ret)
	return ret
}

//ParseReader Parse a Dockerfile from a reader.  A ParseError may occur.
func ParseReader(file io.Reader) ([]Command, error) {
	directive := parser.Directive{LookingForDirectives: true}
	parser.SetEscapeToken(parser.DefaultEscapeToken, &directive)
	ast, err := parser.Parse(file, &directive)
	if err != nil {
		return nil, ParseError{err.Error()}
	}

	var ret []Command
	for _, child := range ast.Children {
		cmd := Command{
			Cmd:       child.Value,
			Original:  child.Original,
			StartLine: child.StartLine,
			Flags:     child.Flags,
		}

		// Only happens for ONBUILD
		if child.Next != nil && len(child.Next.Children) > 0 {
			cmd.SubCmd = child.Next.Children[0].Value
			child = child.Next.Children[0]
		}

		cmd.Json = child.Attributes["json"]
		for n := child.Next; n != nil; n = n.Next {
			cmd.Value = append(cmd.Value, n.Value)
		}

		ret = append(ret, cmd)
	}
	return ret, nil
}

//ParseFile Parse a Dockerfile from a filename.  An IOError or ParseError may occur.
func ParseFile(filename string) ([]Command, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, IOError{err.Error()}
	}
	defer file.Close()

	return ParseReader(file)
}
