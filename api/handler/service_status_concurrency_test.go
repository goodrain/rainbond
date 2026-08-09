package handler

import (
	"errors"
	"testing"
	"time"

	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchServiceRuntimeStateRunsStatusAndDeployInfoConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	result := make(chan struct {
		status string
		info   *pb.DeployInfo
		err    error
	}, 1)

	go func() {
		status, info, err := fetchServiceRuntimeState(
			func() string {
				started <- "status"
				<-release
				return "running"
			},
			func() (*pb.DeployInfo, error) {
				started <- "deploy"
				<-release
				return &pb.DeployInfo{StartTime: "now"}, nil
			},
		)
		result <- struct {
			status string
			info   *pb.DeployInfo
			err    error
		}{status: status, info: info, err: err}
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("status and deploy-info calls did not overlap")
		}
	}
	close(release)

	state := <-result
	require.NoError(t, state.err)
	assert.Equal(t, "running", state.status)
	assert.Equal(t, "now", state.info.GetStartTime())
}

func TestFetchServiceRuntimeStateKeepsDeployInfoError(t *testing.T) {
	wantErr := errors.New("deploy info unavailable")

	status, info, err := fetchServiceRuntimeState(
		func() string { return "running" },
		func() (*pb.DeployInfo, error) { return nil, wantErr },
	)

	assert.Equal(t, "running", status)
	assert.Nil(t, info)
	assert.ErrorIs(t, err, wantErr)
}
