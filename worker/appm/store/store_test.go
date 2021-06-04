// RAINBOND, Application Management Platform
// Copyright (C) 2014-2021 Goodrain Co., Ltd.

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

package store

import (
	"testing"

	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/stretchr/testify/assert"
)

func TestGetAppStatus(t *testing.T) {
	tests := []struct {
		name     string
		statuses map[string]string
		want     pb.AppStatus_Status
	}{
		{
			name: "nocomponent",
			want: pb.AppStatus_NIL,
		},
		{
			name: "undeploy",
			statuses: map[string]string{
				"apple":  v1.UNDEPLOY,
				"banana": v1.UNDEPLOY,
			},
			want: pb.AppStatus_NIL,
		},
		{
			name: "closed",
			statuses: map[string]string{
				"apple":  v1.UNDEPLOY,
				"banana": v1.CLOSED,
				"cat":    v1.CLOSED,
			},
			want: pb.AppStatus_CLOSED,
		},
		{
			name: "abnormal",
			statuses: map[string]string{
				"apple":  v1.ABNORMAL,
				"banana": v1.SOMEABNORMAL,
				"cat":    v1.RUNNING,
				"dog":    v1.CLOSED,
			},
			want: pb.AppStatus_ABNORMAL,
		},
		{
			name: "starting",
			statuses: map[string]string{
				"cat":  v1.RUNNING,
				"dog":  v1.CLOSED,
				"food": v1.STARTING,
			},
			want: pb.AppStatus_STARTING,
		},
		{
			name: "stopping",
			statuses: map[string]string{
				"apple":  v1.STOPPING,
				"banana": v1.CLOSED,
			},
			want: pb.AppStatus_STOPPING,
		},
		{
			name: "stopping2",
			statuses: map[string]string{
				"apple":  v1.STOPPING,
				"banana": v1.CLOSED,
				"cat":    v1.RUNNING,
			},
			want: pb.AppStatus_RUNNING,
		},
		{
			name: "running",
			statuses: map[string]string{
				"apple":  v1.RUNNING,
				"banana": v1.CLOSED,
				"cat":    v1.UNDEPLOY,
			},
			want: pb.AppStatus_RUNNING,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			status := getAppStatus(tc.statuses)
			assert.Equal(t, tc.want, status)
		})
	}
}
