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

package utils

import (
	"math/rand"
	"time"
)

// ASCII values 33 ~ 126
const _dcl = 126 - 33 + 1

var defaultCharacters [_dcl]byte

func init() {
	for i := 0; i < _dcl; i++ {
		defaultCharacters[i] = byte(i + 33)
	}

	rand.Seed(time.Now().UnixNano())
}

func RandString(length int, characters ...byte) string {
	if len(characters) == 0 {
		characters = defaultCharacters[:]
	}

	n := len(characters)
	var rs = make([]byte, length)

	for i := 0; i < length; i++ {
		rs[i] = characters[rand.Intn(n-1)]
	}

	return string(rs)
}
