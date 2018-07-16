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
	"strings"
	"sort"
	"github.com/goodrain/rainbond/discover/config"
	"os"
	"syscall"
	"github.com/Sirupsen/logrus"
	"os/signal"
)

func TrimAndSort(endpoints []*config.Endpoint) []string {
	arr := make([]string, 0, len(endpoints))
	for _, end := range endpoints {
		url := strings.TrimLeft(end.URL, "shttp://")
		arr = append(arr, url)
	}

	sort.Strings(arr)

	return arr
}

func ArrCompare(arr1, arr2 []string) bool {
	if len(arr1) != len(arr2) {
		return false
	}

	for i, item := range arr1 {
		if item != arr2[i] {
			return false
		}
	}

	return true
}

func ListenStop() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM)

	sig := <- sigs
	signal.Ignore(syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM)

	logrus.Warn("monitor manager received signal: ", sig.String())
	close(sigs)
}
